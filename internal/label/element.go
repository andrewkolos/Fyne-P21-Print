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
