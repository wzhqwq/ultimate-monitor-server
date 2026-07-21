package main

import "path"

var currentDataset *Dataset
var baseEntry *ExpFolderEntry
var sessionWatcher *SessionWatcher

func InitProject(projectPath string) {
	expPath := path.Join(projectPath, "exp")
	dataPath := path.Join(projectPath, "data")

	baseEntry = NewEntry(expPath, nil)
	currentDataset = NewDataset(dataPath)
	sessionWatcher = NewSessionWatcher(expPath)
}
