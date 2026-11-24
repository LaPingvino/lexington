//go:build !js && !windows && !(darwin && cgo)

package pdf

import "os/exec"

// osFallbackName names the system's fallback in the font trace.
const osFallbackName = "fontconfig"

// osFallback is fontconfig's ranking (fc-match) of the faces for runes in
// a language, at most maxRanked of them, the same for each rune; nil
// without fontconfig (Linux and the BSDs have it; macOS built without
// cgo where it is installed).
func osFallback(runes []rune, bold bool, l fallbackLang) map[rune][]string {
	fc, err := exec.LookPath("fc-match")
	if err != nil {
		return nil
	}
	out, err := exec.Command(fc, "-s", "--format=%{file}#%{index}\n", fcPattern(runes, bold, l)).Output()
	if err != nil {
		return nil
	}
	return ranked(runes, fcFaces(string(out)))
}
