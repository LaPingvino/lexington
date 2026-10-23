//go:build js && wasm

// Command lexington-web is Lexington in the browser: Go compiled to
// WebAssembly (scripts/build-web.sh), on an in-memory file system
// (web/memfs.js). It offers globalThis.lexington: formats() and
// presets() return JSON, convert(name, bytes, to, elements, page) returns
// {data: Uint8Array, name} or {error}.
package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"path"
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
		"convert": js.FuncOf(func(_ js.Value, args []js.Value) any {
			data, err := convert(args[0].String(), bytesOf(args[1]), args[2].String(), args[3].String(), args[4].String())
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

// convert converts a file (its name tells its format) to the format to.
func convert(name string, data []byte, to, elements, page string) ([]byte, error) {
	dir, err := os.MkdirTemp("/", "job")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	in := path.Join(dir, path.Base(name))
	if err := os.WriteFile(in, data, 0o644); err != nil {
		return nil, err
	}
	f, _ := cli.FormatFor(to)
	out := path.Join(dir, "out."+strings.TrimPrefix(f.Extension(), "."))
	err = cli.Convert(context.Background(), cli.Options{
		Input: in, Output: out, To: to, Elements: elements, Page: page, ConfigFile: path.Join(dir, "none.toml"),
	})
	if err != nil {
		return nil, err
	}
	return os.ReadFile(out)
}
