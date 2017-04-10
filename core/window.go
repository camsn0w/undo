package core

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	"github.com/mibk/syd/ui"
	"github.com/mibk/syd/ui/term"
)

const EOF = utf8.MaxRune + 1

type Window struct {
	ed       *Editor
	filename string
	win      *term.Window
	con      Content

	buf  *UndoBuffer
	head *Text
	body *Text
}

func (win *Window) SetFilename(filename string) {
	win.filename = filename
	win.head.buf.Insert(0, filename)
	// TODO: Move the cursor to the end of the line.
}

// Size returns the size of win.
func (win *Window) Size() (w, h int) { return win.win.Size() }

func (win *Window) Frame() *term.Frame { return win.body.text.Frame() } // TODO: delete

func (win *Window) LoadText() {
	win.win.Clear()
	win.head.loadText()
	win.body.loadText()
}

func (win *Window) Undo() { win.buf.Undo() }
func (win *Window) Redo() { win.buf.Redo() }

func (win *Window) Close() error {
	win.win.Delete()
	win.ed.deleteWindow(win)
	return win.con.Close()
}

func (win *Window) execute(command string) {
	switch command {
	case "Exit":
		// TODO: This is just a temporary solution
		// until a proper solution is found.
		go func() {
			ui.Events <- ui.Quit
		}()
	case "New":
		win.ed.NewWindow()
	case "Del":
		win.Close()
	case "Put":
		if win.filename == "" {
			var runes []rune
			var p int64
			for {
				r := win.head.ReadRuneAt(p)
				if r == 0 || r == EOF {
					break
				}
				runes = append(runes, r)
				p++
			}
			if len(runes) == 0 {
				return
			}
			win.filename = string(runes)
		}
		if err := win.saveFile(); err != nil {
			panic(err)
		}
	case "Undo":
		win.Undo()
	case "Redo":
		win.Redo()
	default:
		// TODO: Implement this using io.Reader; read directly
		// from the buffer.
		var selected []rune
		q0, q1 := win.body.Selected()
		for p := q0; p < q1; p++ {
			r := win.body.ReadRuneAt(p)
			selected = append(selected, r)
		}
		var buf bytes.Buffer
		rd := strings.NewReader(string(selected))
		cmd := exec.Command(command)
		cmd.Stdin = rd
		cmd.Stdout = &buf
		// TODO: Redirect stderr somewhere.
		switch err := cmd.Run(); err := err.(type) {
		case *exec.Error:
			if err.Err == exec.ErrNotFound {
				return
			}
			panic(err)
		case error:
			panic(err)
		}
		s := buf.String()
		win.body.Insert(s)
		win.body.Select(q0, q0+int64(utf8.RuneCountInString(s)))

		// TODO: Come up with a better solution
		win.buf.buf.CommitChanges()
	}
}

func (win *Window) saveFile() error {
	// TODO: Read bytes directly from the undo.Buffer.
	// TODO: Don't use '~' suffix, make saving safer.
	f, err := os.Create(win.filename + "~")
	if err != nil {
		return err
	}

	var buf [64]byte
	var i int

	for p := int64(0); ; p++ {
		r := win.body.ReadRuneAt(p)
		if r == EOF || len(buf[i:]) < utf8.UTFMax {
			if _, err := f.Write(buf[:i]); err != nil {
				return err
			}
			i = 0
		}
		if r == EOF {
			break
		}
		i += utf8.EncodeRune(buf[i:], r)
	}
	f.Close()

	return os.Rename(win.filename+"~", win.filename)
}
