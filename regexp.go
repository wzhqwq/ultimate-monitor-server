package main

import (
	"log"
	"regexp"
	"strconv"
)

var pcFolderRE = regexp.MustCompile(`debug_(\d+)`)
var pcFileRE = regexp.MustCompile(`debug_ep(\d+)_pc_(\d+)\.ply`)
var objFileRE = regexp.MustCompile(`obj(\d+)_ep(\d+)_mesh\.obj`)
var boundaryFileRE = regexp.MustCompile(`boundary_sigma=([\d.]+)_s=(\d+).ply`)
var fixedLengthStrRE = regexp.MustCompile(`<U(\d+)`)

func matchPcFolder(folderName string) (int, bool) {
	matches := pcFolderRE.FindStringSubmatch(folderName)
	if len(matches) > 1 {
		index, err := strconv.Atoi(matches[1])
		if err != nil {
			log.Fatal(err)
		}
		return index, true
	}
	return 0, false
}

func matchPcFile(filename string) (int, int, bool) {
	matches := pcFileRE.FindStringSubmatch(filename)
	if len(matches) > 2 {
		epoch, err := strconv.Atoi(matches[1])
		if err != nil {
			log.Fatal(err)
		}
		count, err := strconv.Atoi(matches[2])
		if err != nil {
			log.Fatal(err)
		}
		return epoch, count, true
	}
	return 0, 0, false
}

func matchObjFile(filename string) (int, int, bool) {
	matches := objFileRE.FindStringSubmatch(filename)
	if len(matches) > 2 {
		index, err := strconv.Atoi(matches[1])
		if err != nil {
			log.Fatal(err)
		}
		epoch, err := strconv.Atoi(matches[2])
		if err != nil {
			log.Fatal(err)
		}
		return index, epoch, true
	}
	return 0, 0, false
}

func matchBDFile(filename string) (float64, int, bool) {
	matches := boundaryFileRE.FindStringSubmatch(filename)
	if len(matches) > 2 {
		sigma, err := strconv.ParseFloat(matches[1], 64)
		if err != nil {
			log.Fatal(err)
		}
		sampleNum, err := strconv.Atoi(matches[2])
		if err != nil {
			log.Fatal(err)
		}
		return sigma, sampleNum, true
	}
	return 0, 0, false
}

func matchFixedLenStr(describer string) (int, bool) {
	matches := fixedLengthStrRE.FindStringSubmatch(describer)
	if len(matches) > 1 {
		length, err := strconv.Atoi(matches[1])
		if err != nil {
			log.Fatal(err)
		}
		return length, true
	}
	return 0, false
}
