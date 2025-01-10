package main

import "path"

func InitProject(projectPath string) {
	expPath := path.Join(projectPath, "exp")
	dataPath := path.Join(projectPath, "data")

	baseEntry = NewEntry(expPath, nil)
	currentDataset = NewDataset(dataPath)
}
