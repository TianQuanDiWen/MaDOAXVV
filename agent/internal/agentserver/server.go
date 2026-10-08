package agentserver

import (
	"flag"
	"fmt"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/TianQuanDiWen/MaDOAXVV/agent/internal/clickaway"
	"github.com/TianQuanDiWen/MaDOAXVV/agent/internal/runtimepath"
)

// Run 启动供 MXU 或 MPE 连接的 MaaFramework AgentServer。
// MXU 会将通信标识符追加到 child_args 末尾；MPE 或手动模式可使用 --identifier 或默认 "pi-agent-1"。
func Run(args []string) error {
	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	root := flags.String("root", ".", "project root containing maafw and resource")
	identifierFlag := flags.String("identifier", "pi-agent-1", "socket identifier for MaaFramework agent")
	if err := flags.Parse(args); err != nil {
		return err
	}
	remaining := flags.Args()
	var identifier string
	switch len(remaining) {
	case 0:
		identifier = *identifierFlag
	case 1:
		identifier = remaining[0]
	default:
		return fmt.Errorf("agent mode accepts at most one socket identifier, got %d", len(remaining))
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

	// 挂载防遮挡退避旁路监听器
	evasionSink := clickaway.NewEvasionSink()
	_ = maa.AgentServerAddContextSink(evasionSink)

	// 自定义识别和动作必须在启动通信服务前完成注册。
	if err := maa.AgentServerStartUp(identifier); err != nil {
		return fmt.Errorf("start AgentServer: %w", err)
	}
	defer maa.AgentServerShutDown()
	maa.AgentServerJoin()
	return nil
}
