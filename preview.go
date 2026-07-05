// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

type Preview struct {
	widget.BaseWidget

	state *AppState

	text *canvas.Text
}

func NewPreview(state *AppState) *Preview {

	p := &Preview{
		state: state,
		text:  canvas.NewText("Preview", nil),
	}

	p.ExtendBaseWidget(p)

	return p

}

func (p *Preview) CreateRenderer() fyne.WidgetRenderer {

	return widget.NewSimpleRenderer(p.text)

}
