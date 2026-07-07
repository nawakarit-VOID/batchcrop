// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// silence "imported and not used" for gif encoder constant usage guard
var _ = gif.Options{}

const maxPreviewSize = 640

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
}

// ---------------------------------------------------------------------
// cropSelector: custom widget that shows a reference image and lets the
// user drag out a rectangle on it to define the crop area.
// ---------------------------------------------------------------------

type cropSelector struct {
	widget.BaseWidget

	imgOriginalSize image.Point
	scale           float32
	dispSize        fyne.Size

	bgImage *canvas.Image
	rectObj *canvas.Rectangle

	dragging  bool
	dragStart fyne.Position

	onChanged func(rectOriginal image.Rectangle)
}

func newCropSelector() *cropSelector {
	c := &cropSelector{}
	c.bgImage = canvas.NewImageFromImage(nil)
	c.bgImage.FillMode = canvas.ImageFillStretch
	c.rectObj = canvas.NewRectangle(color.NRGBA{R: 0, G: 150, B: 255, A: 60})
	c.rectObj.StrokeColor = color.NRGBA{R: 0, G: 150, B: 255, A: 255}
	c.rectObj.StrokeWidth = 2
	c.dispSize = fyne.NewSize(200, 200)
	c.ExtendBaseWidget(c)
	return c
}

func (c *cropSelector) CreateRenderer() fyne.WidgetRenderer {
	return &cropSelectorRenderer{c: c, objects: []fyne.CanvasObject{c.bgImage, c.rectObj}}
}

type cropSelectorRenderer struct {
	c       *cropSelector
	objects []fyne.CanvasObject
}

func (r *cropSelectorRenderer) Layout(size fyne.Size) {
	r.c.bgImage.Resize(size)
	r.c.bgImage.Move(fyne.NewPos(0, 0))
}
func (r *cropSelectorRenderer) MinSize() fyne.Size           { return r.c.dispSize }
func (r *cropSelectorRenderer) Refresh()                     { canvas.Refresh(r.c) }
func (r *cropSelectorRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *cropSelectorRenderer) Destroy()                     {}

// SetImage loads a new reference image, scaling it down for display if needed.
func (c *cropSelector) SetImage(img image.Image) {
	c.imgOriginalSize = img.Bounds().Size()
	c.bgImage.Image = img

	w, h := float32(c.imgOriginalSize.X), float32(c.imgOriginalSize.Y)
	scale := float32(1.0)
	if w > maxPreviewSize || h > maxPreviewSize {
		if w > h {
			scale = maxPreviewSize / w
		} else {
			scale = maxPreviewSize / h
		}
	}
	c.scale = scale
	c.dispSize = fyne.NewSize(w*scale, h*scale)
	c.Resize(c.dispSize)
	c.Refresh()
}

// Dragged implements fyne.Draggable - lets the user rubber-band select a rect.
func (c *cropSelector) Dragged(ev *fyne.DragEvent) {
	if c.scale == 0 {
		return
	}
	if !c.dragging {
		c.dragging = true
		c.dragStart = fyne.NewPos(ev.Position.X-ev.Dragged.DX, ev.Position.Y-ev.Dragged.DY)
	}
	pos := ev.Position
	x1, y1 := c.dragStart.X, c.dragStart.Y
	x2, y2 := pos.X, pos.Y
	left, top := minF(x1, x2), minF(y1, y2)
	right, bottom := maxF(x1, x2), maxF(y1, y2)
	left = clampF(left, 0, c.dispSize.Width)
	top = clampF(top, 0, c.dispSize.Height)
	right = clampF(right, 0, c.dispSize.Width)
	bottom = clampF(bottom, 0, c.dispSize.Height)

	c.rectObj.Move(fyne.NewPos(left, top))
	c.rectObj.Resize(fyne.NewSize(right-left, bottom-top))
	c.rectObj.Refresh()

	if c.onChanged != nil {
		c.onChanged(c.currentRectOriginal())
	}
}

func (c *cropSelector) DragEnd() { c.dragging = false }

func (c *cropSelector) currentRectOriginal() image.Rectangle {
	pos := c.rectObj.Position()
	size := c.rectObj.Size()
	x0 := int(pos.X / c.scale)
	y0 := int(pos.Y / c.scale)
	x1 := int((pos.X + size.Width) / c.scale)
	y1 := int((pos.Y + size.Height) / c.scale)
	return image.Rect(x0, y0, x1, y1)
}

// SetRectOriginal updates the overlay rectangle from a rect given in the
// reference image's original pixel coordinates (e.g. from manual entry).
func (c *cropSelector) SetRectOriginal(rect image.Rectangle) {
	if c.scale == 0 {
		return
	}
	x := float32(rect.Min.X) * c.scale
	y := float32(rect.Min.Y) * c.scale
	w := float32(rect.Dx()) * c.scale
	h := float32(rect.Dy()) * c.scale
	c.rectObj.Move(fyne.NewPos(x, y))
	c.rectObj.Resize(fyne.NewSize(w, h))
	c.rectObj.Refresh()
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
func clampF(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---------------------------------------------------------------------
// cropping logic
// ---------------------------------------------------------------------

func cropAndSave(srcPath, outDir string, rect image.Rectangle) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("อ่านไฟล์ %s ไม่ได้: %w", filepath.Base(srcPath), err)
	}

	r := rect.Intersect(img.Bounds())
	if r.Empty() {
		return fmt.Errorf("พื้นที่ครอปอยู่นอกขอบเขตของภาพ %s", filepath.Base(srcPath))
	}

	dst := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(dst, dst.Bounds(), img, r.Min, draw.Src)

	name := filepath.Base(srcPath)
	base := strings.TrimSuffix(name, filepath.Ext(name))

	var outPath string
	switch format {
	case "png":
		outPath = filepath.Join(outDir, base+".png")
	case "gif":
		// gif encoding needs a paletted image; save as png instead to keep full color/quality
		outPath = filepath.Join(outDir, base+".png")
	default:
		outPath = filepath.Join(outDir, base+".jpg")
	}

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	switch format {
	case "png", "gif":
		return png.Encode(out, dst)
	default:
		return jpeg.Encode(out, dst, &jpeg.Options{Quality: 95})
	}
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

	folderLabel := widget.NewLabel("ยังไม่ได้เลือกโฟลเดอร์ภาพต้นทาง")
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

	chooseInputBtn := widget.NewButtonWithIcon("เลือกโฟลเดอร์ภาพต้นทาง", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			inputFolder = uri.Path()
			folderLabel.SetText("ต้นทาง: " + inputFolder)

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
				selector.SetImage(img)
				b := img.Bounds()
				defaultRect := image.Rect(0, 0, b.Dx()/2, b.Dy()/2)
				setEntriesFromRect(defaultRect)
				selector.SetRectOriginal(defaultRect)
			} else {
				dialog.ShowInformation("แจ้งเตือน", "ไม่พบไฟล์ภาพ (.jpg .jpeg .png .gif) ในโฟลเดอร์นี้", w)
			}
		}, w)
	})

	chooseOutputBtn := widget.NewButtonWithIcon("เลือกโฟลเดอร์ปลายทาง", theme.FolderIcon(), func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			outputFolder = uri.Path()
			outLabel.SetText("ปลายทาง: " + outputFolder)
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

	rectForm := container.NewGridWithColumns(4,
		widget.NewLabel("X:"), xEntry,
		widget.NewLabel("Y:"), yEntry,
		widget.NewLabel("กว้าง:"), wEntry,
		widget.NewLabel("สูง:"), hEntry,
	)

	topBar := container.NewVBox(
		container.NewHBox(chooseInputBtn, folderLabel),
		fileCountLabel,
		container.NewHBox(chooseOutputBtn, outLabel),
		widget.NewSeparator(),
		widget.NewLabel("ลากเมาส์บนภาพด้านล่างเพื่อเลือกพื้นที่ครอป (จะใช้ตำแหน่ง/ขนาดเดียวกันกับทุกภาพ) หรือกรอกตัวเลขเอง:"),
		rectForm,
		cropAllBtn,
		progress,
		widget.NewSeparator(),
	)

	scrollPreview := container.NewScroll(selector)

	content := container.NewBorder(topBar, nil, nil, nil, scrollPreview)
	w.SetContent(content)
	w.ShowAndRun()
}
