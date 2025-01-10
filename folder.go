package main

import (
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"log"
	"os"
	"path"
	"strings"
)

type ExpFolderEntry struct {
	Path string
	Name string

	Items  []*ExpFolderEntry
	Parent *ExpFolderEntry

	Experiment   *Experiment
	IsExperiment bool

	Watcher *fsnotify.Watcher
}

var baseEntry *ExpFolderEntry

func NewEntry(p string, parent *ExpFolderEntry) *ExpFolderEntry {
	checkpointsPath := path.Join(p, "checkpoints")
	var entry *ExpFolderEntry
	if _, err := os.Stat(checkpointsPath); os.IsNotExist(err) {
		log.Printf("Found Folder: %s", p)
		entry = &ExpFolderEntry{
			Path:         p,
			Name:         path.Base(p),
			Parent:       parent,
			IsExperiment: false,
		}
		entry.Refresh()
	} else {
		log.Printf("Found Experiment: %s", p)
		entry = &ExpFolderEntry{
			Path:         p,
			Name:         path.Base(p),
			Parent:       parent,
			Experiment:   FindOrCreateExperiment(p),
			IsExperiment: true,
		}
	}
	entry.Watch()
	return entry
}

func (e *ExpFolderEntry) Refresh() {
	if e.IsExperiment {
		return
	}
	entries, err := os.ReadDir(e.Path)
	if err != nil {
		log.Fatal(err)
	}
	var items []*ExpFolderEntry
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			log.Fatal(err)
		}
		if info.IsDir() {
			items = append(items, NewEntry(path.Join(e.Path, info.Name()), e))
		}
	}
	for _, item := range e.Items {
		item.Dispose()
	}
	e.Items = items
}

func (e *ExpFolderEntry) Get() gin.H {
	if e.IsExperiment {
		return gin.H{
			"full_path": e.Path,
			"dirName":   e.Name,
			"name":      e.Experiment.Info.Name,
			"id":        e.Experiment.Info.ID,
		}
	}
	var items []gin.H
	for _, entry := range e.Items {
		items = append(items, entry.Get())
	}
	return gin.H{
		"full_path": e.Path,
		"name":      e.Name,
		"items":     items,
	}
}

func (e *ExpFolderEntry) Rename(name string) error {
	newPath := path.Join(path.Dir(e.Path), name)

	err := os.Rename(e.Path, newPath)
	if err != nil {
		return err
	}

	e.Path = newPath
	e.Name = name
	if e.IsExperiment {
		e.Experiment.UpdatePath(newPath)
	}

	if e.Parent != nil {
		e.Parent.Refresh()
	}

	return nil
}

func (e *ExpFolderEntry) Find(path string) *ExpFolderEntry {
	split := strings.SplitN(path, "/", 2)
	for _, entry := range e.Items {
		if entry.Name == split[0] {
			if len(split) == 2 {
				return entry.Find(split[1])
			} else {
				return entry
			}
		}
	}
	return nil
}

func (e *ExpFolderEntry) Watch() {
	if e.IsExperiment {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	e.Watcher = watcher

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				//log.Printf("Event: %s %d", event.Name, event.Op)
				if event.Has(fsnotify.Create | fsnotify.Remove) {
					e.Refresh()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("Error:", err)
			}
		}
	}()

	err = watcher.Add(e.Path)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("ExpFolderEntry Watching: %s", e.Path)
}

func (e *ExpFolderEntry) NotifyChange() {
	sessions.Notify(GenerateMessage("FS_CHANGE", gin.H{
		"full_path": e.Path,
	}))
}

func (e *ExpFolderEntry) Dispose() {
	if e.Watcher != nil {
		e.Watcher.Close()
		e.Watcher = nil
	}
}
