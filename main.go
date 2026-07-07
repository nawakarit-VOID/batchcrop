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

	bgImage *canvas.Image
	rectObj *canvas.Rectangle
	handles [8]*canvas.Rectangle // TL,TR,BL,BR,T,B,L,R

	rectPos  fyne.Position
	rectSize fyne.Size

	dragging bool
	dragZone dragZone

	onChanged func(rectOriginal image.Rectangle)
}

func newCropSelector() *cropSelector {
	c := &cropSelector{}
	c.bgImage = canvas.NewImageFromImage(nil)
	c.bgImage.FillMode = canvas.ImageFillStretch

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
	r.c.bgImage.Resize(size)
	r.c.bgImage.Move(fyne.NewPos(0, 0))
}
func (r *cropSelectorRenderer) MinSize() fyne.Size           { return r.c.dispSize }
func (r *cropSelectorRenderer) Refresh()                     { canvas.Refresh(r.c) }
func (r *cropSelectorRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *cropSelectorRenderer) Destroy()                     {}

// SetImage loads a new reference image (scaled down to fit the preview)
// and places a default crop box centered at 60% of the image size.
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

	boxW := c.dispSize.Width * 0.6
	boxH := c.dispSize.Height * 0.6
	c.rectPos = fyne.NewPos((c.dispSize.Width-boxW)/2, (c.dispSize.Height-boxH)/2)
	c.rectSize = fyne.NewSize(boxW, boxH)
	c.applyRectToCanvas()

	c.Refresh()
	if c.onChanged != nil {
		c.onChanged(c.currentRectOriginal())
	}
}

// zoneAt figures out which part of the crop box (if any) a point hits.
func (c *cropSelector) zoneAt(pos fyne.Position) dragZone {
	left := c.rectPos.X
	top := c.rectPos.Y
	right := c.rectPos.X + c.rectSize.Width
	bottom := c.rectPos.Y + c.rectSize.Height

	nearLeft := absF(pos.X-left) <= handleHitZone
	nearRight := absF(pos.X-right) <= handleHitZone
	nearTop := absF(pos.Y-top) <= handleHitZone
	nearBottom := absF(pos.Y-bottom) <= handleHitZone

	withinX := pos.X >= left-handleHitZone && pos.X <= right+handleHitZone
	withinY := pos.Y >= top-handleHitZone && pos.Y <= bottom+handleHitZone
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
	case nearTop && pos.X > left && pos.X < right:
		return zoneT
	case nearBottom && pos.X > left && pos.X < right:
		return zoneB
	case nearLeft && pos.Y > top && pos.Y < bottom:
		return zoneL
	case nearRight && pos.Y > top && pos.Y < bottom:
		return zoneR
	}

	if pos.X > left && pos.X < right && pos.Y > top && pos.Y < bottom {
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
	c.rectObj.Move(c.rectPos)
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
		c.handles[i].Move(fyne.NewPos(p.X-half, p.Y-half))
		c.handles[i].Refresh()
	}
}

func (c *cropSelector) currentRectOriginal() image.Rectangle {
	x0 := int(c.rectPos.X / c.scale)
	y0 := int(c.rectPos.Y / c.scale)
	x1 := int((c.rectPos.X + c.rectSize.Width) / c.scale)
	y1 := int((c.rectPos.Y + c.rectSize.Height) / c.scale)
	return image.Rect(x0, y0, x1, y1)
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
				selector.SetImage(img) // this also places a default crop box + fires onChanged
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
		widget.NewLabel("ลากตรงกลางกรอบเพื่อเลื่อน หรือลากที่มุม/ขอบเพื่อยืด-หดขนาดกรอบครอป:"),
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
