package common

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/internal/cache"
)

// CanvasForObject returns the canvas for the specified object.
func CanvasForObject(obj fyne.CanvasObject) fyne.Canvas {
	canvas := cache.GetCanvasForObject(obj)
	if canvas != nil {
		return canvas
	}

	if widget, ok := obj.(fyne.Widget); ok {
		return CanvasForWidget(widget)
	}

	return nil
}

func CanvasForWidget(widget fyne.Widget) fyne.Canvas {
	return cache.GetCanvasForWidget(widget)
}
