package main

import (
	"fmt"
	"os"

	"github.com/TianQuanDiWen/MaDOAXVV/agent/internal/agentserver"
	"github.com/TianQuanDiWen/MaDOAXVV/agent/internal/launcher"
)

// main 将命令行参数交给模式分发器，并将错误写入标准错误后返回非零退出码。
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "MaDOAXVV Agent failed: %v\n", err)
		os.Exit(1)
	}
}

// run 根据第一个参数选择运行模式。
// 单一可执行文件可以让 MXU 预任务和 AgentServer 共用版本及 MaaFramework binding。
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing mode: expected agent or launch-game")
	}

	switch args[0] {
	case "agent":
		return agentserver.Run(args[1:])
	case "launch-game":
		return launcher.Run(args[1:])
	default:
		return fmt.Errorf("unknown mode %q: expected agent or launch-game", args[0])
	}
}
