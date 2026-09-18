package app

import (
	"fyne.io/fyne/v2/internal/driver/mobile"
)

// StartCamera opens a persistent in-app camera session (using Android's Camera2
// API). The camera stays open and shows a live preview until StopCamera is
// called. Each photo is captured by calling TakePicture.
//
// The callback is invoked once for every captured photo, with the URI of the
// saved image (empty string if the capture failed or permission was denied).
// The callback stays registered until StopCamera is called.
//
// This API is only available on mobile builds; on other platforms it does
// nothing.
func StartCamera(callback func(string)) {
	mobile.StartCamera(callback)
}

// StopCamera closes the persistent camera session and releases the device camera.
func StopCamera() {
	mobile.StopCamera()
}

// TakePicture captures one photo while the camera session is open. The captured
// image URI is delivered to the callback registered with StartCamera.
func TakePicture() {
	mobile.TakePicture()
}
