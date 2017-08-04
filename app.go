package main

import (
	"fmt"
	"time"
	electron "./electronbridge_golib"
)

const (
	WIDTH = 40
	HEIGHT = 3
)

func main() {
	main_window := electron.NewGridWindow("World", "pages/grid.html", WIDTH, HEIGHT, 12, 20, 100, false, true)
	electron.AllowQuit()

	i := 0

	for {
		i++

		s := fmt.Sprintf("%d", i)

		for x := 0; x < len(s); x++ {
			main_window.Set(x + 1, 1, string(s[x]), "g")
		}

		main_window.Flip()

		time.Sleep(1 * time.Millisecond)
	}
}

/*

import (
	"math/rand"
	"time"
	electron "./electronbridge_golib"
)

const (
	WIDTH = 40
	HEIGHT = 30
)

func main() {
	main_window := electron.NewGridWindow("World", "pages/grid.html", WIDTH, HEIGHT, 12, 20, 100, false, true)
	text_window := electron.NewTextWindow("Text", "pages/log.html", 400, 300, false, true)
	electron.AllowQuit()

	i := 0

	for {
		i++

		x := rand.Intn(WIDTH)
		y := rand.Intn(HEIGHT)

		main_window.Set(x, y, "*", "g")
		main_window.Flip()

		if i % 50 == 0 {
			text_window.Printf("Reached i %d\n", i)
		}

		time.Sleep(50 * time.Millisecond)
	}
}

*/
