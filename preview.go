// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

type Preview struct {
	state *AppState

	root *fyne.Container

	background *canvas.Rectangle
	border     *canvas.Rectangle
	label      *canvas.Text
}

func NewPreview(state *AppState) *Preview {

	bg := canvas.NewRectangle(color.NRGBA{
		R: 45,
		G: 45,
		B: 45,
		A: 255,
	})

	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = color.NRGBA{180, 180, 180, 255}
	border.StrokeWidth = 2

	label := canvas.NewText("Preview Area", color.White)
	label.TextSize = 20

	root := container.NewWithoutLayout(
		bg,
		border,
		label,
	)

	p := &Preview{
		state:      state,
		root:       root,
		background: bg,
		border:     border,
		label:      label,
	}

	root.Resize(fyne.NewSize(800, 600))

	p.Layout()

	return p
}

func (p *Preview) CanvasObject() fyne.CanvasObject {
	return p.root
}

func (p *Preview) Layout() {

	size := p.root.Size()

	p.background.Resize(size)

	margin := float32(40)

	p.border.Move(fyne.NewPos(margin, margin))
	p.border.Resize(fyne.NewSize(
		size.Width-margin*2,
		size.Height-margin*2,
	))

	labelSize := p.label.MinSize()

	p.label.Move(fyne.NewPos(
		(size.Width-labelSize.Width)/2,
		(size.Height-labelSize.Height)/2,
	))
}
