// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package model

type AppState struct {
	Images []string

	Current int

	CropX int
	CropY int
	CropW int
	CropH int

	Zoom float32

	OutputDir string
}

func NewState() *AppState {
	return &AppState{
		Zoom: 1.0,
	}
}
