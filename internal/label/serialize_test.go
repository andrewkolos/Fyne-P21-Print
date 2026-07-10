package label

import (
	"image"
	"image/color"
	"testing"
)

func TestDocumentRoundTrip(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 3))
	src.Set(1, 1, color.RGBA{0, 0, 0, 255})

	doc := &Document{}

	te := NewText("Hello")
	te.SetBounds(Rect{1, 2, 100, 40})
	te.SetAngle(90)
	te.FontSize = 18
	te.Invert = true
	doc.Elements = append(doc.Elements, te)

	bc := NewBarcode(QR, "DATA")
	bc.SetBounds(Rect{5, 6, 60, 60})
	doc.Elements = append(doc.Elements, bc)

	sh := NewShape(Rectangle)
	sh.SetBounds(Rect{0, 0, 30, 10})
	sh.StrokeWidth = 3
	doc.Elements = append(doc.Elements, sh)

	ie := NewImage(src)
	ie.SetBounds(Rect{2, 2, 20, 20})
	ie.Threshold = 100
	doc.Elements = append(doc.Elements, ie)

	data, err := MarshalDocument(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := UnmarshalDocument(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Elements) != 4 {
		t.Fatalf("element count = %d, want 4", len(got.Elements))
	}

	gt, ok := got.Elements[0].(*TextElement)
	if !ok || gt.Text != "Hello" || gt.FontSize != 18 || !gt.Invert || gt.Angle() != 90 {
		t.Errorf("text element round-trip mismatch: %+v", got.Elements[0])
	}
	if b := gt.Bounds(); b.X != 1 || b.Y != 2 || b.W != 100 || b.H != 40 {
		t.Errorf("text bounds mismatch: %+v", b)
	}
	gb, ok := got.Elements[1].(*BarcodeElement)
	if !ok || gb.Kind != QR || gb.Data != "DATA" {
		t.Errorf("barcode round-trip mismatch: %+v", got.Elements[1])
	}
	gs, ok := got.Elements[2].(*ShapeElement)
	if !ok || gs.Kind != Rectangle || gs.StrokeWidth != 3 {
		t.Errorf("shape round-trip mismatch: %+v", got.Elements[2])
	}
	gi, ok := got.Elements[3].(*ImageElement)
	if !ok || gi.Threshold != 100 {
		t.Errorf("image round-trip mismatch: %+v", got.Elements[3])
	}
	if gi.Src.Bounds().Dx() != 3 || gi.Src.Bounds().Dy() != 3 {
		t.Errorf("image src dims mismatch: %v", gi.Src.Bounds())
	}
}

func TestUnmarshalRejectsBadFormat(t *testing.T) {
	if _, err := UnmarshalDocument([]byte(`{"format":"nope","elements":[]}`)); err == nil {
		t.Error("expected error for bad format")
	}
}
