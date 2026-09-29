package imaging

import (
	"image"
	"strings"
	"testing"

	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/goregular"
)

// 14x40 label as rendered for text along its length (imaging.Vertical).
const labelW, labelH = 284, 96

func testFont(t *testing.T) *truetype.Font {
	t.Helper()
	f, err := truetype.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// inkBox returns the bounding box of dark pixels.
func inkBox(img image.Image) image.Rectangle {
	var box image.Rectangle
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if r, _, _, _ := img.At(x, y).RGBA(); r < 0x8000 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return box
}

func TestAutoFitStaysInsideMargins(t *testing.T) {
	for _, text := range []string{
		"A",
		"Random self-tapping screws",
		"fwefwefwefweffwefwe",
		"M3 x 10\nSHCS",
		strings.Repeat("word ", 40),
	} {
		for _, wordOnly := range []bool{true, false} {
			img, err := RenderTextWithOptions(text, labelH, labelW, TextOptions{
				Orientation: Vertical, WordBreakOnly: wordOnly})
			if err != nil {
				t.Fatal(err)
			}
			// Undo the print rotation to check in reading orientation.
			ink := inkBox(rotate90CCW(img))
			inner := image.Rect(textMarginX-1, textMarginY-1, labelW-textMarginX+1, labelH-textMarginY+1)
			if ink.Empty() || !ink.In(inner) {
				t.Errorf("%q wordOnly=%v: ink %v outside %v", text, wordOnly, ink, inner)
			}
		}
	}
}

func TestAutoFitFillsTheLabel(t *testing.T) {
	f := testFont(t)
	// A single short word is limited by the label height, so its ink should
	// use most of it.
	size := FitFontSize(f, "Hi", labelW, labelH, true)
	b := layoutText(f, "Hi", labelW, size, true)
	if b.inkH < (labelH-2*textMarginY)*9/10 {
		t.Errorf("ink height %d at %.2fpt, want most of %d", b.inkH, size, labelH-2*textMarginY)
	}
	// Longer text must come out smaller.
	if long := FitFontSize(f, "Random self-tapping screws", labelW, labelH, true); long >= size {
		t.Errorf("long text %.2fpt not smaller than short %.2fpt", long, size)
	}
}

func TestAutoFitDoesNotSplitWordsInWordOnlyMode(t *testing.T) {
	f := testFont(t)
	text := "Random self-tapping screws"
	size := FitFontSize(f, text, labelW, labelH, true)
	b := layoutText(f, text, labelW, size, true)
	for _, line := range b.lines {
		for _, w := range strings.Fields(line) {
			if !strings.Contains(text, w) || !containsWord(text, w) {
				t.Errorf("word split: line %q", line)
			}
		}
	}
}

func containsWord(text, w string) bool {
	for _, f := range strings.Fields(text) {
		if f == w {
			return true
		}
	}
	return false
}

func TestWordOnlyBreaksLongFirstWord(t *testing.T) {
	f := testFont(t)
	face := truetype.NewFace(f, &truetype.Options{Size: 24, DPI: textDPI})
	for _, line := range wrapTextWordOnly("Supercalifragilistic word", face, 86) {
		if w := measureString(face, line); w > 86 {
			t.Errorf("line %q is %dpx wide, max 86", line, w)
		}
	}
}
