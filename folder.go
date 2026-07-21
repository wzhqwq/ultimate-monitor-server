package main

import (
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
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

func NewEntry(p string, parent *ExpFolderEntry) *ExpFolderEntry {
	name := path.Base(p)
	var entry *ExpFolderEntry
	if matched, err := regexp.Match(`p=\d+`, []byte(p)); !matched || err != nil {
		entry = &ExpFolderEntry{
			Path:         p,
			Name:         name,
			Parent:       parent,
			IsExperiment: false,
		}
		entry.Refresh()
	} else {
		entry = &ExpFolderEntry{
			Path:         p,
			Name:         name,
			Parent:       parent,
			Experiment:   FindOrCreateExperiment(p),
			IsExperiment: true,
		}
	}
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
	items := []*ExpFolderEntry{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			log.Fatal(err)
		}
		if info.IsDir() && info.Name() != "sessions" {
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

func (e *ExpFolderEntry) FindOrCreate(path string) *ExpFolderEntry {
	split := strings.SplitN(path, "/", 2)
	for _, entry := range e.Items {
		if entry.Name == split[0] {
			if len(split) == 2 {
				return entry.FindOrCreate(split[1])
			} else {
				return entry
			}
		}
	}
	stat, err := os.Stat(filepath.Join(e.Path, split[0]))
	if err != nil {
		return nil
	}
	if stat.IsDir() {
		entry := NewEntry(filepath.Join(e.Path, split[0]), e)
		e.Items = append(e.Items, entry)
		e.NotifyChange()
		return entry
	}
	return nil
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
