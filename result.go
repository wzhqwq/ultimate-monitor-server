package main

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Result struct {
	ObjIndex int
	Exp      Experiment

	PcPath  string
	ObjPath string

	Pcs  []ResultRecord
	Objs []ResultRecord

	Watcher *fsnotify.Watcher
}

type ResultRecord struct {
	epoch int
	Name  string
}

func getAllPcs(pcPath string) []ResultRecord {
	// get all the file names in pcPath
	entries, err := os.ReadDir(pcPath)
	if err != nil {
		log.Fatal(err)
	}
	var pcs []ResultRecord
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			log.Fatal(err)
		}
		if !info.IsDir() {
			name := info.Name()
			if epoch, _, ok := matchPcFile(name); ok {
				pcs = append(pcs, ResultRecord{epoch, name})
			}
		}
	}
	return pcs
}

func getAllObjs(objPath string, objIndex int) []ResultRecord {
	// get all the file names in objPath
	entries, err := os.ReadDir(objPath)
	if err != nil {
		log.Fatal(err)
	}
	var objs []ResultRecord
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			log.Fatal(err)
		}
		if !info.IsDir() {
			name := info.Name()
			if index, epoch, ok := matchObjFile(name); ok && index == objIndex {
				objs = append(objs, ResultRecord{epoch, name})
			}
		}
	}
	return objs
}

func NewResult(expPath string, objIndex int, exp *Experiment) *Result {
	result := &Result{
		ObjIndex: objIndex,
		Exp:      *exp,
	}
	result.UpdateExpPath(expPath)

	return result
}

func (r *Result) UpdateExpPath(expPath string) {
	// pcs is in {expPath}/debug_{objIndex}, like debug_ep0_pc_300.ply
	// objs is like {expPath}/obj0_ep0_mesh.obj
	pcPath := filepath.Join(expPath, fmt.Sprintf("debug_%d", r.ObjIndex))
	r.PcPath = pcPath
	r.ObjPath = expPath

	r.WatchPcs()
	r.Refresh()
}

func (r *Result) Refresh() {
	r.RefreshPcs()
	r.RefreshObjs()
}

func (r *Result) RefreshPcs() {
	r.Pcs = getAllPcs(r.PcPath)
	r.NotifyPcChanges()
}

func (r *Result) RefreshObjs() {
	r.Objs = getAllObjs(r.ObjPath, r.ObjIndex)
}

func (r *Result) AccessPcs(w http.ResponseWriter, afterEpoch int) error {
	var paths []string
	for _, pc := range r.Pcs {
		if pc.epoch > afterEpoch {
			paths = append(paths, filepath.Join(r.PcPath, pc.Name))
		}
	}
	err := packFilesIntoResponse(w, paths)
	return err
}

func (r *Result) AccessObjs(w http.ResponseWriter, afterEpoch int) error {
	var paths []string
	for _, obj := range r.Objs {
		if obj.epoch > afterEpoch {
			paths = append(paths, filepath.Join(r.ObjPath, obj.Name))
		}
	}
	err := packFilesIntoResponse(w, paths)
	return err
}

func (r *Result) NotifyPcChanges() {
	maxEpoch := 0
	for _, pc := range r.Pcs {
		maxEpoch = max(maxEpoch, pc.epoch)
	}
	sessions.Notify(GenerateMessage("PC_CHANGE", gin.H{
		"max_epoch":    maxEpoch,
		"exp_id":       r.Exp.Info.ID,
		"object_index": r.ObjIndex,
	}))
}

func (r *Result) WatchPcs() {
	info, err := os.Stat(r.PcPath)
	if err != nil {
		log.Fatal(err)
	}
	if info.ModTime().Add(time.Minute*10).Unix() < time.Now().Unix() {
		return
	}

	if r.Watcher == nil {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Fatal(err)
		}

		r.Watcher = watcher

		go func() {
			defer func() {
				r.Watcher.Close()
				r.Watcher = nil
			}()
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					if event.Has(fsnotify.Create) {
						r.RefreshPcs()
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Println("Error:", err)
				case <-time.After(time.Second * 10):
					log.Printf("Timed out, assuming that the experiment has stopped %s", r.PcPath)
					return
				}
			}
		}()
	}

	err = r.Watcher.Add(r.PcPath)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Result Pcs Watching: %s", r.PcPath)
}
