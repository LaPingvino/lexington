//go:build js && wasm

// Command lexington-web is Lexington in the browser: Go compiled to
// WebAssembly (scripts/build-web.sh), on an in-memory file system
// (web/memfs.js). It offers globalThis.lexington: formats() and
// presets(), choices(settings) and defaultSettings() return JSON or
// text; convert(...) returns {data: Uint8Array} or {error}.
package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"path"
	"sort"
	"strings"
	"syscall/js"

	"github.com/LaPingvino/lexington/cli"
	"github.com/LaPingvino/lexington/rules"
)

func main() {
	log.SetOutput(io.Discard)
	api := map[string]any{
		"formats": js.FuncOf(func(js.Value, []js.Value) any {
			type format struct{ Key, Name, Ext string }
			var list []format
			for _, f := range cli.Formats {
				if f.Needs == "" { // no other programs in a browser
					list = append(list, format{f.Key, f.Name, f.Extension()})
				}
			}
			b, _ := json.Marshal(list)
			return string(b)
		}),
		"presets": js.FuncOf(func(js.Value, []js.Value) any {
			b, _ := json.Marshal(rules.Presets())
			return string(b)
		}),
		"defaultSettings": js.FuncOf(func(js.Value, []js.Value) any {
			s, err := defaultSettings()
			if err != nil {
				return ""
			}
			return s
		}),
		// choices(settings): the element sets and scene languages the
		// settings (lexington.toml text, "" for the defaults) have
		"choices": js.FuncOf(func(_ js.Value, args []js.Value) any {
			b, _ := json.Marshal(choices(args[0].String()))
			return string(b)
		}),
		// convert(name, bytes, to, elements, page, sceneIn, sceneOut, settings)
		"convert": js.FuncOf(func(_ js.Value, args []js.Value) any {
			data, err := convert(args[0].String(), bytesOf(args[1]), cli.Options{To: args[2].String(),
				Elements: args[3].String(), Page: args[4].String(), SceneIn: args[5].String(), SceneOut: args[6].String()},
				args[7].String())
			out := js.Global().Get("Object").New()
			if err != nil {
				out.Set("error", err.Error())
				return out
			}
			arr := js.Global().Get("Uint8Array").New(len(data))
			js.CopyBytesToJS(arr, data)
			out.Set("data", arr)
			return out
		}),
	}
	js.Global().Set("lexington", js.ValueOf(api))
	select {}
}

func bytesOf(v js.Value) []byte {
	b := make([]byte, v.Get("length").Int())
	js.CopyBytesToGo(b, v)
	return b
}

// settingsFile writes settings (lexington.toml text) to dir, for
// rules.GetConf; "" leaves the defaults.
func settingsFile(dir, settings string) string {
	p := path.Join(dir, "lexington.toml")
	if settings != "" {
		_ = os.WriteFile(p, []byte(settings), 0o644)
	}
	return p
}

// defaultSettings are Lexington's default settings as TOML.
func defaultSettings() (string, error) {
	dir, err := os.MkdirTemp("/", "settings")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	p := path.Join(dir, "lexington.toml")
	if err := rules.Dump(p); err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	return string(b), err
}

type choice struct{ Key, Name, Description string }

// choices are the element sets (the presets, with their names, then the
// settings' own) and the scene heading languages.
func choices(settings string) (out struct{ Elements, Scenes []choice }) {
	dir, err := os.MkdirTemp("/", "choices")
	if err != nil {
		return out
	}
	defer os.RemoveAll(dir)
	conf := rules.GetConf(settingsFile(dir, settings))
	named := map[string]bool{}
	for _, p := range rules.Presets() {
		out.Elements = append(out.Elements, choice{p.Key, p.Name, p.Description})
		named[p.Key] = true
	}
	var own []string
	for k := range conf.Elements {
		if !named[k] {
			own = append(own, k)
		}
	}
	sort.Strings(own)
	for _, k := range own {
		out.Elements = append(out.Elements, choice{Key: k, Name: k + " (your settings)"})
	}
	var langs []string
	for k := range conf.Scenes {
		langs = append(langs, k)
	}
	sort.Strings(langs)
	for _, k := range langs {
		out.Scenes = append(out.Scenes, choice{Key: k, Name: k + ": " + strings.Join(conf.Scenes[k], ", ")})
	}
	return out
}

// convert converts a file (its name tells its format) as o says, with
// settings (lexington.toml text, "" for the defaults).
func convert(name string, data []byte, o cli.Options, settings string) ([]byte, error) {
	dir, err := os.MkdirTemp("/", "job")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	in := path.Join(dir, path.Base(name))
	if err := os.WriteFile(in, data, 0o644); err != nil {
		return nil, err
	}
	f, _ := cli.FormatFor(o.To)
	out := path.Join(dir, "out."+strings.TrimPrefix(f.Extension(), "."))
	o.Input, o.Output, o.ConfigFile = in, out, settingsFile(dir, settings)
	err = cli.Convert(context.Background(), o)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(out)
}
