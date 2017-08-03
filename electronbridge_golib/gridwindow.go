package electronbridge

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	CLEAR_COLOUR = "w"
)

type StringSlice []string	// For convenience, things that should really be runes are stored as strings

func (s StringSlice) MarshalJSON() ([]byte, error) {	// Marshalling them means concatenation
	str := strings.Join(s, "")
	return json.Marshal(str)
}

type GridWindow struct {
	Uid				int							`json:"uid"`
	Width			int							`json:"width"`
	Height			int							`json:"height"`
	Chars			StringSlice					`json:"chars"`
	Colours			StringSlice					`json:"colours"`
	Highlight		Point						`json:"highlight"`

	LastFlip		[20]byte					`json:"-"`
	Mutex			sync.Mutex					`json:"-"`
}

type NewGridWinMsg struct {
	Name			string						`json:"name"`
	Page			string						`json:"page"`
	Uid				int							`json:"uid"`
	Width			int							`json:"width"`
	Height			int							`json:"height"`
	BoxWidth		int							`json:"boxwidth"`
	BoxHeight		int							`json:"boxheight"`
	FontPercent		int							`json:"fontpercent"`
	StartHidden		bool						`json:"starthidden"`
	Resizable		bool						`json:"resizable"`
}

func NewGridWindow(name, page string, width, height, boxwidth, boxheight, fontpercent int, starthidden, resizable bool) *GridWindow {

	uid := id_maker.next()

	w := GridWindow{Uid: uid, Width: width, Height: height}

	w.Chars = make([]string, width * height)
	w.Colours = make([]string, width * height)

	w.Clear()

	// Create the message to send to the server...

	m := OutgoingMessage{Command: "new", Content: NewGridWinMsg{
			Name: name,
			Page: page,
			Uid: uid,
			Width: width,
			Height: height,
			BoxWidth: boxwidth,
			BoxHeight: boxheight,
			FontPercent: fontpercent,
			StartHidden: starthidden,
			Resizable: resizable,
		},
	}

	s, err := json.Marshal(m)
	if err != nil {
		panic("Failed to Marshal")
	}

	OUT_msg_chan <- fmt.Sprintf("%s\n", string(s))

	return &w
}

func (w *GridWindow) Set(x, y int, char string, colour string) {

	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	if utf8.RuneCountInString(char) != 1 {
		panic("GridWindow.Set(): utf8.RuneCountInString(char) != 1")
	}

	if utf8.RuneCountInString(colour) != 1 {
		panic("GridWindow.Set(): utf8.RuneCountInString(colour) != 1")
	}

	index := y * w.Width + x
	if index < 0 || index >= len(w.Chars) || x < 0 || x >= w.Width || y < 0 || y >= w.Height {
		return
	}

	w.Chars[index] = char
	w.Colours[index] = colour
}

func (w *GridWindow) SetPointSpot(point Point, spot Spot) {
	w.Set(point.X, point.Y, spot.Char, spot.Colour)
}

func (w *GridWindow) Get(x, y int) Spot {

	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	index := y * w.Width + x
	if index < 0 || index >= len(w.Chars) || x < 0 || x >= w.Width || y < 0 || y >= w.Height {
		return Spot{Char: " ", Colour: CLEAR_COLOUR}
	}

	char := w.Chars[index]
	colour := w.Colours[index]

	return Spot{Char: char, Colour: colour}
}

func (w *GridWindow) SetHighlight(x, y int) {

	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	w.Highlight = Point{x, y}
}

func (w *GridWindow) Clear() {

	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	for n := 0; n < len(w.Chars); n++ {
		w.Chars[n] = " "
		w.Colours[n] = CLEAR_COLOUR
	}
	w.Highlight = Point{-1, -1}
}

func (w *GridWindow) Flip() {

	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	m := OutgoingMessage{
		Command: "update",
		Content: w,
	}

	s, err := json.Marshal(m)
	if err != nil {
		panic("Failed to Marshal")
	}

	// We cache the last flip and don't repeat it if we don't need to.

	sum := sha1.Sum(s)

	if sum != w.LastFlip {
		w.LastFlip = sum
		OUT_msg_chan <- fmt.Sprintf("%s\n", string(s))
	}
}
