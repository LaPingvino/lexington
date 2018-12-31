package rules

type Format struct{
	Width float64
	Left float64
	Font string
	Style string
	Size float64
	Hide bool
}

type Set map[string]Format

func (s Set) Get(action string) (f Format) {
	f, ok := s[action]
	if !ok {
		f = s["action"]
		f.Hide = true
	}
	if f.Font == "" {
		f.Font = "courier"
	}
	if f.Size == 0 {
		f.Size = 12
	}
	return f
}

var Default = Set{
	"action":  {
		Left: 1.5,
		Width: 6,
	},
	"speaker": {
		Left: 4.2,
		Width: 3.3,
	},
	"dialog":  {
		Left: 2.9,
		Width: 3.3,
	},
	"scene":   {
		Left: 1.5,
		Width: 6,
		Style: "b",
	},
	"paren":   {
		Left: 3.6,
		Width: 4,
	},
	"trans":   {
		Left: 6,
		Width: 1.5,
	},
	"note":    {
		Left: 1.5,
		Width: 6,
	},
	"allcaps": {
		Left: 1.5,
		Width: 6,
	},
	"empty": {
		Left: 1.5,
		Width: 6,
	},
	"title": {
		Left: 3.6,
		Width: 4,
	},
	"meta": {
		Left: 1.5,
		Width: 3,
	},
}
