# Designer Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a third "Designer" tab: a WYSIWYG label editor for placing, resizing, and 45-degree-snap rotating text, B/W graphics, barcodes, and shapes, composited into the existing print pipeline.

**Architecture:** A UI-free `internal/label` package holds the element model + renderer. Elements render themselves black-on-transparent, rotated by a 45-degree-snapped angle. `Render` composites them onto a white landscape canvas (long axis horizontal), rotates 90 degrees to the printer's portrait `96 x PixelH` buffer, and the result feeds the existing `imaging.ToMonochrome -> tspl.BuildPrintJob -> printer.Print` path unchanged. A custom Fyne `labelCanvas` widget provides drag/resize/rotate editing; a property strip edits the selected element.

**Tech Stack:** Go 1.22, Fyne v2.4.4, freetype + goregular (existing), `github.com/boombuler/barcode` v1.1.0 (new, pure Go).

## Global Constraints

- Go 1.22; module path `nelko-print`.
- No CGO-only dependencies; builds must keep working on Linux/Windows/macOS.
- Printer buffer is always portrait `PixelW = 96` wide x `PixelH` tall. The Designer must produce an `image.Image` consumable by `imaging.ToMonochrome(img, 96, PixelH, threshold, !invert)`.
- Rotation angles are always a multiple of 45 (0,45,...,315).
- Label-space coordinates are landscape: x grows along the long axis (width = `PixelH`), y along the short axis (height = `PixelW`).
- Do not modify the Image/Text tabs, connection code, or `internal/printer`.
- New dependency line must be `github.com/boombuler/barcode v1.1.0`.

---

### Task 1: Add dependency + geometry foundations

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/label/geometry.go`
- Test: `internal/label/geometry_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Rect struct{ X, Y, W, H float64 }`
  - `func SnapAngle(deg int) int` — normalizes to `[0,360)` then rounds to nearest 45.
  - `func RotateImage(src image.Image, deg int) image.Image` — nearest-neighbor rotation about center, transparent fill, returns RGBA sized to the rotated bounding box. `deg==0` returns `src` unchanged.
  - `func ScaleNearest(src image.Image, w, h int) image.Image` — nearest-neighbor scale to exactly `w x h` RGBA (w,h >= 1).

- [ ] **Step 1: Add the dependency**

Run:
```bash
go get github.com/boombuler/barcode@v1.1.0
```
Expected: `go.mod` gains `github.com/boombuler/barcode v1.1.0`.

- [ ] **Step 2: Write the failing test**

Create `internal/label/geometry_test.go`:
```go
package label

import (
	"image"
	"image/color"
	"testing"
)

func TestSnapAngle(t *testing.T) {
	cases := map[int]int{0: 0, 22: 45, 23: 45, 44: 45, 90: 90, 200: 180, 360: 0, -45: 315, -1: 0}
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/label/...`
Expected: FAIL (package/functions undefined).

- [ ] **Step 4: Implement geometry.go**

Create `internal/label/geometry.go`:
```go
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
	if ((deg % 360) + 360) % 360 == 0 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	theta := float64(deg) * math.Pi / 180.0
	sin, cos := math.Sin(theta), math.Cos(theta)
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
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/label/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/label/geometry.go internal/label/geometry_test.go
git commit -m "feat(label): geometry helpers + barcode dependency"
```

---

### Task 2: Element interface, base element, and shapes

**Files:**
- Create: `internal/label/element.go`, `internal/label/shape.go`
- Test: `internal/label/shape_test.go`

**Interfaces:**
- Consumes: `Rect`, `RotateImage` (Task 1).
- Produces:
  - `type Element interface { Bounds() Rect; SetBounds(Rect); Angle() int; SetAngle(int); Raster(scale float64) image.Image }`
  - `type Base struct{ B Rect; A int }` implementing `Bounds/SetBounds/Angle/SetAngle` (SetAngle calls `SnapAngle`).
  - `func RenderRotated(content image.Image, angle int) image.Image` — convenience used by all elements: returns `RotateImage(content, angle)`.
  - `type ShapeKind int` with `Line, Rectangle, FilledRect`.
  - `type ShapeElement struct{ Base; Kind ShapeKind; StrokeWidth int }`
  - `func NewShape(kind ShapeKind) *ShapeElement`

- [ ] **Step 1: Write the failing test**

Create `internal/label/shape_test.go`:
```go
package label

import "testing"

func hasDark(img interface{ At(int, int) interface{ RGBA() (uint32, uint32, uint32, uint32) } }) bool {
	return false // replaced below
}

func TestShapeRasterDims(t *testing.T) {
	s := NewShape(FilledRect)
	s.SetBounds(Rect{0, 0, 20, 10})
	img := s.Raster(1.0)
	if img.Bounds().Dx() != 20 || img.Bounds().Dy() != 10 {
		t.Errorf("dims=%v want 20x10", img.Bounds())
	}
	// center pixel should be black-opaque for a filled rect
	r, _, _, a := img.At(10, 5).RGBA()
	if a == 0 || r != 0 {
		t.Errorf("center = r%d a%d, want black opaque", r, a)
	}
}

func TestSetAngleSnaps(t *testing.T) {
	s := NewShape(Line)
	s.SetAngle(40)
	if s.Angle() != 45 {
		t.Errorf("angle=%d want 45", s.Angle())
	}
}
```
(Delete the unused `hasDark` stub if it triggers a vet/compile error — it is illustrative; keep only the two test functions.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/label/...`
Expected: FAIL (Element/ShapeElement undefined).

- [ ] **Step 3: Implement element.go**

Create `internal/label/element.go`:
```go
package label

import "image"

// Element is one item placed on the label.
type Element interface {
	Bounds() Rect
	SetBounds(Rect)
	Angle() int
	SetAngle(int)
	// Raster renders the element black-on-transparent at scale (1.0 = label
	// resolution), already rotated by Angle. Returned dims may exceed the
	// element box when rotated.
	Raster(scale float64) image.Image
}

// Base provides bounds + snapped angle storage for elements.
type Base struct {
	B Rect
	A int
}

func (b *Base) Bounds() Rect     { return b.B }
func (b *Base) SetBounds(r Rect) { b.B = r }
func (b *Base) Angle() int       { return b.A }
func (b *Base) SetAngle(d int)   { b.A = SnapAngle(d) }

// RenderRotated rotates rendered content by angle (helper for elements).
func RenderRotated(content image.Image, angle int) image.Image {
	return RotateImage(content, angle)
}
```

- [ ] **Step 4: Implement shape.go**

Create `internal/label/shape.go`:
```go
package label

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

type ShapeKind int

const (
	Line ShapeKind = iota
	Rectangle
	FilledRect
)

type ShapeElement struct {
	Base
	Kind        ShapeKind
	StrokeWidth int
}

func NewShape(kind ShapeKind) *ShapeElement {
	return &ShapeElement{Kind: kind, StrokeWidth: 2}
}

func (s *ShapeElement) Raster(scale float64) image.Image {
	w := int(math.Round(s.B.W * scale))
	h := int(math.Round(s.B.H * scale))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	sw := int(math.Round(float64(s.StrokeWidth) * scale))
	if sw < 1 {
		sw = 1
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	black := color.RGBA{0, 0, 0, 255}
	fill := func(x0, y0, x1, y1 int) {
		if x1 > w {
			x1 = w
		}
		if y1 > h {
			y1 = h
		}
		draw.Draw(img, image.Rect(x0, y0, x1, y1), &image.Uniform{black}, image.Point{}, draw.Src)
	}
	switch s.Kind {
	case Line:
		fill(0, (h-sw)/2, w, (h-sw)/2+sw) // horizontal line, centered
	case Rectangle:
		fill(0, 0, w, sw)         // top
		fill(0, h-sw, w, h)       // bottom
		fill(0, 0, sw, h)         // left
		fill(w-sw, 0, w, h)       // right
	case FilledRect:
		fill(0, 0, w, h)
	}
	return RenderRotated(img, s.A)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/label/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/label/element.go internal/label/shape.go internal/label/shape_test.go
git commit -m "feat(label): element interface, base, and shapes"
```

---

### Task 3: Text element (share wrap helpers)

**Files:**
- Modify: `internal/imaging/text.go` (export wrap/measure helpers)
- Create: `internal/label/text.go`
- Test: `internal/label/text_test.go`

**Interfaces:**
- Consumes: `Base`, `RenderRotated` (Task 2).
- Produces:
  - In `imaging`: `func MeasureString(face font.Face, s string) int`, `func WrapText(text string, face font.Face, maxWidth int) []string`, `func WrapWordOnly(text string, face font.Face, maxWidth int) []string` (exported wrappers calling the existing private functions).
  - `type TextElement struct{ Base; Text string; FontSize float64; Invert bool }`
  - `func NewText(text string) *TextElement` (defaults FontSize 24)

- [ ] **Step 1: Export the wrap helpers (no behavior change)**

In `internal/imaging/text.go`, add at the end:
```go
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
```

- [ ] **Step 2: Write the failing test**

Create `internal/label/text_test.go`:
```go
package label

import "testing"

func TestTextRasterNonEmpty(t *testing.T) {
	te := NewText("Hi")
	te.SetBounds(Rect{0, 0, 120, 40})
	img := te.Raster(1.0)
	// expect some dark pixels (the glyphs)
	dark := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, _, _, a := img.At(x, y).RGBA()
			if a > 0 && r == 0 {
				dark++
			}
		}
	}
	if dark == 0 {
		t.Error("expected dark glyph pixels, got none")
	}
}

func TestTextRotatedDimsChange(t *testing.T) {
	te := NewText("Hello")
	te.SetBounds(Rect{0, 0, 100, 30})
	flat := te.Raster(1.0).Bounds()
	te.SetAngle(90)
	rot := te.Raster(1.0).Bounds()
	if flat.Dx() == rot.Dx() && flat.Dy() == rot.Dy() {
		t.Error("expected rotated raster to swap dims")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/label/...`
Expected: FAIL (TextElement undefined).

- [ ] **Step 4: Implement text.go**

Create `internal/label/text.go`:
```go
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
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/label/... ./internal/imaging/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/imaging/text.go internal/label/text.go internal/label/text_test.go
git commit -m "feat(label): text element; export imaging wrap helpers"
```

---

### Task 4: Image element

**Files:**
- Create: `internal/label/image.go`
- Test: `internal/label/image_test.go`

**Interfaces:**
- Consumes: `Base`, `ScaleNearest`, `RenderRotated`.
- Produces:
  - `type ImageElement struct{ Base; Src image.Image; Threshold uint8; Invert bool }`
  - `func NewImage(src image.Image) *ImageElement` (defaults Threshold 128, sets B.W/B.H to src dims clamped)

- [ ] **Step 1: Write the failing test**

Create `internal/label/image_test.go`:
```go
package label

import (
	"image"
	"image/color"
	"testing"
)

func TestImageElementThresholds(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.Set(x, y, color.RGBA{0, 0, 0, 255}) // all black
		}
	}
	ie := NewImage(src)
	ie.SetBounds(Rect{0, 0, 8, 8})
	img := ie.Raster(1.0)
	if img.Bounds().Dx() != 8 || img.Bounds().Dy() != 8 {
		t.Errorf("dims=%v want 8x8", img.Bounds())
	}
	r, _, _, a := img.At(4, 4).RGBA()
	if a == 0 || r != 0 {
		t.Errorf("black source pixel should stay black opaque, got r%d a%d", r, a)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/label/...`
Expected: FAIL (ImageElement undefined).

- [ ] **Step 3: Implement image.go**

Create `internal/label/image.go`:
```go
package label

import (
	"image"
	"image/color"
	"math"
)

type ImageElement struct {
	Base
	Src       image.Image
	Threshold uint8
	Invert    bool
}

func NewImage(src image.Image) *ImageElement {
	b := src.Bounds()
	return &ImageElement{
		Base:      Base{B: Rect{0, 0, float64(b.Dx()), float64(b.Dy())}},
		Src:       src,
		Threshold: 128,
	}
}

func gray(c color.Color) uint8 {
	r, g, b, _ := c.RGBA()
	return uint8((0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256)
}

func (e *ImageElement) Raster(scale float64) image.Image {
	w := int(math.Round(e.B.W * scale))
	h := int(math.Round(e.B.H * scale))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	scaled := ScaleNearest(e.Src, w, h)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dark := gray(scaled.At(x, y)) < e.Threshold
			if e.Invert {
				dark = !dark
			}
			if dark {
				out.Set(x, y, color.RGBA{0, 0, 0, 255})
			}
			// else leave transparent
		}
	}
	return RenderRotated(out, e.A)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/label/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/label/image.go internal/label/image_test.go
git commit -m "feat(label): image element with threshold/invert"
```

---

### Task 5: Barcode element

**Files:**
- Create: `internal/label/barcode.go`
- Test: `internal/label/barcode_test.go`

**Interfaces:**
- Consumes: `Base`, `ScaleNearest`, `RenderRotated`.
- Produces:
  - `type BarcodeKind int` with `Aztec, QR, DataMatrix, Code128`
  - `type BarcodeElement struct{ Base; Kind BarcodeKind; Data string }`
  - `func NewBarcode(kind BarcodeKind, data string) *BarcodeElement` (defaults box 80x80)
  - `func (b *BarcodeElement) Encoded() (image.Image, error)` — raw barcode image (square for 2D), or error.

- [ ] **Step 1: Write the failing test**

Create `internal/label/barcode_test.go`:
```go
package label

import "testing"

func TestBarcodeEncodesAllKinds(t *testing.T) {
	for _, k := range []BarcodeKind{Aztec, QR, DataMatrix, Code128} {
		b := NewBarcode(k, "HELLO123")
		if _, err := b.Encoded(); err != nil {
			t.Errorf("kind %d failed to encode: %v", k, err)
		}
	}
}

func TestBarcodeRasterDims(t *testing.T) {
	b := NewBarcode(Aztec, "X")
	b.SetBounds(Rect{0, 0, 60, 60})
	img := b.Raster(1.0)
	if img.Bounds().Dx() != 60 || img.Bounds().Dy() != 60 {
		t.Errorf("dims=%v want 60x60", img.Bounds())
	}
}

func TestBarcodeEmptyDataNoPanic(t *testing.T) {
	b := NewBarcode(QR, "")
	b.SetBounds(Rect{0, 0, 40, 40})
	_ = b.Raster(1.0) // must not panic; placeholder allowed
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/label/...`
Expected: FAIL (BarcodeElement undefined).

- [ ] **Step 3: Implement barcode.go**

Create `internal/label/barcode.go`:
```go
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

// Encoded returns the raw (unscaled) barcode image.
func (b *BarcodeElement) Encoded() (image.Image, error) {
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/label/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/label/barcode.go internal/label/barcode_test.go
git commit -m "feat(label): barcode element (Aztec/QR/DataMatrix/Code128)"
```

---

### Task 6: Document + renderer + element placement

**Files:**
- Create: `internal/label/render.go`
- Test: `internal/label/render_test.go`

**Interfaces:**
- Consumes: `Element`, `RotateImage`, all element types; `tspl.LabelSize`.
- Produces:
  - `type Document struct{ Elements []Element }`
  - `func Place(el Element, scale float64) (img image.Image, x, y float64)` — returns the element raster and its scaled top-left, computed so the rotated raster is centered on the element box center.
  - `func Render(doc *Document, size tspl.LabelSize) image.Image` — returns a portrait `size.PixelW x size.PixelH` white-background image with all elements composited (landscape compose -> rotate 90 CW -> portrait).

- [ ] **Step 1: Write the failing test**

Create `internal/label/render_test.go`:
```go
package label

import (
	"testing"

	"nelko-print/internal/tspl"
)

func TestRenderDims(t *testing.T) {
	doc := &Document{}
	img := Render(doc, tspl.Label14x40)
	if img.Bounds().Dx() != tspl.Label14x40.PixelW || img.Bounds().Dy() != tspl.Label14x40.PixelH {
		t.Errorf("dims=%v want %dx%d", img.Bounds(), tspl.Label14x40.PixelW, tspl.Label14x40.PixelH)
	}
}

func TestRenderEmptyIsWhite(t *testing.T) {
	img := Render(&Document{}, tspl.Label14x40)
	r, g, b, _ := img.At(10, 10).RGBA()
	if r < 0xf000 || g < 0xf000 || b < 0xf000 {
		t.Errorf("empty label pixel not white: %d,%d,%d", r, g, b)
	}
}

func TestRenderPlacesDarkPixels(t *testing.T) {
	doc := &Document{}
	s := NewShape(FilledRect)
	s.SetBounds(Rect{0, 0, 50, 50}) // top-left of landscape canvas
	doc.Elements = []Element{s}
	img := Render(doc, tspl.Label14x40)
	dark := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			if r < 0x4000 {
				dark++
			}
		}
	}
	if dark == 0 {
		t.Error("expected dark pixels from filled rect, found none")
	}
}

func TestPlaceCentersRotated(t *testing.T) {
	s := NewShape(FilledRect)
	s.SetBounds(Rect{10, 10, 20, 20})
	s.SetAngle(45)
	img, x, y := Place(s, 1.0)
	// rotated 20x20 box bounding box is ~29x29, centered on (20,20)
	cx := x + float64(img.Bounds().Dx())/2
	cy := y + float64(img.Bounds().Dy())/2
	if cx < 18 || cx > 22 || cy < 18 || cy > 22 {
		t.Errorf("center=(%.1f,%.1f) want ~(20,20)", cx, cy)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/label/...`
Expected: FAIL (Render/Place/Document undefined).

- [ ] **Step 3: Implement render.go**

Create `internal/label/render.go`:
```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/label/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/label/render.go internal/label/render_test.go
git commit -m "feat(label): document, placement, and renderer"
```

---

### Task 7: labelCanvas editing widget

**Files:**
- Create: `cmd/nelko-print/labelcanvas.go`
- Manual test only (Fyne UI; no headless test harness in this project).

**Interfaces:**
- Consumes: `label.Document`, `label.Element`, `label.Place`, `label.SnapAngle`, `tspl.LabelSize`.
- Produces:
  - `type labelCanvas struct{ widget.BaseWidget; ... }`
  - `func newLabelCanvas(doc *label.Document, size tspl.LabelSize, onChange func()) *labelCanvas`
  - `func (c *labelCanvas) SetSize(size tspl.LabelSize)` — updates label size + display scale, rebuilds.
  - `func (c *labelCanvas) Selected() label.Element` — currently selected element or nil.
  - `func (c *labelCanvas) Rebuild()` — re-lay children from the document.
  - `func (c *labelCanvas) Add(el label.Element)` / `func (c *labelCanvas) DeleteSelected()`

**Design notes (read before coding):**
- Use `container.NewWithoutLayout()` as the child container; position children with `Move`/`Resize` in pixels = label-space * `displayScale`.
- `displayScale` chosen so the landscape label (`size.PixelH` wide) fits a target on-screen width (e.g. 560px): `displayScale = 560 / float64(size.PixelH)` (clamp to >= 2).
- Layered children (bottom to top): (1) gray backdrop rectangle filling the widget; (2) white label rectangle (`canvas.Rectangle` with `StrokeColor` dark, `StrokeWidth` 1) sized to the landscape label at scale; (3) one `canvas.Image` per element; (4) selection overlay: a `canvas.Rectangle` border + resize handle (`canvas.Rectangle`) + rotate handle (`canvas.Circle`).
- The widget implements `fyne.Tappable` (tap empty space deselects) and per-element hit-testing happens via small wrapper objects. Simplest robust approach: make each element a `*draggableImage` (a custom widget embedding `widget.BaseWidget`) that implements `Tappable` + `Draggable`; on `Tapped` it calls `c.selectElement(el)`, on `Dragged` it updates `el` bounds and calls `onChange`.
- Resize handle and rotate handle are separate tiny `*draggableHandle` widgets shown only for the selected element; their `Dragged` updates W/H or angle (snap via `label.SnapAngle`).
- A dimension caption (`canvas.Text`, e.g. `"14 x 40 mm  (284 x 96 px)"`) is drawn at the top-left of the backdrop.
- After any mutation call `c.Rebuild()` then `c.onChange()` (which triggers `App.rebuild()` to re-render the preview).

- [ ] **Step 1: Implement labelcanvas.go**

Create `cmd/nelko-print/labelcanvas.go`:
```go
package main

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"nelko-print/internal/label"
	"nelko-print/internal/tspl"
)

const canvasTargetW = 560.0

type labelCanvas struct {
	widget.BaseWidget
	doc      *label.Document
	size     tspl.LabelSize
	scale    float64
	selected label.Element
	onChange func()
	objects  *fyne.Container
}

func newLabelCanvas(doc *label.Document, size tspl.LabelSize, onChange func()) *labelCanvas {
	c := &labelCanvas{doc: doc, size: size, onChange: onChange}
	c.setScale()
	c.objects = container.NewWithoutLayout()
	c.ExtendBaseWidget(c)
	c.Rebuild()
	return c
}

func (c *labelCanvas) setScale() {
	c.scale = canvasTargetW / float64(c.size.PixelH)
	if c.scale < 2 {
		c.scale = 2
	}
}

func (c *labelCanvas) SetSize(size tspl.LabelSize) {
	c.size = size
	c.setScale()
	c.Rebuild()
	c.fire()
}

func (c *labelCanvas) Selected() label.Element { return c.selected }

func (c *labelCanvas) Add(el label.Element) {
	// place at landscape center
	b := el.Bounds()
	b.X = float64(c.size.PixelH)/2 - b.W/2
	b.Y = float64(c.size.PixelW)/2 - b.H/2
	el.SetBounds(b)
	c.doc.Elements = append(c.doc.Elements, el)
	c.selected = el
	c.Rebuild()
	c.fire()
}

func (c *labelCanvas) DeleteSelected() {
	if c.selected == nil {
		return
	}
	for i, e := range c.doc.Elements {
		if e == c.selected {
			c.doc.Elements = append(c.doc.Elements[:i], c.doc.Elements[i+1:]...)
			break
		}
	}
	c.selected = nil
	c.Rebuild()
	c.fire()
}

func (c *labelCanvas) selectElement(el label.Element) {
	c.selected = el
	c.Rebuild()
}

func (c *labelCanvas) fire() {
	if c.onChange != nil {
		c.onChange()
	}
}

// Rebuild lays out backdrop, label area, element images, and selection handles.
func (c *labelCanvas) Rebuild() {
	objs := []fyne.CanvasObject{}

	labW := float32(float64(c.size.PixelH) * c.scale)
	labH := float32(float64(c.size.PixelW) * c.scale)

	backdrop := canvas.NewRectangle(color.NRGBA{0xDD, 0xDD, 0xDD, 0xFF})
	backdrop.Resize(fyne.NewSize(labW+40, labH+50))
	backdrop.Move(fyne.NewPos(0, 0))
	objs = append(objs, backdrop)

	labelRect := canvas.NewRectangle(color.White)
	labelRect.StrokeColor = color.NRGBA{0x33, 0x33, 0x33, 0xFF}
	labelRect.StrokeWidth = 1
	labelRect.Resize(fyne.NewSize(labW, labH))
	labelRect.Move(fyne.NewPos(20, 30))
	objs = append(objs, labelRect)

	caption := canvas.NewText(fmt.Sprintf("%s  (%d x %d px)", c.size.Name, c.size.PixelH, c.size.PixelW), color.Black)
	caption.TextSize = 11
	caption.Move(fyne.NewPos(20, 8))
	objs = append(objs, caption)

	for _, el := range c.doc.Elements {
		img, x, y := label.Place(el, c.scale)
		ci := canvas.NewImageFromImage(img)
		ci.Resize(fyne.NewSize(float32(img.Bounds().Dx()), float32(img.Bounds().Dy())))
		ci.Move(fyne.NewPos(20+float32(x), 30+float32(y)))
		di := newDraggableImage(c, el, ci)
		objs = append(objs, di)
	}

	if c.selected != nil {
		objs = append(objs, c.selectionOverlay()...)
	}

	c.objects.Objects = objs
	c.objects.Refresh()
}

func (c *labelCanvas) selectionOverlay() []fyne.CanvasObject {
	b := c.selected.Bounds()
	x := 20 + float32(b.X*c.scale)
	y := 30 + float32(b.Y*c.scale)
	w := float32(b.W * c.scale)
	h := float32(b.H * c.scale)

	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = color.NRGBA{0x00, 0x77, 0xCC, 0xFF}
	border.StrokeWidth = 1
	border.Resize(fyne.NewSize(w, h))
	border.Move(fyne.NewPos(x, y))

	resize := newDraggableHandle(c, handleResize, color.NRGBA{0x00, 0x77, 0xCC, 0xFF})
	resize.Resize(fyne.NewSize(12, 12))
	resize.Move(fyne.NewPos(x+w-6, y+h-6))

	rotate := newDraggableHandle(c, handleRotate, color.NRGBA{0xCC, 0x55, 0x00, 0xFF})
	rotate.Resize(fyne.NewSize(12, 12))
	rotate.Move(fyne.NewPos(x+w/2-6, y-18))

	return []fyne.CanvasObject{border, resize, rotate}
}

func (c *labelCanvas) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(c.objects)
}

func (c *labelCanvas) MinSize() fyne.Size {
	labW := float32(float64(c.size.PixelH) * c.scale)
	labH := float32(float64(c.size.PixelW) * c.scale)
	return fyne.NewSize(labW+40, labH+50)
}

// Tapped on empty canvas deselects.
func (c *labelCanvas) Tapped(_ *fyne.PointEvent) {
	c.selected = nil
	c.Rebuild()
}

var _ fyne.Tappable = (*labelCanvas)(nil)
var _ desktop.Mouseable = (*labelCanvas)(nil)

func (c *labelCanvas) MouseDown(_ *desktop.MouseEvent) {}
func (c *labelCanvas) MouseUp(_ *desktop.MouseEvent)   {}
```

- [ ] **Step 2: Implement the draggable element + handle widgets**

Append to `cmd/nelko-print/labelcanvas.go`:
```go
// draggableImage wraps an element's canvas.Image to handle select + move.
type draggableImage struct {
	widget.BaseWidget
	c   *labelCanvas
	el  label.Element
	img *canvas.Image
}

func newDraggableImage(c *labelCanvas, el label.Element, img *canvas.Image) *draggableImage {
	d := &draggableImage{c: c, el: el, img: img}
	d.ExtendBaseWidget(d)
	d.Move(img.Position())
	d.Resize(img.Size())
	return d
}

func (d *draggableImage) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(d.img)
}
func (d *draggableImage) Tapped(_ *fyne.PointEvent) { d.c.selectElement(d.el) }
func (d *draggableImage) Dragged(e *fyne.DragEvent) {
	d.c.selectElement(d.el)
	b := d.el.Bounds()
	b.X += float64(e.Dragged.DX) / d.c.scale
	b.Y += float64(e.Dragged.DY) / d.c.scale
	d.el.SetBounds(b)
	d.c.Rebuild()
	d.c.fire()
}
func (d *draggableImage) DragEnd() {}

var _ fyne.Tappable = (*draggableImage)(nil)
var _ fyne.Draggable = (*draggableImage)(nil)

type handleKind int

const (
	handleResize handleKind = iota
	handleRotate
)

// draggableHandle is a small square/circle handle for resize/rotate.
type draggableHandle struct {
	widget.BaseWidget
	c    *labelCanvas
	kind handleKind
	col  color.Color
}

func newDraggableHandle(c *labelCanvas, kind handleKind, col color.Color) *draggableHandle {
	h := &draggableHandle{c: c, kind: kind, col: col}
	h.ExtendBaseWidget(h)
	return h
}

func (h *draggableHandle) CreateRenderer() fyne.WidgetRenderer {
	if h.kind == handleRotate {
		circle := canvas.NewCircle(h.col)
		return widget.NewSimpleRenderer(circle)
	}
	rect := canvas.NewRectangle(h.col)
	return widget.NewSimpleRenderer(rect)
}

func (h *draggableHandle) Dragged(e *fyne.DragEvent) {
	el := h.c.selected
	if el == nil {
		return
	}
	b := el.Bounds()
	if h.kind == handleResize {
		b.W += float64(e.Dragged.DX) / h.c.scale
		b.H += float64(e.Dragged.DY) / h.c.scale
		if b.W < 8 {
			b.W = 8
		}
		if b.H < 8 {
			b.H = 8
		}
		el.SetBounds(b)
	} else {
		// rotate: angle from element center to current pointer
		cx := 20 + float32((b.X+b.W/2)*h.c.scale)
		cy := 30 + float32((b.Y+b.H/2)*h.c.scale)
		px := h.Position().X + e.Position.X
		py := h.Position().Y + e.Position.Y
		ang := int(degAtan2(float64(py-cy), float64(px-cx))) + 90
		el.SetAngle(label.SnapAngle(ang))
	}
	h.c.Rebuild()
	h.c.fire()
}
func (h *draggableHandle) DragEnd() {}

var _ fyne.Draggable = (*draggableHandle)(nil)
```

- [ ] **Step 3: Add the angle helper**

Append to `cmd/nelko-print/labelcanvas.go`:
```go
import "math" // ensure math is imported in this file's import block

func degAtan2(y, x float64) float64 { return math.Atan2(y, x) * 180 / math.Pi }
```
(If the file already imports `math`, just add the function and do not duplicate the import.)

- [ ] **Step 4: Build to verify it compiles**

Run: `make build-macos`
Expected: builds with no errors.

- [ ] **Step 5: Commit**

```bash
git add cmd/nelko-print/labelcanvas.go
git commit -m "feat(ui): labelCanvas editing widget with drag/resize/rotate"
```

---

### Task 8: Designer tab, property strip, export, integration

**Files:**
- Create: `cmd/nelko-print/designer.go`
- Modify: `cmd/nelko-print/main.go`

**Interfaces:**
- Consumes: `labelCanvas` (Task 7), `label.*`, existing `App` fields (`labelSize`, `sourceImg`, `previewImg`, `printBtn`, `printer`, `window`).
- Produces:
  - `func (a *App) buildDesignerTab() fyne.CanvasObject`
  - `func (a *App) rebuildDesigner()` — `a.sourceImg = label.Render(a.designDoc, a.labelSize)`; sets `a.orientation = imaging.Horizontal`; refresh preview; enable Print if connected.
  - New `App` fields: `designDoc *label.Document`, `designCanvas *labelCanvas`, `designProps *fyne.Container`.

- [ ] **Step 1: Add App fields and document init**

In `cmd/nelko-print/main.go`, add to the `App` struct (near the Text-mode fields):
```go
	// Designer mode
	designDoc    *label.Document
	designCanvas *labelCanvas
	designProps  *fyne.Container
```
Add the import `"nelko-print/internal/label"` to `main.go`'s import block.
In `main()` where `nelkoApp` is constructed, add field:
```go
		designDoc: &label.Document{},
```

- [ ] **Step 2: Implement designer.go**

Create `cmd/nelko-print/designer.go`:
```go
package main

import (
	"fmt"
	"image"
	"image/png"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"

	"nelko-print/internal/imaging"
	"nelko-print/internal/label"
)

func (a *App) buildDesignerTab() fyne.CanvasObject {
	a.designCanvas = newLabelCanvas(a.designDoc, a.labelSize, a.rebuildDesigner)
	a.designProps = container.NewVBox(widget.NewLabel("Select an element to edit"))

	addText := widget.NewButton("+ Text", func() {
		a.designCanvas.Add(label.NewText("Text"))
		a.refreshDesignerProps()
	})
	addBarcode := widget.NewButton("+ Barcode", func() {
		a.designCanvas.Add(label.NewBarcode(label.Aztec, "12345"))
		a.refreshDesignerProps()
	})
	addShape := widget.NewButton("+ Shape", func() {
		a.designCanvas.Add(label.NewShape(label.FilledRect))
		a.refreshDesignerProps()
	})
	addImage := widget.NewButton("+ Image", func() { a.designerLoadImage() })
	delBtn := widget.NewButton("Delete", func() {
		a.designCanvas.DeleteSelected()
		a.refreshDesignerProps()
	})
	exportBtn := widget.NewButton("Export PNG", func() { a.designerExportPNG() })

	toolbar := container.NewHBox(addText, addImage, addBarcode, addShape, delBtn, exportBtn)

	return container.NewBorder(
		toolbar,
		a.designProps,
		nil, nil,
		container.NewScroll(a.designCanvas),
	)
}

func (a *App) rebuildDesigner() {
	if a.designDoc == nil {
		return
	}
	a.orientation = imaging.Horizontal
	a.sourceImg = label.Render(a.designDoc, a.labelSize)
	a.updatePreview()
	if a.printer != nil {
		a.printBtn.Enable()
	}
	a.refreshDesignerProps()
}

func (a *App) designerLoadImage() {
	fd := dialog.NewFileOpen(func(r fyne.URIReadCloser, err error) {
		if err != nil || r == nil {
			return
		}
		defer r.Close()
		img, _, derr := image.Decode(r)
		if derr != nil {
			dialog.ShowError(derr, a.window)
			return
		}
		a.designCanvas.Add(label.NewImage(img))
		a.refreshDesignerProps()
	}, a.window)
	fd.SetFilter(storage.NewExtensionFileFilter([]string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp"}))
	fd.Show()
}

func (a *App) designerExportPNG() {
	fd := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
		if err != nil || w == nil {
			return
		}
		defer w.Close()
		img := label.Render(a.designDoc, a.labelSize)
		if perr := png.Encode(w, img); perr != nil {
			dialog.ShowError(perr, a.window)
		}
	}, a.window)
	fd.SetFileName("label.png")
	fd.Show()
}

// refreshDesignerProps rebuilds the property strip for the selected element.
func (a *App) refreshDesignerProps() {
	sel := a.designCanvas.Selected()
	if sel == nil {
		a.designProps.Objects = []fyne.CanvasObject{widget.NewLabel("Select an element to edit")}
		a.designProps.Refresh()
		return
	}
	var content fyne.CanvasObject
	switch el := sel.(type) {
	case *label.TextElement:
		entry := widget.NewEntry()
		entry.SetText(el.Text)
		entry.OnChanged = func(s string) { el.Text = s; a.designCanvas.Rebuild(); a.rebuildDesigner() }
		fontSlider := widget.NewSlider(6, 72)
		fontSlider.Value = el.FontSize
		fontSlider.OnChanged = func(f float64) { el.FontSize = f; a.designCanvas.Rebuild(); a.rebuildDesigner() }
		inv := widget.NewCheck("Invert", func(b bool) { el.Invert = b; a.designCanvas.Rebuild(); a.rebuildDesigner() })
		inv.SetChecked(el.Invert)
		content = widget.NewForm(
			widget.NewFormItem("Text", entry),
			widget.NewFormItem("Size", fontSlider),
			widget.NewFormItem("", inv),
		)
	case *label.BarcodeElement:
		data := widget.NewEntry()
		data.SetText(el.Data)
		data.OnChanged = func(s string) { el.Data = s; a.designCanvas.Rebuild(); a.rebuildDesigner() }
		kinds := widget.NewSelect([]string{"Aztec", "QR", "DataMatrix", "Code128"}, func(s string) {
			switch s {
			case "Aztec":
				el.Kind = label.Aztec
			case "QR":
				el.Kind = label.QR
			case "DataMatrix":
				el.Kind = label.DataMatrix
			case "Code128":
				el.Kind = label.Code128
			}
			a.designCanvas.Rebuild()
			a.rebuildDesigner()
		})
		kinds.SetSelected(barcodeKindName(el.Kind))
		content = widget.NewForm(
			widget.NewFormItem("Type", kinds),
			widget.NewFormItem("Data", data),
		)
	case *label.ImageElement:
		thr := widget.NewSlider(0, 255)
		thr.Value = float64(el.Threshold)
		thr.OnChanged = func(f float64) { el.Threshold = uint8(f); a.designCanvas.Rebuild(); a.rebuildDesigner() }
		inv := widget.NewCheck("Invert", func(b bool) { el.Invert = b; a.designCanvas.Rebuild(); a.rebuildDesigner() })
		inv.SetChecked(el.Invert)
		content = widget.NewForm(
			widget.NewFormItem("Threshold", thr),
			widget.NewFormItem("", inv),
		)
	case *label.ShapeElement:
		kinds := widget.NewSelect([]string{"Line", "Rectangle", "FilledRect"}, func(s string) {
			switch s {
			case "Line":
				el.Kind = label.Line
			case "Rectangle":
				el.Kind = label.Rectangle
			case "FilledRect":
				el.Kind = label.FilledRect
			}
			a.designCanvas.Rebuild()
			a.rebuildDesigner()
		})
		kinds.SetSelected(shapeKindName(el.Kind))
		content = widget.NewForm(widget.NewFormItem("Shape", kinds))
	default:
		content = widget.NewLabel("No properties")
	}
	a.designProps.Objects = []fyne.CanvasObject{content}
	a.designProps.Refresh()
}

func barcodeKindName(k label.BarcodeKind) string {
	switch k {
	case label.QR:
		return "QR"
	case label.DataMatrix:
		return "DataMatrix"
	case label.Code128:
		return "Code128"
	default:
		return "Aztec"
	}
}

func shapeKindName(k label.ShapeKind) string {
	switch k {
	case label.Line:
		return "Line"
	case label.Rectangle:
		return "Rectangle"
	default:
		return "FilledRect"
	}
}

// ensure os import used (export path validation placeholder)
var _ = os.Getenv
```

- [ ] **Step 3: Wire the tab and label-size change into main.go**

In `cmd/nelko-print/main.go`, change the tabs construction:
```go
	tabs := container.NewAppTabs(
		container.NewTabItem("Image", imageTab),
		container.NewTabItem("Text", textTab),
		container.NewTabItem("Designer", a.buildDesignerTab()),
	)
```
In the `sizeSelect` `OnChanged` handler, after `a.labelSize = size`, also resize the design canvas if it exists:
```go
			if size.Name == s {
				a.labelSize = size
				if a.designCanvas != nil {
					a.designCanvas.SetSize(size)
				}
				a.updatePreview()
				break
			}
```

- [ ] **Step 4: Build to verify it compiles**

Run: `make build-macos`
Expected: builds; remove the `var _ = os.Getenv` line and its `os` import if `os` ends up unused (it is only a guard — delete it once the file compiles without it).

- [ ] **Step 5: Manual functional test**

Run: `./nelko-print`
Verify:
- Designer tab present. Backdrop is gray, label area white with a visible dark border and a dimension caption.
- `+ Text` adds a selectable text box; editing the Text field updates it live; drag moves it; corner handle resizes; rotate handle snaps to 45-degree steps.
- `+ Barcode` adds an Aztec code; changing Type/Data re-renders; QR/DataMatrix/Code128 all render.
- `+ Image` loads a file and places it; threshold slider changes the result.
- `+ Shape` adds a filled rect; Shape dropdown switches Line/Rectangle/FilledRect.
- Changing Label Size resizes the canvas.
- Right-side mono preview matches the on-canvas layout (rotated to portrait).
- `Export PNG` writes a file that visually matches the preview.

- [ ] **Step 6: Print test (hardware)**

Connect the P21, add a couple of elements, click Print. Confirm the printed label matches the on-screen design orientation. If orientation is mirrored/rotated wrong, flip the rotation in `label.Render` (use `RotateImage(canvas, -90)`), rebuild, reprint.

- [ ] **Step 7: Commit**

```bash
git add cmd/nelko-print/designer.go cmd/nelko-print/main.go
git commit -m "feat(ui): Designer tab with property strip, export, and print integration"
```

---

### Task 9: Docs + final verification

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Document the Designer mode**

In `README.md`, under the Features section, add:
```markdown
- **Designer mode (WYSIWYG)**: Place and arrange multiple elements on a visual
  label - text, B/W graphics, barcodes (Aztec/QR/DataMatrix/Code128), and
  shapes. Drag to move, drag the corner handle to resize, drag the top handle
  to rotate in 45-degree steps. Export the composed label as PNG.
```

- [ ] **Step 2: Run the full test suite**

Run: `go test ./...`
Expected: PASS (label package tests; existing packages unaffected).

- [ ] **Step 3: Run go vet and build**

Run: `go vet ./... && make build-macos`
Expected: no vet errors; clean build.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: document Designer mode"
```

---

## Self-Review

**Spec coverage:**
- Element model (text/image/barcode/shape) → Tasks 2-5. ✓
- 45-degree-snap rotation → `SnapAngle` (Task 1), used by `Base.SetAngle` (Task 2), rotate handle (Task 7). ✓
- Landscape model + compose→rotate→portrait → `Render` (Task 6). ✓
- Reuse existing print path → `rebuildDesigner` sets `a.sourceImg` (Task 8). ✓
- Drag-move/resize/rotate handles → Task 7. ✓
- Property strip (inline, per type) → Task 8. ✓
- Label visibility (gray backdrop, white area, border, dimension caption) → Task 7 `Rebuild`. ✓
- Export PNG, no project save/load → Task 8 `designerExportPNG`. ✓
- Barcode default Aztec, all four kinds via boombuler → Task 5. ✓
- New dep `github.com/boombuler/barcode v1.1.0`, no CGO → Task 1. ✓
- 3rd tab, Image/Text untouched → Task 8 tab wiring. ✓
- Error handling: barcode encode error placeholder, empty data no panic → Task 5 tests + `placeholder`. ✓
- Tests for label package → Tasks 1-6. ✓

**Placeholder scan:** The only guard placeholders (`var _ = os.Getenv`, the illustrative `hasDark` stub) are explicitly flagged for deletion in their steps. No "TBD"/"add error handling"-style gaps.

**Type consistency:** `Element`, `Base`, `Rect`, `SnapAngle`, `RotateImage`, `ScaleNearest`, `Place`, `Render`, `Document`, and element constructors (`NewText/NewImage/NewBarcode/NewShape`) are used with consistent names/signatures across tasks. `rebuildDesigner` / `Rebuild` are distinct (App method vs widget method) and used consistently. Barcode/shape kind enums match between element definitions (Tasks 2,5) and property strip (Task 8).
