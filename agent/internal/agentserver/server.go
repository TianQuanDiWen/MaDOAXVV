package agentserver

import (
	"flag"
	"fmt"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/TianQuanDiWen/MaDOAXVV/agent/internal/runtimepath"
)

// Run 启动供 MXU 连接的 MaaFramework AgentServer。
// MXU 会将通信标识符追加到 child_args 末尾，因此这里要求恰好有一个位置参数。
func Run(args []string) error {
	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	root := flags.String("root", ".", "project root containing maafw and resource")
	if err := flags.Parse(args); err != nil {
		return err
	}
	remaining := flags.Args()
	if len(remaining) != 1 {
		return fmt.Errorf("agent mode requires the MXU socket identifier")
	}

	paths, err := runtimepath.Resolve(*root)
	if err != nil {
		return err
	}
	if err := maa.Init(
		maa.WithLibDir(paths.Lib),
		maa.WithLogDir(paths.Debug),
	); err != nil {
		return fmt.Errorf("initialize MaaFramework: %w", err)
	}
	defer maa.Release()

	registry, err := BuildRegistry()
	if err != nil {
		return fmt.Errorf("build extension registry: %w", err)
	}
	if err := registry.RegisterAgentServer(); err != nil {
		return err
	}

	identifier := remaining[0]
	// 自定义识别和动作必须在启动通信服务前完成注册。
	if err := maa.AgentServerStartUp(identifier); err != nil {
		return fmt.Errorf("start AgentServer: %w", err)
	}
	defer maa.AgentServerShutDown()
	maa.AgentServerJoin()
	return nil
}
