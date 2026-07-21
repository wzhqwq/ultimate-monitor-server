package main

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"log"
	"os"
	"path"
	"path/filepath"
)

var experiments = make(map[string]*Experiment)

type ExperimentInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Params      map[string]string `json:"params"`
}

func loadExpInfo(expPath string) (*ExperimentInfo, error) {
	// load experiment info from {expPath}/info.json
	file, err := os.Open(path.Join(expPath, "info.json"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	expInfo := &ExperimentInfo{}
	err = decoder.Decode(expInfo)
	if err != nil {
		return nil, err
	}

	return expInfo, nil
}

func saveExpInfo(expPath string, expInfo *ExperimentInfo) error {
	// save experiment info to {expPath}/info.json
	file, err := os.Create(path.Join(expPath, "info.json"))
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(expInfo)
	if err != nil {
		return err
	}

	return nil
}

type Experiment struct {
	Path    string
	Results map[int]*Result
	Info    *ExperimentInfo
}

func FindOrCreateExperiment(path string) *Experiment {
	info, err := loadExpInfo(path)
	if err != nil {
		newInfo := &ExperimentInfo{
			ID:          uuid.New().String(),
			Name:        "",
			Description: "",
			Params:      make(map[string]string),
		}
		err = saveExpInfo(path, newInfo)
		if err != nil {
			log.Fatal(err)
		}
		info = newInfo
	}

	if exp, ok := experiments[info.ID]; ok {
		exp.UpdatePath(path)
		return exp
	}

	exp := NewExperiment(path, info)
	experiments[info.ID] = exp
	return exp
}

func NewExperiment(path string, info *ExperimentInfo) *Experiment {
	// results is in {path}/debug_{objIndex}
	entries, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}

	exp := &Experiment{
		Path:    path,
		Results: make(map[int]*Result),
		Info:    info,
	}

	for _, entry := range entries {
		fileInfo, err := entry.Info()
		if err != nil {
			log.Fatal(err)
		}
		if fileInfo.IsDir() {
			name := fileInfo.Name()
			if objIndex, ok := matchPcFolder(name); ok {
				exp.Results[objIndex] = NewResult(path, objIndex, exp)
			}
		}
	}

	return exp
}

func (e *Experiment) Get() gin.H {
	objIndexes := make([]int, 0, len(e.Results))
	for objIndex := range e.Results {
		objIndexes = append(objIndexes, objIndex)
	}
	return gin.H{
		"id":      e.Info.ID,
		"name":    e.Info.Name,
		"dirName": path.Base(e.Path),
		"results": objIndexes,
	}
}

func (e *Experiment) GetResult(objIndex int) *Result {
	r, ok := e.Results[objIndex]
	if !ok {
		resultPath := filepath.Join(e.Path, fmt.Sprintf("debug_%d", objIndex))
		if _, err := os.Stat(resultPath); err == nil {
			r = NewResult(e.Path, objIndex, e)
			e.Results[objIndex] = r
		}
	}

	return r
}

func (e *Experiment) Update(info *ExperimentInfo) error {
	if info.Name != "" {
		e.Info.Name = info.Name
	}
	if info.Description != "" {
		e.Info.Description = info.Description
	}
	if info.Params != nil {
		e.Info.Params = make(map[string]string)
		for key, value := range info.Params {
			e.Info.Params[key] = value
		}
	}
	return saveExpInfo(e.Path, e.Info)
}

func (e *Experiment) UpdatePath(newPath string) {
	if e.Path == newPath {
		return
	}
	e.Path = newPath
	for _, result := range e.Results {
		result.UpdateExpPath(e.Path)
	}
}
