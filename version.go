package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

var supportedVersions = []string{
	"5.0.2",
	"5.0.0",
	"4.2.9",
}

type AppVersion struct {
	Version string `json:"version"`
}

func parseAppVersion(r io.Reader) (AppVersion, error) {
	d, err := io.ReadAll(r)
	if err != nil {
		return AppVersion{}, fmt.Errorf("failed to unmarshal version: %w", err)
	}

	var av AppVersion
	err = json.Unmarshal(d, &av)
	if err != nil {
		return AppVersion{}, fmt.Errorf("failed to unmarshal version: %w", err)
	}

	return av, nil
}

func (g *gui) makeConnect(lastserver string, success func(string)) fyne.CanvasObject {
	ipres := widget.NewEntry()
	ipres.Text = lastserver
	ipres.PlaceHolder = "127.0.0.1:9090"
	ipres.TextStyle = fyne.TextStyle{Monospace: true}
	ipres.Wrapping = fyne.TextWrapOff

	spin := newLoadingSpinner("Connecting...", g.w)

	form := widget.NewForm(
		widget.NewFormItem("IP Address", ipres),
	)
	form.SubmitText = "Connect"

	form.OnSubmit = func() {
		serveraddress := ipres.PlaceHolder
		if ipres.Text != "" {
			serveraddress = ipres.Text
		}

		invoke, err := url.Parse("/api/v1/app/version")
		if err != nil {
			dialog.ShowError(err, g.w)
			return
		}
		invoke.Scheme = "http"
		invoke.Host = serveraddress

		req, err := http.NewRequest("GET", invoke.String(), nil)
		if err != nil {
			dialog.ShowError(err, g.w)
			return
		}

		spin.show()
		// we cant g.servercall yet
		resp, err := g.h.Do(req)
		spin.hide()

		if err != nil {
			dialog.ShowError(err, g.w)
			return
		}
		defer resp.Body.Close()

		av, err := parseAppVersion(resp.Body)
		if err != nil {
			dialog.ShowError(err, g.w)
			return
		}

		for _, version := range supportedVersions {
			if version == av.Version {
				success(serveraddress)
				return
			}
		}

		dialog.ShowError(fmt.Errorf("Version %s is untested!\nThings might not work as expected, but i will let you continue anyway. Have fun.\n\nSupported Versions: %s", av.Version, strings.Join(supportedVersions, ", ")), g.w)
		success(serveraddress)
	}

	ready := make(chan struct{})
	if lastserver != "" {
		go func() {
			// should be good enough to not race
			// otherwise just do it properly
			<-ready
			runtime.Gosched()
			form.OnSubmit()
		}()
	}

	defer close(ready)
	return form
}
