package main

import (
	"bufio"
	"encoding/json"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"
)

var currentDataset *Dataset

func getAllBoundaries(path string) []Boundary {
	// get all the file names in pcPath
	entries, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	var bds []Boundary
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			log.Fatal(err)
		}
		if !info.IsDir() {
			name := info.Name()
			if sigma, sample, ok := matchBDFile(name); ok {
				bds = append(bds, Boundary{name, sigma, sample})
			}
		}
	}
	return bds
}

type Dataset struct {
	sync.Mutex

	Path string

	Shapes []*Shape
}

func NewDataset(dataPath string) *Dataset {
	d := &Dataset{
		Path: dataPath,
	}
	d.Refresh()
	return d
}

func (d *Dataset) Refresh() {
	sheetPath := path.Join(d.Path, "object_list.txt")
	sheet, err := os.Open(sheetPath)
	if err != nil {
		log.Fatal(err)
	}
	// read lines of sheet text
	defer sheet.Close()

	var paths []string
	scanner := bufio.NewScanner(sheet)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		paths = append(paths, line)
	}

	var shapes []*Shape
	for _, shapePath := range paths {
		shapes = append(shapes, NewShape(path.Join(d.Path, shapePath)))
	}
	d.Shapes = shapes
}

func (d *Dataset) Get() gin.H {
	var names []string
	for _, shape := range d.Shapes {
		names = append(names, shape.Name)
	}
	return gin.H{
		"names": names,
		"path":  d.Path,
	}
}

type Shape struct {
	Path string
	Name string

	InitConfig *ShapeInitConfig
	Boundaries []Boundary

	Watcher *fsnotify.Watcher
}

func NewShape(shapePath string) *Shape {
	shape := &Shape{
		Path: shapePath,
		Name: path.Base(shapePath),
	}
	shape.Refresh()
	//shape.Watch()

	return shape
}

func (s *Shape) Refresh() {
	s.Boundaries = getAllBoundaries(s.Path)
	config, err := loadShapeConfig(s.Path)
	if os.IsNotExist(err) {
		newConfig := &ShapeInitConfig{
			PositiveSpheres: []Sphere{
				{Center: []float32{0.0, 0.0, 0.0}, Radius: 0.3, Ratio: 1.0},
			},
		}
		err := saveShapeConfig(s.Path, newConfig)
		if err != nil {
			log.Fatal(err)
		}
		config = newConfig
	}
	s.InitConfig = config
}

func (s *Shape) GetMeshFileName() string {
	return path.Join(s.Path, "surface_normalized.obj")
}

func (s *Shape) AccessBoundary(w http.ResponseWriter, sampleCount int) error {
	var paths []string
	s.Refresh()
	for _, boundary := range s.Boundaries {
		if boundary.SampleCount == sampleCount {
			boundaryPath := path.Join(s.Path, boundary.Name)
			paths = append(paths, boundaryPath)

			octTreeFileName := strings.Replace(boundary.Name, "boundary", "octTree", -1)
			octTreeFileName = strings.Replace(octTreeFileName, "ply", "obj", -1)
			octTreePath := path.Join(s.Path, octTreeFileName)
			log.Println(octTreePath)
			if _, err := os.Stat(octTreePath); err == nil {
				paths = append(paths, octTreePath)
			}
		}
	}
	err := packFilesIntoResponse(w, paths)
	return err
}

func (s *Shape) Watch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	s.Watcher = watcher

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
					s.Refresh()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("Error:", err)
			}
		}
	}()

	err = watcher.Add(s.Path)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Shape Watching: %s", s.Path)
}

type Boundary struct {
	Name        string
	Sigma       float64
	SampleCount int
}

type Sphere struct {
	Center []float32 `json:"center"`
	Radius float32   `json:"radius"`
	Ratio  float32   `json:"ratio"`
}

type ShapeInitConfig struct {
	PositiveSpheres []Sphere `json:"positiveSpheres"`
}

func loadShapeConfig(expPath string) (*ShapeInitConfig, error) {
	file, err := os.Open(path.Join(expPath, "initialization.json"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	expInfo := &ShapeInitConfig{}
	err = decoder.Decode(expInfo)
	if err != nil {
		return nil, err
	}

	return expInfo, nil
}

func saveShapeConfig(expPath string, expInfo *ShapeInitConfig) error {
	file, err := os.Create(path.Join(expPath, "initialization.json"))
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
