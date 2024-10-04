package main

import (
	"image/color"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

func appendAndScroll(we *widget.Entry, s string) {
	we.Append(s)
	we.TypedKey(&fyne.KeyEvent{Name: fyne.KeyPageDown})
}

type spinner struct {
	spin *widget.Activity
	d    *dialog.CustomDialog
}

func newLoadingSpinner(text string, w fyne.Window) *spinner {
	s := spinner{}

	s.spin = widget.NewActivity()
	prop := canvas.NewRectangle(color.Transparent)
	prop.SetMinSize(fyne.NewSize(80, 80))
	s.d = dialog.NewCustomWithoutButtons(text, container.NewStack(prop, s.spin), w)

	return &s
}

func (s *spinner) show() {
	s.spin.Start()
	s.d.Show()
}

func (s *spinner) hide() {
	s.d.Hide()
	s.spin.Stop()
}

// dont forget to defer uc.Close()
func (g *gui) openfile(title string, location *fyne.ListableURI, cb func(uc fyne.URIReadCloser) bool) *widget.Button {
	var buttonopen *widget.Button

	buttonopen = widget.NewButton(title, func() {
		d := dialog.NewFileOpen(func(uc fyne.URIReadCloser, err error) {
			if uc == nil || err != nil {
				return
			}

			if cb(uc) {
				buttonopen.Importance = widget.SuccessImportance
			} else {
				buttonopen.Importance = widget.WarningImportance
			}
			buttonopen.Refresh()
		}, g.w)

		if location != nil && *location != nil {
			d.SetLocation(*location)
		} else {
			d.SetLocation(currentPathAsURI(g.baseworkdir))
		}

		d.Show()
		d.Resize(d.MinSize().Add(d.MinSize()))
	})
	buttonopen.Importance = widget.WarningImportance

	return buttonopen
}

func filesinfolder(lu fyne.ListableURI) []fyne.URI {
	items, _ := lu.List()
	files := make([]fyne.URI, 0, len(items))
	for _, uri := range items {
		fileinfo, err := os.Lstat(uri.Path())
		if err != nil {
			continue
		}
		mode := fileinfo.Mode()

		if mode.IsRegular() {
			files = append(files, uri)
		}
	}
	return files
}

func (g *gui) openfolder(title string, location *fyne.ListableURI, cb func(fyne.ListableURI, []fyne.URI)) *widget.Button {
	return widget.NewButton(title, func() {
		d := dialog.NewFolderOpen(func(lu fyne.ListableURI, err error) {
			if err != nil || lu == nil {
				return
			}

			cb(lu, filesinfolder(lu))
		}, g.w)

		if location != nil && *location != nil {
			d.SetLocation(*location)
		} else {
			d.SetLocation(currentPathAsURI(g.baseworkdir))
		}

		d.Show()
		d.Resize(d.MinSize().Add(d.MinSize()))
	})
}

func currentPathAsURI(basepath string) fyne.ListableURI {
	// if we can open the current dir, do so
	rootdir, err := filepath.Abs(basepath)
	if err != nil {
		rootdir, err = filepath.Abs(".")
		if err != nil {
			return nil
		}
	}
	fileuri := storage.NewFileURI(rootdir)
	diruri, err := storage.ListerForURI(fileuri)
	if err != nil {
		return nil
	}

	return diruri
}

func pathToListableURI(path string) fyne.ListableURI {
	rootdir, err := filepath.Abs(path)
	if err != nil {
		return nil
	}
	fileuri := storage.NewFileURI(rootdir)
	diruri, err := storage.ListerForURI(fileuri)
	if err != nil {
		return nil
	}

	return diruri
}
