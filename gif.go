package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

func convertToGif(c *gin.Context) {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "ffmpeg not found in $PATH"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	files := form.File["video"]

	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No files supplied"})
		return
	}

	file := files[0]

	tempPath, err := os.UserCacheDir()
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	}

	processPath, err := os.MkdirTemp(tempPath, "um-conversion-*")

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	sourcePath := filepath.Join(processPath, "input.mp4")
	outputPath := filepath.Join(processPath, "output.gif")

	if err := c.SaveUploadedFile(file, sourcePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	convertCmd := "ffmpeg -i " + sourcePath + " -vf \"fps=5\" -c:v pam -f image2pipe - | convert -delay 5 - -loop 0 -layers optimize " + outputPath
	_, err = exec.Command("bash", "-c", convertCmd).CombinedOutput()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.File(outputPath)

	os.RemoveAll(processPath)
}
