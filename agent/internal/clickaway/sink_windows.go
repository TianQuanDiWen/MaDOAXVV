package clickaway

import (
	"encoding/json"
	"errors"
	"image"
	"runtime"
	"syscall"
	"unsafe"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/MaaXYZ/maa-framework-go/v4/controller/win32"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	getForegroundWindow = user32.NewProc("GetForegroundWindow")
	getClientRect       = user32.NewProc("GetClientRect")
	getCursorPos        = user32.NewProc("GetCursorPos")
	screenToClient      = user32.NewProc("ScreenToClient")
	setThreadDPI        = user32.NewProc("SetThreadDpiAwarenessContext")
)

func init() {
	readCursorGeometry = win32CursorGeometry
}

// win32CursorGeometry 用 Maa 的原始截图和识别图尺寸准备鼠标坐标换算。
func win32CursorGeometry(ctx *maa.Context) (cursorGeometry, error) {
	tasker := ctx.GetTasker()
	if tasker == nil {
		return cursorGeometry{}, errors.New("no tasker")
	}
	controller := tasker.GetController()
	if controller == nil {
		return cursorGeometry{}, errors.New("no controller")
	}
	info, err := controller.GetInfo()
	if err != nil {
		return cursorGeometry{}, err
	}
	hwnd, err := foregroundControllerWindow(info)
	if err != nil {
		return cursorGeometry{}, err
	}
	rawWidth, rawHeight, err := controller.GetResolution()
	if err != nil || rawWidth <= 0 || rawHeight <= 0 {
		return cursorGeometry{}, errors.New("controller capture resolution unavailable")
	}
	frame, err := controller.CacheImage()
	if err != nil || frame == nil || frame.Bounds().Empty() {
		return cursorGeometry{}, errors.New("recognition image size unavailable")
	}
	bounds := frame.Bounds().Sub(frame.Bounds().Min)
	captureSize := image.Pt(int(rawWidth), int(rawHeight))
	// 按缩放比例给落点留出余量，避免坐标取整后落回识别区域。
	margin := func(scaled, raw int) int {
		if scaled == raw {
			return 0
		}
		return (scaled + raw - 1) / raw
	}
	return cursorGeometry{
		bounds: bounds,
		margin: image.Pt(margin(bounds.Dx(), captureSize.X), margin(bounds.Dy(), captureSize.Y)),
		cursor: func() (image.Point, bool) {
			return cursorInImage(hwnd, captureSize, bounds.Size())
		},
	}, nil
}

func foregroundControllerWindow(info string) (uintptr, error) {
	var cfg struct {
		Type        string            `json:"type"`
		HWND        uintptr           `json:"hwnd"`
		MouseMethod win32.InputMethod `json:"mouse_method"`
	}
	if err := json.Unmarshal([]byte(info), &cfg); err != nil {
		return 0, err
	}
	// 后台输入会恢复桌面鼠标位置，无法据此判断游戏窗口中的悬停位置。
	if cfg.Type != "win32" || cfg.MouseMethod != win32.InputSeize {
		return 0, errors.New("controller does not use persistent foreground cursor input")
	}
	if cfg.HWND == 0 {
		return 0, errors.New("invalid controller window handle")
	}
	return cfg.HWND, nil
}

// cursorInImage 将桌面鼠标坐标转成目标窗口的客户区坐标，再缩放到识别图。
func cursorInImage(hwnd uintptr, captureSize, imageSize image.Point) (image.Point, bool) {
	// DPI 设置按线程生效，坐标读取和换算必须在同一系统线程执行。
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := setThreadDPI.Find(); err != nil {
		return image.Point{}, false
	}
	previous, _, _ := setThreadDPI.Call(^uintptr(3)) // -4 表示按显示器感知 DPI（V2）。
	if previous == 0 {
		return image.Point{}, false
	}
	defer setThreadDPI.Call(previous)

	if visible, _, _ := getForegroundWindow.Call(); visible != hwnd {
		return image.Point{}, false
	}
	var rect struct{ left, top, right, bottom int32 }
	var point struct{ x, y int32 }
	rectOK, _, _ := getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	cursorOK, _, _ := getCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	if rectOK == 0 || cursorOK == 0 || image.Pt(int(rect.right-rect.left), int(rect.bottom-rect.top)) != captureSize {
		return image.Point{}, false
	}
	converted, _, _ := screenToClient.Call(hwnd, uintptr(unsafe.Pointer(&point)))
	x, y := int(point.x), int(point.y)
	if converted == 0 || x < 0 || y < 0 || x >= captureSize.X || y >= captureSize.Y {
		return image.Point{}, false
	}
	return image.Pt(x*imageSize.X/captureSize.X, y*imageSize.Y/captureSize.Y), true
}
