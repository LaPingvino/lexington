//go:build !(js && wasm)

// Command lexington-web is Lexington in the browser; it is built for it:
// GOOS=js GOARCH=wasm (scripts/build-web.sh).
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "lexington-web runs in a browser: build it with scripts/build-web.sh")
	os.Exit(2)
}
