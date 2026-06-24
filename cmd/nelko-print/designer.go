package main

import (
	"image"
	"image/png"

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

// renderDesignPreview re-renders the label into the shared preview without
// touching the property strip (so it is safe to call from a property's
// OnChanged handler while that widget has focus).
func (a *App) renderDesignPreview() {
	if a.designDoc == nil {
		return
	}
	a.orientation = imaging.Horizontal
	a.sourceImg = label.Render(a.designDoc, a.labelSize)
	a.updatePreview()
	if a.printer != nil {
		a.printBtn.Enable()
	}
}

// rebuildDesigner is the canvas onChange callback: a structural or selection
// change happened, so refresh both the preview and the property strip.
func (a *App) rebuildDesigner() {
	a.renderDesignPreview()
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
		entry.OnChanged = func(s string) { el.Text = s; a.designCanvas.Rebuild(); a.renderDesignPreview() }
		fontSlider := widget.NewSlider(6, 72)
		fontSlider.Value = el.FontSize
		fontSlider.OnChanged = func(f float64) { el.FontSize = f; a.designCanvas.Rebuild(); a.renderDesignPreview() }
		inv := widget.NewCheck("Invert", func(b bool) { el.Invert = b; a.designCanvas.Rebuild(); a.renderDesignPreview() })
		inv.SetChecked(el.Invert)
		content = widget.NewForm(
			widget.NewFormItem("Text", entry),
			widget.NewFormItem("Size", fontSlider),
			widget.NewFormItem("", inv),
		)
	case *label.BarcodeElement:
		data := widget.NewEntry()
		data.SetText(el.Data)
		data.OnChanged = func(s string) { el.Data = s; a.designCanvas.Rebuild(); a.renderDesignPreview() }
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
			a.renderDesignPreview()
		})
		kinds.SetSelected(barcodeKindName(el.Kind))
		content = widget.NewForm(
			widget.NewFormItem("Type", kinds),
			widget.NewFormItem("Data", data),
		)
	case *label.ImageElement:
		thr := widget.NewSlider(0, 255)
		thr.Value = float64(el.Threshold)
		thr.OnChanged = func(f float64) { el.Threshold = uint8(f); a.designCanvas.Rebuild(); a.renderDesignPreview() }
		inv := widget.NewCheck("Invert", func(b bool) { el.Invert = b; a.designCanvas.Rebuild(); a.renderDesignPreview() })
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
			a.renderDesignPreview()
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
