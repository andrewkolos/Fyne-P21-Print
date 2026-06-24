package label

import "testing"

func TestShapeRasterDims(t *testing.T) {
	s := NewShape(FilledRect)
	s.SetBounds(Rect{0, 0, 20, 10})
	img := s.Raster(1.0)
	if img.Bounds().Dx() != 20 || img.Bounds().Dy() != 10 {
		t.Errorf("dims=%v want 20x10", img.Bounds())
	}
	// center pixel should be black-opaque for a filled rect
	r, _, _, a := img.At(10, 5).RGBA()
	if a == 0 || r != 0 {
		t.Errorf("center = r%d a%d, want black opaque", r, a)
	}
}

func TestSetAngleSnaps(t *testing.T) {
	s := NewShape(Line)
	s.SetAngle(40)
	if s.Angle() != 45 {
		t.Errorf("angle=%d want 45", s.Angle())
	}
}
