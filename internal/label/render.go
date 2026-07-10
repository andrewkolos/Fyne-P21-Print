package label

import (
	"image"
	"image/color"
	"image/draw"

	"nelko-print/internal/tspl"
)

type Document struct {
	Elements []Element
}

// Place returns el's raster plus the scaled top-left at which to draw it so
// the (possibly rotated) raster is centered on the element box center.
func Place(el Element, scale float64) (image.Image, float64, float64) {
	img := el.Raster(scale)
	b := el.Bounds()
	cx := (b.X + b.W/2) * scale
	cy := (b.Y + b.H/2) * scale
	x := cx - float64(img.Bounds().Dx())/2
	y := cy - float64(img.Bounds().Dy())/2
	return img, x, y
}

// Render composites all elements onto a white landscape canvas
// (w = size.PixelH, h = size.PixelW), then rotates 90 CW into the portrait
// (size.PixelW x size.PixelH) buffer the print path consumes.
func Render(doc *Document, size tspl.LabelSize) image.Image {
	landW, landH := size.PixelH, size.PixelW
	canvas := image.NewRGBA(image.Rect(0, 0, landW, landH))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	for _, el := range doc.Elements {
		img, x, y := Place(el, 1.0)
		ib := img.Bounds()
		dst := image.Rect(int(x), int(y), int(x)+ib.Dx(), int(y)+ib.Dy())
		draw.Draw(canvas, dst, img, ib.Min, draw.Over)
	}

	// Rotate 90 CW: landscape (landW x landH) -> portrait (landH x landW) =
	// (PixelW x PixelH). Compose onto white so transparent areas read white.
	rotated := RotateImage(canvas, 90)
	out := image.NewRGBA(image.Rect(0, 0, size.PixelW, size.PixelH))
	draw.Draw(out, out.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), rotated, rotated.Bounds().Min, draw.Over)
	return out
}
