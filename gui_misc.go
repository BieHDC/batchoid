package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	ffprobe "gopkg.in/vansante/go-ffprobe.v2"
)

func (g *gui) miscstuffmastertab() fyne.CanvasObject {
	return container.NewAppTabs(
		container.NewTabItem("Rescale Video", g.rescaleVideo()),
		container.NewTabItem("Check Dependencies", g.checkffmpegexists()),
		container.NewTabItem("Prettify JSON", g.prettifyJSON()),
		container.NewTabItem("Set Base Work Dir", g.setbaseworkdir()),
		container.NewTabItem("Other FFMPEG Commands", widget.NewLabel("None so far...")),
		container.NewTabItem("Obtain enqueue_batch", getbatcher()),
	)
}

func getbatcher() fyne.CanvasObject {
	return widget.NewRichTextFromMarkdown(`1. Prepare your image to image generation you want to apply to every batch target.
2. Make a new dummy board the image will be generated into. We need this to populate the board_id field in the json.
3. Open the Developer Console with F12.
4. Hit "Invoke" to trigger the generation.
5. Select the POST method with "enqueue_batch".
6. Select "Request" and change the RAW slider to display raw text.
7. Triple-Tap the text line and copy it.
8. Open this Program, select the Misc Tab and select "Prettify JSON".
9. Hit "Paste JSON from Clipboard".
10. Hit "Copy Processed JSON to Clipboard".
11. Save the contents to a file.json.
12. Open the file.
13. Search for "board_id". If it doesnt show up, you failed at step 2.
14. Make a new file called the same as in 11. but change the line ending to .csv.
15. Copy the GUID from 13. and save it to the csv file like "board_id;xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx".
16. Now search the ".png" and copy it's GUID aswell and save it to the csv like "image_id;yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy.png"
17. Save the file.
18. All Preperation is done.`)
}

func (g *gui) setbaseworkdir() fyne.CanvasObject {
	expl := widget.NewLabel("This will be the default directory to be open in file and folder pickers")
	dirpicker := g.openfolder("Set Base Work Directory", nil, func(lu fyne.ListableURI, u []fyne.URI) {
		g.baseworkdir = lu.Path()
		g.a.Preferences().SetString("baseworkdir", g.baseworkdir)
	})
	reset := widget.NewButton("Reset Set Base Work Directory to nothing", func() {
		g.baseworkdir = ""
		g.a.Preferences().SetString("baseworkdir", "")
	})

	return container.NewVBox(expl, dirpicker, reset)
}

func (g *gui) rescaleVideo() fyne.CanvasObject {
	commandOutput := widget.NewMultiLineEntry()
	commandOutput.TextStyle = fyne.TextStyle{Monospace: true}

	var rescale command_ffmpeg_scalevideo
	rescalecontainer := container.NewStack()

	setCommand := func(co fyne.CanvasObject) {
		rescalecontainer.Objects = []fyne.CanvasObject{co}
		rescalecontainer.Refresh()
	}
	setCommand(g.commandler(rescale, commandOutput))

	newwidth := widget.NewEntry()
	newwidth.TextStyle = fyne.TextStyle{Monospace: true}
	newwidth.OnChanged = func(s string) {
		rescale.scalew = s
		setCommand(g.commandler(rescale, commandOutput))
	}

	newheight := widget.NewEntry()
	newheight.TextStyle = fyne.TextStyle{Monospace: true}
	newheight.OnChanged = func(s string) {
		rescale.scaleh = s
		setCommand(g.commandler(rescale, commandOutput))
	}

	sourcevideoopen := g.openfile("Source Video", nil, func(uc fyne.URIReadCloser) bool {
		defer uc.Close()
		rescale = command_ffmpeg_scalevideo{}

		data, err := ffprobe.ProbeReader(context.TODO(), uc)
		if err != nil {
			dialog.ShowError(err, g.w)
			return false
		}

		video := data.FirstVideoStream()
		if video == nil {
			dialog.ShowError(errors.New("no video stream in file"), g.w)
			return false
		}

		rescale.filein = uc.URI().Path()
		rescale.fileout = strings.TrimRight(uc.URI().Path(), uc.URI().Extension()) + "_scaled.mp4"
		rescale.scalew = fmt.Sprintf("%d", video.Width)
		rescale.scaleh = fmt.Sprintf("%d", video.Height)

		newwidth.SetText(fmt.Sprintf("%d", video.Width))
		newheight.SetText(fmt.Sprintf("%d", video.Height))

		setCommand(g.commandler(rescale, commandOutput))
		commandOutput.SetText("")

		return true
	})

	return container.NewBorder(
		container.NewVBox(
			sourcevideoopen,
			widget.NewForm(
				widget.NewFormItem("New Width", newwidth),
				widget.NewFormItem("New Height", newheight),
			),
			rescalecontainer,
		), nil, nil, nil, commandOutput)
}

func (g *gui) prettifyJSON() fyne.CanvasObject {
	output := widget.NewMultiLineEntry()
	output.TextStyle = fyne.TextStyle{Monospace: true}

	process := func(b []byte) bool {
		var asPretty bytes.Buffer
		err := json.Indent(&asPretty, b, "", "\t")
		if err != nil {
			dialog.ShowError(err, g.w)
			return false
		}

		output.SetText(asPretty.String())
		return true
	}

	pastefromclipboard := widget.NewButton("Paste JSON from Clipboard", func() {
		process([]byte(g.w.Clipboard().Content()))
	})

	copytoclipboard := widget.NewButton("Copy Processed JSON to Clipboard", func() {
		g.w.Clipboard().SetContent(output.Text)
	})

	jsonfile := g.openfile("JSON File", nil, func(uc fyne.URIReadCloser) bool {
		defer uc.Close()

		jsonbytes, err := io.ReadAll(uc)
		if err != nil {
			dialog.ShowError(err, g.w)
			return false
		}

		return process(jsonbytes)
	})
	jsonfile.Importance = widget.MediumImportance

	return container.NewBorder(
		container.NewBorder(
			nil, nil, container.NewHBox(jsonfile, pastefromclipboard), copytoclipboard,
		),
		nil, nil, nil, output)
}

func (g *gui) checkffmpegexists() fyne.CanvasObject {
	checkExists := func(s string) bool {
		_, err := exec.LookPath(s)
		if err != nil {
			return false
		} else {
			return true
		}
	}

	ffmpeg := widget.NewLabel("FFMPEG Status: ?")
	ffprobe := widget.NewLabel("FFPROBE Status: ?")
	check := widget.NewButton("Check Dependencies", func() {
		mpeg := checkExists("ffmpeg")
		if mpeg {
			ffmpeg.SetText("FFMPEG Status: Success")
		} else {
			ffmpeg.SetText("FFMPEG Status: Fail")
		}

		probe := checkExists("ffprobe")
		if probe {
			ffprobe.SetText("FFPROBE Status: Success")
		} else {
			ffprobe.SetText("FFPROBE Status: Fail")
		}
	})
	check.Tapped(&fyne.PointEvent{}) // query right away

	return container.NewVBox(
		check, ffmpeg, ffprobe,
	)
}
