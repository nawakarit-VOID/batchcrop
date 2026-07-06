// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type UI struct {
	Window fyne.Window
	State  *AppState

	Preview *Preview

	Status *widget.Label
}

func BuildUI(a fyne.App, state *AppState) *UI {

	ui := &UI{
		State: state,
	}

	ui.Window = a.NewWindow("BatchCrop Mini")

	ui.Preview = NewPreview(state)

	ui.Status = widget.NewLabel("Ready")

	btnInput := widget.NewButton("Open Folder", func() {

		dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {

			if err != nil || uri == nil {
				return
			}

			state.InputDir = uri.Path()

			files, err := ScanImages(state.InputDir)
			if err != nil {

				dialog.ShowError(err, ui.Window)

				return
			}

			state.Images = files

			if len(files) == 0 {

				ui.Status.SetText("No images found")

				return
			}

			if err := ui.LoadImage(files[0]); err != nil {

				dialog.ShowError(err, ui.Window)

				return
			}

			ui.Status.SetText(
				fmt.Sprintf("%d images", len(files)),
			)

		}, ui.Window).Show()

	})

	content := container.NewBorder(

		container.NewVBox(
			btnInput,
		),

		ui.Status,

		nil,
		nil,

		ui.Preview.CanvasObject(),
	)

	ui.Window.SetContent(content)

	ui.Window.Resize(fyne.NewSize(1000, 700))

	return ui

}

func (ui *UI) LoadImage(path string) error {

	file, err := os.Open(path)
	if err != nil {
		return err
	}

	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return err
	}

	ui.Preview.SetImage(img)

	return nil

}
