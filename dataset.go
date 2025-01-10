package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"path"
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
	npyPath := path.Join(d.Path, "objects.npy")
	file, err := os.Open(npyPath)
	if err != nil {
		log.Fatal(err)
	}
	paths, err := ReadNpyStrArray(file)
	if err != nil {
		log.Fatal(err)
	}

	var shapes []*Shape
	for _, shapePath := range paths {
		shapes = append(shapes, NewShape(path.Join(d.Path, "..", shapePath)))
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
}

func NewShape(shapePath string) *Shape {
	shape := &Shape{
		Path: shapePath,
		Name: path.Base(shapePath),
	}
	shape.Refresh()

	return shape
}

func (s *Shape) Refresh() {
	s.Boundaries = getAllBoundaries(s.Path)
	config, err := loadShapeConfig(s.Path)
	if err != nil {
		newConfig := &ShapeInitConfig{
			PositiveSpheres: []Sphere{
				{Center: []float32{0.0, 0.0, 0.0}, Radius: 0.0},
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

func (r *Shape) GetMeshFileName() string {
	return path.Join(r.Path, "surface_normalized.obj")
}

func (r *Shape) AccessBoundary(w http.ResponseWriter, sampleCount int) error {
	var paths []string
	for _, boundary := range r.Boundaries {
		if boundary.SampleCount == sampleCount {
			paths = append(paths, path.Join(r.Path, boundary.Name))
		}
	}
	err := packFilesIntoResponse(w, paths)
	return err
}

type Boundary struct {
	Name        string
	Sigma       float64
	SampleCount int
}

type Sphere struct {
	Center []float32 `json:"center"`
	Radius float32   `json:"radius"`
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
