package layout

import (
	"strings"
	"testing"

	"github.com/LaPingvino/lexington/fountain"
	"github.com/LaPingvino/lexington/rules"
)

// Colors from notes only when the rules ask for them; what is no color
// gives none.
func TestColors(t *testing.T) {
	s := fountain.Parse([]string{"INT"}, strings.NewReader("INT. A [[COLOR BLUE]]\n\nRain.\n\nINT. B [[color whatever]]\n\nINT. C [[#3366FF]]\n"))
	colors := func(set rules.Set) (out []string) {
		for _, p := range Paragraphs(s, set) {
			if p.Type == "scene" {
				out = append(out, p.Color)
			}
		}
		return out
	}
	if got := strings.Join(colors(nil), ","); got != ",," {
		t.Errorf("by default: %q", got)
	}
	if got := strings.Join(colors(rules.Default.WithColors()), ","); got != "#0081ef,,#3366ff" {
		t.Errorf("with colors: %q", got)
	}
	if Margin(rules.Default.WithColors()) != Margin(rules.Default) {
		t.Error("the switch changes the margin")
	}
}
