package label

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/aztec"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/datamatrix"
	"github.com/boombuler/barcode/qr"
)

type BarcodeKind int

const (
	Aztec BarcodeKind = iota
	QR
	DataMatrix
	Code128
)

type BarcodeElement struct {
	Base
	Kind BarcodeKind
	Data string
}

func NewBarcode(kind BarcodeKind, data string) *BarcodeElement {
	return &BarcodeElement{
		Base: Base{B: Rect{0, 0, 80, 80}},
		Kind: kind,
		Data: data,
	}
}

// Encoded returns the raw (unscaled) barcode (which is also an image.Image).
func (b *BarcodeElement) Encoded() (barcode.Barcode, error) {
	if b.Data == "" {
		return nil, errors.New("empty barcode data")
	}
	switch b.Kind {
	case Aztec:
		return aztec.Encode([]byte(b.Data), 23, 0)
	case QR:
		return qr.Encode(b.Data, qr.M, qr.Auto)
	case DataMatrix:
		return datamatrix.Encode(b.Data)
	case Code128:
		return code128.Encode(b.Data)
	}
	return nil, errors.New("unknown barcode kind")
}

func (b *BarcodeElement) Raster(scale float64) image.Image {
	w := int(math.Round(b.B.W * scale))
	h := int(math.Round(b.B.H * scale))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	enc, err := b.Encoded()
	if err != nil {
		return RenderRotated(placeholder(w, h), b.A)
	}
	// Scale up the encoded code so module edges stay crisp, then fit to box.
	scaled, serr := barcode.Scale(enc, max(w, enc.Bounds().Dx()), max(h, enc.Bounds().Dy()))
	if serr != nil {
		scaled = enc
	}
	fitted := ScaleNearest(scaled, w, h)
	// barcode lib draws black-on-white; convert white -> transparent.
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := fitted.At(x, y).RGBA()
			if (r+g+bl)/3 < 0x8000 {
				out.Set(x, y, color.RGBA{0, 0, 0, 255})
			}
		}
	}
	return RenderRotated(out, b.A)
}

func placeholder(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	black := &image.Uniform{color.RGBA{0, 0, 0, 255}}
	// hollow box border to signal "encode error"
	draw.Draw(img, image.Rect(0, 0, w, 2), black, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, h-2, w, h), black, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(0, 0, 2, h), black, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(w-2, 0, w, h), black, image.Point{}, draw.Src)
	return img
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
