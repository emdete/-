package main

import (
	"log"
	"os"
	// +tcell
	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/encoding"
	"github.com/mattn/go-runewidth"
)

var frames Frames
var status Status
var query Query
var enumeration Enumeration
var threads Threads

func emitStr(s tcell.Screen, x, y int, style tcell.Style, str string, width int) {
	for _, c := range str {
		if width > 0 {
			var comb []rune
			w := runewidth.RuneWidth(c)
			if w == 0 {
				comb = []rune{c}
				c = ' '
				w = 1
			}
			s.SetContent(x, y, c, comb, style)
			x += w
			width--
		}
	}
	for width > 0 {
		var comb []rune
		s.SetContent(x, y, ' ', comb, style)
		x += 1
		width--
	}
}

func updateScreen(s tcell.Screen) {
	w, h := s.Size()
	s.Clear()
	frames.Draw(s, 0, 0, w, h)
	status.Draw(s, 0, 0, w, 1)
	query.Draw(s, 0, 1, w, 1)
	enumeration.Draw(s, 0, 3, frames.pos_vertical_bar-1, h-3)
	s.Show()
	//s.Sync()
}

func _log() {
	log.SetPrefix("email ")
	log.SetFlags(log.Ldate|log.Lmicroseconds|log.LUTC|log.Lshortfile)
	log.SetOutput(os.Stderr)
}

func main() {
	// log
	_log()
	log.Printf("main")
	// tcell
	// see ~/go/pkg/mod/github.com/gdamore/tcell/v2@v2.4.1-0.20210905002822-f057f0a857a1/
	encoding.Register()
	if s, err := tcell.NewScreen(); err != nil {
		panic(err)
	} else {
		defer s.Fini()
		if err := s.Init(); err != nil {
			panic(err)
		}
		s.SetStyle(tcell.StyleDefault.Background(tcell.ColorBlack).Foreground(tcell.ColorWhite))
		s.EnableMouse()
		s.EnablePaste()
		// Frames
		frames = NewFrames(s, 61)
		// Status
		status = NewStatus(s)
		// Query
		query = NewQuery(s)
		// Enumeration
		enumeration = NewEnumeration(s)
		update := true
		for {
			if update {
				updateScreen(s)
				update = false
			}
			event := s.PollEvent()
			switch ev := event.(type) {
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyRune, tcell.KeyLeft, tcell.KeyRight,
				tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyEnter,
				tcell.KeyDelete, tcell.KeyHome, tcell.KeyEnd, tcell.KeyTab:
					update = update || query.EventHandler(s, event)
				case tcell.KeyUp, tcell.KeyDown:
					update = update || enumeration.EventHandler(s, event)
				case tcell.KeyPgUp, tcell.KeyPgDn:
					update = update || threads.EventHandler(s, event)
				case tcell.KeyEscape:
					s.Fini()
					os.Exit(0)
				case tcell.KeyCtrlB:
					s.Beep()
				}
			case *tcell.EventResize:
				s.Sync()
				update = true
			case *tcell.EventPaste:
				update = update || query.EventHandler(s, event)
			case *tcell.EventMouse:
				update = update || enumeration.EventHandler(s, event)
			case *EventThreadsStatus:
				update = update || status.EventHandler(s, event)
			case *EventThreadsMail:
				update = update || threads.EventHandler(s, event)
			case *EventQuery:
				update = update || enumeration.EventHandler(s, event)
			}
		}
	}
}
