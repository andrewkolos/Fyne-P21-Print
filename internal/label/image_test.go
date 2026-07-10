package label

import (
	"image"
	"image/color"
	"testing"
)

func TestImageElementThresholds(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.Set(x, y, color.RGBA{0, 0, 0, 255}) // all black
		}
	}
	ie := NewImage(src)
	ie.SetBounds(Rect{0, 0, 8, 8})
	img := ie.Raster(1.0)
	if img.Bounds().Dx() != 8 || img.Bounds().Dy() != 8 {
		t.Errorf("dims=%v want 8x8", img.Bounds())
	}
	r, _, _, a := img.At(4, 4).RGBA()
	if a == 0 || r != 0 {
		t.Errorf("black source pixel should stay black opaque, got r%d a%d", r, a)
	}
}
