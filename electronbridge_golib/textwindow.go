package electronbridge

import (
	"fmt"
)

type TextWindow struct {
	Uid				int							`json:"uid"`
}

func (self *TextWindow) GetUID() int {
	return self.Uid
}

type new_text_win_msg struct {
	Name			string						`json:"name"`
	Page			string						`json:"page"`
	Uid				int							`json:"uid"`
	Width			int							`json:"width"`
	Height			int							`json:"height"`
	StartHidden		bool						`json:"starthidden"`
	Resizable		bool						`json:"resizable"`
}

type text_update_content struct {
	Uid				int							`json:"uid"`
	Msg				string						`json:"msg"`
}

func NewTextWindow(name, page string, width, height int, starthidden, resizable bool) *TextWindow {

	uid := id_maker.next()

	w := TextWindow{Uid: uid}

	c := new_text_win_msg{
		Name: name,
		Page: page,
		Uid: uid,
		Width: width,
		Height: height,
		StartHidden: starthidden,
		Resizable: resizable,
	}

	send_command_and_content("new", c)

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

	c := text_update_content{
		Uid: w.Uid,
		Msg: msg,
	}

	send_command_and_content("update", c)
}
