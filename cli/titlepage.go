package cli

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/LaPingvino/lexington/lex"
	"github.com/LaPingvino/lexington/rules"
	"github.com/LaPingvino/lexington/titlepage"
)

// titlePageFor sets o's title page template (o.TitlePage in conf, for the
// script's language) and, when o has none given, reads the pictures its
// title page names.
func titlePageFor(o *Options, conf rules.TOMLConf, s lex.Screenplay) error {
	tpl, ok := titlepage.For(conf, o.TitlePage, o.SceneIn)
	if !ok {
		return fmt.Errorf("no title page template %q in %s or the presets (see -titlepages)", o.TitlePage, o.ConfigFile)
	}
	o.tpl = &tpl
	if o.TitlePictures == nil {
		o.TitlePictures = titlePictures(*o, tpl, s)
	}
	return nil
}

// titlePictures are the files the title page's pictures name, read from
// the input's bundle (the name as it is, else in assets/) or from beside
// the input. A picture on the web is never fetched; one not found is
// left out (the writers print its name).
func titlePictures(o Options, tpl titlepage.Template, s lex.Screenplay) map[string][]byte {
	out := map[string][]byte{}
	for _, b := range tpl.Layout(s) {
		for _, f := range b.Fields {
			name := f.Picture
			if name == "" || out[name] != nil || remote(name) {
				continue
			}
			if data := readPicture(o, name); data != nil {
				out[name] = data
			}
		}
	}
	return out
}

// remote tells whether a picture's name is an address on the web.
func remote(name string) bool {
	i := strings.Index(name, "://")
	return i > 0 && !strings.HasPrefix(strings.ToLower(name), "file://")
}

// readPicture reads a picture o's input names, nil if it is not there.
func readPicture(o Options, name string) []byte {
	name = strings.TrimPrefix(name, "file://")
	if o.src != nil {
		rel := path.Clean(strings.TrimPrefix(name, "./"))
		for _, p := range []string{rel, path.Join("assets", rel)} {
			if data, err := o.src.ReadFile(p); err == nil {
				return data
			}
		}
		return nil
	}
	if filepath.IsAbs(name) {
		data, _ := os.ReadFile(name)
		return data
	}
	if o.Input == "" || o.Input == "-" {
		return nil
	}
	data, _ := os.ReadFile(filepath.Join(filepath.Dir(o.Input), filepath.FromSlash(name)))
	return data
}
