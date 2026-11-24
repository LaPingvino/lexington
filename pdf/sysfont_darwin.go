//go:build darwin && cgo

package pdf

/*
#cgo LDFLAGS: -framework CoreText -framework CoreFoundation
#include <CoreText/CoreText.h>
#include <stdlib.h>

// lex_base is the font the fallback is asked from: Courier, bold if
// asked (made once).
static CTFontRef lex_base(int bold) {
	static CTFontRef base[2];
	if (!base[bold]) {
		base[bold] = CTFontCreateWithName(bold ? CFSTR("Courier-Bold") : CFSTR("Courier"), 12.0, NULL);
	}
	return base[bold];
}

// lex_font_for writes the file path and PostScript name of the font
// CoreText takes for the n UTF-16 units of chars in a language ("" for
// the system's preferred), and tells whether it found one.
static int lex_font_for(const UniChar *chars, long n, const char *lang, int bold,
		char *path, int pathlen, char *ps, int pslen) {
	CTFontRef base = lex_base(bold);
	if (!base) return 0;
	CFStringRef str = CFStringCreateWithCharacters(NULL, chars, n);
	if (!str) return 0;
	CFStringRef language = NULL;
	if (lang && *lang) language = CFStringCreateWithCString(NULL, lang, kCFStringEncodingUTF8);
	CTFontRef font = CTFontCreateForStringWithLanguage(base, str, CFRangeMake(0, n), language);
	int ok = 0;
	if (font) {
		CFURLRef url = (CFURLRef)CTFontCopyAttribute(font, kCTFontURLAttribute);
		CFStringRef name = CTFontCopyPostScriptName(font);
		if (url && name &&
			CFURLGetFileSystemRepresentation(url, true, (UInt8 *)path, pathlen) &&
			CFStringGetCString(name, ps, pslen, kCFStringEncodingUTF8)) {
			ok = 1;
		}
		if (url) CFRelease(url);
		if (name) CFRelease(name);
		CFRelease(font);
	}
	if (language) CFRelease(language);
	CFRelease(str);
	return ok;
}
*/
import "C"

import (
	"fmt"
	"unsafe"

	"golang.org/x/image/font/sfnt"
)

// osFallbackName names the system's fallback in the font trace.
const osFallbackName = "CoreText"

// osFallback is CoreText's font for each of runes in a language
// (CTFontCreateForStringWithLanguage from Courier: the system's cascade
// list for it), as a face of its file: the face of that PostScript name.
func osFallback(runes []rune, bold bool, l fallbackLang) map[rune][]string {
	lang := C.CString(l.apple)
	defer C.free(unsafe.Pointer(lang))
	b := C.int(0)
	if bold {
		b = 1
	}
	var path [4096]C.char
	var ps [512]C.char
	fontFor := func(u []uint16) (string, string, bool) {
		if len(u) == 0 {
			return "", "", false
		}
		if C.lex_font_for((*C.UniChar)(unsafe.Pointer(&u[0])), C.long(len(u)), lang, b,
			&path[0], C.int(len(path)), &ps[0], C.int(len(ps))) == 0 {
			return "", "", false
		}
		return C.GoString(&path[0]), C.GoString(&ps[0]), true
	}
	return eachRune(runes, fontFor, postScriptFace)
}

// postScriptFace is the face of a font file whose PostScript name is ps
// ("path#index"; the first face if none is: a file of one font); "" if
// the file cannot be read (sys.mu held).
func postScriptFace(path, ps string) string {
	if openFont(path+"#0") == nil {
		return ""
	}
	sf := sys.files[path]
	var buf sfnt.Buffer
	for i := range sf.c.NumFonts() {
		if f := openFont(fmt.Sprintf("%s#%d", path, i)); f != nil {
			if name, err := f.Name(&buf, sfnt.NameIDPostScript); err == nil && name == ps {
				return fmt.Sprintf("%s#%d", path, i)
			}
		}
	}
	return path + "#0"
}
