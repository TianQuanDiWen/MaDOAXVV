package runtimepath

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths 描述 MXU 发布目录中 Agent 会使用的固定路径。
type Paths struct {
	Root     string
	Lib      string
	Resource string
	Debug    string
}

// Resolve 根据 Project Interface 的工作目录解析运行路径。
// 发布包使用 maafw/resource，源码目录下的 MaaTools 调试使用 deps/bin/assets/resource。
func Resolve(root string) (Paths, error) {
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return Paths{}, fmt.Errorf("get working directory: %w", err)
		}
	}

	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, fmt.Errorf("resolve project root: %w", err)
	}

	paths := Paths{
		Root:     absoluteRoot,
		Lib:      filepath.Join(absoluteRoot, "maafw"),
		Resource: filepath.Join(absoluteRoot, "resource"),
		Debug:    filepath.Join(absoluteRoot, "debug", "agent"),
	}
	if !isDirectory(paths.Lib) && !isDirectory(paths.Resource) {
		devLib := filepath.Join(absoluteRoot, "deps", "bin")
		devResource := filepath.Join(absoluteRoot, "assets", "resource")
		if isDirectory(devLib) && isDirectory(devResource) {
			paths.Lib = devLib
			paths.Resource = devResource
			paths.Debug = filepath.Join(absoluteRoot, "cache", "maa-logs", "agent")
		}
	}
	if err := requireDirectory(paths.Lib, "MaaFramework library"); err != nil {
		return Paths{}, err
	}
	if err := requireDirectory(paths.Resource, "resource"); err != nil {
		return Paths{}, err
	}
	// Agent 日志独立存放，避免与 MXU 主日志混在一起。
	if err := os.MkdirAll(paths.Debug, 0o755); err != nil {
		return Paths{}, fmt.Errorf("create agent debug directory: %w", err)
	}
	return paths, nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// requireDirectory 校验运行所需目录存在且确实是目录，并在错误中附带用途标签。
func requireDirectory(path, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s directory is unavailable at %s: %w", label, path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s path is not a directory: %s", label, path)
	}
	return nil
}
