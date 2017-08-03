package electronbridge

import (
	"encoding/json"
	"fmt"
)

type TextWindow struct {
	Uid				int							`json:"uid"`
}

type NewTextWinMsg struct {
	Name			string						`json:"name"`
	Page			string						`json:"page"`
	Uid				int							`json:"uid"`
	Width			int							`json:"width"`
	Height			int							`json:"height"`
	Resizable		bool						`json:"resizable"`
}

type TextUpdateContent struct {
	Uid				int							`json:"uid"`
	Msg				string						`json:"msg"`
}

func NewTextWindow(name, page string, width, height int, resizable bool) *TextWindow {

	uid := id_maker.next()

	w := TextWindow{Uid: uid}

		m := OutgoingMessage{Command: "new", Content: NewTextWinMsg{
			Name: name,
			Page: page,
			Uid: uid,
			Width: width,
			Height: height,
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

func (w *TextWindow) Printf(format_string string, args ...interface{}) {

	msg := fmt.Sprintf(format_string, args...)

	if len(msg) < 1 {
		return
	}

	if msg[len(msg) - 1] != '\n' {
		msg += "\n"
	}

	m := OutgoingMessage{
		Command: "update",
		Content: TextUpdateContent{
			Uid: w.Uid,
			Msg: msg,
		},
	}

	s, err := json.Marshal(m)
	if err != nil {
		panic("Failed to Marshal")
	}
	OUT_msg_chan <- fmt.Sprintf("%s\n", string(s))
}
