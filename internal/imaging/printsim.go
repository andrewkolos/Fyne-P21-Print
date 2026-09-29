package imaging

import (
	"image"
	"image/color"
	"math"

	xdraw "golang.org/x/image/draw"
)

const (
	dotsPerMM = 203 / 25.4 // printer resolution

	// A heated thermal dot darkens a little past its 0.125 mm cell and
	// thermal black is deep grey rather than pure black.
	dotSpreadSigma = 0.45 // dots
	inkGray        = 30
	cornerRadiusMM = 1.0 // die-cut label corners
)

// SimulatePrint renders a dot image (one pixel per printer dot, dark = ink)
// as it looks printed: dots spread slightly, the print area is centred on a
// paperWmm x paperHmm label with rounded corners, and the result is resampled
// to pxPerMM screen pixels so it can be shown at actual size. Downsampling
// averages the dots, which gives the soft, anti-aliased look of real prints.
func SimulatePrint(dots image.Image, paperWmm, paperHmm, pxPerMM float64) image.Image {
	db := dots.Bounds()
	pw := max(int(math.Round(paperWmm*dotsPerMM)), db.Dx())
	ph := max(int(math.Round(paperHmm*dotsPerMM)), db.Dy())

	// Ink coverage on the whole label at dot resolution, print area centred.
	ink := make([]float64, pw*ph)
	ox, oy := (pw-db.Dx())/2, (ph-db.Dy())/2
	for y := 0; y < db.Dy(); y++ {
		for x := 0; x < db.Dx(); x++ {
			if color.GrayModel.Convert(dots.At(db.Min.X+x, db.Min.Y+y)).(color.Gray).Y < 128 {
				ink[(oy+y)*pw+ox+x] = 1
			}
		}
	}
	ink = gaussianBlur(ink, pw, ph, dotSpreadSigma)

	paper := image.NewGray(image.Rect(0, 0, pw, ph))
	for i, v := range ink {
		paper.Pix[i] = uint8(255 - math.Min(v, 1)*(255-inkGray))
	}

	ow := max(int(math.Round(paperWmm*pxPerMM)), 1)
	oh := max(int(math.Round(paperHmm*pxPerMM)), 1)
	out := image.NewRGBA(image.Rect(0, 0, ow, oh))
	xdraw.CatmullRom.Scale(out, out.Bounds(), paper, paper.Bounds(), xdraw.Src, nil)
	roundCorners(out, cornerRadiusMM*pxPerMM)
	return out
}

// gaussianBlur blurs a w x h float image with a separable kernel.
func gaussianBlur(src []float64, w, h int, sigma float64) []float64 {
	r := int(math.Ceil(sigma * 3))
	k := make([]float64, 2*r+1)
	var sum float64
	for i := range k {
		d := float64(i - r)
		k[i] = math.Exp(-d * d / (2 * sigma * sigma))
		sum += k[i]
	}
	for i := range k {
		k[i] /= sum
	}

	tmp := make([]float64, len(src))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var v float64
			for i, kv := range k {
				if xx := x + i - r; xx >= 0 && xx < w {
					v += src[y*w+xx] * kv
				}
			}
			tmp[y*w+x] = v
		}
	}
	dst := make([]float64, len(src))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var v float64
			for i, kv := range k {
				if yy := y + i - r; yy >= 0 && yy < h {
					v += tmp[yy*w+x] * kv
				}
			}
			dst[y*w+x] = v
		}
	}
	return dst
}

// roundCorners makes the corners outside a radius-r arc transparent, with an
// anti-aliased edge. img is premultiplied, so all channels are scaled.
func roundCorners(img *image.RGBA, r float64) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	ri := int(math.Ceil(r))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if (x >= ri && x < w-ri) || (y >= ri && y < h-ri) {
				continue
			}
			cx, cy := math.Max(r, math.Min(float64(x)+0.5, float64(w)-r)), math.Max(r, math.Min(float64(y)+0.5, float64(h)-r))
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			a := math.Max(0, math.Min(1, r-d+0.5))
			if a >= 1 {
				continue
			}
			i := img.PixOffset(b.Min.X+x, b.Min.Y+y)
			for c := 0; c < 4; c++ {
				img.Pix[i+c] = uint8(float64(img.Pix[i+c]) * a)
			}
		}
	}
}
