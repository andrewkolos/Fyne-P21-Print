package imaging

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
	"unicode"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

type Orientation int

const (
	Horizontal Orientation = iota
	Vertical
)

// TextOptions configures text rendering
type TextOptions struct {
	FontSize      float64 // points; <= 0 picks the largest size that fits
	Orientation   Orientation
	Invert        bool // White text on black background
	WordBreakOnly bool // Only break lines on spaces, not mid-word
}

const (
	textDPI     = 203 // printer resolution
	textMarginX = 5   // px kept clear at each end of a line
	textMarginY = 4   // px kept clear above and below the text block
	minFontSize = 4.0
)

// RenderText creates an image from text (legacy wrapper)
func RenderText(text string, width, height int, fontSize float64, orientation Orientation) (image.Image, error) {
	return RenderTextWithOptions(text, width, height, TextOptions{
		FontSize:    fontSize,
		Orientation: orientation,
	})
}

// RenderTextWithOptions creates an image from text with full options. The
// text block is centered on its ink (not its line boxes), so auto-fitted text
// sits visually centered and uses the whole label.
func RenderTextWithOptions(text string, width, height int, opts TextOptions) (image.Image, error) {
	f, err := truetype.Parse(goregular.TTF)
	if err != nil {
		return nil, err
	}

	// For vertical, we swap dimensions for initial render, then rotate
	renderW, renderH := width, height
	if opts.Orientation == Vertical {
		renderW, renderH = height, width
	}

	size := opts.FontSize
	if size <= 0 {
		size = FitFontSize(f, text, renderW, renderH, opts.WordBreakOnly)
	}
	block := layoutText(f, text, renderW, size, opts.WordBreakOnly)

	// Set colors based on invert option
	bgColor := color.White
	fgColor := color.Black
	if opts.Invert {
		bgColor = color.Black
		fgColor = color.White
	}

	// Create background
	img := image.NewRGBA(image.Rect(0, 0, renderW, renderH))
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// Set up freetype context
	c := freetype.NewContext()
	c.SetDPI(textDPI)
	c.SetFont(f)
	c.SetFontSize(size)
	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(&image.Uniform{fgColor})
	c.SetHinting(font.HintingFull)

	y := (renderH-block.inkH)/2 - block.inkTop
	for _, line := range block.lines {
		// Center each line horizontally
		x := (renderW - measureString(block.face, line)) / 2
		c.DrawString(line, freetype.Pt(x, y))
		y += block.lineH
	}

	// Rotate if vertical
	if opts.Orientation == Vertical {
		return rotate90CW(img), nil
	}

	return img, nil
}

// textBlock is wrapped text at one font size, measured in pixels.
type textBlock struct {
	face   font.Face
	lines  []string
	lineH  int
	inkTop int // top of the ink relative to the first baseline (<= 0)
	inkH   int // ink height of the whole block
	maxW   int // widest line advance
}

func layoutText(f *truetype.Font, text string, renderW int, size float64, wordOnly bool) textBlock {
	face := truetype.NewFace(f, &truetype.Options{Size: size, DPI: textDPI, Hinting: font.HintingFull})
	b := textBlock{face: face, lineH: face.Metrics().Height.Ceil()}
	if wordOnly {
		b.lines = wrapTextWordOnly(text, face, renderW-2*textMarginX)
	} else {
		b.lines = wrapText(text, face, renderW-2*textMarginX)
	}

	top, bottom, inked := 0, 0, false
	for i, line := range b.lines {
		if w := measureString(face, line); w > b.maxW {
			b.maxW = w
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		bounds, _ := font.BoundString(face, line)
		lt, lb := i*b.lineH+bounds.Min.Y.Floor(), i*b.lineH+bounds.Max.Y.Ceil()
		if !inked || lt < top {
			top = lt
		}
		if !inked || lb > bottom {
			bottom = lb
		}
		inked = true
	}
	b.inkTop, b.inkH = top, bottom-top
	return b
}

// fits reports whether the block fits inside renderW x renderH with margins.
// In word-only mode a word that had to be split does not count as fitting,
// so auto-sizing shrinks the text instead of breaking words.
func (b textBlock) fits(text string, renderW, renderH int, wordOnly bool) bool {
	maxW := renderW - 2*textMarginX
	if b.maxW > maxW || b.inkH > renderH-2*textMarginY {
		return false
	}
	if wordOnly {
		for _, word := range strings.Fields(text) {
			if measureString(b.face, word) > maxW {
				return false
			}
		}
	}
	return true
}

// FitFontSize returns the largest font size (points, 0.25 pt steps) at which
// text fits the renderW x renderH area, or minFontSize if nothing fits.
func FitFontSize(f *truetype.Font, text string, renderW, renderH int, wordOnly bool) float64 {
	fitsAt := func(size float64) bool {
		return layoutText(f, text, renderW, size, wordOnly).fits(text, renderW, renderH, wordOnly)
	}
	// Ink of a single line is at most ~1.2 em tall, so this bounds the search.
	lo, hi := minFontSize, float64(renderH)*72/textDPI*1.25
	if !fitsAt(lo) {
		return lo
	}
	for hi-lo > 0.25 {
		mid := (lo + hi) / 2
		if fitsAt(mid) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// wrapText splits text into lines that fit within maxWidth (breaks anywhere)
func wrapText(text string, face font.Face, maxWidth int) []string {
	var lines []string
	var currentLine string

	for _, char := range text {
		if char == '\n' {
			lines = append(lines, currentLine)
			currentLine = ""
			continue
		}
		testLine := currentLine + string(char)
		if measureString(face, testLine) > maxWidth && currentLine != "" {
			lines = append(lines, currentLine)
			currentLine = string(char)
		} else {
			currentLine = testLine
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

// wrapTextWordOnly splits text into lines, only breaking at word boundaries
func wrapTextWordOnly(text string, face font.Face, maxWidth int) []string {
	var lines []string

	// First split by explicit newlines
	paragraphs := strings.Split(text, "\n")

	for _, para := range paragraphs {
		words := strings.Fields(para)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		currentLine := words[0]
		if measureString(face, currentLine) > maxWidth {
			currentLine = breakLongWord(currentLine, face, maxWidth, &lines)
		}
		for i := 1; i < len(words); i++ {
			word := words[i]
			testLine := currentLine + " " + word

			if measureString(face, testLine) > maxWidth {
				// Current line is full, start new line
				lines = append(lines, currentLine)

				// Check if single word is too long
				if measureString(face, word) > maxWidth {
					// Word itself is too long, we need to break it
					currentLine = breakLongWord(word, face, maxWidth, &lines)
				} else {
					currentLine = word
				}
			} else {
				currentLine = testLine
			}
		}

		if currentLine != "" {
			lines = append(lines, currentLine)
		}
	}

	return lines
}

// breakLongWord breaks a single word that's too long to fit
func breakLongWord(word string, face font.Face, maxWidth int, lines *[]string) string {
	var currentPart string
	for _, char := range word {
		testPart := currentPart + string(char)
		if measureString(face, testPart) > maxWidth && currentPart != "" {
			*lines = append(*lines, currentPart)
			currentPart = string(char)
		} else {
			currentPart = testPart
		}
	}
	return currentPart
}

// measureString returns the width of a string in pixels
func measureString(face font.Face, s string) int {
	var width fixed.Int26_6
	for _, r := range s {
		adv, ok := face.GlyphAdvance(r)
		if ok {
			width += adv
		}
	}
	return width.Ceil()
}

// rotate90CW rotates an image 90 degrees clockwise
func rotate90CW(src image.Image) image.Image {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, h, w))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(h-1-y, x, src.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}

	return dst
}

// rotate90CCW rotates an image 90 degrees counter-clockwise
func rotate90CCW(src image.Image) image.Image {
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, h, w))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(y, w-1-x, src.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}

	return dst
}

// RotatePreviewForDisplay rotates a vertical-orientation image for on-screen display
// so the text reads correctly (counter-clockwise)
func RotatePreviewForDisplay(img image.Image) image.Image {
	return rotate90CCW(img)
}

// IsWhitespace checks if a rune is whitespace
func IsWhitespace(r rune) bool {
	return unicode.IsSpace(r)
}

// MeasureString returns the pixel width of s rendered with face.
func MeasureString(face font.Face, s string) int { return measureString(face, s) }

// WrapText wraps text to maxWidth, breaking anywhere.
func WrapText(text string, face font.Face, maxWidth int) []string {
	return wrapText(text, face, maxWidth)
}

// WrapWordOnly wraps text to maxWidth, breaking only at spaces.
func WrapWordOnly(text string, face font.Face, maxWidth int) []string {
	return wrapTextWordOnly(text, face, maxWidth)
}
