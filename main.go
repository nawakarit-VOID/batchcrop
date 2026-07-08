// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fmt"
	"image"
	"image/gif"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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

// ---------------------------------------------------------------------
// main / UI
// ---------------------------------------------------------------------

func main() {
	a := app.New()
	w := a.NewWindow("โปรแกรมครอปภาพหลายไฟล์พร้อมกัน")
	w.Resize(fyne.NewSize(950, 750))

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
			//folderLabel.SetText("ต้นทาง: " + inputFolder)
			folderLabel.SetText(inputFolder)

			entries, err := os.ReadDir(inputFolder)
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			imageFiles = nil
			for _, en := range entries {
				if en.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(en.Name()))
				if imageExts[ext] {
					imageFiles = append(imageFiles, filepath.Join(inputFolder, en.Name()))
				}
			}
			sort.Strings(imageFiles)
			fileCountLabel.SetText(fmt.Sprintf("พบ %d ไฟล์ภาพ", len(imageFiles)))

			if len(imageFiles) > 0 {
				f, err := os.Open(imageFiles[0])
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				img, _, err := image.Decode(f)
				f.Close()
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

		go func() {
			total := len(filesToProcess)
			okCount := 0
			var lastErr error
			for i, path := range filesToProcess {
				if err := cropAndSave(path, outDir, cropRect); err != nil {
					lastErr = err
				} else {
					okCount++
				}
				progress.SetValue(float64(i+1) / float64(total))
			}
			progress.Hide()
			cropAllBtn.Enable()
			if lastErr != nil {
				dialog.ShowInformation("เสร็จสิ้น (มีข้อผิดพลาดบางไฟล์)",
					fmt.Sprintf("ครอปสำเร็จ %d/%d ไฟล์\nข้อผิดพลาดล่าสุด: %v", okCount, total, lastErr), w)
			} else {
				dialog.ShowInformation("เสร็จสิ้น", fmt.Sprintf("ครอปสำเร็จทั้งหมด %d ไฟล์ ✅", okCount), w)
			}
		}()
	}

	fullImageBtn := widget.NewButton("เลือกเต็มภาพ", func() {
		full := selector.FullRect()
		if full.Dx() == 0 || full.Dy() == 0 {
			return
		}
		selector.SetRectOriginal(full)
		setEntriesFromRect(full)
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

		container.NewCenter(fileCountLabel), //จำนวนภาพ
		rectForm,                            // X-Y-W-H

		//widget.NewSeparator(),

		fullImageBtn,
		widget.NewLabel(""), // ว่าง

		cropAllBtn,
		progress,
	)

	content := container.NewBorder(nil, nil, nil, R, selector)
	/*
		// สร้างแนวนอน (ซ้าย-ขวา)
		content := container.NewHSplit(L, selector)
		content.SetOffset(0.1) // ตั้งค่าเป็น 70/30
	*/

	w.SetContent(content)
	w.ShowAndRun()
}

//**เพิ่มปุ่ม เพิ่ม ลด ทีละ 1 ของ xy กว้าง สูง
