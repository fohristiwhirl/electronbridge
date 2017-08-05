package main

import (
	"fmt"
	"time"
	electron "./electronbridge_golib"
)

func main() {
	report_window := electron.NewTextWindow("Reports", "pages/log.html", 400, 300, false, true)
	main_window := electron.NewGridWindow("Timer", "pages/grid.html", 40, 3, 12, 20, 100, false, true)
	electron.AllowQuit()

	i := 0

	report_window.Printf("Click in the timer for mouse coordinates")
	report_window.Printf("Press keys on the timer for key reports")

	for {
		i++

		s := fmt.Sprintf("%d", i)

		for x := 0; x < len(s); x++ {
			main_window.Set(x + 1, 1, string(s[x]), "g")
		}

		main_window.Flip()

		click, err := electron.GetMousedown(main_window)
		if err == nil {
			report_window.Printf("%v", click)
		}

		key, err := electron.GetKeypress(main_window)
		if err == nil {
			report_window.Printf("%v", key)
		}

		time.Sleep(1 * time.Millisecond)
	}
}
