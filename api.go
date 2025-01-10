package main

import (
	"github.com/gin-gonic/gin"
	"strconv"
)

func InitApi(engine *gin.Engine) {
	apiG := engine.Group("/api")
	{
		apiG.GET("/ping", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"message": "pong",
			})
		})
		initFsGroup(apiG)
		initExpGroup(apiG)
		initWs(apiG)
		initDatasetGroup(apiG)
	}
}

func initFsGroup(group *gin.RouterGroup) *gin.RouterGroup {
	fsG := group.Group("/fs")
	{
		fsG.GET("", func(c *gin.Context) {
			if baseEntry == nil {
				c.JSON(404, gin.H{
					"status":  "error",
					"message": "experiment folder not found",
				})
				return
			}
			c.JSON(200, baseEntry.Get())
		})
		fsG.GET("/:path", func(c *gin.Context) {
			path := c.Param("path")

			entry := baseEntry.Find(path)
			if entry == nil {
				c.JSON(404, gin.H{
					"status":  "error",
					"message": "folder not found",
				})
				return
			}

			c.JSON(200, entry.Get())
		})
		fsG.POST("/rename", func(c *gin.Context) {
			var req struct {
				Path    string `json:"path"`
				NewName string `json:"newName"`
			}

			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(400, gin.H{
					"status":  "error",
					"message": err.Error(),
				})
				return
			}

			path := req.Path
			newName := req.NewName

			entry := baseEntry.Find(path)
			if entry == nil {
				c.JSON(404, gin.H{
					"status":  "error",
					"message": "folder not found",
				})
				return
			}

			err := entry.Rename(newName)
			if err != nil {
				c.JSON(400, gin.H{
					"status":  "error",
					"message": err.Error(),
				})
				return
			}

			c.JSON(200, gin.H{
				"status":  "success",
				"message": "folder renamed",
			})
		})
	}
	return fsG
}

func initExpGroup(group *gin.RouterGroup) *gin.RouterGroup {
	expG := group.Group("/exp")
	{
		initSingleExpGroup(expG)
	}
	return expG
}

func initSingleExpGroup(group *gin.RouterGroup) *gin.RouterGroup {
	singleExpG := group.Group("/:id")
	{
		singleExpG.GET("", func(c *gin.Context) {
			id := c.Param("id")

			exp, ok := experiments[id]
			if !ok {
				c.JSON(404, gin.H{
					"status":  "error",
					"message": "experiment not found",
				})
				return
			}

			// Handle the request to get the experiment by id
			c.JSON(200, exp.Get())
		})
		singleExpG.PUT("", func(c *gin.Context) {
			id := c.Param("id")

			exp, ok := experiments[id]
			if !ok {
				c.JSON(404, gin.H{
					"status":  "error",
					"message": "experiment not found",
				})
				return
			}

			var updatedInfo ExperimentInfo
			if err := c.ShouldBindJSON(&updatedInfo); err != nil {
				c.JSON(400, gin.H{
					"status":  "error",
					"message": err.Error(),
				})
				return
			}

			err := exp.Update(&updatedInfo)
			if err != nil {
				c.JSON(400, gin.H{
					"status":  "error",
					"message": err.Error(),
				})
			}

			c.JSON(200, gin.H{
				"status":  "success",
				"message": "experiment updated",
			})
		})
		initSingleResultGroup(singleExpG)
	}
	return singleExpG
}

func initSingleResultGroup(group *gin.RouterGroup) *gin.RouterGroup {
	singleResultG := group.Group("/result/:resultId")
	{
		singleResultG.GET("/:type", func(c *gin.Context) {
			id := c.Param("id")
			resultId := c.Param("resultId")
			fileType := c.Param("type")
			after := c.Query("after")
			afterEpoch := -1
			if after != "" {
				num, err := strconv.Atoi(after)
				if err != nil {
					c.JSON(400, gin.H{
						"status":  "error",
						"message": err.Error(),
					})
					return
				}
				afterEpoch = num
			}

			exp, ok := experiments[id]
			if !ok {
				c.JSON(404, gin.H{
					"status":  "error",
					"message": "experiment not found",
				})
				return
			}

			resultIdNum, err := strconv.Atoi(resultId)
			if err != nil {
				c.JSON(404, gin.H{
					"status":  "error",
					"message": err.Error(),
				})
				return
			}

			result, ok := exp.Results[resultIdNum]
			if !ok {
				c.JSON(404, gin.H{
					"status":  "error",
					"message": "result not found",
				})
				return
			}

			switch fileType {
			case "pcs":
				err = result.AccessPcs(c.Writer, afterEpoch)
				if err != nil {
					c.JSON(404, gin.H{
						"status":  "error",
						"message": err.Error(),
					})
				}
			case "objs":
				err = result.AccessObjs(c.Writer, afterEpoch)
				if err != nil {
					c.JSON(404, gin.H{
						"status":  "error",
						"message": err.Error(),
					})
				}
			default:
				c.JSON(404, gin.H{
					"status":  "error",
					"message": "file type not supported",
				})
			}
		})
	}
	return singleResultG
}

func initDatasetGroup(group *gin.RouterGroup) *gin.RouterGroup {
	datasetGroup := group.Group("/dataset")
	{
		datasetGroup.GET("", func(c *gin.Context) {
			c.JSON(200, currentDataset.Get())
		})
		initSingleShapeGroup(datasetGroup)
	}
	return datasetGroup
}

func initSingleShapeGroup(group *gin.RouterGroup) *gin.RouterGroup {
	singleShapeGroup := group.Group("/shape/:index")
	{
		singleShapeGroup.GET("/:type", func(c *gin.Context) {
			index := c.Param("index")
			fileType := c.Param("type")

			indexNum, err := strconv.Atoi(index)
			if err != nil {
				c.JSON(400, gin.H{
					"status":  "error",
					"message": err.Error(),
				})
				return
			}

			if indexNum > len(currentDataset.Shapes) {
				c.JSON(404, gin.H{
					"status":  "error",
					"message": "index out of range",
				})
				return
			}

			shape := currentDataset.Shapes[indexNum]

			switch fileType {
			case "mesh":
				c.File(shape.GetMeshFileName())
			case "boundary":
				sampleCountStr := c.Query("sampleCount")
				sampleCount, err := strconv.Atoi(sampleCountStr)
				if err != nil {
					c.JSON(400, gin.H{
						"status":  "error",
						"message": err.Error(),
					})
					return
				}
				err = shape.AccessBoundary(c.Writer, sampleCount)
				if err != nil {
					c.JSON(404, gin.H{
						"status":  "error",
						"message": err.Error(),
					})
				}
			default:
				c.JSON(404, gin.H{
					"status":  "error",
					"message": "file type not supported",
				})
			}
		})
	}
	return singleShapeGroup
}
