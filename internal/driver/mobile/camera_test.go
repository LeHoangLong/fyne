package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/internal/driver/mobile/app"
)

// cameraApp is a minimal App implementing the persistent camera session methods,
// embedding app.App so it also satisfies the full mobile App interface.
type cameraApp struct {
	app.App
	started int
	stopped int
	shots   int
	uri     string
}

func (c *cameraApp) StartCamera(callback func(string)) {
	c.started++
	if c.uri != "" {
		callback(c.uri)
	}
}

func (c *cameraApp) StopCamera() {
	c.stopped++
}

func (c *cameraApp) TakePicture() {
	c.shots++
}

// cameraTestApp is a minimal fyne.App whose Driver is a *driver, so the camera
// helpers can resolve the underlying mobile driver.
type cameraTestApp struct {
	fyne.App
	driver fyne.Driver
}

func (a *cameraTestApp) Driver() fyne.Driver {
	return a.driver
}

func TestStartCameraCallsDriverApp(t *testing.T) {
	fake := &cameraApp{uri: "content://media/external/images/media/1"}
	d := &driver{app: fake}

	prevApp := fyne.CurrentApp()
	fyne.SetCurrentApp(&cameraTestApp{App: prevApp, driver: d})
	defer fyne.SetCurrentApp(prevApp)

	var got string
	StartCamera(func(uri string) {
		got = uri
	})

	assert.Equal(t, 1, fake.started)
	assert.Equal(t, "content://media/external/images/media/1", got)
}

func TestTakePictureAndStopCameraDispatch(t *testing.T) {
	fake := &cameraApp{}
	d := &driver{app: fake}

	prevApp := fyne.CurrentApp()
	fyne.SetCurrentApp(&cameraTestApp{App: prevApp, driver: d})
	defer fyne.SetCurrentApp(prevApp)

	TakePicture()
	TakePicture()
	StopCamera()

	assert.Equal(t, 2, fake.shots)
	assert.Equal(t, 1, fake.stopped)
}

func TestCameraNilAppIsNoop(t *testing.T) {
	d := &driver{app: nil}

	prevApp := fyne.CurrentApp()
	fyne.SetCurrentApp(&cameraTestApp{App: prevApp, driver: d})
	defer fyne.SetCurrentApp(prevApp)

	assert.NotPanics(t, func() {
		StartCamera(func(_ string) {})
		TakePicture()
		StopCamera()
	})
}
