package loopguard

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// Param 对应流水线节点中 custom_action_param 的 JSON 配置。
type Param struct {
	MaxRetry        int `json:"max_retry"`         // 允许的最大重试次数，默认 4 次
	ResetTimeoutSec int `json:"reset_timeout_sec"` // 判定闲置自动清零的时间窗口（秒），默认 15 秒
}

// Runner 实现 maa.CustomActionRunner 接口，提供独立的重复计数守卫。
type Runner struct {
	mu       sync.Mutex
	counts   map[string]int
	lastTime map[string]time.Time
}

// NewRunner 创建一个循环守卫运行器。
func NewRunner() *Runner {
	return &Runner{
		counts:   make(map[string]int),
		lastTime: make(map[string]time.Time),
	}
}

// Run 执行守卫判定：
// 每次执行，节点计数 +1。
// 若未超过 max_retry，返回 true（成功，放行继续重试）；
// 若超过 max_retry，计数器立即清零并返回 false（失败，触发 on_error 分支）。
func (r *Runner) Run(ctx *maa.Context, arg *maa.CustomActionArg) bool {
	var p Param
	if arg.CustomActionParam != "" {
		_ = json.Unmarshal([]byte(arg.CustomActionParam), &p)
	}
	if p.MaxRetry <= 0 {
		p.MaxRetry = 4
	}
	if p.ResetTimeoutSec <= 0 {
		p.ResetTimeoutSec = 15
	}

	key := arg.CurrentTaskName

	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. 检查闲置超时：若距离上次调用超过 ResetTimeoutSec 秒，说明是新的一轮流程，自动归零
	if time.Since(r.lastTime[key]) > time.Duration(p.ResetTimeoutSec)*time.Second {
		r.counts[key] = 0
	}
	r.lastTime[key] = time.Now()

	r.counts[key]++

	// 2. 检查是否在允许的重试次数以内
	if r.counts[key] <= p.MaxRetry {
		fmt.Printf("[LoopGuard] 节点 %q 局部重试第 %d/%d 次 (放行)\n", key, r.counts[key], p.MaxRetry)
		return true
	}

	// 3. 达到并超过最大重试阈值：立即归零并报错熔断，触发 on_error！
	fmt.Printf("[LoopGuard] 节点 %q 重试已达 %d 次上限，立即清零计数并触发 on_error 熔断逃逸\n", key, p.MaxRetry)
	r.counts[key] = 0
	return false
}
