package main

import (
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Result struct {
	ObjIndex int
	Exp      Experiment

	PcPath  string
	ObjPath string

	Pcs       []ResultRecord
	Particles []ResultRecord
	Axis      []ResultRecord
	Objs      []ResultRecord

	Watcher *fsnotify.Watcher

	PcUpdateCh chan bool
}

type ResultRecord struct {
	epoch int
	Name  string
}

type ByEpoch []ResultRecord

func (a ByEpoch) Len() int           { return len(a) }
func (a ByEpoch) Less(i, j int) bool { return a[i].epoch < a[j].epoch }
func (a ByEpoch) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

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
	sort.Sort(ByEpoch(objs))
	return objs
}

func NewResult(expPath string, objIndex int, exp *Experiment) *Result {
	result := &Result{
		ObjIndex:   objIndex,
		Exp:        *exp,
		PcUpdateCh: make(chan bool, 1),
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
	// get all the file names in pcPath
	entries, err := os.ReadDir(r.PcPath)
	if err != nil {
		log.Fatal(err)
	}
	var pcs []ResultRecord
	var particles []ResultRecord
	var axis []ResultRecord
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if epoch, ok := matchPcFile(name); ok {
				pcs = append(pcs, ResultRecord{epoch, name})
			}
			if epoch, ok := matchParticleFile(name); ok {
				particles = append(particles, ResultRecord{epoch, name})
			}
			if epoch, ok := matchAxisFile(name); ok {
				axis = append(axis, ResultRecord{epoch, name})
			}
		}
	}
	sort.Sort(ByEpoch(pcs))
	sort.Sort(ByEpoch(particles))
	sort.Sort(ByEpoch(axis))

	r.Particles, r.Pcs, r.Axis = particles, pcs, axis
	r.NotifyPcChanges()
}

func (r *Result) RefreshObjs() {
	r.Objs = getAllObjs(r.ObjPath, r.ObjIndex)
}

func (r *Result) AccessFile(w http.ResponseWriter, afterEpoch, limit int, category string) error {
	var paths []string
	var records []ResultRecord
	var basePath = r.PcPath

	switch category {
	case "pcs":
		records = r.Pcs
	case "axis":
		records = r.Axis
	case "particles":
		records = r.Particles
	case "objs":
		records = r.Objs
		basePath = r.ObjPath
	default:
		return errors.New("unknown category")
	}

	start := sort.Search(
		len(records),
		func(i int) bool {
			return records[i].epoch > afterEpoch
		},
	)
	end := start + limit
	if end > len(records) {
		end = len(records)
	}
	for _, pc := range records[start:end] {
		paths = append(paths, filepath.Join(basePath, pc.Name))
	}
	return packFilesIntoResponse(w, paths)
}

func (r *Result) NotifyPcChanges() {
	maxEpoch := 0
	for _, pc := range r.Particles {
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
	if info.ModTime().Add(time.Hour*4).Unix() < time.Now().Unix() {
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
						select {
						case r.PcUpdateCh <- true:
							go func() {
								<-time.After(time.Second)
								<-r.PcUpdateCh
								r.RefreshPcs()
							}()
						default:
						}
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Println("Error:", err)
				case <-time.After(time.Minute * 10):
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
