package label

import (
	"image"
	"image/color"
	"testing"
)

func TestSnapAngle(t *testing.T) {
	cases := map[int]int{0: 0, 22: 0, 23: 45, 44: 45, 90: 90, 200: 180, 360: 0, -45: 315, -1: 0}
	for in, want := range cases {
		if got := SnapAngle(in); got != want {
			t.Errorf("SnapAngle(%d)=%d want %d", in, got, want)
		}
	}
}

func TestRotateImage90SwapsDims(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 10, 4))
	got := RotateImage(src, 90)
	if got.Bounds().Dx() != 4 || got.Bounds().Dy() != 10 {
		t.Errorf("90deg rotate dims = %v, want 4x10", got.Bounds())
	}
}

func TestRotateImageZeroIsIdentity(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 3))
	if RotateImage(src, 0) != image.Image(src) {
		t.Error("0deg rotate should return same image")
	}
}

func TestScaleNearest(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1, 1))
	src.Set(0, 0, color.RGBA{0, 0, 0, 255})
	got := ScaleNearest(src, 5, 7)
	if got.Bounds().Dx() != 5 || got.Bounds().Dy() != 7 {
		t.Errorf("scaled dims = %v, want 5x7", got.Bounds())
	}
	r, _, _, a := got.At(2, 3).RGBA()
	if a == 0 || r != 0 {
		t.Errorf("scaled pixel = r%d a%d, want black opaque", r, a)
	}
}
