package label

import "testing"

func TestBarcodeEncodesAllKinds(t *testing.T) {
	for _, k := range []BarcodeKind{Aztec, QR, DataMatrix, Code128} {
		b := NewBarcode(k, "HELLO123")
		if _, err := b.Encoded(); err != nil {
			t.Errorf("kind %d failed to encode: %v", k, err)
		}
	}
}

func TestBarcodeRasterDims(t *testing.T) {
	b := NewBarcode(Aztec, "X")
	b.SetBounds(Rect{0, 0, 60, 60})
	img := b.Raster(1.0)
	if img.Bounds().Dx() != 60 || img.Bounds().Dy() != 60 {
		t.Errorf("dims=%v want 60x60", img.Bounds())
	}
}

func TestBarcodeEmptyDataNoPanic(t *testing.T) {
	b := NewBarcode(QR, "")
	b.SetBounds(Rect{0, 0, 40, 40})
	_ = b.Raster(1.0) // must not panic; placeholder allowed
}
