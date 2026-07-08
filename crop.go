// Copyright (c) 2026 Nawakarit
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License v3.0.
package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

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
