package main

import (
	"fmt"
	"image/color"
	"math"

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

// SetDocument replaces the document (e.g. after loading from disk).
func (c *labelCanvas) SetDocument(doc *label.Document) {
	c.doc = doc
	c.selected = nil
	c.Rebuild()
}

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
	if c.selected == el {
		return
	}
	c.selected = el
	c.Rebuild()
	c.fire()
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
		w := float32(img.Bounds().Dx())
		h := float32(img.Bounds().Dy())
		ci := canvas.NewImageFromImage(img)
		ci.FillMode = canvas.ImageFillStretch
		di := newDraggableImage(c, el, ci)
		di.Resize(fyne.NewSize(w, h))
		di.Move(fyne.NewPos(20+float32(x), 30+float32(y)))
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
	if c.selected == nil {
		return
	}
	c.selected = nil
	c.Rebuild()
	c.fire()
}

var _ fyne.Tappable = (*labelCanvas)(nil)
var _ desktop.Mouseable = (*labelCanvas)(nil)

func (c *labelCanvas) MouseDown(_ *desktop.MouseEvent) {}
func (c *labelCanvas) MouseUp(_ *desktop.MouseEvent)   {}

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

func degAtan2(y, x float64) float64 { return math.Atan2(y, x) * 180 / math.Pi }
