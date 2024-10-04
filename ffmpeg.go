package main

import (
	"bufio"
	"fmt"
	"os/exec"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type commander interface {
	String() string
	CmdSlice() []string
}

type command_mkdir struct {
	dir string
}

func (c command_mkdir) String() string {
	return fmt.Sprintf("mkdir -p '%s'", c.dir)
}

func (c command_mkdir) CmdSlice() []string {
	return []string{"mkdir", "-p", c.dir}
}

type command_ffmpeg_filetoimgs struct {
	file   string
	folder string
	mode   int
}

func (c command_ffmpeg_filetoimgs) String() string {
	if c.mode > 1 {
		return fmt.Sprintf(`ffmpeg -y -i '%s' -vf "select=not(mod(n\,%d))" -vsync vfr '%s/%%09d.png'`, c.file, c.mode, c.folder)
	} else {
		return fmt.Sprintf("ffmpeg -y -i '%s' '%s/%%09d.png'", c.file, c.folder)
	}
}

func (c command_ffmpeg_filetoimgs) CmdSlice() []string {
	if c.mode > 1 {
		return []string{"ffmpeg", "-i", c.file, "-vf", fmt.Sprintf(`select=not(mod(n\,%d))`, c.mode), "-vsync", "vfr", fmt.Sprintf("%s/%%09d.png", c.folder)}
	} else {
		return []string{"ffmpeg", "-y", "-i", c.file, fmt.Sprintf("%s/%%09d.png", c.folder)}
	}
}

type command_ffmpeg_imgstovideo struct {
	framerate  string
	basefolder string
	outfolder  string
}

// ffmpeg -framerate 30 -pattern_type glob -i 'output/*.png' -c:v libx264 -pix_fmt yuv420p merged.mp4
func (c command_ffmpeg_imgstovideo) String() string {
	return fmt.Sprintf("ffmpeg -y -framerate %s -pattern_type glob -i '%s/*.png' -c:v libx264 -pix_fmt yuv420p '%s/merged.mp4'", c.framerate, c.outfolder, c.basefolder)
}

func (c command_ffmpeg_imgstovideo) CmdSlice() []string {
	return []string{"ffmpeg", "-y", "-framerate", c.framerate, "-pattern_type", "glob", "-i", fmt.Sprintf("%s/*.png", c.outfolder), "-c:v", "libx264", "-pix_fmt", "yuv420p", fmt.Sprintf("%s/merged.mp4", c.basefolder)}
}

type command_ffmpeg_scalevideo struct {
	filein  string
	fileout string
	scalew  string
	scaleh  string
}

// ffmpeg -i merged.mp4 -vf scale=1920:1080 upscaled.mp4
func (c command_ffmpeg_scalevideo) String() string {
	return fmt.Sprintf("ffmpeg -y -i '%s' -vf scale=%s:%s -c:v libx264 '%s'", c.filein, c.scalew, c.scaleh, c.fileout)
}

func (c command_ffmpeg_scalevideo) CmdSlice() []string {
	return []string{"ffmpeg", "-y", "-i", c.filein, "-vf", fmt.Sprintf("scale=%s:%s", c.scalew, c.scaleh), "-c:v", "libx264", c.fileout}
}

type command_ffmpeg_audiofromvideotovideo struct {
	fileinaudio string
	fileinvideo string
	fileout     string
}

// ffmpeg -i videowithaudio.mp4 -i silentvideo.mp4 -map 0:a -map 1 -c copy output.mp4
func (c command_ffmpeg_audiofromvideotovideo) String() string {
	return fmt.Sprintf("ffmpeg -y -i '%s' -i '%s' -map 0:a -map 1 -c copy '%s'", c.fileinaudio, c.fileinvideo, c.fileout)
}

func (c command_ffmpeg_audiofromvideotovideo) CmdSlice() []string {
	return []string{"ffmpeg", "-y", "-i", c.fileinaudio, "-i", c.fileinvideo, "-map", "0:a", "-map", "1", "-c", "copy", c.fileout}
}

type command_ffmpeg_sidebyside struct {
	fileog  string
	filegen string
	fileout string
}

// fixme we could swap vstack by hstack when checking if more height than width
// ffmpeg -i upscaled.mp4 -i OG.webm -filter_complex vstack -c:v libx264 sidebyside.avi
func (c command_ffmpeg_sidebyside) String() string {
	return fmt.Sprintf("ffmpeg -y -i '%s' -i '%s' -filter_complex vstack -c:v libx264 '%s'", c.filegen, c.fileog, c.fileout)
}

func (c command_ffmpeg_sidebyside) CmdSlice() []string {
	return []string{"ffmpeg", "-y", "-i", c.filegen, "-i", c.fileog, "-filter_complex", "vstack", "-c:v", "libx264", c.fileout}
}

func (g *gui) runCommandsBlocking(cmdlines [][]string, output *widget.Entry) error {
	for _, cmdlist := range cmdlines {
		cmd := exec.Command(cmdlist[0], cmdlist[1:]...)
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return fmt.Errorf("%s 1: %w", cmdlist[0], err)
		}
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("%s 2: %w", cmdlist[0], err)
		}

		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			appendAndScroll(output, scanner.Text()+"\n")
		}
		// if err := scanner.Err(); err != nil {} // i dont think i really care
		if err := cmd.Wait(); err != nil {
			return fmt.Errorf("%s 3: %w", cmdlist[0], err)
		}
	}

	return nil
}

func (g *gui) commandler(cmdler commander, commandOutput *widget.Entry) fyne.CanvasObject {
	commandtext := widget.NewMultiLineEntry()
	setCommandText := func() {
		commandtext.Text = cmdler.String() // yes it changes
		commandtext.Refresh()
	}
	commandtext.OnChanged = func(_ string) { setCommandText() }
	commandtext.ActionItem = container.NewVBox(
		widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
			commandOutput.SetText("")
			err := g.runCommandsBlocking([][]string{cmdler.CmdSlice()}, commandOutput)
			if err != nil {
				dialog.ShowError(err, g.w)
				return
			}
			appendAndScroll(commandOutput, "\n\nGREAT SUCCESS")
		}),
		widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
			g.w.Clipboard().SetContent(commandtext.Text)
		}),
	)
	commandtext.Wrapping = fyne.TextWrapBreak
	commandtext.SetMinRowsVisible(4) // min for ActionItem to look nice (i am doing something funky)
	setCommandText()

	return commandtext
}
