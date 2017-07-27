package electronbridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

const (
	CLEAR_COLOUR = 'w'
)

var keypress_chan = make(chan string)
var key_query_chan = make(chan chan string)
var keyclear_chan = make(chan bool)

var mousedown_chan = make(chan Point)
var mouse_query_chan = make(chan chan Point)
var mouseclear_chan = make(chan bool)

var effect_done_channels = make(map[int]chan bool)
var effect_done_channels_MUTEX sync.Mutex

// ----------------------------------------------------------

type id_object struct {
	current			int
}

func (i *id_object) next() int {
	i.current += 1
	return i.current
}

var id_maker			id_object
var effect_id_maker		id_object

// ----------------------------------------------------------

type ByteSlice []byte		// Define this so that such a thing can have its own MarshalJSON() method

func (b ByteSlice) MarshalJSON() ([]byte, error) {
	str := string(b)
	return json.Marshal(str)
}

// ----------------------------------------------------------

type Point struct {
	X				int							`json:"x"`
	Y				int							`json:"y"`
}

type Spot struct {
	Char			byte
	Colour			byte
}

// ----------------------------------------------------------

type OutgoingMessage struct {
	Command			string						`json:"command"`
	Content			interface{}					`json:"content"`
}

// ----------------------------------------------------------

type SpecialMsgContent struct {
	Effect			string						`json:"effect"`
	EffectID		int							`json:"effectid"`
	Uid				int							`json:"uid"`
	Args			[]interface{}				`json:"args"`
}

type SpecialMsg struct {
	Command			string						`json:"command"`
	Content			SpecialMsgContent			`json:"content"`
}

// ----------------------------------------------------------

type IncomingMsgType struct {
	Type			string						`json:"type"`
}

// ----------------------------------------------------------

type IncomingKeyContent struct {
	Down			bool						`json:"down"`
	Uid				int							`json:"uid"`
	Key				string						`json:"key"`
}

type IncomingKey struct {
	Type			string						`json:"type"`
	Content			IncomingKeyContent			`json:"content"`
}

// ----------------------------------------------------------

type IncomingMouseContent struct {
	Down			bool						`json:"down"`
	Uid				int							`json:"uid"`
	X				int							`json:"x"`
	Y				int							`json:"y"`
}

type IncomingMouse struct {
	Type			string						`json:"type"`
	Content			IncomingMouseContent		`json:"content"`
}

// ----------------------------------------------------------

type IncomingEffectDoneContent struct {
	Uid				int							`json:"uid"`
	EffectID		int							`json:"effectid"`
}

type IncomingEffectDone struct {
	Type			string						`json:"type"`
	Content			IncomingEffectDoneContent	`json:"content"`
}

// ----------------------------------------------------------

func init() {
	go listener()
	go keymaster()
	go mousemaster()
}

func listener() {

	scanner := bufio.NewScanner(os.Stdin)

	for {
		scanner.Scan()

		// Logf("%v", scanner.Text())

		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}

		var type_obj IncomingMsgType

		err := json.Unmarshal(scanner.Bytes(), &type_obj)
		if err != nil {
			continue
		}

		if type_obj.Type == "key" {

			var key_msg IncomingKey

			err := json.Unmarshal(scanner.Bytes(), &key_msg)

			if err != nil {
				continue
			}

			if key_msg.Content.Down {
				keypress_chan <- key_msg.Content.Key
			}
		}

		if type_obj.Type == "mouse" {

			var mouse_msg IncomingMouse

			err := json.Unmarshal(scanner.Bytes(), &mouse_msg)

			if err != nil {
				continue
			}

			if mouse_msg.Content.Down {
				mousedown_chan <- Point{mouse_msg.Content.X, mouse_msg.Content.Y}
			}
		}

		if type_obj.Type == "effect_done" {

			var effect_done_msg IncomingEffectDone

			err := json.Unmarshal(scanner.Bytes(), &effect_done_msg)

			if err != nil {
				continue
			}

			effect_done_channels_MUTEX.Lock()
			ch := effect_done_channels[effect_done_msg.Content.EffectID]
			effect_done_channels_MUTEX.Unlock()

			if ch != nil {
				go effect_notifier(ch)
			} else {
				Logf("Received done for effect %d but no notifier was known", effect_done_msg.Content.EffectID)
			}
		}

		if type_obj.Type == "panic" {
			panic("Deliberate panic induced by front end.")
		}
	}
}

func keymaster() {

	var keyqueue []string

	for {
		select {
		case response_chan := <- key_query_chan:
			if len(keyqueue) == 0 {
				response_chan <- ""
			} else {
				response_chan <- keyqueue[0]
				keyqueue = keyqueue[1:]
			}
		case keypress := <- keypress_chan:
			keyqueue = append(keyqueue, keypress)
		case <- keyclear_chan:
			keyqueue = nil
		}
	}
}

func GetKeypress() (string, error) {

	response_chan := make(chan string)
	key_query_chan <- response_chan

	key := <- response_chan
	var err error = nil

	if key == "" {
		err = fmt.Errorf("GetKeypress(): nothing on queue")
	}

	return key, err
}

func ClearKeyQueue() {
	keyclear_chan <- true
}

func mousemaster() {

	var mousequeue []Point

	for {
		select {
		case response_chan := <- mouse_query_chan:
			if len(mousequeue) == 0 {
				response_chan <- Point{-1, -1}					// Note this: -1, -1 is used as a flag for empty queue
			} else {
				response_chan <- mousequeue[0]
				mousequeue = mousequeue[1:]
			}
		case mousedown := <- mousedown_chan:
			mousequeue = append(mousequeue, mousedown)
		case <- mouseclear_chan:
			mousequeue = nil
		}
	}
}

func GetMousedown() (Point, error) {

	response_chan := make(chan Point)
	mouse_query_chan <- response_chan

	point := <- response_chan
	var err error = nil

	if point.X < 0 {											// Note this: -1, -1 is used as a flag for empty queue
		err = fmt.Errorf("GetMousedown(): nothing on queue")
	}

	return point, err
}

func ClearMouseQueue() {
	mouseclear_chan <- true
}

func effect_notifier(ch chan bool) {
	ch <- true
}

// ----------------------------------------------------------

func Alertf(format_string string, args ...interface{}) {

	msg := fmt.Sprintf(format_string, args...)

	m := OutgoingMessage{
		Command: "alert",
		Content: msg,
	}

	s, err := json.Marshal(m)
	if err != nil {
		panic("Failed to Marshal")
	}
	fmt.Printf("%s\n", string(s))
}

func Logf(format_string string, args ...interface{}) {

	// Logging means sending to stderr.
	// The frontend picks such lines up and adds them to its own log window.

	msg := fmt.Sprintf(format_string, args...)

	if len(msg) < 1 {
		return
	}

	if msg[len(msg) - 1] != '\n' {
		msg += "\n"
	}

	fmt.Fprintf(os.Stderr, msg)
}
