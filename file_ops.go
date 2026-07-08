package main

import (
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func loadImageFiles(folder string) ([]string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}

	var imageFiles []string
	for _, en := range entries {
		if en.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(en.Name()))
		if imageExts[ext] {
			imageFiles = append(imageFiles, filepath.Join(folder, en.Name()))
		}
	}

	sort.Strings(imageFiles)
	return imageFiles, nil
}

func decodeImageFile(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}
