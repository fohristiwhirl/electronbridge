package electronbridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

type Window interface {
	GetUID()	int
}

type keypress struct {
	key string
	uid int
}

type key_map_query struct {
	response_chan chan bool
	uid int
	key string
}

type key_queue_query struct {
	response_chan chan string
	uid int
}

type mousepress struct {
	press Point
	uid int
}

type mousequery struct {
	response_chan chan Point
	uid int
}

var OUT_msg_chan = make(chan string)
var ERR_msg_chan = make(chan string)

var keydown_chan = make(chan keypress)
var keyup_chan = make(chan keypress)
var key_map_query_chan = make(chan key_map_query)
var key_queue_query_chan = make(chan key_queue_query)
var key_queue_clear_chan = make(chan int)

var mousedown_chan = make(chan mousepress)
var mouse_query_chan = make(chan mousequery)
var mouseclear_chan = make(chan int)

// ----------------------------------------------------------

type id_object struct {
	mutex			sync.Mutex
	current			int
}

func (i *id_object) next() int {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.current += 1
	return i.current
}

var id_maker			id_object
var effect_id_maker		id_object

// ----------------------------------------------------------

type Point struct {
	X				int							`json:"x"`
	Y				int							`json:"y"`
}

type Spot struct {
	Char			string
	Colour			string
}

// ----------------------------------------------------------

type OutgoingMessage struct {
	Command			string						`json:"command"`
	Content			interface{}					`json:"content"`
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
	go printer()
	go listener()
	go keymaster()
	go mousemaster()
}

// ----------------------------------------------------------

func printer() {
	for {
		select {
		case s := <- OUT_msg_chan:
			fmt.Printf(s)
		case s := <- ERR_msg_chan:
			fmt.Fprintf(os.Stderr, s)
		}
	}
}

// ----------------------------------------------------------

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
				keydown_chan <- keypress{key: key_msg.Content.Key, uid: key_msg.Content.Uid}
			} else {
				keyup_chan <- keypress{key: key_msg.Content.Key, uid: key_msg.Content.Uid}
			}
		}

		if type_obj.Type == "mouse" {

			var mouse_msg IncomingMouse

			err := json.Unmarshal(scanner.Bytes(), &mouse_msg)

			if err != nil {
				continue
			}

			if mouse_msg.Content.Down {
				mousedown_chan <- mousepress{press: Point{mouse_msg.Content.X, mouse_msg.Content.Y}, uid: mouse_msg.Content.Uid}
			}
		}

		if type_obj.Type == "panic" {
			panic("Deliberate panic induced by front end.")
		}
	}
}

// ----------------------------------------------------------

func keymaster() {

	// Note: at some point we might want both the ability to query a queue
	// of keystrokes and an ability to see what keys are down NOW.

	keyqueues := make(map[int][]string)
	keymaps := make(map[int]map[string]bool)

	for {
		select {

		// Query of the map...

		case query := <- key_map_query_chan:

			query.response_chan <- keymaps[query.uid][query.key]

		// Query of the queue...

		case query := <- key_queue_query_chan:

			if len(keyqueues[query.uid]) == 0 {
				query.response_chan <- ""
			} else {
				query.response_chan <- keyqueues[query.uid][0]
				keyqueues[query.uid] = keyqueues[query.uid][1:]
			}

		// Updates...

		case key_msg := <- keydown_chan:

			keyqueues[key_msg.uid] = append(keyqueues[key_msg.uid], key_msg.key)
			if keymaps[key_msg.uid] == nil {
				keymaps[key_msg.uid] = make(map[string]bool)
			}
			keymaps[key_msg.uid][key_msg.key] = true

		case key_msg := <- keyup_chan:

			if keymaps[key_msg.uid] == nil {
				keymaps[key_msg.uid] = make(map[string]bool)
			}
			keymaps[key_msg.uid][key_msg.key] = false

		// Queue clear...

		case clear_uid := <- key_queue_clear_chan:
			keyqueues[clear_uid] = nil
		}
	}
}

func GetKeyDown(w Window, key string) bool {

	uid := w.GetUID()

	response_chan := make(chan bool)

	key_map_query_chan <- key_map_query{response_chan: response_chan, uid: uid, key: key}

	return <- response_chan
}

func GetKeypress(w Window) (string, error) {

	uid := w.GetUID()

	response_chan := make(chan string)

	key_queue_query_chan <- key_queue_query{response_chan: response_chan, uid: uid}

	key := <- response_chan
	var err error = nil

	if key == "" {
		err = fmt.Errorf("GetKeypress(): nothing on queue")
	}

	return key, err
}

func ClearKeyQueue(w Window) {
	key_queue_clear_chan <- w.GetUID()
}

// ----------------------------------------------------------

func mousemaster() {

	mousequeues := make(map[int][]Point)

	for {
		select {
		case query := <- mouse_query_chan:
			if len(mousequeues[query.uid]) == 0 {
				query.response_chan <- Point{-1, -1}					// Note this: -1, -1 is used as a flag for empty queue
			} else {
				query.response_chan <- mousequeues[query.uid][0]
				mousequeues[query.uid] = mousequeues[query.uid][1:]
			}
		case mouse_msg := <- mousedown_chan:
			mousequeues[mouse_msg.uid] = append(mousequeues[mouse_msg.uid], mouse_msg.press)
		case clear_uid := <- mouseclear_chan:
			mousequeues[clear_uid] = nil
		}
	}
}

func GetMousedown(w Window) (Point, error) {

	uid := w.GetUID()

	response_chan := make(chan Point)

	mouse_query_chan <- mousequery{response_chan: response_chan, uid: uid}

	point := <- response_chan
	var err error = nil

	if point.X < 0 {											// Note this: -1, -1 is used as a flag for empty queue
		err = fmt.Errorf("GetMousedown(): nothing on queue")
	}

	return point, err
}

func ClearMouseQueue(w Window) {
	mouseclear_chan <- w.GetUID()
}

// ----------------------------------------------------------

func sendoutgoingmessage(m OutgoingMessage) {

	s, err := json.Marshal(m)
	if err != nil {
		panic("Failed to Marshal")
	}

	OUT_msg_chan <- fmt.Sprintf("%s\n", string(s))
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
	OUT_msg_chan <- fmt.Sprintf("%s\n", string(s))
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

	ERR_msg_chan <- fmt.Sprintf(msg)
}

func AllowQuit() {

	m := OutgoingMessage{
		Command: "allowquit",
		Content: nil,
	}

	sendoutgoingmessage(m)
}
