package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// returns a png image
func (g *gui) downloadFullImage(imageid string) []byte {
	s := fmt.Sprintf("/api/v1/images/i/%s/full", imageid)

	resp, err := g.servercall("GET", s, nil, nil)
	if err != nil {
		dialog.ShowError(err, g.w)
		return nil
	}
	defer resp.Body.Close()

	// we could also return the io.ReadCloser
	data, _ := io.ReadAll(resp.Body)
	return data
}

func (g *gui) downloadImageDetails(imageid string) (imageInfo, error) {
	s := fmt.Sprintf("/api/v1/images/i/%s", imageid)

	resp, err := g.servercall("GET", s, nil, nil)
	if err != nil {
		return imageInfo{}, err
	}
	defer resp.Body.Close()

	var ii imageInfo
	data, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(data, &ii)
	if err != nil {
		return imageInfo{}, err
	}
	return ii, nil
}

type imageInfo struct { //ret
	ImageName   string `json:"image_name"`
	ImageOrigin string `json:"image_origin"` // internal->generated, external->uploaded
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	BoardID     string `json:"board_id"`
}

func (ii imageInfo) String() string {
	return fmt.Sprintf("%s - %dx%d", ii.ImageName, ii.Width, ii.Height)
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func (g *gui) uploadImage(file fyne.URI, boardid string) (imageInfo, error) {
	var ii imageInfo
	pp := fmt.Sprintf("/api/v1/images/upload?image_category=user&is_intermediate=false&board_id=%s", boardid)

	rr, err := storage.Reader(file)
	if err != nil {
		return ii, fmt.Errorf("failed to read file: %w", err)
	}
	defer rr.Close()

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes("file"), escapeQuotes(file.Name())))
	h.Set("Content-Type", "image/png")
	fw, err := w.CreatePart(h)
	if err != nil {
		return ii, fmt.Errorf("failed to upload file: %w", err)
	}
	_, err = io.Copy(fw, rr)
	if err != nil {
		return ii, fmt.Errorf("failed to upload file: %w", err)
	}
	w.Close()

	headers := map[string][]string{
		"Content-Type": {w.FormDataContentType()},
	}

	resp, err := g.servercall("POST", pp, &b, headers)
	if err != nil {
		return ii, fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(data, &ii)
	if err != nil {
		return ii, fmt.Errorf("failed to upload file: %w", err)
	}

	return ii, nil
}

func (g *gui) downloadBoardGeneratedImages(boardid string, text *widget.Label, bar *widget.ProgressBar, parent fyne.ListableURI, numimages int) {
	// download all the files
	image_names := fmt.Sprintf("/api/v1/boards/%s/image_names", boardid)
	resp, err := g.servercall("GET", image_names, nil, nil)
	if err != nil {
		dialog.ShowError(err, g.w)
		return
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var newimages []string
	err = json.Unmarshal(data, &newimages)
	if err != nil {
		dialog.ShowError(err, g.w)
		return
	}
	//fmt.Println(newimages)

	// save them back to disk
	text.SetText("Downloading generated images and saving files to folder")
	outputfolder, err := storage.Child(parent, "output")
	if err != nil {
		dialog.ShowError(err, g.w)
		return
	}
	err = storage.CreateListable(outputfolder)
	if err != nil {
		dialog.ShowError(err, g.w)
		return
	}
	parent, err = storage.ListerForURI(outputfolder)
	if err != nil {
		dialog.ShowError(err, g.w)
		return
	}

	var i int
	for _, imgname := range newimages {
		ii, err := g.downloadImageDetails(imgname)
		if err != nil {
			dialog.ShowError(err, g.w)
			continue
		}

		if ii.ImageOrigin != "internal" {
			continue
		}

		img := g.downloadFullImage(imgname)

		filename := fmt.Sprintf("out_%09d.png", i)
		file, err := storage.Child(parent, filename)
		i++
		if err != nil {
			dialog.ShowError(err, g.w)
			continue
		}

		rw, err := storage.Writer(file)
		if err != nil {
			dialog.ShowError(err, g.w)
			continue
		}
		_, err = rw.Write(img)
		rw.Close()
		if err != nil {
			dialog.ShowError(err, g.w)
			continue
		}

		text.SetText(fmt.Sprintf("Downloading generated images and saving files to folder\nCurrent file: %s", filename))
		bar.SetValue(float64(i) / float64(numimages))
	}
}
