package screenshot

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	dllGdi32    = windows.NewLazyDLL("gdi32.dll")
	dllUser32   = windows.NewLazyDLL("user32.dll")
	dllKernel32 = windows.NewLazyDLL("kernel32.dll")

	entryGetDC           = dllUser32.NewProc("GetDC")
	entryGetWindowDC     = dllUser32.NewProc("GetWindowDC")
	entryReleaseDC       = dllUser32.NewProc("ReleaseDC")
	entryEnumWindows     = dllUser32.NewProc("EnumWindows")
	entryGetWindowText   = dllUser32.NewProc("GetWindowTextW")
	entryGetClientRect   = dllUser32.NewProc("GetClientRect")
	entryIsWindowVisible = dllUser32.NewProc("IsWindowVisible")

	entryGlobalFree   = dllKernel32.NewProc("GlobalFree")
	entryGlobalLock   = dllKernel32.NewProc("GlobalLock")
	entryGlobalAlloc  = dllKernel32.NewProc("GlobalAlloc")
	entryGlobalUnlock = dllKernel32.NewProc("GlobalUnlock")
	entryGetLastError = dllKernel32.NewProc("GetLastError")

	entryBitBlt                 = dllGdi32.NewProc("BitBlt")
	entryDeleteDC               = dllGdi32.NewProc("DeleteDC")
	entryGetDiBits              = dllGdi32.NewProc("GetDIBits")
	entryGetObject              = dllGdi32.NewProc("GetObjectW")
	entrySelectObject           = dllGdi32.NewProc("SelectObject")
	entryDeleteObject           = dllGdi32.NewProc("DeleteObject")
	entryGetDeviceCaps          = dllGdi32.NewProc("GetDeviceCaps")
	entryCreateCompatibleDC     = dllGdi32.NewProc("CreateCompatibleDC")
	entryCreateCompatibleBitmap = dllGdi32.NewProc("CreateCompatibleBitmap")
)

const ( // wingdi.h
	cBI_RGB         = 0
	cDIB_RGB_COLORS = 0
	cHORZRES        = 8
	cVERTRES        = 10
	cSRCCPY         = 0x00CC0020
	cCAPTUREBLT     = 0x40000000
)

const ( // winbase.h
	cGHND = 0x42
)

const ( // win32-error-codes
	cNO_ERROR = 0
)

type bitmapStruct struct { // wingdi.h
	BmType       int32
	BmWidth      int32
	BmHeight     int32
	BmWidthBytes int32
	BmPlanes     uint16
	BmBitsPixel  uint16
	BmBits       uint32
	_            uint64 // aling up to 32 bytes
}

type bitmapInfoHeaderStruct struct { // wingdi.h
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type rectStruct struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

func funcGetDC(hwnd uintptr) (uintptr, error) {
	r0, _, err := entryGetDC.Call(hwnd)
	if r0 == 0 {
		return 0, err
	}
	return r0, nil
}

func funcGetWindowDC(hwnd uintptr) (uintptr, error) {
	r0, _, err := entryGetWindowDC.Call(hwnd)
	if r0 == 0 {
		return 0, err
	}
	return r0, nil
}

func funcGetClientRect(hwnd uintptr) (*rectStruct, error) {
	rect := rectStruct{}
	r0, _, err := entryGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if r0 == 0 {
		return nil, err
	}
	return &rect, nil
}

func funcGetWindowText(hwnd uintptr) (string, error) {
	buffer := make([]uint16, 256)
	r0, _, err := entryGetWindowText.Call(hwnd, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if r0 == 0 {
		return "", err
	}
	title := syscall.UTF16ToString(buffer)
	return title, nil
}

func funcIsWindowVisible(hwnd uintptr) (bool, error) {
	r0, _, _ := entryIsWindowVisible.Call(hwnd)
	return r0 == 1, nil
}

var findWindowCallback uintptr

type findWindowCallbackData struct {
	PWndHandle uintptr
	PWndTitle  string
}

func funcFindWindow(windowName string) (uintptr, error) {
	if findWindowCallback == 0 {
		f := func(h uintptr, p uintptr) uintptr {
			title, _ := funcGetWindowText(h)
			if title == "" {
				return 1
			}
			fwcd := (*findWindowCallbackData)(unsafe.Pointer(p))
			if title == fwcd.PWndTitle {
				fwcd.PWndHandle = h
				return 0
			}
			return 1
		}
		findWindowCallback = syscall.NewCallback(f)
	}
	f := findWindowCallbackData{}
	f.PWndTitle = windowName
	r0, _, _ := entryEnumWindows.Call(findWindowCallback, uintptr(unsafe.Pointer(&f)))
	if r0 == 0 && f.PWndHandle == 0 {
		return 0, fmt.Errorf("entryEnumWindows")
	}
	if f.PWndHandle == 0 {
		return 0, fmt.Errorf("window with title %v not found", windowName)
	}
	return f.PWndHandle, nil
}

func funcReleaseDC(hwnd uintptr, hdc uintptr) error {
	r0, _, err := entryReleaseDC.Call(hwnd, hdc)
	if r0 == 0 || r0 != 1 {
		return err
	}
	return nil
}

func funcGlobalAlloc(flags uintptr, size uintptr) (uintptr, error) {
	r0, _, err := entryGlobalAlloc.Call(flags, size)
	if r0 == 0 {
		return 0, err
	}
	return r0, nil
}

func funcGlobalFree(hMem uintptr) (uintptr, error) {
	r0, _, err := entryGlobalFree.Call(hMem)
	if r0 == 0 {
		return 0, nil
	} else {
		return r0, err
	}
}

func funcGlobalLock(hMem uintptr) (uintptr, error) {
	r0, _, err := entryGlobalLock.Call(hMem)
	if r0 == 0 {
		return 0, err
	}
	return r0, nil
}

func funcGlobalUnlock(hMem uintptr) error {
	r0, _, err := entryGlobalUnlock.Call(hMem)
	r1, _ := funcGetLastError()
	if r0 == 0 && r1 != cNO_ERROR {
		return err
	} else {
		return nil
	}
}

func funcGetLastError() (uintptr, error) {
	r0, _, err := entryGetLastError.Call()
	return r0, err
}

func funcCreateCompatibleDC(hdc uintptr) (uintptr, error) {
	r0, _, err := entryCreateCompatibleDC.Call(hdc)
	if r0 == 0 {
		return 0, err
	}
	return r0, nil
}

func funcCreateCompatibleBitmap(hdc uintptr, width uintptr, height uintptr) (uintptr, error) {
	r0, _, err := entryCreateCompatibleBitmap.Call(hdc, width, height)
	if r0 == 0 {
		return 0, err
	}
	return r0, nil
}

func funcGetDeviceCaps(hdc uintptr, index uintptr) (uintptr, error) {
	r0, _, err := entryGetDeviceCaps.Call(hdc, index)
	if r0 == 0 {
		return 0, err
	}
	return r0, nil
}

func funcGetObject(handle uintptr, c uintptr, pv uintptr) (uintptr, error) {
	r0, _, err := entryGetObject.Call(handle, c, pv)
	if r0 == 0 {
		return 0, err
	}
	return r0, nil
}

func funcSelectObject(hdc uintptr, hgdiobj uintptr) (uintptr, error) {
	r0, _, err := entrySelectObject.Call(hdc, hgdiobj)
	if r0 == 0 {
		return 0, err
	}
	return r0, nil
}

func funcDeleteObject(hgdiobj uintptr) error {
	r0, _, err := entryDeleteObject.Call(hgdiobj)
	if r0 == 0 {
		return err
	}
	return nil
}

func funcGetDiBits(hDC uintptr, hBitmap uintptr, start uintptr, cLines uintptr, lpvBits uintptr, lpBitmapInfo uintptr, usage uintptr) (uintptr, error) {
	r0, _, err := entryGetDiBits.Call(hDC, hBitmap, start, cLines, lpvBits, lpBitmapInfo, usage)
	if r0 == 0 {
		return 0, err
	}
	return r0, nil
}

func funcBitBlt(hdc uintptr, x uintptr, y uintptr, width uintptr, height uintptr, hdcsrc uintptr, x1 uintptr, y1 uintptr, rop uintptr) error {
	r0, _, err := entryBitBlt.Call(hdc, x, y, width, height, hdcsrc, x1, y1, rop)
	if r0 == 0 {
		return err
	}
	return nil
}

func funcDeleteDC(hdc uintptr) error {
	r0, _, err := entryDeleteDC.Call(hdc)
	if r0 == 0 {
		return err
	}
	return nil
}
