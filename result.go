package main

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Result struct {
	ObjIndex int
	Exp      Experiment

	PcPath  string
	ObjPath string

	active          bool
	debugInterval   int
	maxEpoch        int
	startEpoch      int
	hasExternalPc   bool
	hasExternalAxis bool

	particleFileTemplate string
	pcFileTemplate       string
	axisFileTemplate     string

	Objs []ResultRecord
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

	r.Refresh()
}

func (r *Result) Refresh() {
	r.RefreshPcs()
	r.RefreshObjs()
}

func (r *Result) SetActive(active bool) {
	if active && !r.active {
		r.debugInterval = 0
	}
	r.active = active
}

func (r *Result) RefreshPcs() {
	if r.debugInterval > 0 {
		return
	}

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

	if len(particles) > 1 {
		sort.Sort(ByEpoch(particles))
		r.debugInterval = particles[1].epoch - particles[0].epoch
		r.particleFileTemplate = particleFileReplaceRE.ReplaceAllString(particles[0].Name, "$1<EPOCH>$2")
		r.maxEpoch = particles[len(particles)-1].epoch
		r.startEpoch = particles[0].epoch
	}
	if len(pcs) > 0 {
		r.hasExternalPc = true
		r.pcFileTemplate = pcFileReplaceRE.ReplaceAllString(pcs[0].Name, "$1<EPOCH>$2")
	} else {
		r.hasExternalPc = false
	}
	if len(axis) > 0 {
		r.hasExternalAxis = true
		r.axisFileTemplate = axisFileReplaceRE.ReplaceAllString(axis[0].Name, "$1<EPOCH>$2")
	} else {
		r.hasExternalAxis = false
	}
}

func (r *Result) RefreshObjs() {
	r.Objs = getAllObjs(r.ObjPath, r.ObjIndex)
}

func (r *Result) AccessFile(w http.ResponseWriter, afterEpoch, limit int, category string) error {
	switch category {
	case "pcs", "axis", "particles":
		return packFilesIntoResponse(w, r.GetDebugOutputPaths(category, afterEpoch, limit))
	case "objs":
		return packFilesIntoResponse(w, r.GetCheckpointOutputPaths(category, afterEpoch, limit))
	default:
		return errors.New("unknown category")
	}
}

func (r *Result) GetDebugOutputPaths(category string, afterEpoch, limit int) []string {
	if r.debugInterval == 0 {
		return nil
	}

	var paths []string
	var template string

	switch category {
	case "pcs":
		if !r.hasExternalPc {
			return nil
		}
		template = r.pcFileTemplate
	case "axis":
		if !r.hasExternalAxis {
			return nil
		}
		template = r.axisFileTemplate
	case "particles":
		template = r.particleFileTemplate
	}

	epoch := r.startEpoch
	if afterEpoch >= 0 {
		epoch = afterEpoch + r.debugInterval
	}
	for i := 0; i < limit; i++ {
		if epoch > r.maxEpoch {
			break
		}
		paths = append(paths, filepath.Join(r.PcPath, strings.Replace(template, "<EPOCH>", strconv.Itoa(epoch), 1)))
		epoch += r.debugInterval
	}

	return paths
}

func (r *Result) GetCheckpointOutputPaths(category string, afterEpoch, limit int) []string {
	var paths []string
	records := r.Objs

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
		paths = append(paths, filepath.Join(r.ObjPath, pc.Name))
	}

	return paths
}

func (r *Result) NotifyPcChanges(maxEpoch int) {
	r.maxEpoch = maxEpoch
	sessions.Notify(GenerateMessage("PC_CHANGE", gin.H{
		"max_epoch":    maxEpoch,
		"exp_id":       r.Exp.Info.ID,
		"object_index": r.ObjIndex,
	}))
}
