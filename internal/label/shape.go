package label

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

type ShapeKind int

const (
	Line ShapeKind = iota
	Rectangle
	FilledRect
)

type ShapeElement struct {
	Base
	Kind        ShapeKind
	StrokeWidth int
}

func NewShape(kind ShapeKind) *ShapeElement {
	return &ShapeElement{Kind: kind, StrokeWidth: 2}
}

func (s *ShapeElement) Raster(scale float64) image.Image {
	w := int(math.Round(s.B.W * scale))
	h := int(math.Round(s.B.H * scale))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	sw := int(math.Round(float64(s.StrokeWidth) * scale))
	if sw < 1 {
		sw = 1
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	black := color.RGBA{0, 0, 0, 255}
	fill := func(x0, y0, x1, y1 int) {
		if x1 > w {
			x1 = w
		}
		if y1 > h {
			y1 = h
		}
		draw.Draw(img, image.Rect(x0, y0, x1, y1), &image.Uniform{black}, image.Point{}, draw.Src)
	}
	switch s.Kind {
	case Line:
		fill(0, (h-sw)/2, w, (h-sw)/2+sw) // horizontal line, centered
	case Rectangle:
		fill(0, 0, w, sw)   // top
		fill(0, h-sw, w, h) // bottom
		fill(0, 0, sw, h)   // left
		fill(w-sw, 0, w, h) // right
	case FilledRect:
		fill(0, 0, w, h)
	}
	return RenderRotated(img, s.A)
}
