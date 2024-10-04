package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	ffprobe "gopkg.in/vansante/go-ffprobe.v2"
)

type gui struct {
	a fyne.App
	w fyne.Window
	//
	h      http.Client
	server string
	//
	baseworkdir string
}

func main() {
	g := gui{}
	g.h = http.Client{}

	g.a = app.NewWithID("biehdc.priv.invokebatcher")
	g.w = g.a.NewWindow("InvokeAI Batcher")
	g.w.CenterOnScreen()
	g.w.Resize(fyne.NewSize(950, 620))
	g.baseworkdir = g.a.Preferences().StringWithFallback("baseworkdir", "")

	// you have no way to change server unless it cant connect
	// good enough for me
	lastserver := g.a.Preferences().String("lastserver")
	connectScreen := g.makeConnect(lastserver, func(serveraddress string) {
		g.server = serveraddress
		g.a.Preferences().SetString("lastserver", serveraddress)
		g.w.SetContent(container.NewAppTabs(
			container.NewTabItem("Workflow", g.videocontext()),
			container.NewTabItem("Misc", g.miscstuffmastertab()),
		))
	})
	g.w.SetContent(connectScreen)
	g.w.ShowAndRun()
}

type videoinfo struct {
	uri              fyne.URI
	basename         string
	folderwithimages fyne.ListableURI
	queuename        string
	boardname        string
	boardid          string
	//
	width  int
	height int
	fps    string
	//
	nat              *container.AppTabs
	downloadbutton   *widget.Button
	downloadtabindex int
}

func (g *gui) videocontext() fyne.CanvasObject {
	videofile := widget.NewLabel("")

	center := container.NewStack()
	setcenter := func(co fyne.CanvasObject) {
		center.Objects = []fyne.CanvasObject{co}
		center.Refresh()
	}
	setcenter(container.NewCenter(widget.NewLabel("Nothing loaded yet")))

	videofiledialog := g.openfile("Open Video File", nil, func(uc fyne.URIReadCloser) bool {
		defer uc.Close()

		data, err := ffprobe.ProbeReader(context.TODO(), uc)
		if err != nil {
			dialog.ShowError(err, g.w)
			setcenter(container.NewCenter(widget.NewLabel("Nothing loaded")))
			videofile.SetText("")
			return false
		}

		video := data.FirstVideoStream()
		if video == nil {
			dialog.ShowError(errors.New("no video stream in file"), g.w)
			setcenter(container.NewCenter(widget.NewLabel("Nothing loaded")))
			videofile.SetText("")
			return false
		}

		vi := videoinfo{}
		vi.uri = uc.URI()
		vi.basename = strings.TrimRight(vi.uri.Name(), vi.uri.Extension())
		vi.queuename = fmt.Sprintf("q_%s", vi.basename)
		vi.boardname = fmt.Sprintf("batcher_%s", vi.basename)
		//fmt.Println([]string{vi.basename, vi.queuename, vi.boardname})
		//fmt.Printf("len %v\n", data.Format.DurationSeconds)
		//fmt.Printf("%dx%d @ %s fps\n", video.Width, video.Height, video.AvgFrameRate)

		videofile.SetText(fmt.Sprintf("file: %s\n%dx%d at %s fps and %s duration", vi.uri.Path(), video.Width, video.Height, video.AvgFrameRate, data.Format.Duration()))
		vi.width = video.Width
		vi.height = video.Height
		vi.fps = video.AvgFrameRate

		vi.nat = container.NewAppTabs(
			container.NewTabItem("Step 1 - Preperation", g.step1_preparation(&vi)),
			container.NewTabItem("Step 2 - DisBatch", g.step2_batchme(&vi)),
			container.NewTabItem("Step 3 - Watch Progess", g.step3_watchme(&vi)),
			container.NewTabItem("Step 4 - Download", g.step4_downloadimagesinboard(&vi)),
			container.NewTabItem("Step 5 - Reassemble", g.step5_reassemble(&vi)),
			container.NewTabItem("Step 6 - Cleanup", g.step6_cleanupleftovers(&vi)),
		)

		vi.downloadtabindex = 3 // keep in sync with above (0-indexed)
		vi.nat.OnSelected = func(ti *container.TabItem) {
			// this exists to make the thing come alive once the output is downloaded
			if ti.Text == "Step 5 - Reassemble" {
				ti.Content = g.step5_reassemble(&vi)
			}
		}

		setcenter(vi.nat)

		return true
	})

	content := container.NewBorder(
		container.NewVBox(
			videofiledialog,
			videofile,
		),
		nil, nil, nil, center,
	)

	return content
}

func (g *gui) step1_preparation(vi *videoinfo) fyne.CanvasObject {
	commandOutput := widget.NewMultiLineEntry()
	commandOutput.TextStyle = fyne.TextStyle{Monospace: true}
	commandOutput.Wrapping = fyne.TextWrapBreak
	extractallframes := widget.NewMultiLineEntry()

	folder := strings.TrimRight(vi.uri.Path(), vi.uri.Extension())
	mkdir := command_mkdir{dir: folder}
	ffmpeg := command_ffmpeg_filetoimgs{
		folder: folder,
		file:   vi.uri.Path(),
		mode:   1,
	}

	// this is going to fail if its not yet extracted, but success
	// if it already was
	vi.folderwithimages = pathToListableURI(folder)
	//fmt.Println(vi.queuename)

	currentthing := mkdir.String() + "; " + ffmpeg.String()
	setCommand := func() {
		currentthing = mkdir.String() + "; " + ffmpeg.String() // yes it changes
		extractallframes.Text = currentthing
		extractallframes.Refresh()
	}

	mode := widget.NewEntry()
	mode.Text = "1"
	mode.Validator = func(s string) error {
		theint, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		if theint < 1 {
			return errors.New("number must be one or greater")
		}
		ffmpeg.mode = theint
		return nil
	}
	mode.OnChanged = func(_ string) {
		if mode.Validate() != nil {
			extractallframes.Disable()
			extractallframes.ActionItem.Hide()
		} else {
			extractallframes.Enable()
			extractallframes.ActionItem.Show()
			setCommand()
		}
	}

	extractallframes.Text = currentthing
	extractallframes.OnChanged = func(_ string) { setCommand() }
	extractallframes.ActionItem = container.NewVBox(
		widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
			err := g.runCommandsBlocking([][]string{mkdir.CmdSlice(), ffmpeg.CmdSlice()}, commandOutput)
			if err != nil {
				dialog.ShowError(err, g.w)
				return
			}
			appendAndScroll(commandOutput, "\n\nGREAT SUCCESS")
			vi.folderwithimages = pathToListableURI(folder)
			//fmt.Println(vi.folderwithimages)
		}),
		widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
			g.w.Clipboard().SetContent(extractallframes.Text)
		}),
	)
	extractallframes.Wrapping = fyne.TextWrapBreak
	extractallframes.SetMinRowsVisible(4) // min for ActionItem to look nice (i am doing something funky)

	return container.NewBorder(container.NewVBox(
		widget.NewLabel("Prepare the video for batch processing by turning it into individual images"),
		widget.NewLabel("Set frame extraction\n1 extracts every frame from the video, higher numbers every Nth frame. Eg: 10 extracts every 10th frame"),
		mode,
		extractallframes,
	), nil, nil, nil, commandOutput)

}

func (g *gui) step2_batchme(vi *videoinfo) fyne.CanvasObject {
	bar := widget.NewProgressBar()
	text := widget.NewLabel("")

	// load the batch json
	var jsonfile []byte
	jobfile := widget.NewLabel("None")

	var jobfilelocation fyne.ListableURI
	jobfiledialog := g.openfile("Open JSON file", nil, func(uc fyne.URIReadCloser) bool {
		defer uc.Close()

		var err error
		jsonfile, err = io.ReadAll(uc)
		if err != nil {
			dialog.ShowError(err, g.w)
			jsonfile = nil
			return false
		}

		jobfile.SetText(uc.URI().String())
		// assume the desc is next to the json
		jobfilelocation = pathToListableURI(filepath.Dir(uc.URI().Path()))
		return true
	})

	// fixme introduce a specfile and make the widgets automatically
	// no usecase found yet as i only do static tier img2img
	oldboardid := widget.NewEntry()
	oldboardid.PlaceHolder = "xxxxxxxx-yyyy-zzzz-aaaa-bbbbbbbbbbbb"
	oldboardid.TextStyle = fyne.TextStyle{Monospace: true}

	ogimageid := widget.NewEntry()
	ogimageid.PlaceHolder = "kkkkkkkk-bbbb-uuuu-tttt-rrrrrrrrrrrr.png"
	ogimageid.TextStyle = fyne.TextStyle{Monospace: true}
	//^ load from file
	filedescriptor := widget.NewLabel("None")
	jobfiledescriptor := g.openfile("Open JSON file descriptor from CSV", &jobfilelocation, func(uc fyne.URIReadCloser) bool {
		defer uc.Close()

		r := csv.NewReader(uc)
		r.Comma = ';'
		r.Comment = '#'
		r.FieldsPerRecord = 2
		fields, err := r.ReadAll()
		if err != nil {
			dialog.ShowError(fmt.Errorf("error reading descriptor from csv: %w", err), g.w)
			return false
		}
		for _, row := range fields {
			switch row[0] {
			case "board_id":
				oldboardid.SetText(row[1])
			case "image_id":
				ogimageid.SetText(row[1])
			default:
				dialog.ShowError(fmt.Errorf("unknown field: %s:%s", row[0], row[1]), g.w)
			}
		}

		filedescriptor.SetText(uc.URI().String())
		return true
	})

	dispatch := widget.NewButton("Go", func() {
		if jobfiledialog.Importance != widget.SuccessImportance {
			dialog.ShowError(errors.New("Please open the enqueue_batch.json"), g.w)
			return
		}
		if len(oldboardid.PlaceHolder) != len(oldboardid.Text) {
			dialog.ShowError(errors.New("Invalid board id to replace"), g.w)
			return
		}

		// create the board and replace the board in the file with the new one
		text.SetText("Making Board")
		board, err := g.makeBoard(vi.boardname)
		if err != nil {
			dialog.ShowError(err, g.w)
			return
		}
		vi.boardid = board.Board_id
		jsonfilestr := strings.ReplaceAll(string(jsonfile), oldboardid.Text, board.Board_id)

		files := filesinfolder(vi.folderwithimages)
		filelistlen := len(files)
		filelist := make([]string, 0, filelistlen)
		// upload the files and
		// submit job to queue
		text.SetText("Uploading Files")
		for i, file := range files {
			ii, err := g.uploadImage(file, board.Board_id)
			if err != nil {
				text.SetText(err.Error())
				dialog.ShowError(err, g.w)
				continue
			}
			filelist = append(filelist, ii.ImageName)
			text.SetText(fmt.Sprintf("Uploading Files: %s", file.Name()))
			bar.SetValue(float64(i) / float64(filelistlen))
		}

		enqueu_batch := fmt.Sprintf("/api/v1/queue/%s/enqueue_batch", vi.queuename)
		text.SetText("Submitting Jobs")
		// apparently i cant upload and batch at the same time
		// throws use of closed connection
		for i, filename := range filelist {
			uniqfile := strings.ReplaceAll(strings.Clone(jsonfilestr), ogimageid.Text, filename)
			resp, err := g.servercall("POST", enqueu_batch, strings.NewReader(uniqfile), nil)
			if err != nil {
				text.SetText(err.Error())
				dialog.ShowError(err, g.w)
				continue
			}
			resp.Body.Close()

			text.SetText(fmt.Sprintf("Submitting Jobs: %s", filename))
			bar.SetValue(float64(i) / float64(filelistlen))
		}

		text.SetText("Done")
		bar.SetValue(1)
	})

	return container.NewVBox(
		jobfiledialog, jobfile,
		widget.NewSeparator(),
		jobfiledescriptor, filedescriptor,
		widget.NewForm(
			widget.NewFormItem("Board ID to replace", oldboardid),
			widget.NewFormItem("Image ID to replace", ogimageid),
		),
		widget.NewSeparator(),
		dispatch,
		widget.NewSeparator(),
		bar, text,
	)
}

func (g *gui) step3_watchme(vi *videoinfo) fyne.CanvasObject {
	barinf := widget.NewProgressBarInfinite()
	barpro := widget.NewProgressBar()
	barpro.TextFormatter = func() string { return "" }
	barcontainer := container.NewStack(barpro)
	text := widget.NewLabel("")

	downloadafterfinish := widget.NewCheck("Download after finish", func(_ bool) {})

	stillprocessing := errors.New("still processing")
	pollonce := func() error {
		queue_status := fmt.Sprintf("/api/v1/queue/%s/status", vi.queuename)
		resp, err := g.servercall("GET", queue_status, nil, nil)
		if err != nil {
			return err
		}
		status, err := parseStatus(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		if status.Queue.Pending == 0 && status.Queue.InProgress == 0 {
			text.SetText(fmt.Sprintf("Queue %s finished processing", vi.queuename))
			return nil //break
		}

		text.SetText(fmt.Sprintf("Images left to process: %d", status.Queue.Pending+status.Queue.InProgress))
		return stillprocessing
	}

	var isrunning bool
	watcher := func() {
		barinf.Start()
		barcontainer.Objects = []fyne.CanvasObject{barinf}
		barcontainer.Refresh()
		//starttime := time.Now()
		text.SetText("Wait for processing")
		tickme := time.NewTicker(5 * time.Second)
		defer tickme.Stop()
		for {
			// this works but is not fool proof
			<-tickme.C
			//fmt.Println("time to query", t.Unix())
			if !isrunning {
				// we should stop
				barcontainer.Objects = []fyne.CanvasObject{barpro}
				barcontainer.Refresh()
				return
			}

			err := pollonce()
			if err == stillprocessing {
				continue
			}
			if err != nil {
				dialog.ShowError(err, g.w)
				isrunning = false
				return // consider this fatal
			}

			isrunning = false
			barinf.Stop()
			barpro.SetValue(1)
			barcontainer.Objects = []fyne.CanvasObject{barpro}
			barcontainer.Refresh()
			if downloadafterfinish.Checked {
				// switch to download tab
				vi.nat.SelectIndex(vi.downloadtabindex)
				// trigger the button there
				vi.downloadbutton.Tapped(&fyne.PointEvent{})
			}
			return // means we are done processing
		}
	}

	checkstatus := widget.NewButton("Toggle Watcher", func() {
		if isrunning {
			isrunning = false
		} else {
			//start
			isrunning = true
			go watcher()
		}
	})

	cancelqueue := widget.NewButtonWithIcon("Cancel Jobs", theme.CancelIcon(), func() {
		dialog.ShowConfirm("Cancel Jobs?", "Are you sure you want to cancel the jobs? This can not be undone!",
			func(b bool) {
				if !b {
					return
				}
				isrunning = false
				queue_cancel := fmt.Sprintf("/api/v1/queue/%s/clear", vi.queuename)
				resp, err := g.servercall("PUT", queue_cancel, nil, nil)
				if err != nil {
					dialog.ShowError(err, g.w)
					return
				}
				clear, err := parseClear(resp.Body)
				resp.Body.Close()
				if err != nil {
					dialog.ShowError(err, g.w)
					return
				}

				text.SetText(fmt.Sprintf("Jobs Cancelled! Amount: %d", clear.Deleted))
			}, g.w)
	})
	cancelqueue.Importance = widget.DangerImportance

	// "/api/v1/queue/{queue_id}/processor/resume"
	// "/api/v1/queue/{queue_id}/processor/pause"
	// maybe add those too if ever required
	// case would be you are using the webinterface while
	// a job runs and you dont want to force the front of
	// the queue all the time

	return container.NewVBox(
		downloadafterfinish,
		checkstatus,
		widget.NewSeparator(),
		barcontainer, text,
		widget.NewSeparator(),
		widget.NewLabel("Use this button to cancel the batchjob"),
		cancelqueue,
	)
}

func (g *gui) step4_downloadimagesinboard(vi *videoinfo) fyne.CanvasObject {
	bar := widget.NewProgressBar()
	text := widget.NewLabel("")

	// make a go button to start downloading
	vi.downloadbutton = widget.NewButton("Go", func() {
		err := g.getBoardID(vi)
		if err != nil {
			text.SetText(err.Error())
			return
		}

		if vi.boardid == "not found" {
			dialog.ShowError(fmt.Errorf("no board id found for %s", vi.boardname), g.w)
			return
		}

		if vi.folderwithimages == nil {
			dialog.ShowError(fmt.Errorf("please do step 1 first"), g.w)
			return
		}

		bar.SetValue(0)
		text.SetText("Downloading files")

		files := filesinfolder(vi.folderwithimages)
		g.downloadBoardGeneratedImages(vi.boardid, text, bar, vi.folderwithimages, len(files))

		bar.SetValue(1)
		text.SetText("FINISHED")
	})

	return container.NewVBox(
		vi.downloadbutton,
		widget.NewSeparator(),
		bar, text,
	)
}

func (g *gui) step5_reassemble(vi *videoinfo) fyne.CanvasObject {
	if vi.folderwithimages == nil {
		return widget.NewLabel("Please do Step 1")
	}

	generatedimages := pathToListableURI(filepath.Join(vi.folderwithimages.Path(), "output"))
	if generatedimages == nil {
		return widget.NewLabel("Please download the generated images in Step 4 first")
	}

	commandOutput := widget.NewMultiLineEntry()
	commandOutput.TextStyle = fyne.TextStyle{Monospace: true}
	commandOutput.Wrapping = fyne.TextWrapBreak

	// videos out folder
	mkdir := command_mkdir{
		dir: filepath.Join(vi.folderwithimages.Path(), "videos"),
	}
	err := g.runCommandsBlocking([][]string{mkdir.CmdSlice()}, commandOutput)
	if err != nil {
		return widget.NewLabel(fmt.Sprintf("could not make video output dir: %s", err))
	}

	videoutputfolder := mkdir.dir

	// merge
	ffmpegmerge := command_ffmpeg_imgstovideo{
		framerate:  vi.fps,
		basefolder: videoutputfolder,
		outfolder:  generatedimages.Path(),
	}
	mergeframes := g.commandler(ffmpegmerge, commandOutput)

	// if merged exist, offer scale to og res
	ffmpegscale := command_ffmpeg_scalevideo{
		filein:  filepath.Join(videoutputfolder, "merged.mp4"),
		fileout: filepath.Join(videoutputfolder, "scaled.mp4"),
		scalew:  strconv.Itoa(vi.width),
		scaleh:  strconv.Itoa(vi.height),
	}
	scalevideo := g.commandler(ffmpegscale, commandOutput)

	// offer merge in og audio
	ffmpegaudio := command_ffmpeg_audiofromvideotovideo{
		fileinaudio: vi.uri.Path(),
		fileinvideo: filepath.Join(videoutputfolder, "scaled.mp4"),
		fileout:     filepath.Join(videoutputfolder, "genwithaudio.mp4"),
	}
	mergeaudio := g.commandler(ffmpegaudio, commandOutput)

	// side by side (more like top by bottom)
	ffmpegside := command_ffmpeg_sidebyside{
		fileog:  vi.uri.Path(),
		filegen: filepath.Join(videoutputfolder, "scaled.mp4"),
		fileout: filepath.Join(videoutputfolder, "sidebyside.mp4"),
	}
	sidebyside := g.commandler(ffmpegside, commandOutput)

	return container.NewBorder(
		container.NewAppTabs(
			container.NewTabItem("1 - Merge Frames to Video", mergeframes),
			container.NewTabItem("2 - Scale Video to Original Resolution", scalevideo),
			container.NewTabItem("3.1 - Add Audio", mergeaudio),
			container.NewTabItem("3.2 - Side by Side with Original", sidebyside),
		), nil, nil, nil, commandOutput)
}

func (g *gui) step6_cleanupleftovers(vi *videoinfo) fyne.CanvasObject {
	bar := widget.NewProgressBar()
	text := widget.NewLabel("")

	// make a delete board button
	deleteboard := widget.NewButtonWithIcon("Delete Board", theme.DeleteIcon(), func() {
		dialog.ShowConfirm(
			"Delete Board?",
			fmt.Sprintf("Are you sure you want to delete the board named %s?", vi.boardname), func(b bool) {
				if !b {
					return
				}
				// delete the temporary board including the images
				text.SetText(fmt.Sprintf("Deleting temporary board named %s", vi.boardname))
				bar.SetValue(0)

				err := g.getBoardID(vi)
				if err != nil {
					text.SetText(err.Error())
					return
				}

				delete_board := fmt.Sprintf("/api/v1/boards/%s?include_images=true", vi.boardid)
				resp, err := g.servercall("DELETE", delete_board, nil, nil)
				if err != nil {
					dialog.ShowError(err, g.w)
					return
				}
				resp.Body.Close()

				text.SetText(fmt.Sprintf("Deleted %s", vi.boardname))
				bar.SetValue(1)

				vi.boardname = "deleted"
				vi.boardid = "deleted"
			}, g.w)
	})
	deleteboard.Importance = widget.DangerImportance

	return container.NewVBox(
		widget.NewLabel("Delete the processing board inside InvokeAI"),
		deleteboard,
		widget.NewSeparator(),
		bar, text,
	)
}
