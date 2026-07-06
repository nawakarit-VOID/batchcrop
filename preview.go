// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

type Preview struct {
	state *AppState

	root *fyne.Container

	bg *canvas.Rectangle

	img *canvas.Image
}

func NewPreview(state *AppState) *Preview {

	p := &Preview{
		state: state,
	}

	p.bg = canvas.NewRectangle(themeBackground())

	p.img = canvas.NewImageFromImage(nil)
	p.img.FillMode = canvas.ImageFillContain

	p.root = container.NewMax(
		p.bg,
		p.img,
	)

	return p
}

func (p *Preview) CanvasObject() fyne.CanvasObject {
	return p.root
}

func (p *Preview) SetImage(img image.Image) {

	p.state.Img = img

	p.img.Image = img
	p.img.Refresh()

}
