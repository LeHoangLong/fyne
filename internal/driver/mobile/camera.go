package mobile

import (
	"fyne.io/fyne/v2"
)

type hasCamera interface {
	StartCamera(func(string))
	StopCamera()
	TakePicture()
}

// StartCamera opens a persistent in-app camera session (using Android's Camera2
// API). The camera stays open and shows a live preview until StopCamera is
// called. Each photo is captured by calling TakePicture.
//
// The callback is invoked once for every captured photo, with the content:// URI
// of the saved image (empty string if the capture failed or permission was
// denied). The callback stays registered until StopCamera is called.
func StartCamera(callback func(string)) {
	drv := fyne.CurrentApp().Driver().(*driver)
	if a, ok := drv.app.(hasCamera); ok {
		a.StartCamera(callback)
	}
}

// StopCamera closes the persistent camera session and releases the device camera.
func StopCamera() {
	drv := fyne.CurrentApp().Driver().(*driver)
	if a, ok := drv.app.(hasCamera); ok {
		a.StopCamera()
	}
}

// TakePicture captures one photo while the camera session is open. The captured
// image URI is delivered to the callback registered with StartCamera.
func TakePicture() {
	drv := fyne.CurrentApp().Driver().(*driver)
	if a, ok := drv.app.(hasCamera); ok {
		a.TakePicture()
	}
}
