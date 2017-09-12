package main

import (
	"fmt"
	"time"
	electron "./electronbridge_golib"
)

const (
	WIDTH = 40
	HEIGHT = 3
	BOX_WIDTH = 12
	BOX_HEIGHT = 20
	FONT_PERCENT = 100
)

func main() {
	report_window := electron.NewTextWindow("Reports", "pages/log.html", 400, 300, false, true)
	main_window := electron.NewGridWindow("Timer", "pages/grid.html", WIDTH, HEIGHT, BOX_WIDTH, BOX_HEIGHT, 0, 0, FONT_PERCENT, true, false, false)

	electron.RegisterCommand("Menu Item 1", "")
	electron.RegisterCommand("Menu Item 2", "")
	electron.BuildMenu()
	electron.AllowQuit()

	i := 0

	for {
		i++

		s := fmt.Sprintf("%d", i)

		for x := 0; x < len(s); x++ {
			main_window.Set(x + 1, 1, string(s[x]), "g", "0")		// x, y, char, colour, bg-colour
		}

		main_window.Flip(nil)		// Optionally, send a (chan bool) as an argument and get a message when drawing is completed (or aborted).

		for {
			click, err := electron.GetMouseClick(main_window)
			if err != nil {
				break
			}
			report_window.Printf("%v", click)
		}

		for {
			key, err := electron.GetKeypress()
			if err != nil {
				break
			}
			report_window.Printf("%v", key)
		}

		for {
			command, err := electron.GetCommand()
			if err != nil {
				break
			}
			report_window.Printf("%v", command)
		}

		time.Sleep(1 * time.Millisecond)
	}
}
