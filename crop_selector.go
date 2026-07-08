// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

const (
	handleVisual  = 10
	handleHitZone = 14
	minCropSize   = 20
)

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
	handles [8]*canvas.Rectangle

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

func (r *cropSelectorRenderer) Layout(size fyne.Size) { r.c.updateLayout(size) }
func (r *cropSelectorRenderer) MinSize() fyne.Size    { return fyne.NewSize(200, 200) }
func (r *cropSelectorRenderer) Refresh()              { canvas.Refresh(r.c) }
func (r *cropSelectorRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}
func (r *cropSelectorRenderer) Destroy() {}

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
		{X: left, Y: top},
		{X: right, Y: top},
		{X: left, Y: bottom},
		{X: right, Y: bottom},
		{X: midX, Y: top},
		{X: midX, Y: bottom},
		{X: left, Y: midY},
		{X: right, Y: midY},
	}
	for i, p := range pts {
		handlePos := fyne.NewPos(c.imgDisplayPos.X+p.X-half, c.imgDisplayPos.Y+p.Y-half)
		c.handles[i].Move(handlePos)
		c.handles[i].Refresh()
	}
}

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

func (c *cropSelector) FullRect() image.Rectangle {
	return image.Rect(0, 0, c.imgOriginalSize.X, c.imgOriginalSize.Y)
}

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
