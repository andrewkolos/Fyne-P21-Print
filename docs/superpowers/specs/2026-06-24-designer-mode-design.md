# Designer Mode - WYSIWYG Label Editor

Date: 2026-06-24
Status: Approved (pending spec review)

## Goal

Add a third tab, **Designer**, to the Nelko P21 print app: a WYSIWYG label
editor where the user places and arranges multiple elements (text, B/W
graphics, barcodes, shapes) on a visual representation of the label, moving,
resizing, and rotating them, then prints the composed result. The existing
Image and Text tabs remain unchanged.

## Background / Current Pipeline

- A label is a narrow portrait strip: fixed `PixelW = 96` wide, variable
  `PixelH` tall (e.g. 284 for 14x40mm). See `internal/tspl` `LabelSize`.
- Both existing tabs converge on a single `image.Image` stored in
  `App.sourceImg`. Printing runs:
  `imaging.ToMonochrome(sourceImg, PixelW, PixelH, threshold, !invert)` →
  `tspl.BuildPrintJob(...)` → `printer.Printer.Print(...)`.
- `ToMonochrome` resizes-to-fit and thresholds the source into the 1-bit
  buffer the P21 expects. The Designer reuses this path verbatim by producing
  a portrait `96 × PixelH` `image.Image` and assigning it to `sourceImg`.
- `internal/imaging/text.go` already renders text with the bundled
  `goregular` font at 203 DPI and provides `rotate90CW` / `rotate90CCW`.

## Non-Goals (v1)

- No saving/loading of project files. The only persistence is **Export PNG**
  of the final composited label.
- No free-angle rotation. Rotation snaps to 45 degree steps (8 orientations).
- No multi-label sheets, no undo/redo, no alignment guides/snapping between
  elements.
- No changes to the Image/Text tabs, the connection code, or the print code.

## Architecture

### New package: `internal/label`

Holds the editor's data model and the renderer. No Fyne imports here so it can
be unit-tested headless.

```go
// Element is one item placed on the label. Coordinates are in label space
// (printer pixels), landscape orientation: x grows along the long axis.
type Element interface {
    Bounds() Rect              // x, y, w, h in label-space pixels (float64)
    SetBounds(Rect)
    Angle() int                // 0,45,90,...,315 ; always a multiple of 45
    SetAngle(int)
    // Raster returns the element rendered black-on-white at the given scale
    // (1.0 = label resolution), already rotated by Angle. Used for both the
    // on-screen widget and the final composite.
    Raster(scale float64) image.Image
}

type Rect struct{ X, Y, W, H float64 }
```

Concrete element types:

- **TextElement**: `Text string`, `FontSize float64`, `Invert bool`. Renders
  with freetype + `goregular` (same as `imaging/text.go`); reuses the existing
  word-wrap helpers (extract/share them rather than duplicate). Width is the
  element box width; text wraps within it.
- **ImageElement**: holds the loaded source `image.Image` plus `Threshold
  uint8` and `Invert bool`. Raster scales the source to the element box and
  thresholds to B/W.
- **BarcodeElement**: `Kind` (QR | Aztec | DataMatrix | Code128), `Data
  string`. Uses `github.com/boombuler/barcode` (pure Go, no CGO). Default
  kind: **Aztec** (compact, built-in quiet zone) for tiny labels.
- **ShapeElement**: `Kind` (Line | Rect | FilledRect), `StrokeWidth int`.

Rotation strategy: each element stores unrotated content + an angle. `Raster`
renders the unrotated content, then rotates the raster by the snapped angle.
0/90/180/270 use the existing fast `rotate90*` style rotations; 45/135/225/315
use a general rotation onto a transparent-padded canvas. The element's
on-screen bounding box is the rotated raster's bounds.

```go
type Document struct {
    Elements []Element
}

// Render composites every element onto a white landscape canvas
// (w = size.PixelH, h = size.PixelW), then rotates 90 deg CW to produce the
// portrait (size.PixelW x size.PixelH) image the print path consumes.
func Render(doc *Document, size tspl.LabelSize) image.Image
```

The renderer draws elements in document order (later elements on top), each
at its `Bounds`, using its `Raster(1.0)`. Output feeds `App.sourceImg`
directly; `App.orientation` is set to `Horizontal` so `updatePreview` does no
extra rotation.

### UI: `cmd/nelko-print` Designer tab

A new custom widget `labelCanvas` (in `cmd/nelko-print`, or a small
`internal/ui` package) backed by `container.NewWithoutLayout`. It owns the
`*label.Document` and a `displayScale` (chosen so the landscape label fits the
available canvas width, e.g. target ~3-4x the 96px short edge).

**Label visibility (explicit requirement):** the editing surface must make the
label boundary unmistakable. The canvas background (outside the label) uses a
contrasting fill (light gray, e.g. `#DDDDDD`), the label area is white with a
1px solid dark border, and a small caption shows the current dimensions
(e.g. "14 x 40 mm  (284 x 96 px)"). This fixes the current problem where label
and background are both white with no border.

**Per element**: a draggable child widget showing `element.Raster(displayScale)`
via `canvas.Image`, positioned with `Move`/`Resize` at `bounds * displayScale`.

- `Tapped` selects the element (tapping empty canvas deselects).
- The selected element draws a selection rectangle plus two handles:
  - bottom-right **resize handle** (a small square): `Draggable`, updates
    element W/H (min size enforced; aspect ratio free).
  - top-center **rotate handle** (a small circle on a stem): `Draggable`,
    computes the angle from the element center and **snaps to the nearest 45
    degrees**.
- Dragging the element body moves it (`fyne.Draggable`). Movement is clamped so
  the element stays at least partially within the label.
- `Delete`/`Backspace` key removes the selected element.

**Toolbar** (above canvas): buttons `+ Text`, `+ Image`, `+ Barcode`,
`+ Shape`, `Delete`, `Export PNG`. New elements are added at the canvas center
and auto-selected. `+ Image` opens the existing file-open dialog. `Export PNG`
opens a file-save dialog and writes `label.Render(...)` as PNG.

**Property strip** (below canvas): context-sensitive controls for the selected
element, edited inline (not a popup):
- Text: multiline entry (content) + font-size slider + invert check.
- Barcode: kind dropdown (Aztec/QR/DataMatrix/Code128) + data entry.
- Image: threshold slider + invert check.
- Shape: kind dropdown + stroke-width stepper.
Empty/disabled when nothing is selected.

### Print/preview integration

Any change to the document or a property triggers `rebuild()`:
`a.sourceImg = label.Render(doc, a.labelSize)`, then the existing
`a.updatePreview()` and enable Print if connected. The left panel's label-size,
density, and copies controls work unchanged. Changing label size updates the
document's working size and `displayScale`, then rebuilds. `print()` is reused
as-is.

## Data Flow

```
user edits canvas / property strip
        -> Document mutated
        -> label.Render(doc, labelSize)   (compose landscape -> rotate90CW -> portrait)
        -> App.sourceImg
        -> imaging.ToMonochrome -> tspl.BuildPrintJob -> printer.Print
```

## Error Handling

- Barcode encode failure (e.g. data too long for Aztec at that size): show the
  error in the property strip / status label; the element renders a visible
  "encode error" placeholder box rather than crashing.
- Empty text / empty barcode data: element renders as an empty selectable box
  so it can still be positioned or deleted.
- Export PNG failure: `dialog.ShowError`.
- Render is defensive: elements fully outside the label are skipped in the
  composite but kept in the model.

## Testing

`internal/label` is UI-free and unit-tested:
- Element `Raster` produces non-empty images of expected dimensions for each
  type and each 45-degree angle.
- `Angle` setter snaps arbitrary inputs to the nearest multiple of 45.
- `Render` output has exactly `PixelW x PixelH` dimensions for a sample label
  size and places a known element's dark pixels in the expected region
  (sanity check on the landscape->portrait rotation).
- Barcode encode error path returns a placeholder, not a panic.

UI widget behavior (drag/resize/rotate) is verified manually on macOS by
building and running, then printing a test label (the project has no UI test
harness).

## Dependencies

- Add `github.com/boombuler/barcode` (pure Go). No CGO impact; cross-platform
  builds (Linux/Windows/macOS) unaffected.

## Affected Files

- New: `internal/label/element.go`, `internal/label/text.go`,
  `internal/label/image.go`, `internal/label/barcode.go`,
  `internal/label/shape.go`, `internal/label/render.go`,
  `internal/label/*_test.go`.
- New: designer UI widget + tab wiring in `cmd/nelko-print` (e.g.
  `designer.go`, `labelcanvas.go`).
- Modified: `cmd/nelko-print/main.go` (add the 3rd tab, `rebuild` hook,
  label-size change resizes document).
- Modified: `internal/imaging/text.go` only if word-wrap helpers are shared
  (exported) rather than duplicated.
- Modified: `go.mod` / `go.sum` (new dependency).
- Modified: `README.md` (document Designer mode).
```
