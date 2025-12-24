// Package mobile provides mobile specific driver functionality.
package mobile

import "io"

// Device describes functionality only available on mobile
type Device interface {
	// Request that the mobile device show the touch screen keyboard (standard layout)
	ShowVirtualKeyboard()
	// Request that the mobile device show the touch screen keyboard (custom layout)
	ShowVirtualKeyboardType(KeyboardType)
	// Request that the mobile device dismiss the touch screen keyboard
	HideVirtualKeyboard()
	// capture microphone, application must close the reader to signal that recording has finished
	RecordAudio() (io.ReadCloser, error)
}
