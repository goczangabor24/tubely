package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

func getVideoAspectRatio(filepath string) (string, error) {
	type widthHeight struct {
		Streams []struct {
			Width  int `json:"width,omitempty"`
			Height int `json:"height,omitempty"`
		} `json:"streams"`
	}

	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	var b bytes.Buffer
	cmd.Stdout = &b

	if err := cmd.Run(); err != nil {
		return "", err
	}

	var JSONResponse widthHeight

	err := json.Unmarshal(b.Bytes(), &JSONResponse)
	if err != nil {
		return "", err
	}

	if len(JSONResponse.Streams) == 0 {
		return "", fmt.Errorf("No video streams found")
	}

	width := JSONResponse.Streams[0].Width
	height := JSONResponse.Streams[0].Height

	if width <= 0 || height <= 0 {
		return "", fmt.Errorf("Width or height must be positive")
	}

	w, h := aspectRatio(width, height)
	ratio := fmt.Sprintf("%d:%d", w, h)

	if ratio != "16:9" && ratio != "9:16" {
		ratio = "other"
	}

	return ratio, nil
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func aspectRatio(width, height int) (int, int) {
	divisor := gcd(width, height)
	return width / divisor, height / divisor
}
