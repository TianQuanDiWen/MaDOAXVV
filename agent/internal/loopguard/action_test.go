package loopguard

import (
	"testing"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

func TestLoopGuardRunner_Run(t *testing.T) {
	runner := NewRunner()

	arg := &maa.CustomActionArg{
		CurrentTaskName:   "测试_对战守卫",
		CustomActionParam: `{"max_retry": 3, "reset_timeout_sec": 2}`,
	}

	// 1~3 次应该放行（返回 true）
	for i := 1; i <= 3; i++ {
		success := runner.Run(nil, arg)
		if !success {
			t.Fatalf("第 %d 次重试应该成功放行，但返回了 false", i)
		}
	}

	// 第 4 次应该超限熔断（返回 false）
	success := runner.Run(nil, arg)
	if success {
		t.Fatalf("第 4 次超过上限 3 次，应该熔断返回 false，但返回了 true")
	}

	// 熔断后计数应已清零，紧接着下一次（新的一轮）应该再次成功放行
	successAfterReset := runner.Run(nil, arg)
	if !successAfterReset {
		t.Fatalf("熔断归零后第 1 次重试应该放行，但返回了 false")
	}

	// 测试闲置超时重置：等待 2.1 秒超过 reset_timeout_sec
	time.Sleep(2100 * time.Millisecond)
	// 重新计时，此时计数应已被闲置重置为 0，然后本次执行后 count 为 1，应该放行
	if !runner.Run(nil, arg) {
		t.Fatalf("闲置超时归零后执行应该放行，但返回了 false")
	}
}
