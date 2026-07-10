package label

import (
	"testing"

	"nelko-print/internal/tspl"
)

func TestRenderDims(t *testing.T) {
	doc := &Document{}
	img := Render(doc, tspl.Label14x40)
	if img.Bounds().Dx() != tspl.Label14x40.PixelW || img.Bounds().Dy() != tspl.Label14x40.PixelH {
		t.Errorf("dims=%v want %dx%d", img.Bounds(), tspl.Label14x40.PixelW, tspl.Label14x40.PixelH)
	}
}

func TestRenderEmptyIsWhite(t *testing.T) {
	img := Render(&Document{}, tspl.Label14x40)
	r, g, b, _ := img.At(10, 10).RGBA()
	if r < 0xf000 || g < 0xf000 || b < 0xf000 {
		t.Errorf("empty label pixel not white: %d,%d,%d", r, g, b)
	}
}

func TestRenderPlacesDarkPixels(t *testing.T) {
	doc := &Document{}
	s := NewShape(FilledRect)
	s.SetBounds(Rect{0, 0, 50, 50}) // top-left of landscape canvas
	doc.Elements = []Element{s}
	img := Render(doc, tspl.Label14x40)
	dark := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			if r < 0x4000 {
				dark++
			}
		}
	}
	if dark == 0 {
		t.Error("expected dark pixels from filled rect, found none")
	}
}

func TestPlaceCentersRotated(t *testing.T) {
	s := NewShape(FilledRect)
	s.SetBounds(Rect{10, 10, 20, 20})
	s.SetAngle(45)
	img, x, y := Place(s, 1.0)
	// rotated 20x20 box bounding box is ~29x29, centered on (20,20)
	cx := x + float64(img.Bounds().Dx())/2
	cy := y + float64(img.Bounds().Dy())/2
	if cx < 18 || cx > 22 || cy < 18 || cy > 22 {
		t.Errorf("center=(%.1f,%.1f) want ~(20,20)", cx, cy)
	}
}
