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

type Spot struct {
	Char			string
	Colour			string
	Background		string
}

type string_slice []string	// For convenience, things that should really be runes are stored as strings

func (s string_slice) MarshalJSON() ([]byte, error) {	// Marshalling them means concatenation
	str := strings.Join(s, "")
	return json.Marshal(str)
}

type GridWindow struct {
	Uid					int							`json:"uid"`
	Width				int							`json:"width"`
	Height				int							`json:"height"`
	Chars				string_slice				`json:"chars"`
	Colours				string_slice				`json:"colours"`
	Backgrounds			string_slice				`json:"backgrounds"`
	CameraX				int							`json:"camerax"`		// Only used to keep animations in alignment with the world
	CameraY				int							`json:"cameray"`		// Only used to keep animations in alignment with the world
	Title				string						`json:"title"`
	AckRequired			string						`json:"ackrequired"`	// Updated each flip (maybe set to "" though)

	BackendCanDrop		bool						`json:"-"`
	Mutex				sync.Mutex					`json:"-"`
	LastSend			time.Time					`json:"-"`
	CallCount			int64						`json:"-"`
	FlipLatersActive	int							`json:"-"`
	FramesDropped		int							`json:"-"`
	NextDropWarning		int							`json:"-"`
}

func (self *GridWindow) GetUID() int {
	return self.Uid
}

type new_grid_win_msg struct {
	Name				string						`json:"name"`
	Page				string						`json:"page"`
	Uid					int							`json:"uid"`
	Width				int							`json:"width"`
	Height				int							`json:"height"`
	BoxWidth			int							`json:"boxwidth"`
	BoxHeight			int							`json:"boxheight"`
	AnimationXOffset	int							`json:"animation_x_offset"`
	AnimationYOffset	int							`json:"animation_y_offset"`
	FontPercent			int							`json:"fontpercent"`
	StartHidden			bool						`json:"starthidden"`
	Resizable			bool						`json:"resizable"`
}

func NewGridWindow(
			name, page string,
			width, height, boxwidth, boxheight, animation_x_offset, animation_y_offset, fontpercent int,
			backend_can_drop, starthidden, resizable bool) *GridWindow {

	uid := id_maker.next()

	w := GridWindow{Uid: uid, Width: width, Height: height}

	w.Chars = make([]string, width * height)
	w.Colours = make([]string, width * height)
	w.Backgrounds = make([]string, width * height)

	w.Title = name

	w.BackendCanDrop = backend_can_drop
	w.NextDropWarning = 1

	w.Clear()

	// Create the message to send to the server...

	c := new_grid_win_msg{
		Name: name,
		Page: page,
		Uid: uid,
		Width: width,
		Height: height,
		BoxWidth: boxwidth,
		BoxHeight: boxheight,
		AnimationXOffset: animation_x_offset,
		AnimationYOffset: animation_y_offset,
		FontPercent: fontpercent,
		StartHidden: starthidden,
		Resizable: resizable,
	}
	send_command_and_content("new", c)

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

func (w *GridWindow) Clear() {

	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	for n := 0; n < len(w.Chars); n++ {
		w.Chars[n] = CLEAR_CHAR
		w.Colours[n] = CLEAR_COLOUR
		w.Backgrounds[n] = CLEAR_BACKGROUND
	}
}

func (w *GridWindow) SetTitle(s string) {

	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	w.Title = s
}

func (w *GridWindow) Flip(ack_channel chan bool) {

	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	if w.BackendCanDrop {

		// Optionally, don't send frames in very rapid succession; just ignore such frames instead.
		// If an ack was requested, we close the channel so the waiter receives false.

		w.CallCount++
		now := time.Now()

		if now.Sub(w.LastSend) < 9 * time.Millisecond {

			w.FramesDropped++
			if w.FramesDropped == w.NextDropWarning {
				w.NextDropWarning *= 2
				word := "frames"; if w.FramesDropped == 1 { word = "frame" }
				Silentf("Grid (Golang backend) UID %d has now dropped %d %s.", w.GetUID(), w.FramesDropped, word)
			}

			if ack_channel != nil {
				close(ack_channel)
			}

			// Spin up a goroutine that will eventually draw the frame if nothing else happens.

			if w.FlipLatersActive < 100 {		// I don't want a zillion of these goroutines running.
				w.FlipLatersActive++
				go w.FlipLater(w.CallCount)
			}
			return
		}

		w.LastSend = now
	}

	if ack_channel == nil {

		w.AckRequired = ""

	} else {

		// The ack we want from the frontend is a unique string. When listener() gets it, it sends true down the channel.

		w.AckRequired = ack_maker.next()			// This is the unique string.
		register_ack(w.AckRequired, ack_channel)

		// To ensure a waiter will eventually get a message, spin up a goroutine that eventually closes the channel.
		// The waiter will then receive false when reading from the channel. If the ack comes after this, the normal
		// sender will panic. This therefore needs a recover statement in the engine.go file. See ack_sender() there.

		go func() {
			time.Sleep(100 * time.Millisecond)
			close(ack_channel)
		}()
	}

	send_command_and_content("update", w)
}

func (w *GridWindow) FlipLater(call_count int64) {

	// We skipped a frame because it came too fast. But what if there are no more frames afterwards?
	// In that case it needs to be drawn! This goroutine does this.

	time.Sleep(25 * time.Millisecond)

	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	if w.CallCount == call_count {		// Flip() was never called since the skip.
		w.LastSend = time.Now()
		w.AckRequired = ""
		send_command_and_content("update", w)
	}

	w.FlipLatersActive--
}

func (w *GridWindow) FlipWithCamera(CameraX, CameraY int, ack_channel chan bool) {

	// It can be useful to send "camera" values to the frontend.
	// This function facilitates this. Not every app needs this though.

	w.Mutex.Lock()
	defer w.Flip(ack_channel)				// Note this...
	defer w.Mutex.Unlock()

	w.CameraX = CameraX
	w.CameraY = CameraY
}
