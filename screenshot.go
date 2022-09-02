package screenshot

import (
	"fmt"
	"image"
	"reflect"
	"syscall"
	"unsafe"
)

type ScreenshotState struct {
	hWnd       uintptr
	hDC        uintptr // dstr
	hMemoryDC  uintptr // dstr
	width      uintptr
	height     uintptr
	hBitmap    uintptr // dstr
	hOldBitmap uintptr
	imageData  []byte // dstr
}

func (ss *ScreenshotState) Width() int {
	return int(ss.width)
}

func (ss *ScreenshotState) Height() int {
	return int(ss.height)
}

func CreateState() (*ScreenshotState, error) {
	ss := ScreenshotState{}
	var err error
	ss.hDC, err = funcGetDC(0)
	if err != nil {
		ss.Destroy()
		return nil, err
	}
	ss.hMemoryDC, err = funcCreateCompatibleDC(ss.hDC)
	if err != nil {
		ss.Destroy()
		return nil, err
	}
	ss.width, err = funcGetDeviceCaps(ss.hDC, cHORZRES)
	if err != nil {
		ss.Destroy()
		return nil, err
	}
	ss.height, err = funcGetDeviceCaps(ss.hDC, cVERTRES)
	if err != nil {
		ss.Destroy()
		return nil, err
	}
	ss.hBitmap, err = funcCreateCompatibleBitmap(ss.hDC, ss.width, ss.height)
	if err != nil {
		ss.Destroy()
		return nil, err
	}
	ss.imageData = make([]byte, 0)
	return &ss, nil
}

func CreateStateWindow(title string) (*ScreenshotState, error) {
	ss := ScreenshotState{}
	var err error
	ss.hWnd, err = funcFindWindow(title)
	if err != nil {
		ss.Destroy()
		return nil, err
	}
	ss.hDC, err = funcGetDC(ss.hWnd)
	if err != nil {
		ss.Destroy()
		return nil, err
	}
	ss.hMemoryDC, err = funcCreateCompatibleDC(ss.hDC)
	if err != nil {
		ss.Destroy()
		return nil, err
	}
	rect, err := funcGetClientRect(ss.hWnd)
	if err != nil {
		ss.Destroy()
		return nil, err
	}
	ss.width = uintptr(rect.Right - rect.Left)
	ss.height = uintptr(rect.Bottom - rect.Top)
	ss.hBitmap, err = funcCreateCompatibleBitmap(ss.hDC, ss.width, ss.height)
	if err != nil {
		ss.Destroy()
		return nil, err
	}
	return &ss, nil
}

func (ss *ScreenshotState) MakeScreenshot() (*image.RGBA, error) {
	var err error

	if ss.hWnd != 0 {
		rect, _ := funcGetClientRect(ss.hWnd)
		if rect == nil {
			return nil, fmt.Errorf("invalid client rect")
		}
		if ss.width != uintptr(rect.Right-rect.Left) ||
			ss.height != uintptr(rect.Bottom-rect.Top) {
			return nil, fmt.Errorf("window has changed size")
		}
	}

	ss.hOldBitmap, err = funcSelectObject(ss.hMemoryDC, ss.hBitmap)
	if err != nil {
		return nil, fmt.Errorf("select object hBitmap error")
	}
	err = funcBitBlt(ss.hMemoryDC, 0, 0,
		ss.width, ss.height, ss.hDC, 0, 0,
		cSRCCPY)
	if err != nil {
		funcSelectObject(ss.hMemoryDC, ss.hOldBitmap)
		return nil, fmt.Errorf("bitblt error")
	}
	_, err = funcSelectObject(ss.hMemoryDC, ss.hOldBitmap)
	if err != nil {
		return nil, fmt.Errorf("select object hOldBitmap error")
	}

	bmpScreen := bitmapStruct{}
	funcGetObject(ss.hBitmap, unsafe.Sizeof(bmpScreen), uintptr(unsafe.Pointer(&bmpScreen)))
	if err != nil {
		return nil, fmt.Errorf("get object error")
	}

	bmpInfo := bitmapInfoHeaderStruct{}
	bmpInfo.BiSize = uint32(unsafe.Sizeof(bmpInfo))
	bmpInfo.BiWidth = bmpScreen.BmWidth
	bmpInfo.BiHeight = -bmpScreen.BmHeight
	bmpInfo.BiPlanes = bmpScreen.BmPlanes
	bmpInfo.BiBitCount = bmpScreen.BmBitsPixel
	bmpInfo.BiCompression = cBI_RGB

	dwBmpSize := int(((bmpScreen.BmWidth*int32(bmpInfo.BiBitCount) + 31) / 32) * 4 * bmpScreen.BmHeight)

	hDIB, err := funcGlobalAlloc(cGHND, uintptr(dwBmpSize))
	if err != nil {
		return nil, fmt.Errorf("global alloc error")
	}
	defer funcGlobalFree(hDIB)

	lpBitmap, err := funcGlobalLock(hDIB)
	if err != nil {
		return nil, fmt.Errorf("global lock error")
	}
	defer funcGlobalUnlock(hDIB)

	_, err = funcGetDiBits(ss.hDC, ss.hBitmap, 0, uintptr(bmpScreen.BmHeight), lpBitmap, uintptr(unsafe.Pointer(&bmpInfo)), cDIB_RGB_COLORS)
	if err != nil {
		return nil, fmt.Errorf("get dibits error")
	}

	var captData []byte
	cdHeader := (*reflect.SliceHeader)(unsafe.Pointer(&captData))
	cdHeader.Data = lpBitmap
	cdHeader.Len = int(dwBmpSize)
	cdHeader.Cap = int(dwBmpSize)

	if len(ss.imageData) != dwBmpSize {
		ss.imageData = make([]byte, dwBmpSize)
	}

	for pixIdx := 0; pixIdx < int(dwBmpSize); pixIdx += 4 {
		ss.imageData[pixIdx], ss.imageData[pixIdx+1], ss.imageData[pixIdx+2], ss.imageData[pixIdx+3] = captData[pixIdx+2], captData[pixIdx+1], captData[pixIdx], 0xFF
	}

	img := image.RGBA{}
	img.Pix = ss.imageData
	img.Stride = 4 * int(bmpInfo.BiWidth)
	img.Rect = image.Rect(0, 0, int(bmpInfo.BiWidth), -int(bmpInfo.BiHeight))

	return &img, nil
}

func (ss *ScreenshotState) Destroy() {
	if ss.hBitmap != 0 {
		funcDeleteObject(ss.hBitmap)
		ss.hBitmap = 0
	}
	if ss.hMemoryDC != 0 {
		funcDeleteDC(ss.hMemoryDC)
		ss.hMemoryDC = 0
	}
	if ss.hDC != 0 {
		funcReleaseDC(0, ss.hDC)
		ss.hDC = 0
	}
	ss.imageData = make([]byte, 0)
}

type WindowStruct struct {
	HWND  uintptr
	Title string
}

func EnumWindowList() []WindowStruct {
	var enumWindowCallback uintptr
	ww := make([]WindowStruct, 0)
	if enumWindowCallback == 0 {
		f := func(h uintptr, p uintptr) uintptr {
			title, err := funcGetWindowText(h)
			if err != nil {
				return 1
			}
			if len(title) != 0 {
				if b, _ := funcIsWindowVisible(h); b {
					ww = append(ww, WindowStruct{HWND: h, Title: title})
				}
			}
			return 1
		}
		enumWindowCallback = syscall.NewCallback(f)
	}
	entryEnumWindows.Call(enumWindowCallback, 0)
	return ww
}
