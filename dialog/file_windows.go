//go:build windows

package dialog

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"

	"github.com/harry1453/go-common-file-dialog/cfd"
	"github.com/harry1453/go-common-file-dialog/cfdutil"
)

func driveMask() uint32 {
	dll, err := syscall.LoadLibrary("kernel32.dll")
	if err != nil {
		fyne.LogError("Error loading kernel32.dll", err)
		return 0
	}
	handle, err := syscall.GetProcAddress(dll, "GetLogicalDrives")
	if err != nil {
		fyne.LogError("Could not find GetLogicalDrives call", err)
		return 0
	}

	ret, _, err := syscall.Syscall(uintptr(handle), 0, 0, 0, 0)
	if err != syscall.Errno(0) { // for some reason Syscall returns something not nil on success
		fyne.LogError("Error calling GetLogicalDrives", err)
		return 0
	}

	return uint32(ret)
}

func listDrives() []string {
	var drives []string
	mask := driveMask()

	for i := 0; i < 26; i++ {
		if mask&1 == 1 {
			letter := string('A' + rune(i))
			drives = append(drives, letter+":")
		}
		mask >>= 1
	}

	return drives
}

func (f *fileDialog) getPlaces() []favoriteItem {
	drives := listDrives()
	places := make([]favoriteItem, len(drives))
	for i, drive := range drives {
		driveRoot := drive + string(os.PathSeparator) // capture loop var
		driveRootURI, _ := storage.ListerForURI(storage.NewURI("file://" + driveRoot))
		places[i] = favoriteItem{
			drive,
			theme.StorageIcon(),
			driveRootURI,
		}
	}
	return places
}

func isHidden(file fyne.URI) bool {
	if file.Scheme() != "file" {
		fyne.LogError("Cannot check if non file is hidden", nil)
		return false
	}

	path := file.String()[len(file.Scheme())+3:]

	point, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		fyne.LogError("Error making string pointer", err)
		return false
	}
	attr, err := syscall.GetFileAttributes(point)
	if err != nil {
		fyne.LogError("Error getting file attributes", err)
		return false
	}

	return attr&syscall.FILE_ATTRIBUTE_HIDDEN != 0
}

func hideFile(filename string) (err error) {
	// git does not preserve windows hidden flag so we have to set it.
	filenameW, err := syscall.UTF16PtrFromString(filename)
	if err != nil {
		return err
	}
	return syscall.SetFileAttributes(filenameW, syscall.FILE_ATTRIBUTE_HIDDEN)
}

func fileOpenOSOverride(d *FileDialog) bool {
	go func() {
		folderCallback, folder := d.callback.(func(fyne.ListableURI, error))
		hwnd := parentWindowHandle(d)
		if folder {
			openFolder(hwnd, folderCallback, d)
			return
		}

		fileCallback := d.callback.(func(fyne.URIReadCloser, error))
		openFile(hwnd, fileCallback, d)
	}()
	return true
}

func fileSaveOSOverride(d *FileDialog) bool {
	go func() {
		callback := d.callback.(func(fyne.URIWriteCloser, error))
		openSave(parentWindowHandle(d), callback, d)
	}()
	return true
}

func openFolder(parentWindowHandle uintptr, callback func(fyne.ListableURI, error), d *FileDialog) {
	config := cfd.DialogConfig{
		Title:              nativeDialogTitle(d.confirmText, "Select Folder"),
		Role:               "fyne-folder-open",
		ParentWindowHandle: parentWindowHandle,
		Folder:             d.startingFolder(),
		DefaultFolder:      d.startingFolder(),
	}

	result, err := cfdutil.ShowPickFolderDialog(config)
	if err == cfd.ErrorCancelled {
		callback(nil, nil)
		return
	}
	if err != nil {
		callback(nil, err)
		return
	}

	uri := storage.NewFileURI(result)
	callback(storage.ListerForURI(uri))
}

func openFile(parentWindowHandle uintptr, callback func(fyne.URIReadCloser, error), d *FileDialog) {
	config := cfd.DialogConfig{
		Title:              nativeDialogTitle(d.confirmText, "Open File"),
		Role:               "fyne-file-open",
		ParentWindowHandle: parentWindowHandle,
		Folder:             d.startingFolder(),
		DefaultFolder:      d.startingFolder(),
		FileFilters:        nativeFileFilters(d.filter),
	}

	result, err := cfdutil.ShowOpenFileDialog(config)
	if err == cfd.ErrorCancelled {
		callback(nil, nil)
		return
	}
	if err != nil {
		callback(nil, err)
		return
	}

	uri := storage.NewFileURI(result)
	callback(storage.Reader(uri))
}

func openSave(parentWindowHandle uintptr, callback func(fyne.URIWriteCloser, error), d *FileDialog) {
	config := cfd.DialogConfig{
		Title:              nativeDialogTitle(d.confirmText, "Save File"),
		Role:               "fyne-file-save",
		ParentWindowHandle: parentWindowHandle,
		Folder:             d.startingFolder(),
		DefaultFolder:      d.startingFolder(),
		FileName:           d.initialFileName,
		DefaultExtension:   fileExtension(d.initialFileName),
		FileFilters:        nativeFileFilters(d.filter),
	}

	result, err := cfdutil.ShowSaveFileDialog(config)
	if err == cfd.ErrorCancelled {
		callback(nil, nil)
		return
	}
	if err != nil {
		callback(nil, err)
		return
	}

	uri := storage.NewFileURI(result)
	callback(storage.Writer(uri))
}

// parentWindowHandle returns the HWND of the Fyne parent window so the native
// dialog can be shown modally over it. It returns 0 when there is no parent or
// the window does not expose a native handle.
func parentWindowHandle(d *FileDialog) uintptr {
	if d.parent == nil {
		return 0
	}
	window, ok := d.parent.(driver.NativeWindow)
	if !ok {
		return 0
	}

	var hwnd uintptr
	window.RunNative(func(context any) {
		hwnd = context.(driver.WindowsWindowContext).HWND
	})
	return hwnd
}

// startingFolder returns the path of the dialog's starting location, or an
// empty string when none has been set.
func (d *FileDialog) startingFolder() string {
	if d.startingLocation != nil {
		return d.startingLocation.Path()
	}
	return ""
}

// nativeDialogTitle uses the user-provided confirm text as the dialog title,
// falling back to the supplied default when it is empty.
func nativeDialogTitle(confirmText, fallback string) string {
	if confirmText != "" {
		return confirmText
	}
	return fallback
}

// nativeFileFilters converts a Fyne storage.FileFilter into the file filters
// understood by the native Windows dialog. A nil or unrecognised filter results
// in no specific filters being set (the native dialog defaults to all files).
func nativeFileFilters(filter storage.FileFilter) []cfd.FileFilter {
	switch filter := filter.(type) {
	case *storage.ExtensionFileFilter:
		if len(filter.Extensions) == 0 {
			return nil
		}
		var patterns []string
		for _, ext := range filter.Extensions {
			ext = strings.TrimPrefix(ext, ".")
			if ext != "" {
				patterns = append(patterns, "*."+ext)
			}
		}
		return []cfd.FileFilter{{
			DisplayName: strings.Join(patterns, ", "),
			Pattern:     strings.Join(patterns, ";"),
		}}
	case *storage.MimeTypeFileFilter:
		var patterns []string
		for _, mimeType := range filter.MimeTypes {
			patterns = appendMimePatterns(patterns, mimeType)
		}
		if len(patterns) == 0 {
			return nil
		}
		return []cfd.FileFilter{{
			DisplayName: strings.Join(patterns, ", "),
			Pattern:     strings.Join(patterns, ";"),
		}}
	}
	return nil
}

// appendMimePatterns expands a mime type (optionally containing a glob, e.g.
// "image/*") into the corresponding list of file extension patterns.
func appendMimePatterns(patterns []string, mimeType string) []string {
	mimeType = strings.TrimSpace(strings.ToLower(mimeType))
	mimeType, _, _ = strings.Cut(mimeType, ";")

	if mimeType == "*" || mimeType == "application/octet-stream" {
		return append(patterns, "*.*")
	}

	kind, subtype, found := strings.Cut(mimeType, "/")
	if !found {
		return patterns
	}

	extensions := mimeExtensions[subtype]
	if subtype == "*" {
		extensions = mimeGroupExtensions[kind]
	}
	for _, ext := range extensions {
		patterns = append(patterns, "*."+ext)
	}
	return patterns
}

// mimeGroupExtensions maps mime type groups (e.g. "image") to their common file
// extensions; used to expand glob filters such as "image/*".
var mimeGroupExtensions = map[string][]string{
	"image":       {"png", "jpg", "jpeg", "gif", "bmp", "webp", "tiff", "svg"},
	"audio":       {"mp3", "wav", "ogg", "flac", "m4a", "aac"},
	"video":       {"mp4", "webm", "avi", "mov", "mkv"},
	"text":        {"txt", "md", "csv", "html", "xml", "json"},
	"application": {"txt", "pdf", "zip", "gz", "json"},
}

// mimeExtensions maps specific mime subtypes to their common file extensions.
var mimeExtensions = map[string][]string{
	"jpeg":       {"jpg", "jpeg"},
	"png":        {"png"},
	"gif":        {"gif"},
	"bmp":        {"bmp"},
	"webp":       {"webp"},
	"svg+xml":    {"svg"},
	"tiff":       {"tif", "tiff"},
	"mpeg":       {"mp3"},
	"wav":        {"wav"},
	"ogg":        {"ogg", "oga"},
	"flac":       {"flac"},
	"mp4":        {"mp4", "m4v"},
	"x-msvideo":  {"avi"},
	"quicktime":  {"mov"},
	"webm":       {"webm"},
	"x-matroska": {"mkv"},
	"plain":      {"txt"},
	"markdown":   {"md"},
	"csv":        {"csv"},
	"html":       {"html", "htm"},
	"xml":        {"xml"},
	"json":       {"json"},
	"pdf":        {"pdf"},
	"zip":        {"zip"},
	"gzip":       {"gz"},
	"x-gzip":     {"gz"},
}

// fileExtension returns the extension (without the leading dot) of a file name.
func fileExtension(name string) string {
	return strings.TrimPrefix(filepath.Ext(name), ".")
}

func getFavoriteLocations() (map[string]fyne.ListableURI, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	homeURI := storage.NewFileURI(homeDir)

	favoriteNames := getFavoriteOrder()
	home, _ := storage.ListerForURI(homeURI)
	favoriteLocations := map[string]fyne.ListableURI{
		"Home": home,
	}
	for _, favName := range favoriteNames {
		uri, err1 := storage.Child(homeURI, favName)
		if err1 != nil {
			err = err1
			continue
		}

		listURI, err1 := storage.ListerForURI(uri)
		if err1 != nil {
			err = err1
			continue
		}
		favoriteLocations[favName] = listURI
	}

	return favoriteLocations, err
}
