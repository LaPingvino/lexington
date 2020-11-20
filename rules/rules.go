// The rules package of Lexington provides the tools around configuration of how a screenplay should look. The default should work but can be adjusted for a personal touch..
package rules

type Format struct {
	Width   float64
	Left    float64
	Font    string
	Style   string
	Size    float64
	Hide    bool
	Align   string
	Prefix  string
	Postfix string
}

type Set map[string]Format

func (s Set) Get(action string) (f Format) {
	f, ok := s[action]
	if !ok {
		f = s["action"]
		f.Hide = true
	}
	if f.Font == "" {
		f.Font = "CourierPrime"
	}
	if f.Size == 0 {
		f.Size = 12
	}
	if f.Align == "" {
		f.Align = "L"
	}
	return f
}

var Default = Set{
	"action": {
		Left:  1.5,
		Width: 6,
	},
	"speaker": {
		Left:  3.7,
		Width: 6.5-3.7,
	},
	"dialog": {
		Left:  2.5,
		Width: 6.5-2.5,
	},
	"scene": {
		Left:  1.5,
		Width: 6,
		Style: "b",
	},
	"paren": {
		Left:  3.1,
		Width: 6.5-3.1,
	},
	"trans": {
		Left:  1.5,
		Width: 6,
		Align: "R",
	},
	"note": {
		Left:  1.5,
		Width: 6,
	},
	"allcaps": {
		Left:  1.5,
		Width: 6,
	},
	"empty": {
		Left:  1.5,
		Width: 6,
	},
	"title": {
		Left:  1.5,
		Width: 6,
		Align: "C",
	},
	"meta": {
		Left:  1.5,
		Width: 6,
	},
	"center": {
		Left:  1.5,
		Width: 6,
		Align: "C",
	},
	"lyrics": {
		Left:  2,
		Width: 5,
		Style: "i",
		Font:  "Helvetica",
	},
}
