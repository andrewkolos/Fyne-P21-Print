package label

import (
	"image"
	"math"
)

// Rect is a position+size box in label-space pixels (landscape).
type Rect struct{ X, Y, W, H float64 }

// SnapAngle normalizes deg to [0,360) and rounds to the nearest multiple of 45.
func SnapAngle(deg int) int {
	d := ((deg % 360) + 360) % 360
	snapped := int(math.Round(float64(d)/45.0)) * 45
	return snapped % 360
}

// RotateImage rotates src about its center by deg degrees (any value),
// using nearest-neighbor sampling and a transparent background. The result
// is an RGBA image sized to the rotated bounding box. deg==0 returns src.
func RotateImage(src image.Image, deg int) image.Image {
	if ((deg%360)+360)%360 == 0 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	theta := float64(deg) * math.Pi / 180.0
	sin, cos := math.Sin(theta), math.Cos(theta)
	// Kill floating-point noise so multiples of 90 give exact integer dims.
	if math.Abs(sin) < 1e-12 {
		sin = 0
	}
	if math.Abs(cos) < 1e-12 {
		cos = 0
	}
	nw := int(math.Ceil(math.Abs(float64(w)*cos) + math.Abs(float64(h)*sin)))
	nh := int(math.Ceil(math.Abs(float64(w)*sin) + math.Abs(float64(h)*cos)))
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	cx, cy := float64(w)/2, float64(h)/2
	ncx, ncy := float64(nw)/2, float64(nh)/2
	for y := 0; y < nh; y++ {
		for x := 0; x < nw; x++ {
			dx := float64(x) - ncx
			dy := float64(y) - ncy
			// inverse rotation: dst -> src
			sx := cos*dx + sin*dy + cx
			sy := -sin*dx + cos*dy + cy
			ix, iy := int(math.Round(sx)), int(math.Round(sy))
			if ix >= 0 && ix < w && iy >= 0 && iy < h {
				dst.Set(x, y, src.At(b.Min.X+ix, b.Min.Y+iy))
			}
		}
	}
	return dst
}

// ScaleNearest scales src to exactly w x h using nearest-neighbor.
func ScaleNearest(src image.Image, w, h int) image.Image {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		sy := y * sh / h
		for x := 0; x < w; x++ {
			sx := x * sw / w
			dst.Set(x, y, src.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return dst
}
