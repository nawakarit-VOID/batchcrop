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
	"math"
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

// keep the gif import used (needed so image.Decode recognises .gif files)
var _ = gif.Options{}

const (
	maxPreviewSize = 640
	handleVisual   = 10 // visual size (px) of the little squares at corners/edges
	handleHitZone  = 14 // how close (px) the pointer must be to grab a handle
	minCropSize    = 20 // minimum crop box size in display px
)

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
}

// ---------------------------------------------------------------------
// cropSelector: shows a reference image with a crop box on top of it.
// The box can be moved (drag inside it) or resized (drag a corner/edge
// handle). Coordinates are converted to the original image's pixel
// space whenever the box changes.
// ---------------------------------------------------------------------

type dragZone int

const (
	zoneNone dragZone = iota
	zoneBody
	zoneTL
	zoneTR
	zoneBL
	zoneBR
	zoneT
	zoneB
	zoneL
	zoneR
)

type cropSelector struct {
	widget.BaseWidget

	imgOriginalSize image.Point
	scale           float32
	dispSize        fyne.Size
	imgDisplayPos   fyne.Position
	imgDisplaySize  fyne.Size

	bgImage *canvas.Image
	rectObj *canvas.Rectangle
	handles [8]*canvas.Rectangle // TL,TR,BL,BR,T,B,L,R

	rectPos  fyne.Position
	rectSize fyne.Size

	rectOriginal      image.Rectangle
	rectOriginalValid bool

	dragging bool
	dragZone dragZone

	onChanged func(rectOriginal image.Rectangle)
}

func newCropSelector() *cropSelector {
	c := &cropSelector{}
	c.bgImage = canvas.NewImageFromImage(nil)
	c.bgImage.FillMode = canvas.ImageFillContain

	c.rectObj = canvas.NewRectangle(color.NRGBA{R: 0, G: 150, B: 255, A: 45})
	c.rectObj.StrokeColor = color.NRGBA{R: 0, G: 150, B: 255, A: 255}
	c.rectObj.StrokeWidth = 2

	for i := range c.handles {
		h := canvas.NewRectangle(color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		h.StrokeColor = color.NRGBA{R: 0, G: 120, B: 220, A: 255}
		h.StrokeWidth = 2
		h.Resize(fyne.NewSize(handleVisual, handleVisual))
		c.handles[i] = h
	}

	c.dispSize = fyne.NewSize(200, 200)
	c.ExtendBaseWidget(c)
	return c
}

func (c *cropSelector) CreateRenderer() fyne.WidgetRenderer {
	objs := []fyne.CanvasObject{c.bgImage, c.rectObj}
	for _, h := range c.handles {
		objs = append(objs, h)
	}
	return &cropSelectorRenderer{c: c, objects: objs}
}

type cropSelectorRenderer struct {
	c       *cropSelector
	objects []fyne.CanvasObject
}

func (r *cropSelectorRenderer) Layout(size fyne.Size) {
	r.c.updateLayout(size)
}
func (r *cropSelectorRenderer) MinSize() fyne.Size {
	return fyne.NewSize(200, 200)
}
func (r *cropSelectorRenderer) Refresh()                     { canvas.Refresh(r.c) }
func (r *cropSelectorRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *cropSelectorRenderer) Destroy()                     {}

func (c *cropSelector) Resize(size fyne.Size) {
	c.BaseWidget.Resize(size)
	c.updateLayout(size)
}

func (c *cropSelector) updateLayout(size fyne.Size) {
	if size.Width <= 0 || size.Height <= 0 {
		size = c.dispSize
	}
	if size.Width <= 0 || size.Height <= 0 {
		size = fyne.NewSize(200, 200)
	}

	if c.imgOriginalSize.X <= 0 || c.imgOriginalSize.Y <= 0 {
		c.dispSize = size
		c.imgDisplayPos = fyne.NewPos(0, 0)
		c.imgDisplaySize = size
		c.bgImage.Resize(size)
		c.bgImage.Move(fyne.NewPos(0, 0))
		return
	}

	imgW := float32(c.imgOriginalSize.X)
	imgH := float32(c.imgOriginalSize.Y)
	aspect := imgW / imgH
	widgetAspect := size.Width / size.Height

	boxW, boxH := size.Width, size.Height
	if aspect > widgetAspect {
		boxW = size.Width
		boxH = boxW / aspect
	} else {
		boxH = size.Height
		boxW = boxH * aspect
	}

	c.imgDisplayPos = fyne.NewPos((size.Width-boxW)/2, (size.Height-boxH)/2)
	c.imgDisplaySize = fyne.NewSize(boxW, boxH)
	c.dispSize = c.imgDisplaySize
	c.scale = boxW / imgW
	if boxH/imgH < c.scale {
		c.scale = boxH / imgH
	}

	c.bgImage.Resize(c.imgDisplaySize)
	c.bgImage.Move(c.imgDisplayPos)

	if c.rectOriginalValid {
		c.rectPos = fyne.NewPos(float32(c.rectOriginal.Min.X)*c.scale, float32(c.rectOriginal.Min.Y)*c.scale)
		c.rectSize = fyne.NewSize(float32(c.rectOriginal.Dx())*c.scale, float32(c.rectOriginal.Dy())*c.scale)
		c.clampRect()
		c.applyRectToCanvas()
	} else {
		boxW = c.dispSize.Width * 0.6
		boxH = c.dispSize.Height * 0.6
		c.rectPos = fyne.NewPos((c.dispSize.Width-boxW)/2, (c.dispSize.Height-boxH)/2)
		c.rectSize = fyne.NewSize(boxW, boxH)
		c.applyRectToCanvas()
		c.rectOriginal = c.currentRectOriginal()
		c.rectOriginalValid = true
	}
}

// SetImage loads a new reference image (scaled down to fit the preview)
// and places a default crop box centered at 60% of the image size.
func (c *cropSelector) SetImage(img image.Image) {
	c.imgOriginalSize = img.Bounds().Size()
	c.bgImage.Image = img
	c.rectOriginalValid = false

	startSize := c.Size()
	if startSize.Width <= 0 || startSize.Height <= 0 {
		startSize = fyne.NewSize(200, 200)
	}
	c.updateLayout(startSize)

	c.Refresh()
	if c.onChanged != nil {
		c.onChanged(c.currentRectOriginal())
	}
}

// zoneAt figures out which part of the crop box (if any) a point hits.
func (c *cropSelector) zoneAt(pos fyne.Position) dragZone {
	relPos := fyne.NewPos(pos.X-c.imgDisplayPos.X, pos.Y-c.imgDisplayPos.Y)

	left := c.rectPos.X
	top := c.rectPos.Y
	right := c.rectPos.X + c.rectSize.Width
	bottom := c.rectPos.Y + c.rectSize.Height

	nearLeft := absF(relPos.X-left) <= handleHitZone
	nearRight := absF(relPos.X-right) <= handleHitZone
	nearTop := absF(relPos.Y-top) <= handleHitZone
	nearBottom := absF(relPos.Y-bottom) <= handleHitZone

	withinX := relPos.X >= left-handleHitZone && relPos.X <= right+handleHitZone
	withinY := relPos.Y >= top-handleHitZone && relPos.Y <= bottom+handleHitZone
	if !withinX || !withinY {
		return zoneNone
	}

	switch {
	case nearLeft && nearTop:
		return zoneTL
	case nearRight && nearTop:
		return zoneTR
	case nearLeft && nearBottom:
		return zoneBL
	case nearRight && nearBottom:
		return zoneBR
	case nearTop && relPos.X > left && relPos.X < right:
		return zoneT
	case nearBottom && relPos.X > left && relPos.X < right:
		return zoneB
	case nearLeft && relPos.Y > top && relPos.Y < bottom:
		return zoneL
	case nearRight && relPos.Y > top && relPos.Y < bottom:
		return zoneR
	}

	if relPos.X > left && relPos.X < right && relPos.Y > top && relPos.Y < bottom {
		return zoneBody
	}
	return zoneNone
}

// Dragged implements fyne.Draggable.
func (c *cropSelector) Dragged(ev *fyne.DragEvent) {
	if c.scale == 0 {
		return
	}
	if !c.dragging {
		c.dragging = true
		startPos := fyne.NewPos(ev.Position.X-ev.Dragged.DX, ev.Position.Y-ev.Dragged.DY)
		c.dragZone = c.zoneAt(startPos)
	}
	if c.dragZone == zoneNone {
		return
	}
	dx, dy := ev.Dragged.DX, ev.Dragged.DY

	switch c.dragZone {
	case zoneBody:
		c.rectPos.X += dx
		c.rectPos.Y += dy
	case zoneTL:
		c.rectPos.X += dx
		c.rectPos.Y += dy
		c.rectSize.Width -= dx
		c.rectSize.Height -= dy
	case zoneTR:
		c.rectPos.Y += dy
		c.rectSize.Width += dx
		c.rectSize.Height -= dy
	case zoneBL:
		c.rectPos.X += dx
		c.rectSize.Width -= dx
		c.rectSize.Height += dy
	case zoneBR:
		c.rectSize.Width += dx
		c.rectSize.Height += dy
	case zoneT:
		c.rectPos.Y += dy
		c.rectSize.Height -= dy
	case zoneB:
		c.rectSize.Height += dy
	case zoneL:
		c.rectPos.X += dx
		c.rectSize.Width -= dx
	case zoneR:
		c.rectSize.Width += dx
	}

	c.clampRect()
	c.applyRectToCanvas()
	c.rectOriginal = c.currentRectOriginal()
	c.rectOriginalValid = true

	if c.onChanged != nil {
		c.onChanged(c.currentRectOriginal())
	}
}

func (c *cropSelector) DragEnd() { c.dragging = false }

func (c *cropSelector) clampRect() {
	if c.rectSize.Width < minCropSize {
		c.rectSize.Width = minCropSize
	}
	if c.rectSize.Height < minCropSize {
		c.rectSize.Height = minCropSize
	}
	if c.rectSize.Width > c.dispSize.Width {
		c.rectSize.Width = c.dispSize.Width
	}
	if c.rectSize.Height > c.dispSize.Height {
		c.rectSize.Height = c.dispSize.Height
	}
	if c.rectPos.X < 0 {
		c.rectPos.X = 0
	}
	if c.rectPos.Y < 0 {
		c.rectPos.Y = 0
	}
	if c.rectPos.X+c.rectSize.Width > c.dispSize.Width {
		c.rectPos.X = c.dispSize.Width - c.rectSize.Width
	}
	if c.rectPos.Y+c.rectSize.Height > c.dispSize.Height {
		c.rectPos.Y = c.dispSize.Height - c.rectSize.Height
	}
}

// applyRectToCanvas pushes c.rectPos/c.rectSize onto the visible rectangle
// and repositions the 8 little grab-handles around it.
func (c *cropSelector) applyRectToCanvas() {
	c.rectObj.Move(fyne.NewPos(c.imgDisplayPos.X+c.rectPos.X, c.imgDisplayPos.Y+c.rectPos.Y))
	c.rectObj.Resize(c.rectSize)
	c.rectObj.Refresh()

	left := c.rectPos.X
	top := c.rectPos.Y
	right := c.rectPos.X + c.rectSize.Width
	bottom := c.rectPos.Y + c.rectSize.Height
	midX := left + c.rectSize.Width/2
	midY := top + c.rectSize.Height/2
	half := float32(handleVisual) / 2

	pts := [8]fyne.Position{
		{X: left, Y: top},     // TL
		{X: right, Y: top},    // TR
		{X: left, Y: bottom},  // BL
		{X: right, Y: bottom}, // BR
		{X: midX, Y: top},     // T
		{X: midX, Y: bottom},  // B
		{X: left, Y: midY},    // L
		{X: right, Y: midY},   // R
	}
	for i, p := range pts {
		handlePos := fyne.NewPos(c.imgDisplayPos.X+p.X-half, c.imgDisplayPos.Y+p.Y-half)
		c.handles[i].Move(handlePos)
		c.handles[i].Refresh()
	}
}

// currentRectOriginal converts the on-screen box back to the reference
// image's pixel coordinates. Edges that are within a couple of display
// pixels of the image border are snapped exactly to that border, so
// dragging a handle "all the way" reliably yields the full image size
// instead of falling a pixel or two short due to rounding.
func (c *cropSelector) currentRectOriginal() image.Rectangle {
	const snapPx = 3.0

	left, top := c.rectPos.X, c.rectPos.Y
	right, bottom := c.rectPos.X+c.rectSize.Width, c.rectPos.Y+c.rectSize.Height

	if left <= snapPx {
		left = 0
	}
	if top <= snapPx {
		top = 0
	}
	if c.dispSize.Width-right <= snapPx {
		right = c.dispSize.Width
	}
	if c.dispSize.Height-bottom <= snapPx {
		bottom = c.dispSize.Height
	}

	x0 := int(math.Round(float64(left / c.scale)))
	y0 := int(math.Round(float64(top / c.scale)))
	x1 := int(math.Round(float64(right / c.scale)))
	y1 := int(math.Round(float64(bottom / c.scale)))

	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > c.imgOriginalSize.X {
		x1 = c.imgOriginalSize.X
	}
	if y1 > c.imgOriginalSize.Y {
		y1 = c.imgOriginalSize.Y
	}
	return image.Rect(x0, y0, x1, y1)
}

// FullRect returns a rectangle covering the whole reference image, in its
// original pixel coordinates.
func (c *cropSelector) FullRect() image.Rectangle {
	return image.Rect(0, 0, c.imgOriginalSize.X, c.imgOriginalSize.Y)
}

// SetRectOriginal updates the box from a rect given in the reference
// image's original pixel coordinates (used when the user types numbers).
func (c *cropSelector) SetRectOriginal(rect image.Rectangle) {
	if c.scale == 0 {
		return
	}
	c.rectPos = fyne.NewPos(float32(rect.Min.X)*c.scale, float32(rect.Min.Y)*c.scale)
	c.rectSize = fyne.NewSize(float32(rect.Dx())*c.scale, float32(rect.Dy())*c.scale)
	c.clampRect()
	c.applyRectToCanvas()
	c.rectOriginal = rect
	c.rectOriginalValid = true
}

func absF(v float32) float32 {
	if v < 0 {
		return -v
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
		// gif needs a paletted image; save as png instead to keep full color/quality
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

	rectForm := container.NewGridWithColumns(4,
		widget.NewLabel("X:"), xEntry,
		widget.NewLabel("Y:"), yEntry,
		widget.NewLabel("กว้าง:"), wEntry,
		widget.NewLabel("สูง:"), hEntry,
	)

	L := container.NewVBox(
		container.NewHBox(chooseInputBtn, folderLabel),
		fileCountLabel,
		container.NewHBox(chooseOutputBtn, outLabel),

		//widget.NewSeparator(),
		container.NewHBox(rectForm, fullImageBtn, cropAllBtn, progress),
		//widget.NewSeparator(),
	)

	content := container.NewBorder(L, nil, nil, nil, selector)
	/*
		// สร้างแนวนอน (ซ้าย-ขวา)
		content := container.NewHSplit(L, selector)
		content.SetOffset(0.1) // ตั้งค่าเป็น 70/30
	*/

	w.SetContent(content)
	w.ShowAndRun()
}

//**เพิ่มปุ่ม เพิ่ม ลด ทีละ 1 ของ xy กว้าง สูง
