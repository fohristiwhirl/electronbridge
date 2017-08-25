package electronbridge

type effect struct {

	// We can expand this struct later with whatever fields are needed by any effect.
	// The actual interpretation of the values is up to the animating JS.

	Function		string						`json:"function"`
	Uid				int							`json:"uid"`
	X				int							`json:"x"`
	Y				int							`json:"y"`
	X1				int							`json:"x1"`
	Y1				int							`json:"y1"`
	X2				int							`json:"x2"`
	Y2				int							`json:"y2"`
	R				int							`json:"r"`
	G				int							`json:"g"`
	B				int							`json:"b"`
	Duration		int							`json:"duration"`
	Radius			int							`json:"radius"`
	Opacity			float64						`json:"opacity"`
	Points			[]Point						`json:"points"`
}

func MakeShot(w Window, x1, y1, x2, y2, r, g, b, duration int) {

	c := effect{
		Function: "make_shot",
		Uid: w.GetUID(),
		X1: x1,
		Y1: y1,
		X2: x2,
		Y2: y2,
		R: r,
		G: g,
		B: b,
		Duration: duration,
	}

	send_command_and_content("effect", c)
}

func MakeFlash(w Window, x, y, r, g, b, duration int, opacity float64) {

	c := effect{
		Function: "make_flash",
		Uid: w.GetUID(),
		X: x,
		Y: y,
		R: r,
		G: g,
		B: b,
		Duration: duration,
		Opacity: opacity,
	}

	send_command_and_content("effect", c)
}

func MakeExplosion(w Window, x, y, duration, radius int) {

	c := effect{
		Function: "make_explosion",
		Uid: w.GetUID(),
		X: x,
		Y: y,
		Duration: duration,
		Radius: radius,
	}

	send_command_and_content("effect", c)
}

func MakeCascade(w Window, r, g, b, duration int, opacity float64, points []Point) {

	c := effect{
		Function: "make_cascade",
		Uid: w.GetUID(),
		R: r,
		G: g,
		B: b,
		Duration: duration,
		Opacity: opacity,
		Points: points,
	}

	send_command_and_content("effect", c)
}
