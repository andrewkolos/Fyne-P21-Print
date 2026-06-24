package label

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/png"
)

// fileFormat is the on-disk version tag for saved label designs.
const fileFormat = "nelko-label-v1"

// elementDTO is the serializable form of every element type. The Type field
// discriminates which of the type-specific fields are meaningful.
type elementDTO struct {
	Type  string  `json:"type"` // "text" | "image" | "barcode" | "shape"
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Angle int     `json:"angle"`

	// text
	Text     string  `json:"text,omitempty"`
	FontSize float64 `json:"fontSize,omitempty"`
	Invert   bool    `json:"invert,omitempty"`

	// barcode
	BarcodeKind int    `json:"barcodeKind,omitempty"`
	Data        string `json:"data,omitempty"`

	// shape
	ShapeKind   int `json:"shapeKind,omitempty"`
	StrokeWidth int `json:"strokeWidth,omitempty"`

	// image (PNG bytes, base64-encoded by encoding/json)
	ImagePNG  []byte `json:"imagePng,omitempty"`
	Threshold uint8  `json:"threshold,omitempty"`
}

type documentDTO struct {
	Format   string       `json:"format"`
	Elements []elementDTO `json:"elements"`
}

// MarshalDocument serializes a document to JSON bytes.
func MarshalDocument(doc *Document) ([]byte, error) {
	out := documentDTO{Format: fileFormat}
	for _, el := range doc.Elements {
		b := el.Bounds()
		d := elementDTO{X: b.X, Y: b.Y, W: b.W, H: b.H, Angle: el.Angle()}
		switch e := el.(type) {
		case *TextElement:
			d.Type = "text"
			d.Text = e.Text
			d.FontSize = e.FontSize
			d.Invert = e.Invert
		case *BarcodeElement:
			d.Type = "barcode"
			d.BarcodeKind = int(e.Kind)
			d.Data = e.Data
		case *ShapeElement:
			d.Type = "shape"
			d.ShapeKind = int(e.Kind)
			d.StrokeWidth = e.StrokeWidth
		case *ImageElement:
			d.Type = "image"
			d.Threshold = e.Threshold
			d.Invert = e.Invert
			var buf bytes.Buffer
			if err := png.Encode(&buf, e.Src); err != nil {
				return nil, fmt.Errorf("encode image element: %w", err)
			}
			d.ImagePNG = buf.Bytes()
		default:
			return nil, fmt.Errorf("unknown element type %T", el)
		}
		out.Elements = append(out.Elements, d)
	}
	return json.MarshalIndent(out, "", "  ")
}

// UnmarshalDocument parses JSON bytes into a document.
func UnmarshalDocument(data []byte) (*Document, error) {
	var in documentDTO
	if err := json.Unmarshal(data, &in); err != nil {
		return nil, err
	}
	if in.Format != fileFormat {
		return nil, fmt.Errorf("unrecognized file format %q", in.Format)
	}
	doc := &Document{}
	for _, d := range in.Elements {
		var el Element
		switch d.Type {
		case "text":
			t := NewText(d.Text)
			t.FontSize = d.FontSize
			t.Invert = d.Invert
			el = t
		case "barcode":
			el = NewBarcode(BarcodeKind(d.BarcodeKind), d.Data)
		case "shape":
			s := NewShape(ShapeKind(d.ShapeKind))
			s.StrokeWidth = d.StrokeWidth
			el = s
		case "image":
			img, err := png.Decode(bytes.NewReader(d.ImagePNG))
			if err != nil {
				return nil, fmt.Errorf("decode image element: %w", err)
			}
			ie := NewImage(img)
			ie.Threshold = d.Threshold
			ie.Invert = d.Invert
			el = ie
		default:
			return nil, fmt.Errorf("unknown element type %q", d.Type)
		}
		el.SetBounds(Rect{X: d.X, Y: d.Y, W: d.W, H: d.H})
		el.SetAngle(d.Angle)
		doc.Elements = append(doc.Elements, el)
	}
	return doc, nil
}
