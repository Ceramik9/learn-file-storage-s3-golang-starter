package main

import (
	"os/exec"
	"bytes"
	"encoding/json"
	"fmt"
	"errors"
)

func getVideoAspectRatio(filePath string) (string, error) {

	type resolution struct {
		Streams []struct {
			Height int `json:"height"`
			Width  int `json:"width"`
		} `json:"streams"`
	}

	cmd := exec.Command(
		"ffprobe",
		"-v",
		"error",
		"-print_format",
		"json",
		"-show_streams",
		filePath,
	)

	var buffer bytes.Buffer
	cmd.Stdout = &buffer
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	var params resolution

	err = json.Unmarshal(buffer.Bytes(), &params)
	if err != nil {
		return "", err
	}

	if len(params.Streams) == 0 {
		return "", errors.New("Streams array is empty")
	}

	aspectRatio, err := getAspectRatio(
		params.Streams[0].Width,
		params.Streams[0].Height,
	)
	if err != nil {
		return "", err
	}

	return aspectRatio, nil
}


func getAspectRatio(width, height int) (string, error) {

	if width <= 0 || height <= 0 {
		return "", errors.New("invalid resolution")
	}

	divisor := gdc(width, height)
	
	aspectRatio := fmt.Sprintf("%d:%d", width/divisor, height/divisor)

	return aspectRatio, nil
}


func gdc(a, b int) int {

	for b != 0 {
		a, b = b, a % b
	}

	return a
}


