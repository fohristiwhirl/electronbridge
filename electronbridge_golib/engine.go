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

type key_map_query struct {
	response_chan	chan bool
	key				string
}

type MousePress struct {
	Point
	Uid				int
	Button			int		// 0 == left, 1 == middle, 2 == right
}

type mouse_query struct {
	response_chan	chan MousePress
	uid				int
}

// ----------------------------------------------------------

var OUT_msg_chan = make(chan []byte)
var ERR_msg_chan = make(chan []byte)

var key_down_chan = make(chan string)
var key_up_chan = make(chan string)
var key_map_query_chan = make(chan key_map_query)
var key_queue_query_chan = make(chan chan string)
var key_queue_clear_chan = make(chan bool)

var mouse_down_chan = make(chan MousePress)
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
		Button			int							`json:"button"`
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
				key_down_chan <- msg.Content.Key
			} else {
				key_up_chan <- msg.Content.Key
			}
		}

		if msg.Type == "mouse" {		// Note: uses the same struct as below
			if msg.Content.Down {
				mouse_down_chan <- MousePress{Point: Point{msg.Content.X, msg.Content.Y}, Uid: msg.Content.Uid, Button: msg.Content.Button}
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

	// This used to keep track of what windows each key was pressed on, see:
	// https://github.com/fohristiwhirl/klaarheid/tree/40f0f55ef96b785e6b724a794032104fe265841d
	//
	// But for my actual use-case it's better to not bother.
	// Key presses do not (logically) belong to a window the way mouseclicks do.

	var keyqueue []string
	var keymap = make(map[string]bool)		// Lowercase only

	for {
		select {

		// Query of the map...

		case query := <- key_map_query_chan:

			query.response_chan <- keymap[strings.ToLower(query.key)]

		// Query of the queue...

		case response_chan := <- key_queue_query_chan:

			if len(keyqueue) == 0 {
				response_chan <- ""
			} else {
				response_chan <- keyqueue[0]
				keyqueue = keyqueue[1:]
			}

		// Updates...

		case key := <- key_down_chan:

			keyqueue = append(keyqueue, key)
			keymap[strings.ToLower(key)] = true

		case key := <- key_up_chan:

			keymap[strings.ToLower(key)] = false

		// Queue clear...

		case <- key_queue_clear_chan:

			keyqueue = nil
		}
	}
}

func GetKeyDown(key string) bool {

	response_chan := make(chan bool)

	key_map_query_chan <- key_map_query{response_chan: response_chan, key: key}

	return <- response_chan
}

func GetKeypress() (string, error) {

	response_chan := make(chan string)

	key_queue_query_chan <- response_chan

	key := <- response_chan
	var err error = nil

	if key == "" {
		err = fmt.Errorf("GetKeypress(): nothing on queue")
	}

	return key, err
}

func ClearKeyQueue() {
	key_queue_clear_chan <- true
}

// ----------------------------------------------------------

func mouse_click_hub() {

	mousequeues := make(map[int][]MousePress)

	EMPTY_QUEUE_REPLY := MousePress{Point: Point{-1, -1}, Uid: -1, Button: -1}

	for {
		select {
		case query := <- mouse_query_chan:
			if len(mousequeues[query.uid]) == 0 {
				query.response_chan <- EMPTY_QUEUE_REPLY
			} else {
				query.response_chan <- mousequeues[query.uid][0]
				mousequeues[query.uid] = mousequeues[query.uid][1:]
			}
		case mouse_msg := <- mouse_down_chan:
			mousequeues[mouse_msg.Uid] = append(mousequeues[mouse_msg.Uid], mouse_msg)
		case clear_uid := <- mouse_clear_chan:
			mousequeues[clear_uid] = nil
		}
	}
}

func GetMouseClick(w Window) (MousePress, error) {

	uid := w.GetUID()

	response_chan := make(chan MousePress)

	mouse_query_chan <- mouse_query{response_chan: response_chan, uid: uid}

	press := <- response_chan
	var err error = nil

	if press.Point.X < 0 {											// Note this: -1, -1 is used as a flag for empty queue
		err = fmt.Errorf("GetMouseClick(): nothing on queue")
	}

	return press, err
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
