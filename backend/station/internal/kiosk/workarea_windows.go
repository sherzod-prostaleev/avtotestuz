//go:build windows

package kiosk

import (
	"syscall"
	"unsafe"
)

// workArea returns the primary monitor's work area -- the screen minus the
// taskbar -- as Chrome's --window-position and --window-size want it.
//
// Those two switches speak DIPs: logical pixels at 96 DPI. This process is
// DPI-unaware, because cmd/genwinres embeds an icon and a version block and no
// application manifest, so Windows virtualises these coordinates to 96 DPI
// before handing them over. The two units therefore already agree, and a
// classroom screen running at 125% or 150% needs no arithmetic here. If the
// agent ever gains a manifest declaring DPI awareness, this stops being true
// and the result has to be divided by the monitor's scale factor.
func workArea() (x, y, w, h int, ok bool) {
	const spiGetWorkArea = 0x0030
	var r struct{ Left, Top, Right, Bottom int32 }
	ret, _, _ := syscall.NewLazyDLL("user32.dll").
		NewProc("SystemParametersInfoW").
		Call(uintptr(spiGetWorkArea), 0, uintptr(unsafe.Pointer(&r)), 0)
	if ret == 0 {
		return 0, 0, 0, 0, false
	}
	w, h = int(r.Right-r.Left), int(r.Bottom-r.Top)
	if w <= 0 || h <= 0 {
		return 0, 0, 0, 0, false
	}
	return int(r.Left), int(r.Top), w, h, true
}
