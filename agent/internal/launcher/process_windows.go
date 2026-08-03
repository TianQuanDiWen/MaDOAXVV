//go:build windows

package launcher

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const (
	th32csSnapProcess = 0x00000002
	maxPath           = 260
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	createToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	process32FirstW          = kernel32.NewProc("Process32FirstW")
	process32NextW           = kernel32.NewProc("Process32NextW")
	user32                   = syscall.NewLazyDLL("user32.dll")
	getWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
)

// processEntry32 对应 Windows PROCESSENTRY32W，只保留调用 Toolhelp API 所需的布局。
type processEntry32 struct {
	Size              uint32
	Usage             uint32
	ProcessID         uint32
	DefaultHeapID     uintptr
	ModuleID          uint32
	Threads           uint32
	ParentProcessID   uint32
	PriorityClassBase int32
	Flags             uint32
	ExeFile           [maxPath]uint16
}

// processIDsByExecutable 精确匹配可执行文件名，返回所有同名进程 PID。
// 这里直接使用 Windows API，避免为了单次进程查询引入额外运行时或第三方工具。
func processIDsByExecutable(executable string) (map[uint32]struct{}, error) {
	target := strings.ToLower(executable)
	snapshot, _, callErr := createToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if syscall.Handle(snapshot) == syscall.InvalidHandle {
		return nil, fmt.Errorf("create process snapshot: %w", callErr)
	}
	defer syscall.CloseHandle(syscall.Handle(snapshot))

	entry := processEntry32{Size: uint32(unsafe.Sizeof(processEntry32{}))}
	result := make(map[uint32]struct{})
	ok, _, _ := process32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		name := strings.ToLower(syscall.UTF16ToString(entry.ExeFile[:]))
		if name == target {
			result[entry.ProcessID] = struct{}{}
		}
		ok, _, _ = process32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	}
	return result, nil
}

// windowProcessID 查询桌面窗口所属的进程 PID。
func windowProcessID(handle unsafe.Pointer) uint32 {
	var processID uint32
	getWindowThreadProcessID.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&processID)),
	)
	return processID
}

