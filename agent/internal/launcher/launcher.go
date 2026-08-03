package launcher

import (
	"errors"
	"flag"
	"fmt"
	"os/exec"
	"regexp"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
	"github.com/MaaXYZ/maa-framework-go/v4/controller/win32"
	"github.com/TianQuanDiWen/MaDOAXVV/agent/internal/runtimepath"
)

const launcherEntry = "Launcher_ClickStartGame"

// config 只保存启动器流程所需参数；默认策略集中在 Run 中初始化。
type config struct {
	root                 string
	steamURI             string
	launcherProcess      string
	gameWindowPattern    string
	timeout              time.Duration
	retryClickAfter      time.Duration
	stableWindowDuration time.Duration
}

// Run 解析启动游戏模式参数，并执行完整的启动器引导流程。
func Run(args []string) error {
	flags := flag.NewFlagSet("launch-game", flag.ContinueOnError)
	root := flags.String("root", ".", "project root containing maafw and resource")
	steamURI := flags.String("steam-uri", "", "Steam URI used to start the game")
	launcherProcess := flags.String("launcher-process", "", "exact launcher executable name")
	gameWindow := flags.String("game-window", "", "regular expression for the game window title")
	timeout := flags.Duration("timeout", 5*time.Minute, "overall startup timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *steamURI == "" || *launcherProcess == "" || *gameWindow == "" {
		return errors.New("steam-uri, launcher-process, and game-window are required")
	}

	cfg := config{
		root:                 *root,
		steamURI:             *steamURI,
		launcherProcess:      *launcherProcess,
		gameWindowPattern:    *gameWindow,
		timeout:              *timeout,
		retryClickAfter:      15 * time.Second,
		stableWindowDuration: 3 * time.Second,
	}
	return run(cfg)
}

// run 初始化 MaaFramework，驱动启动器 OCR，并在游戏主窗口稳定后结束预任务。
func run(cfg config) error {
	gameWindowRegex, err := regexp.Compile(cfg.gameWindowPattern)
	if err != nil {
		return fmt.Errorf("compile game window regular expression: %w", err)
	}
	paths, err := runtimepath.Resolve(cfg.root)
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
	if err := maa.ConfigInitOption(paths.Debug, "{}"); err != nil {
		return fmt.Errorf("initialize MaaToolkit: %w", err)
	}

	// 游戏已经运行时直接返回，避免重复打开 Steam 或误操作启动器。
	ready, err := hasGameWindow(gameWindowRegex)
	if err != nil {
		return err
	}
	if ready {
		logf("game is already running")
		return nil
	}

	resource, err := maa.NewResource()
	if err != nil {
		return fmt.Errorf("create resource: %w", err)
	}
	defer resource.Destroy()
	logf("loading launcher OCR resource")
	if job := resource.PostBundle(paths.Resource).Wait(); !job.Success() {
		return fmt.Errorf("load resource failed with status %s", job.Status())
	}

	logf("starting game through Steam: %s", cfg.steamURI)
	if err := openURI(cfg.steamURI); err != nil {
		return err
	}

	deadline := time.Now().Add(cfg.timeout)
	// 启动器可能短暂重建窗口；按窗口句柄分别控制 OCR 重试频率。
	lastAttempts := make(map[uintptr]time.Time)
	buttonClicked := false
	for time.Now().Before(deadline) {
		ready, err := hasGameWindow(gameWindowRegex)
		if err != nil {
			return err
		}
		if ready {
			if err := waitForStableGameWindow(gameWindowRegex, deadline, cfg.stableWindowDuration); err != nil {
				return err
			}
			logf("game window is ready; handing control back to MXU")
			return nil
		}

		window, err := findLauncherWindow(cfg.launcherProcess)
		if err != nil {
			return err
		}
		if window != nil {
			handle := uintptr(window.Handle)
			if last, found := lastAttempts[handle]; !found || time.Since(last) >= cfg.retryClickAfter {
				lastAttempts[handle] = time.Now()
				logf("scanning %s window: %s", cfg.launcherProcess, window.WindowName)
				clicked, clickErr := clickStartGame(resource, window)
				if clickErr != nil {
					logf("launcher recognition attempt failed: %v", clickErr)
				} else if clicked {
					buttonClicked = true
					logf("Start Game was recognized and clicked")
				}
			}
		}
		time.Sleep(time.Second)
	}

	if buttonClicked {
		return fmt.Errorf("timed out waiting for the game window after clicking Start Game (%s)", cfg.timeout)
	}
	return fmt.Errorf("Start Game was not recognized within %s", cfg.timeout)
}

// openURI 交给 Windows Shell 处理 steam:// 协议，不绑定 Steam 的安装路径。
func openURI(uri string) error {
	if err := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", uri).Start(); err != nil {
		return fmt.Errorf("open Steam URI: %w", err)
	}
	return nil
}

// hasGameWindow 枚举桌面窗口并判断是否存在标题符合规则的游戏窗口。
func hasGameWindow(pattern *regexp.Regexp) (bool, error) {
	windows, err := maa.FindDesktopWindows()
	if err != nil {
		return false, fmt.Errorf("enumerate desktop windows: %w", err)
	}
	for _, window := range windows {
		if pattern.MatchString(window.WindowName) {
			return true, nil
		}
	}
	return false, nil
}

// waitForStableGameWindow 要求目标窗口连续存在指定时间，避免命中过渡窗口。
func waitForStableGameWindow(pattern *regexp.Regexp, deadline time.Time, duration time.Duration) error {
	var stableSince time.Time
	for time.Now().Before(deadline) {
		ready, err := hasGameWindow(pattern)
		if err != nil {
			return err
		}
		if ready {
			if stableSince.IsZero() {
				stableSince = time.Now()
				logf("game window found; waiting for it to become stable")
			} else if time.Since(stableSince) >= duration {
				return nil
			}
		} else {
			stableSince = time.Time{}
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("game window did not remain stable before timeout")
}

// findLauncherWindow 先精确匹配进程，再用 PID 关联窗口，避免依赖易变化的启动器标题。
func findLauncherWindow(executable string) (*maa.DesktopWindow, error) {
	processIDs, err := processIDsByExecutable(executable)
	if err != nil {
		return nil, fmt.Errorf("find launcher process: %w", err)
	}
	if len(processIDs) == 0 {
		return nil, nil
	}
	windows, err := maa.FindDesktopWindows()
	if err != nil {
		return nil, fmt.Errorf("enumerate launcher windows: %w", err)
	}
	for _, window := range windows {
		if _, found := processIDs[windowProcessID(window.Handle)]; found {
			return window, nil
		}
	}
	return nil, nil
}

// clickStartGame 为启动器临时创建 Controller 和 Tasker，并确认 OCR 点击动作确实成功。
func clickStartGame(resource *maa.Resource, window *maa.DesktopWindow) (bool, error) {
	controller, err := maa.NewWin32Controller(
		window.Handle,
		win32.ScreencapFramePool,
		win32.InputPostMessageWithCursorPos,
		win32.InputPostMessageWithCursorPos,
	)
	if err != nil {
		return false, fmt.Errorf("create launcher controller: %w", err)
	}
	defer controller.Destroy()
	if job := controller.PostConnect().Wait(); !job.Success() {
		return false, fmt.Errorf("connect launcher controller failed with status %s", job.Status())
	}

	tasker, err := maa.NewTasker()
	if err != nil {
		return false, fmt.Errorf("create launcher tasker: %w", err)
	}
	defer tasker.Destroy()
	if err := tasker.BindResource(resource); err != nil {
		return false, err
	}
	if err := tasker.BindController(controller); err != nil {
		return false, err
	}
	if !tasker.Initialized() {
		return false, errors.New("launcher tasker was not initialized")
	}

	job := tasker.PostTask(launcherEntry).Wait()
	if !job.Success() {
		return false, fmt.Errorf("launcher OCR task failed with status %s", job.Status())
	}
	detail, err := job.GetDetail()
	if err != nil {
		return false, fmt.Errorf("get launcher OCR task detail: %w", err)
	}
	// 任务结束不等于按钮一定被点击，需要确认目标节点及其 Action 都成功完成。
	for _, nodeRef := range detail.Nodes {
		node, err := nodeRef.GetDetail()
		if err != nil {
			return false, fmt.Errorf("get launcher OCR node detail: %w", err)
		}
		if node.Name == launcherEntry && node.RunCompleted && node.Action != nil && node.Action.Success {
			return true, nil
		}
	}
	return false, nil
}

// logf 为启动器流程输出带统一前缀的运行日志，便于在 MXU 日志中筛选。
func logf(format string, args ...any) {
	fmt.Printf("[Game launcher] "+format+"\n", args...)
}
