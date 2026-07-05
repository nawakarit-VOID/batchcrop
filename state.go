// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import "image"

type AppState struct {
	InputDir  string
	OutputDir string

	Images  []string
	Current int

	Image image.Image

	Crop image.Rectangle
}

func NewState() *AppState {
	return &AppState{}
}
