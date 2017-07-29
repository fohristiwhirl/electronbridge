package electronbridge

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strings"
	"time"
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
	Resizable		bool						`json:"resizable"`
}

func NewGridWindow(name, page string, width, height, boxwidth, boxheight, fontpercent int, resizable bool) *GridWindow {

	uid := id_maker.next()

	w := GridWindow{Uid: uid, Width: width, Height: height}

	w.Chars = make([]string, width * height)
	w.Colours = make([]string, width * height)

	w.Highlight = Point{-1, -1}

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
			Resizable: resizable,
		},
	}

	s, err := json.Marshal(m)
	if err != nil {
		panic("Failed to Marshal")
	}

	fmt.Printf("%s\n", string(s))

	return &w
}

func (w *GridWindow) Set(x, y int, char string, colour string) {
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
	index := y * w.Width + x
	if index < 0 || index >= len(w.Chars) || x < 0 || x >= w.Width || y < 0 || y >= w.Height {
		return Spot{Char: " ", Colour: CLEAR_COLOUR}
	}
	char := w.Chars[index]
	colour := w.Colours[index]
	return Spot{Char: char, Colour: colour}
}

func (w *GridWindow) SetHighlight(x, y int) {
	w.Highlight = Point{x, y}
}

func (w *GridWindow) Clear() {
	for n := 0; n < len(w.Chars); n++ {
		w.Chars[n] = " "
		w.Colours[n] = CLEAR_COLOUR
	}
	w.Highlight = Point{-1, -1}
}

func (w *GridWindow) Flip() {

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
		fmt.Printf("%s\n", string(s))
	}
}

func (w *GridWindow) Special(effect string, timeout_duration time.Duration, args []interface{}) {

	// Special effects. What is available depends on the contents of the html page.

	c := SpecialMsgContent{
		Effect: effect,
		Uid: w.Uid,
		EffectID: effect_id_maker.next(),
		Args: args,
	}

	m := OutgoingMessage{
		Command: "special",
		Content: c,
	}

	s, err := json.Marshal(m)
	if err != nil {
		panic("Failed to Marshal")
	}

	// We make a channel for the purpose of receiving a message when the effect completes,
	// and add it to the global map of such channels.

	ch := make(chan bool)

	timeout := time.NewTimer(timeout_duration)

	effect_done_channels_MUTEX.Lock()
	effect_done_channels[c.EffectID] = ch
	effect_done_channels_MUTEX.Unlock()

	fmt.Printf("%s\n", string(s))

	// Now we wait for the message that the effect completed...
	// Or the timeout ticker to fire.

	ChanLoop:
	for {
		select {
		case <- ch:
			break ChanLoop
		case <- timeout.C:
			Logf("Timed out waiting for effect %d", c.EffectID)
			break ChanLoop
		}
	}

	effect_done_channels_MUTEX.Lock()
	delete(effect_done_channels, c.EffectID)
	effect_done_channels_MUTEX.Unlock()
}
