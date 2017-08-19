package electronbridge

import (
	"encoding/json"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	CLEAR_CHAR = " "
	CLEAR_COLOUR = "w"
	CLEAR_BACKGROUND = "0"
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
	Backgrounds		StringSlice					`json:"backgrounds"`
	Highlight		Point						`json:"highlight"`
	CameraX			int							`json:"camerax"`		// Only used to keep animations in alignment with the world
	CameraY			int							`json:"cameray"`		// Only used to keep animations in alignment with the world
	Title			string						`json:"title"`

	Mutex			sync.Mutex					`json:"-"`
	LastSend		time.Time					`json:"-"`
}

func (self *GridWindow) GetUID() int {
	return self.Uid
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
	w.Backgrounds = make([]string, width * height)

	w.Title = name

	w.Clear()

	// Create the message to send to the server...

	m := OutgoingMessage{
		Command: "new",
		Content: NewGridWinMsg{
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

	sendoutgoingmessage(m)

	return &w
}


func (w *GridWindow) Set(x, y int, char, colour, background string) {

	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	if utf8.RuneCountInString(char) != 1 {
		panic("GridWindow.Set(): utf8.RuneCountInString(char) != 1")
	}

	if utf8.RuneCountInString(colour) != 1 {
		panic("GridWindow.Set(): utf8.RuneCountInString(colour) != 1")
	}

	if utf8.RuneCountInString(background) != 1 {
		panic("GridWindow.Set(): utf8.RuneCountInString(background) != 1")
	}

	index := y * w.Width + x
	if index < 0 || index >= len(w.Chars) || x < 0 || x >= w.Width || y < 0 || y >= w.Height {
		return
	}

	w.Chars[index] = char
	w.Colours[index] = colour
	w.Backgrounds[index] = background
}

func (w *GridWindow) Get(x, y int) Spot {

	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	index := y * w.Width + x
	if index < 0 || index >= len(w.Chars) || x < 0 || x >= w.Width || y < 0 || y >= w.Height {
		return Spot{Char: CLEAR_CHAR, Colour: CLEAR_COLOUR, Background: CLEAR_BACKGROUND}
	}

	char := w.Chars[index]
	colour := w.Colours[index]
	background := w.Backgrounds[index]

	return Spot{Char: char, Colour: colour, Background: background}
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
		w.Chars[n] = CLEAR_CHAR
		w.Colours[n] = CLEAR_COLOUR
		w.Backgrounds[n] = CLEAR_BACKGROUND
	}
	w.Highlight = Point{-1, -1}
}

func (w *GridWindow) SetTitle(s string) {

	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	w.Title = s
}

func (w *GridWindow) Flip() {

	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	now := time.Now()

	if now.Sub(w.LastSend) < 9 * time.Millisecond {
		return
	}

	w.LastSend = now

	m := OutgoingMessage{
		Command: "update",
		Content: w,
	}

	sendoutgoingmessage(m)
}

func (w *GridWindow) FlipWithCamera(CameraX, CameraY int) {

	// It can be useful to send "camera" values to the frontend.
	// This function facilitates this. Not every app needs this though.

	w.Mutex.Lock()
	defer w.Flip()				// Note this...
	defer w.Mutex.Unlock()

	w.CameraX = CameraX
	w.CameraY = CameraY
}
