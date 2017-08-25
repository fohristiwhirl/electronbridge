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

type key_press struct {
	key				string
	uid				int
}

type key_map_query struct {
	response_chan	chan bool
	uid				int
	key				string
}

type key_queue_query struct {
	response_chan	chan string
	uid				int
}

type mouse_press struct {
	press			Point
	uid				int
}

type mouse_query struct {
	response_chan	chan Point
	uid				int
}

// ----------------------------------------------------------

var OUT_msg_chan = make(chan []byte)
var ERR_msg_chan = make(chan []byte)

var key_down_chan = make(chan key_press)
var key_up_chan = make(chan key_press)
var key_map_query_chan = make(chan key_map_query)
var key_queue_query_chan = make(chan key_queue_query)
var key_queue_clear_chan = make(chan int)

var mouse_down_chan = make(chan mouse_press)
var mouse_query_chan = make(chan mouse_query)
var mouse_clear_chan = make(chan int)

var mouse_xy_chan = make(chan MouseLocation)
var mouse_xy_query = make(chan chan MouseLocation)

var quit_chan = make(chan bool)
var quit_query_chan = make(chan chan bool)

var cmd_chan = make(chan string)
var cmd_query_chan = make(chan chan string)

// ----------------------------------------------------------

var pending_acks = make(map[string]chan bool)
var pending_acks_mutex sync.Mutex

// ----------------------------------------------------------

type id_object struct {
	mutex			sync.Mutex
	current			int
}

func (self *id_object) next() int {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	self.current += 1
	return self.current
}

type ack_object struct {
	mutex			sync.Mutex
	current			int64
}

func (self *ack_object) next() string {
	self.mutex.Lock()
	defer self.mutex.Unlock()
	self.current += 1
	return fmt.Sprintf("%d", self.current)
}

var id_maker			id_object
var ack_maker			ack_object

// ----------------------------------------------------------

type Point struct {
	X				int							`json:"x"`
	Y				int							`json:"y"`
}

type MouseLocation struct {
	X				int							`json:"x"`
	Y				int							`json:"y"`
	Uid				int							`json:"uid"`
}

// ----------------------------------------------------------

type outgoing_msg struct {
	Command			string						`json:"command"`
	Content			interface{}					`json:"content"`
}

// ----------------------------------------------------------

func init() {
	go printer()
	go listener()
	go key_hub()
	go mouse_click_hub()
	go mouse_location_hub()
	go quit_hub()
	go command_hub()
}

// ----------------------------------------------------------

func printer() {
	for {
		select {
		case s := <- OUT_msg_chan:
			os.Stdout.Write(s)
		case s := <- ERR_msg_chan:
			os.Stderr.Write(s)
		}
	}
}

// ----------------------------------------------------------

func listener() {

	type incoming_msg_content struct {		// Used for all incoming message types. Not every field will be needed.
		Uid				int							`json:"uid"`
		X				int							`json:"x"`
		Y				int							`json:"y"`
		Down			bool						`json:"down"`
		Key				string						`json:"key"`
		Cmd				string						`json:"cmd"`
		AckMessage		string						`json:"ackmessage"`
	}

	type incoming_msg struct {
		Type			string						`json:"type"`
		Content			incoming_msg_content		`json:"content"`
	}

	// ----------------------------------

	scanner := bufio.NewScanner(os.Stdin)

	for {
		scanner.Scan()

		// Logf("%v", scanner.Text())

		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}

		var msg incoming_msg

		err := json.Unmarshal(scanner.Bytes(), &msg)
		if err != nil {
			continue
		}

		if msg.Type == "key" {
			if msg.Content.Down {
				key_down_chan <- key_press{key: msg.Content.Key, uid: msg.Content.Uid}
			} else {
				key_up_chan <- key_press{key: msg.Content.Key, uid: msg.Content.Uid}
			}
		}

		if msg.Type == "mouse" {		// Note: uses the same struct as below
			if msg.Content.Down {
				mouse_down_chan <- mouse_press{press: Point{msg.Content.X, msg.Content.Y}, uid: msg.Content.Uid}
			}
		}

		if msg.Type == "mouseover" {
			mouse_xy_chan <- MouseLocation{Uid: msg.Content.Uid, X: msg.Content.X, Y: msg.Content.Y}
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

		if msg.Type == "ack" {

			// We got an ack, the content of which is some unique string. Look it up in our map of acks,
			// and retrieve the channel down which we are supposed to send true.

			pending_acks_mutex.Lock()
			ch := pending_acks[msg.Content.AckMessage]
			delete(pending_acks, msg.Content.AckMessage)
			pending_acks_mutex.Unlock()

			if ch != nil {
				go ack_sender(ch)	// Spin up a new goroutine so we don't deadlock even if the ack-requester gave up waiting. Also, this can panic/recover.
			} else {
				Logf("listener: got ack '%s' but no channel existed to receive it", msg.Content.AckMessage)
			}
		}
	}
}

// ----------------------------------------------------------

func ack_sender(ch chan bool) {

	defer func() {
		recover()
	}()

	ch <- true		// This can panic if ch has been closed, which is possible, e.g. gridwindow.go closes its ack channels after a timeout.
}

func register_ack(desired_ack string, ack_channel chan bool) {

	// Safely add to our list of ack messages we're waiting for.

	pending_acks_mutex.Lock()
	pending_acks[desired_ack] = ack_channel
	pending_acks_mutex.Unlock()
}

// ----------------------------------------------------------

func key_hub() {

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

		case key_msg := <- key_down_chan:

			keyqueues[key_msg.uid] = append(keyqueues[key_msg.uid], key_msg.key)

			make_keymap_if_needed(key_msg.uid)
			keymaps[key_msg.uid][strings.ToLower(key_msg.key)] = true

		case key_msg := <- key_up_chan:

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

func mouse_click_hub() {

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
		case mouse_msg := <- mouse_down_chan:
			mousequeues[mouse_msg.uid] = append(mousequeues[mouse_msg.uid], mouse_msg.press)
		case clear_uid := <- mouse_clear_chan:
			mousequeues[clear_uid] = nil
		}
	}
}

func GetMousedown(w Window) (Point, error) {

	uid := w.GetUID()

	response_chan := make(chan Point)

	mouse_query_chan <- mouse_query{response_chan: response_chan, uid: uid}

	point := <- response_chan
	var err error = nil

	if point.X < 0 {											// Note this: -1, -1 is used as a flag for empty queue
		err = fmt.Errorf("GetMousedown(): nothing on queue")
	}

	return point, err
}

func ClearMouseQueue(w Window) {
	mouse_clear_chan <- w.GetUID()
}

// ----------------------------------------------------------

func mouse_location_hub() {

	var loc MouseLocation

	for {
		select {
		case loc = <- mouse_xy_chan:
			// no other action
		case response_chan := <- mouse_xy_query:
			response_chan <- loc
		}
	}
}

func MouseXY() MouseLocation {
	response_chan := make(chan MouseLocation)
	mouse_xy_query <- response_chan
	return <- response_chan
}

// ----------------------------------------------------------

func quit_hub() {

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

func command_hub() {

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

func RegisterCommand(s string, accel string) {

	type item struct {
		Label			string		`json:"label"`
		Accelerator		string		`json:"accelerator"`
	}

	send_command_and_content("register", item{s, accel})
}

func RegisterSeparator() {
	send_command_and_content("separator", nil)
}

func GetCommand() (string, error) {
	response_chan := make(chan string)
	cmd_query_chan <- response_chan

	cmd := <- response_chan
	var err error = nil

	if cmd == "" {
		err = fmt.Errorf("GetCommand(): nothing on queue")
	}

	return cmd, err
}

// ----------------------------------------------------------

func BuildMenu() {
	send_command_and_content("buildmenu", nil)
}

func SetAbout(s string) {
	send_command_and_content("about", s)
}

// ----------------------------------------------------------

func send_command_and_content(command string, content interface{}) {

	m := outgoing_msg{
		Command: command,
		Content: content,
	}

	b, err := json.Marshal(m)
	if err != nil {
		panic("Failed to Marshal")
	}

	b = append(b, '\n')
	OUT_msg_chan <- b
}

// ----------------------------------------------------------

func Alertf(format_string string, args ...interface{}) {
	msg := fmt.Sprintf(format_string, args...)
	send_command_and_content("alert", msg)
}

func Logf(format_string string, args ...interface{}) {

	// Logging means sending to stderr.
	// The frontend picks such lines up and adds them to its own devlog window.

	msg := fmt.Sprintf(format_string, args...)

	if len(msg) < 1 {
		return
	}

	if msg[len(msg) - 1] != '\n' {
		msg += "\n"
	}

	ERR_msg_chan <- []byte(msg)
}

func Silentf(format_string string, args ...interface{}) {

	// We can also log by sending a normal message to the frontend.
	// This type of log message won't bring the devlog to the front.

	msg := fmt.Sprintf(format_string, args...)
	if len(msg) > 0 {
		send_command_and_content("silentlog", msg)
	}
}

func AllowQuit() {
	send_command_and_content("allowquit", nil)
}

func BringToFront(w Window) {
	send_command_and_content("front", w.GetUID())
}
