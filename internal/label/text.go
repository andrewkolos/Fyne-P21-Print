package label

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"

	"nelko-print/internal/imaging"
)

type TextElement struct {
	Base
	Text     string
	FontSize float64
	Invert   bool
}

func NewText(text string) *TextElement {
	return &TextElement{Text: text, FontSize: 24}
}

func (t *TextElement) Raster(scale float64) image.Image {
	w := int(math.Round(t.B.W * scale))
	h := int(math.Round(t.B.H * scale))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Transparent background unless inverted (then opaque black box).
	if t.Invert {
		draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0, 0, 0, 255}}, image.Point{}, draw.Src)
	}
	if t.Text == "" {
		return RenderRotated(img, t.A)
	}

	f, err := truetype.Parse(goregular.TTF)
	if err != nil {
		return RenderRotated(img, t.A)
	}
	size := t.FontSize * scale
	fg := color.RGBA{0, 0, 0, 255}
	if t.Invert {
		fg = color.RGBA{255, 255, 255, 255}
	}
	c := freetype.NewContext()
	c.SetDPI(203)
	c.SetFont(f)
	c.SetFontSize(size)
	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(&image.Uniform{fg})
	c.SetHinting(font.HintingFull)

	face := truetype.NewFace(f, &truetype.Options{Size: size, DPI: 203})
	metrics := face.Metrics()
	lineH := metrics.Height.Ceil()
	lines := imaging.WrapWordOnly(t.Text, face, w-2)
	y := metrics.Ascent.Ceil()
	for _, line := range lines {
		lw := imaging.MeasureString(face, line)
		x := (w - lw) / 2
		if x < 0 {
			x = 0
		}
		c.DrawString(line, freetype.Pt(x, y))
		y += lineH
	}
	return RenderRotated(img, t.A)
}
