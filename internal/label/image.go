package label

import (
	"image"
	"image/color"
	"math"
)

type ImageElement struct {
	Base
	Src       image.Image
	Threshold uint8
	Invert    bool
}

func NewImage(src image.Image) *ImageElement {
	b := src.Bounds()
	return &ImageElement{
		Base:      Base{B: Rect{0, 0, float64(b.Dx()), float64(b.Dy())}},
		Src:       src,
		Threshold: 128,
	}
}

func gray(c color.Color) uint8 {
	r, g, b, _ := c.RGBA()
	return uint8((0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256)
}

func (e *ImageElement) Raster(scale float64) image.Image {
	w := int(math.Round(e.B.W * scale))
	h := int(math.Round(e.B.H * scale))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	scaled := ScaleNearest(e.Src, w, h)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dark := gray(scaled.At(x, y)) < e.Threshold
			if e.Invert {
				dark = !dark
			}
			if dark {
				out.Set(x, y, color.RGBA{0, 0, 0, 255})
			}
			// else leave transparent
		}
	}
	return RenderRotated(out, e.A)
}
