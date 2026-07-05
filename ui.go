// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

func BuildUI(a fyne.App, state *AppState) fyne.Window {

	w := a.NewWindow("BatchCrop Mini")

	preview := NewPreview(state)

	ui := container.NewBorder(
		nil,
		nil,
		nil,
		nil,
		preview,
	)

	w.SetContent(ui)

	w.Resize(fyne.NewSize(1000, 700))

	return w

}
