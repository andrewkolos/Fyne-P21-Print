package label

import "testing"

func TestTextRasterNonEmpty(t *testing.T) {
	te := NewText("Hi")
	te.SetBounds(Rect{0, 0, 120, 40})
	img := te.Raster(1.0)
	// expect some dark pixels (the glyphs)
	dark := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, _, _, a := img.At(x, y).RGBA()
			if a > 0 && r == 0 {
				dark++
			}
		}
	}
	if dark == 0 {
		t.Error("expected dark glyph pixels, got none")
	}
}

func TestTextRotatedDimsChange(t *testing.T) {
	te := NewText("Hello")
	te.SetBounds(Rect{0, 0, 100, 30})
	flat := te.Raster(1.0).Bounds()
	te.SetAngle(90)
	rot := te.Raster(1.0).Bounds()
	if flat.Dx() == rot.Dx() && flat.Dy() == rot.Dy() {
		t.Error("expected rotated raster to swap dims")
	}
}
