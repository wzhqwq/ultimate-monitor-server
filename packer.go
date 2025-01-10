package main

import (
	"archive/zip"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func packFilesIntoResponse(w http.ResponseWriter, files []string) error {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\"packed_files.zip\"")

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for _, file := range files {
		err := addFileToZip(zipWriter, file)
		if err != nil {
			return err
		}
	}

	return nil
}

func addFileToZip(zipWriter *zip.Writer, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer, err := zipWriter.Create(filepath.Base(filename))
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}
