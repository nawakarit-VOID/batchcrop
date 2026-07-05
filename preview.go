// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

type Preview struct {
	widget.BaseWidget

	state *AppState

	bg     *canvas.Rectangle
	border *canvas.Rectangle
	label  *canvas.Text
}

func NewPreview(state *AppState) *Preview {
	p := &Preview{
		state: state,
		bg: canvas.NewRectangle(color.NRGBA{
			R: 45,
			G: 45,
			B: 45,
			A: 255,
		}),
		border: canvas.NewRectangle(color.Transparent),
		label:  canvas.NewText("Preview Area", color.White),
	}

	p.border.StrokeColor = color.NRGBA{R: 160, G: 160, B: 160, A: 255}
	p.border.StrokeWidth = 2

	p.ExtendBaseWidget(p)

	return p
}

func (p *Preview) CreateRenderer() fyne.WidgetRenderer {
	objects := []fyne.CanvasObject{
		p.bg,
		p.border,
		p.label,
	}

	return &previewRenderer{
		preview: p,
		objects: objects,
	}
}

type previewRenderer struct {
	preview *Preview
	objects []fyne.CanvasObject
}

func (r *previewRenderer) Layout(size fyne.Size) {

	r.preview.bg.Resize(size)

	margin := float32(40)

	r.preview.border.Move(fyne.NewPos(margin, margin))
	r.preview.border.Resize(fyne.NewSize(
		size.Width-margin*2,
		size.Height-margin*2,
	))

	labelSize := r.preview.label.MinSize()

	r.preview.label.Move(fyne.NewPos(
		(size.Width-labelSize.Width)/2,
		(size.Height-labelSize.Height)/2,
	))
}

func (r *previewRenderer) MinSize() fyne.Size {
	return fyne.NewSize(400, 300)
}

func (r *previewRenderer) Refresh() {
	canvas.Refresh(r.preview.bg)
	canvas.Refresh(r.preview.border)
	canvas.Refresh(r.preview.label)
}

func (r *previewRenderer) Destroy() {}

func (r *previewRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}
