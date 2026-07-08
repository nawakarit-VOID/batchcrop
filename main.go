// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"embed"
	"fmt"
	"image"
	"image/gif"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// keep the gif import used (needed so image.Decode recognises .gif files)
var _ = gif.Options{}

const maxPreviewSize = 640

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
}

// โหลด icon
func loadIcon(size int) fyne.Resource {
	var file string

	switch {
	case size >= 512:
		file = "assets/icons/icon-512.png" ///ที่อยู่
	case size >= 256:
		file = "assets/icons/icon-256.png"
	case size >= 128:
		file = "assets/icons/icon-128.png"
	default:
		file = "assets/icons/icon-64.png"
	}

	data, _ := iconFS.ReadFile(file)
	return fyne.NewStaticResource(file, data)
}

//go:embed assets/icons/*
var iconFS embed.FS

//go:embed assets/font/Itim-Regular.ttf
var fontItim []byte
var myFont = fyne.NewStaticResource("Itim-Regular.ttf", fontItim)

// ---------------------------------------------------------------------
// main / UI
// ---------------------------------------------------------------------

func main() {
	a := app.NewWithID("com.nawakarit.batchcrop")
	a.Settings().SetTheme(&MyTheme{})

	icons := loadIcon(64) //เอา data มาใช้
	a.SetIcon(icons)

	w := a.NewWindow("batchcrop : โปรแกรมครอปภาพหลายไฟล์พร้อมกัน")
	w.Resize(fyne.NewSize(950, 750))
	w.SetIcon(icons)

	var (
		inputFolder  string
		outputFolder string
		imageFiles   []string
	)

	selector := newCropSelector()

	folderLabel := widget.NewLabel("ยังไม่ได้เลือกโฟลเดอร์ต้นทาง")
	outLabel := widget.NewLabel("ยังไม่ได้เลือกโฟลเดอร์ปลายทาง")
	fileCountLabel := widget.NewLabel("พบ 0 ไฟล์ภาพ")

	xEntry := widget.NewEntry()
	yEntry := widget.NewEntry()
	wEntry := widget.NewEntry()
	hEntry := widget.NewEntry()
	for _, e := range []*widget.Entry{xEntry, yEntry, wEntry, hEntry} {
		e.SetText("0")
	}

	updatingFromDrag := false

	setEntriesFromRect := func(r image.Rectangle) {
		updatingFromDrag = true
		xEntry.SetText(strconv.Itoa(r.Min.X))
		yEntry.SetText(strconv.Itoa(r.Min.Y))
		wEntry.SetText(strconv.Itoa(r.Dx()))
		hEntry.SetText(strconv.Itoa(r.Dy()))
		updatingFromDrag = false
	}
	selector.onChanged = setEntriesFromRect

	applyEntriesToOverlay := func() {
		if updatingFromDrag {
			return
		}
		x, errX := strconv.Atoi(xEntry.Text)
		y, errY := strconv.Atoi(yEntry.Text)
		wv, errW := strconv.Atoi(wEntry.Text)
		hv, errH := strconv.Atoi(hEntry.Text)
		if errX != nil || errY != nil || errW != nil || errH != nil || wv <= 0 || hv <= 0 {
			return
		}
		selector.SetRectOriginal(image.Rect(x, y, x+wv, y+hv))
	}
	xEntry.OnChanged = func(string) { applyEntriesToOverlay() }
	yEntry.OnChanged = func(string) { applyEntriesToOverlay() }
	wEntry.OnChanged = func(string) { applyEntriesToOverlay() }
	hEntry.OnChanged = func(string) { applyEntriesToOverlay() }

	chooseInputBtn := widget.NewButtonWithIcon("IN", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			inputFolder = uri.Path()
			folderLabel.SetText(inputFolder)

			files, err := loadImageFiles(inputFolder)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			imageFiles = files
			fileCountLabel.SetText(fmt.Sprintf("พบ %d ไฟล์ภาพ", len(imageFiles)))

			if len(imageFiles) > 0 {
				img, err := decodeImageFile(imageFiles[0])
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				selector.SetImage(img) // this also places a default crop box + fires onChanged
			} else {
				dialog.ShowInformation("แจ้งเตือน", "ไม่พบไฟล์ภาพ (.jpg .jpeg .png .gif) ในโฟลเดอร์นี้", w)
			}
		}, w)
	})

	chooseOutputBtn := widget.NewButtonWithIcon("OUT", theme.FolderIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			outputFolder = uri.Path()
			//outLabel.SetText("ปลายทาง: " + outputFolder)
			outLabel.SetText(outputFolder)

		}, w)
	})

	progress := widget.NewProgressBar()
	progress.Hide()

	cropAllBtn := widget.NewButtonWithIcon("เริ่มครอปทั้งหมด", theme.ContentCopyIcon(), nil)
	cropAllBtn.OnTapped = func() {
		if inputFolder == "" || len(imageFiles) == 0 {
			dialog.ShowInformation("แจ้งเตือน", "กรุณาเลือกโฟลเดอร์ภาพต้นทางที่มีไฟล์ภาพก่อน", w)
			return
		}
		if outputFolder == "" {
			dialog.ShowInformation("แจ้งเตือน", "กรุณาเลือกโฟลเดอร์ปลายทางก่อน", w)
			return
		}
		x, errX := strconv.Atoi(xEntry.Text)
		y, errY := strconv.Atoi(yEntry.Text)
		wv, errW := strconv.Atoi(wEntry.Text)
		hv, errH := strconv.Atoi(hEntry.Text)
		if errX != nil || errY != nil || errW != nil || errH != nil || wv <= 0 || hv <= 0 {
			dialog.ShowInformation("แจ้งเตือน", "ตัวเลขตำแหน่ง/ขนาดของพื้นที่ครอปไม่ถูกต้อง", w)
			return
		}
		cropRect := image.Rect(x, y, x+wv, y+hv)

		cropAllBtn.Disable()
		progress.Show()
		progress.SetValue(0)

		filesToProcess := append([]string(nil), imageFiles...)
		outDir := outputFolder

		startBatchCrop(
			w,
			func(pct float64) { progress.SetValue(pct) },
			func(okCount, total int, lastErr error) {
				progress.Hide()
				cropAllBtn.Enable()
				if lastErr != nil {
					dialog.ShowInformation("เสร็จสิ้น (มีข้อผิดพลาดบางไฟล์)",
						fmt.Sprintf("ครอปสำเร็จ %d/%d ไฟล์\nข้อผิดพลาดล่าสุด: %v", okCount, total, lastErr), w)
				} else {
					dialog.ShowInformation("เสร็จสิ้น", fmt.Sprintf("ครอปสำเร็จทั้งหมด %d ไฟล์ ✅", okCount), w)
				}
			},
			filesToProcess,
			outDir,
			cropRect,
		)
	}

	fullImageBtn := widget.NewButton("เลือกเต็มภาพ", func() {
		full := selector.FullRect()
		if full.Dx() == 0 || full.Dy() == 0 {
			return
		}
		selector.SetRectOriginal(full)
		setEntriesFromRect(full)
	})

	abbtn := widget.NewButton("!", func() {
		dialog.ShowInformation("about", "*ไฟล์ภาพทั้งโฟลเดอร์ต้องมีขนาด ความกว้าง ความยาว เท่ากัน\n\nBy nawakarit - เจช์ (วัดดงหมี)\nhttps://github.com/nawakarit-VOID\n© 2026", w)
	})

	rectForm := container.NewCenter(
		container.NewVBox(
			container.NewHBox(container.NewGridWrap(fyne.NewSize(100, 40), widget.NewLabel("X :")), container.NewGridWrap(fyne.NewSize(100, 40), xEntry)),
			container.NewHBox(container.NewGridWrap(fyne.NewSize(100, 40), widget.NewLabel("Y :")), container.NewGridWrap(fyne.NewSize(100, 40), yEntry)),
			container.NewHBox(container.NewGridWrap(fyne.NewSize(100, 40), widget.NewLabel("W :")), container.NewGridWrap(fyne.NewSize(100, 40), wEntry)),
			container.NewHBox(container.NewGridWrap(fyne.NewSize(100, 40), widget.NewLabel("H :")), container.NewGridWrap(fyne.NewSize(100, 40), hEntry)),
		))

	R := container.NewVBox(
		container.NewHBox( //เลือกโฟลเดอร์ไฟล์
			container.NewGridWrap(fyne.NewSize(100, 35), chooseInputBtn),
			container.NewGridWrap(fyne.NewSize(200, 35), container.NewHScroll(folderLabel))),
		container.NewHBox(
			container.NewGridWrap(fyne.NewSize(100, 35), chooseOutputBtn),
			container.NewGridWrap(fyne.NewSize(200, 35), container.NewHScroll(outLabel))),

		container.NewCenter(
			container.NewHBox(
				fileCountLabel, abbtn)), //จำนวนภาพ
		rectForm, // X-Y-W-H

		//widget.NewSeparator(),

		fullImageBtn,
		widget.NewLabel(""), // ว่าง

		cropAllBtn,
		progress,
	)

	content := container.NewBorder(nil, nil, nil, R, selector)

	w.SetContent(content)
	w.ShowAndRun()
}
