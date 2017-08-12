package electronbridge

type Effect struct {

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
	Duration		float64						`json:"duration"`
	Colour			string						`json:"colour"`
}

func MakeShot(w Window, x1, y1, x2, y2 int, duration float64, colour string) {

	m := OutgoingMessage{
		Command: "effect",
		Content: Effect{
			Function: "make_shot",
			Uid: w.GetUID(),
			X1: x1,
			Y1: y1,
			X2: x2,
			Y2: y2,
			Duration: duration,
			Colour: colour,
		},
	}

	sendoutgoingmessage(m)
}

func MakeFlash(w Window, x, y, r, g, b int) {

	m := OutgoingMessage{
		Command: "effect",
		Content: Effect{
			Function: "make_flash",
			Uid: w.GetUID(),
			X: x,
			Y: y,
			R: r,
			G: g,
			B: b,
		},
	}

	sendoutgoingmessage(m)
}
