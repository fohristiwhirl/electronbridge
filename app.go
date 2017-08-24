package main

import (
	"fmt"
	"time"
	electron "./electronbridge_golib"
)

func main() {
	report_window := electron.NewTextWindow("Reports", "pages/log.html", 400, 300, false, true)
	main_window := electron.NewGridWindow("Timer", "pages/grid.html", 40, 3, 12, 20, 100, false, false)

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
			click, err := electron.GetMousedown(main_window)
			if err != nil {
				break
			}
			report_window.Printf("%v", click)
		}

		for {
			key, err := electron.GetKeypress(main_window)
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
