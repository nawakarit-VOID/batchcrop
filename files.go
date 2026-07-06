// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func ScanImages(dir string) ([]string, error) {

	var files []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, e := range entries {

		if e.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(e.Name()))

		switch ext {

		case ".jpg",
			".jpeg",
			".png",
			".bmp",
			".gif",
			".webp":

			files = append(files,
				filepath.Join(dir, e.Name()))

		}

	}

	sort.Strings(files)

	return files, nil

}
