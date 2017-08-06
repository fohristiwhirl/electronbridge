package electronbridge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

// ----------------------------------------------------------

type Window interface {
	GetUID()	int
}

// ----------------------------------------------------------

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

// ----------------------------------------------------------

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

var mouse_xy_chan = make(chan Point)
var mouse_xy_query = make(chan chan Point)

var quit_chan = make(chan bool)
var quit_query_chan = make(chan chan bool)

var cmd_chan = make(chan string)
var cmd_query_chan = make(chan chan string)

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

type IncomingMsg struct {
	Type			string						`json:"type"`
	Content			IncomingMsgContent			`json:"content"`
}

type IncomingMsgContent struct {

	// Used for all incoming message types. Not every field will be needed.

	Uid				int							`json:"uid"`
	Key				string						`json:"key"`
	Down			bool						`json:"down"`
	X				int							`json:"x"`
	Y				int							`json:"y"`
	Cmd				string						`json:"cmd"`
}

// ----------------------------------------------------------

func init() {
	go printer()
	go listener()
	go keymaster()
	go mousemaster()
	go simple_mouse_location_master()
	go quitmaster()
	go commandmaster()
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

		var msg IncomingMsg

		err := json.Unmarshal(scanner.Bytes(), &msg)
		if err != nil {
			continue
		}

		if msg.Type == "key" {
			if msg.Content.Down {
				keydown_chan <- keypress{key: msg.Content.Key, uid: msg.Content.Uid}
			} else {
				keyup_chan <- keypress{key: msg.Content.Key, uid: msg.Content.Uid}
			}
		}

		if msg.Type == "mouse" {		// Note: uses the same struct as below
			if msg.Content.Down {
				mousedown_chan <- mousepress{press: Point{msg.Content.X, msg.Content.Y}, uid: msg.Content.Uid}
			}
		}

		if msg.Type == "mouseover" {
			mouse_xy_chan <- Point{msg.Content.X, msg.Content.Y}
		}

		if msg.Type == "panic" {
			panic("Deliberate panic induced by front end.")
		}

		if msg.Type == "quit" {
			quit_chan <- true
		}

		if msg.Type == "cmd" {
			cmd_chan <- msg.Content.Cmd
		}
	}
}

// ----------------------------------------------------------

func keymaster() {

	// Note: at some point we might want both the ability to query a queue
	// of keystrokes and an ability to see what keys are down NOW.

	keyqueues := make(map[int][]string)
	keymaps := make(map[int]map[string]bool)		// Lowercase only

	make_keymap_if_needed := func (uid int) {
		if keymaps[uid] == nil {
			keymaps[uid] = make(map[string]bool)
		}
	}

	for {
		select {

		// Query of the map...

		case query := <- key_map_query_chan:

			query.response_chan <- keymaps[query.uid][strings.ToLower(query.key)]	// if keymaps[query.uid] is nil, this is false

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

			make_keymap_if_needed(key_msg.uid)
			keymaps[key_msg.uid][strings.ToLower(key_msg.key)] = true

		case key_msg := <- keyup_chan:

			make_keymap_if_needed(key_msg.uid)
			keymaps[key_msg.uid][strings.ToLower(key_msg.key)] = false

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

func simple_mouse_location_master() {

	// Has no concept of multiple windows.

	var point Point

	for {
		select {
		case point = <- mouse_xy_chan:
			// no other action
		case response_chan := <- mouse_xy_query:
			response_chan <- point
		}
	}
}

func MouseXY() Point {
	response_chan := make(chan Point)
	mouse_xy_query <- response_chan
	return <- response_chan
}

// ----------------------------------------------------------

func quitmaster() {

	var quit bool

	for {
		select {
		case <- quit_chan:
			quit = true
		case response_chan := <- quit_query_chan:
			response_chan <- quit
		}
	}
}

func WeShouldQuit() bool {
	response_chan := make(chan bool)
	quit_query_chan <- response_chan
	return <- response_chan
}

// ----------------------------------------------------------

func commandmaster() {

	var queue []string

	for {
		select {
		case cmd := <- cmd_chan:
			queue = append(queue, cmd)
		case response_chan := <- cmd_query_chan:
			if len(queue) == 0 {
				response_chan <- ""
			} else {
				response_chan <- queue[0]
				queue = queue[1:]
			}
		}
	}
}

func RegisterCommand(s string) {

	m := OutgoingMessage{
		Command: "register",
		Content: s,
	}

	sendoutgoingmessage(m)
}

func GetCommand() string {
	response_chan := make(chan string)
	cmd_query_chan <- response_chan
	return <- response_chan
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
