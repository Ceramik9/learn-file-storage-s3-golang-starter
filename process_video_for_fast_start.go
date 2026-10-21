package main

import (
	"fmt"
	"os/exec"

)

func processVideoForFastStart(filePath string) (string, error) {
	
	newFilePath := fmt.Sprintf("%s.processing", filePath)
	
	cmd := exec.Command(
		"ffmpeg",
		"-i",
		filePath,
		"-c",
		"copy",
		"-movflags",
		"faststart",
		"-f",
		"mp4",
		newFilePath,
	)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	
	return newFilePath, nil
}
