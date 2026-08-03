//go:build !windows

package launcher

import (
	"fmt"
	"unsafe"
)

// 非 Windows 构筑保留清晰的错误信息；当前启动器任务只支持 Win32 Controller。
func processIDsByExecutable(string) (map[uint32]struct{}, error) {
	return nil, fmt.Errorf("launcher process lookup is only supported on Windows")
}

// windowProcessID 在非 Windows 平台返回空 PID；该分支不会进入实际启动器流程。
func windowProcessID(unsafe.Pointer) uint32 {
	return 0
}
