package main

import (
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/edsrzf/mmap-go"
	"github.com/mibk/syd/event"
	"github.com/mibk/syd/text"
	"github.com/mibk/syd/ui/console"
	"github.com/mibk/syd/view"
)

var (
	ui       console.Console
	filename = ""

	textBuf  *text.Text
	viewport *view.View
)

func main() {
	ui.Init()
	defer ui.Close()

	var initContent []byte
	if len(os.Args) > 1 {
		filename = os.Args[1]
		m, err := readFile(filename)
		if err != nil {
			panic(err)
		}
		defer m.Unmap()
		initContent = []byte(m)
	} else {
		initContent = []byte("\n")
	}

	textBuf = text.New(initContent)
	viewport = view.New(textBuf)
	normalMode()
}

func readFile(filename string) (mmap.MMap, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	m, err := mmap.Map(f, 0, 0)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func normalMode() {
Loop:
	for {
		viewport.Draw(ui)
		printFoot()
		ui.Flush()
		ev := event.PollEvent()
		switch ev := ev.(type) {
		case event.KeyPress:
			switch ev.Key {
			case 'j':
				viewport.MoveDown()
			case 'k':
				viewport.MoveUp()
			case 'h':
				viewport.MoveLeft()
			case 'l':
				viewport.MoveRight()
			case 'q':
				break Loop
			case 'u':
				textBuf.Undo()
			case 'r':
				if ev.Ctrl {
					textBuf.Redo()
				}

			case 'i':
				insertMode()
			case ':':
				commandMode()
			}

		}
	}
}

func insertMode() {
	for {
		viewport.Draw(ui)
		_, h := ui.Size()
		printFoot()
		print(0, h-1, "-- INSERT --", console.AttrBold)
		ui.Flush()
		ev := event.PollEvent()
		switch ev := ev.(type) {
		case event.KeyPress:
			switch ev.Key {
			case event.Escape:
				textBuf.CommitChanges()
				return
			case event.Backspace:
				viewport.MoveLeft()
				fallthrough
			case event.Delete:
				c := viewport.CurrentCell()
				length := utf8.RuneLen(c.R)
				textBuf.Delete(c.Off, length)
			case event.Enter:
				textBuf.Insert(viewport.CurrentCell().Off, []byte("\n"))
				viewport.ReadLines()
				viewport.MoveDown()
				viewport.ToTheStartColumn()
			default:
				buf := make([]byte, 4)
				n := utf8.EncodeRune(buf, rune(ev.Key))
				textBuf.Insert(viewport.CurrentCell().Off, buf[:n])
				viewport.ReadLines()
				viewport.MoveRight()
			}
		}
	}
}

func commandMode() {
	cmd := make([]rune, 0, 20)
	cur := 0
Loop:
	for {
		viewport.Draw(ui)
		printFoot()
		_, h := ui.Size()
		print(0, h-1, ":"+string(cmd), console.AttrDefault)
		ui.SetCursor(cur+1, h-1)
		ui.Flush()
		ev := event.PollEvent()
		switch ev := ev.(type) {
		case event.KeyPress:
			switch ev.Key {
			case event.Escape:
				return
			case event.Backspace:
				if cur > 0 {
					cur--
					cmd = cmd[:cur]
				}
			case event.Enter:
				break Loop
			default:
				cmd = append(cmd, rune(ev.Key))
				cur++
			}
		}
	}
	exec(string(cmd))
}

var writeRE = regexp.MustCompile(`w( .+)?`)

func exec(cmd string) {
	if match := writeRE.FindStringSubmatch(cmd); match != nil {
		if match[1] != "" {
			filename = strings.Trim(match[1], " \t")
		}
		if filename == "" {
			_, h := ui.Size()
			print(0, h-1, "no filename! (press any key)", console.AttrDefault)
			ui.Flush()
			event.PollEvent()
		} else {
			if err := saveFile(filename); err != nil {
				panic(err)
			}
		}
	}
}

func saveFile(filename string) error {
	textBuf.Save()
	tmpFile := filename + "~"
	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	io.Copy(f, view.ReaderFrom(textBuf, 0))
	f.Close()

	if err := os.Rename(tmpFile, filename); err != nil {
		return err
	}
	return nil
}

func print(x, y int, s string, attrs uint8) {
	for _, r := range []rune(s) {
		ui.SetCell(x, y, r, attrs)
		x++
	}
}

func printFoot() {
	w, h := ui.Size()
	for x := 0; x < w; x++ {
		ui.SetCell(x, h-2, ' ', console.AttrReverse|console.AttrBold)
	}
	filename := filename
	if filename == "" {
		filename = "[No Name]"
	}
	if textBuf.Modified() {
		filename += " [+]"
	}
	print(0, h-2, filename, console.AttrReverse|console.AttrBold)
}