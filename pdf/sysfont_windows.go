//go:build windows

package pdf

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// DirectWrite's system font fallback (IDWriteFactory2's
// GetSystemFontFallback, its MapCharacters), through COM without cgo:
// the font Windows takes for each run of the characters in the
// language's locale, read as its file's path and its index in it.

// osFallbackName names the system's fallback in the font trace.
const osFallbackName = "DirectWrite"

// comObj is a COM object: its table of methods first.
type comObj struct{ vtbl *[64]uintptr }

// call calls a COM object's method (its index in the table, IUnknown's
// three first) and gives its HRESULT. Its arguments' pointers are moved
// to the heap (uintptrescapes): DirectWrite calls back into Go during a
// call, and a goroutine's stack may move then.
//
//go:uintptrescapes
func (o *comObj) call(method int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtbl[method], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return r
}

func (o *comObj) release() {
	if o != nil {
		o.call(2)
	}
}

func failed(hr uintptr) bool { return int32(hr) < 0 }

// The methods' indexes in their tables (dwrite.h, dwrite_2.h).
const (
	qiMethod                = 0  // IUnknown::QueryInterface
	getSystemFontFallback   = 26 // IDWriteFactory2, after IDWriteFactory's 21 and IDWriteFactory1's 2
	mapCharacters           = 3  // IDWriteFontFallback
	createFontFace          = 13 // IDWriteFont
	fontFaceGetFiles        = 4  // IDWriteFontFace
	fontFaceGetIndex        = 5
	fileGetReferenceKey     = 3 // IDWriteFontFile
	fileGetLoader           = 4
	localPathLengthFromKey  = 4 // IDWriteLocalFontFileLoader, after IDWriteFontFileLoader's 1
	localPathFromKey        = 5
	dwriteFactoryTypeShared = 0
	dwriteWeightNormal      = 400
	dwriteWeightBold        = 700
	dwriteStretchNormal     = 5
)

var (
	iidFactory2        = windows.GUID{Data1: 0x0439fc60, Data2: 0xca44, Data3: 0x4994, Data4: [8]byte{0x8d, 0xee, 0x3a, 0x9a, 0xf7, 0xb7, 0x32, 0xec}}
	iidLocalLoader     = windows.GUID{Data1: 0xb2d9f3ec, Data2: 0xc9fe, Data3: 0x4a11, Data4: [8]byte{0xa2, 0xec, 0xd8, 0x62, 0x08, 0xf7, 0xc0, 0xa2}}
	iidUnknown         = windows.GUID{Data1: 0x00000000, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidAnalysisSource  = windows.GUID{Data1: 0x688e1a58, Data2: 0x5094, Data3: 0x47c8, Data4: [8]byte{0xad, 0xc8, 0xfb, 0xce, 0xa6, 0x0a, 0xe9, 0x2b}}
	dwrite             = windows.NewLazySystemDLL("dwrite.dll")
	dwriteCreate       = dwrite.NewProc("DWriteCreateFactory")
	userDefaultLocale  = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	dwOnce             sync.Once
	dwFallback         *comObj // the system's IDWriteFontFallback, nil if there is none (before Windows 8.1)
	dwErr              string  // why there is none
	sourceVtbl         *[8]uintptr
	localeNameMaxChars = 85
)

// analysisSource is an IDWriteTextAnalysisSource of a textSource: its
// table first, as COM has it.
type analysisSource struct {
	vtbl *[8]uintptr
	src  *textSource
}

// The IDWriteTextAnalysisSource methods, called by DirectWrite (this
// first). Their out-parameters are DirectWrite's memory: a pointer is
// written there as a uintptr, not through a Go pointer type (no write
// barrier reads a slot not Go's).

func asQueryInterface(this *analysisSource, iid *windows.GUID, out *uintptr) uintptr {
	if *iid == iidUnknown || *iid == iidAnalysisSource {
		*out = uintptr(unsafe.Pointer(this))
		return 0
	}
	*out = 0
	return 0x80004002 // E_NOINTERFACE
}

func asAddRef(*analysisSource) uintptr  { return 1 }
func asRelease(*analysisSource) uintptr { return 1 }

func asTextAt(this *analysisSource, pos uintptr, text *uintptr, n *uint32) uintptr {
	i, l := this.src.at(int(uint32(pos)))
	*text, *n = 0, uint32(l)
	if i >= 0 {
		*text = uintptr(unsafe.Pointer(&this.src.text[i]))
	}
	return 0
}

func asTextBefore(this *analysisSource, pos uintptr, text *uintptr, n *uint32) uintptr {
	i, l := this.src.before(int(uint32(pos)))
	*text, *n = 0, uint32(l)
	if i >= 0 {
		*text = uintptr(unsafe.Pointer(&this.src.text[i]))
	}
	return 0
}

func asDirection(*analysisSource) uintptr { return 0 } // DWRITE_READING_DIRECTION_LEFT_TO_RIGHT

func asLocale(this *analysisSource, pos uintptr, n *uint32, locale *uintptr) uintptr {
	*n = uint32(this.src.localeRun(int(uint32(pos))))
	*locale = uintptr(unsafe.Pointer(&this.src.locale[0]))
	return 0
}

func asNumberSubstitution(this *analysisSource, pos uintptr, n *uint32, sub *uintptr) uintptr {
	*n = uint32(this.src.localeRun(int(uint32(pos))))
	*sub = 0
	return 0
}

// dwInit makes the system's font fallback, once.
func dwInit() {
	dwOnce.Do(func() {
		sourceVtbl = &[8]uintptr{
			syscall.NewCallback(asQueryInterface),
			syscall.NewCallback(asAddRef),
			syscall.NewCallback(asRelease),
			syscall.NewCallback(asTextAt),
			syscall.NewCallback(asTextBefore),
			syscall.NewCallback(asDirection),
			syscall.NewCallback(asLocale),
			syscall.NewCallback(asNumberSubstitution),
		}
		if err := dwriteCreate.Find(); err != nil {
			dwErr = err.Error()
			return
		}
		var factory *comObj
		hr, _, _ := dwriteCreate.Call(dwriteFactoryTypeShared, uintptr(unsafe.Pointer(&iidFactory2)), uintptr(unsafe.Pointer(&factory)))
		if failed(hr) || factory == nil {
			dwErr = fmt.Sprintf("DWriteCreateFactory(IDWriteFactory2): HRESULT %#x", uint32(hr)) // before Windows 8.1
			return
		}
		var fb *comObj
		if hr := factory.call(getSystemFontFallback, uintptr(unsafe.Pointer(&fb))); failed(hr) || fb == nil {
			dwErr = fmt.Sprintf("GetSystemFontFallback: HRESULT %#x", uint32(hr))
			factory.release()
			return
		}
		dwFallback = fb // kept, with its factory, for the program's life
	})
}

// locale is the language's locale name for DirectWrite: the user's
// default where there is no language.
func locale(l fallbackLang) []uint16 {
	if l.win != "" {
		if u, err := windows.UTF16FromString(l.win); err == nil {
			return u
		}
	}
	buf := make([]uint16, localeNameMaxChars)
	if userDefaultLocale.Find() == nil {
		if n, _, _ := userDefaultLocale.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf))); n > 0 {
			return buf[:n] // n counts the NUL
		}
	}
	return []uint16{'e', 'n', '-', 'U', 'S', 0}
}

// osFallback is DirectWrite's font for each run of runes in the
// language's locale, from Courier New as the base: the system's own
// fallback, which picks a Han face by the locale (Yu Gothic for ja-JP,
// Microsoft YaHei for zh-CN, JhengHei for zh-TW, Malgun Gothic for
// ko-KR).
func osFallback(runes []rune, bold bool, l fallbackLang) map[rune][]string {
	dwInit()
	if dwFallback == nil || len(runes) == 0 {
		if traceFonts && dwFallback == nil {
			fmt.Fprintf(os.Stderr, "lexington font: DirectWrite has no system font fallback: %s\n", dwErr)
		}
		return nil
	}
	weight := uintptr(dwriteWeightNormal)
	if bold {
		weight = dwriteWeightBold
	}
	base, _ := windows.UTF16PtrFromString("Courier New")
	src := &textSource{locale: locale(l)}
	as := &analysisSource{vtbl: sourceVtbl, src: src}
	out := mapRuns(runes, func(text []uint16, pos, n int) (int, string) {
		src.text = text
		var mapped uint32
		var font *comObj
		var scale float32
		hr := dwFallback.call(mapCharacters, uintptr(unsafe.Pointer(as)), uintptr(pos), uintptr(n),
			0, uintptr(unsafe.Pointer(base)), weight, 0, dwriteStretchNormal,
			uintptr(unsafe.Pointer(&mapped)), uintptr(unsafe.Pointer(&font)), uintptr(unsafe.Pointer(&scale)))
		runtime.KeepAlive(as)
		if failed(hr) {
			if traceFonts {
				fmt.Fprintf(os.Stderr, "lexington font: DirectWrite MapCharacters failed: HRESULT %#x\n", uint32(hr))
			}
			return 0, ""
		}
		if font == nil {
			return int(mapped), ""
		}
		defer font.release()
		return int(mapped), fontPath(font)
	})
	runtime.KeepAlive(src)
	return out
}

// fontPath is an IDWriteFont's face as "path#index"; "" if it is not of
// a local file.
func fontPath(font *comObj) string {
	var face *comObj
	if hr := font.call(createFontFace, uintptr(unsafe.Pointer(&face))); failed(hr) || face == nil {
		return ""
	}
	defer face.release()
	var n uint32
	if hr := face.call(fontFaceGetFiles, uintptr(unsafe.Pointer(&n)), 0); failed(hr) || n == 0 {
		return ""
	}
	files := make([]*comObj, n)
	if hr := face.call(fontFaceGetFiles, uintptr(unsafe.Pointer(&n)), uintptr(unsafe.Pointer(&files[0]))); failed(hr) {
		return ""
	}
	defer func() {
		for _, f := range files {
			f.release()
		}
	}()
	index := uint32(face.call(fontFaceGetIndex))
	file := files[0]
	if file == nil {
		return ""
	}
	var key unsafe.Pointer
	var keySize uint32
	if hr := file.call(fileGetReferenceKey, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&keySize))); failed(hr) {
		return ""
	}
	var loader *comObj
	if hr := file.call(fileGetLoader, uintptr(unsafe.Pointer(&loader))); failed(hr) || loader == nil {
		return ""
	}
	defer loader.release()
	var local *comObj
	if hr := loader.call(qiMethod, uintptr(unsafe.Pointer(&iidLocalLoader)), uintptr(unsafe.Pointer(&local))); failed(hr) || local == nil {
		return "" // a font not of a file (a downloaded one, in memory)
	}
	defer local.release()
	var length uint32
	if hr := local.call(localPathLengthFromKey, uintptr(key), uintptr(keySize), uintptr(unsafe.Pointer(&length))); failed(hr) {
		return ""
	}
	path := make([]uint16, length+1)
	if hr := local.call(localPathFromKey, uintptr(key), uintptr(keySize), uintptr(unsafe.Pointer(&path[0])), uintptr(len(path))); failed(hr) {
		return ""
	}
	return fmt.Sprintf("%s#%d", windows.UTF16ToString(path), index)
}
