package clickaway

import (
	"encoding/json"
	"math"
	"testing"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// TestCalculateEvasionTarget_WithROI 验证有 ROI 时退避坐标落在 ROI 外部。
func TestCalculateEvasionTarget_WithROI(t *testing.T) {
	roi := maa.Rect{100, 200, 150, 80} // x=100, y=200, w=150, h=80

	for i := 0; i < 200; i++ {
		x, y := CalculateEvasionTarget(roi)

		insideX := x >= roi.X() && x < roi.X()+roi.Width()
		insideY := y >= roi.Y() && y < roi.Y()+roi.Height()
		if insideX && insideY {
			t.Errorf(
				"iteration %d: evasion target (%d, %d) is inside ROI {x=%d, y=%d, w=%d, h=%d}",
				i, x, y, roi.X(), roi.Y(), roi.Width(), roi.Height(),
			)
		}
	}
}

// TestCalculateEvasionTarget_WithROI_DistanceRange 验证移出距离在配置范围内。
func TestCalculateEvasionTarget_WithROI_DistanceRange(t *testing.T) {
	roi := maa.Rect{300, 300, 100, 50}

	for i := 0; i < 200; i++ {
		x, y := CalculateEvasionTarget(roi)

		// 计算到 ROI 最近边缘的距离
		var distToEdge int
		if x >= roi.X()+roi.Width() {
			distToEdge = x - (roi.X() + roi.Width())
		} else if x < roi.X() {
			distToEdge = roi.X() - x
		} else if y >= roi.Y()+roi.Height() {
			distToEdge = y - (roi.Y() + roi.Height())
		} else if y < roi.Y() {
			distToEdge = roi.Y() - y
		}

		if distToEdge < MinEvasionOffset || distToEdge > MaxEvasionOffset {
			t.Errorf(
				"iteration %d: distance to ROI edge = %d, want [%d, %d]",
				i, distToEdge, MinEvasionOffset, MaxEvasionOffset,
			)
		}
	}
}

// TestCalculateEvasionTarget_NoROI 验证无 ROI 时漂移距离在配置范围内。
func TestCalculateEvasionTarget_NoROI(t *testing.T) {
	zeroROI := maa.Rect{} // 零值，表示无 ROI

	for i := 0; i < 200; i++ {
		x, y := CalculateEvasionTarget(zeroROI)

		// 计算到屏幕中心 (640, 360) 的距离
		dx := float64(x - 640)
		dy := float64(y - 360)
		dist := math.Sqrt(dx*dx + dy*dy)

		// 考虑到 int 截断引起的微小浮点误差，留出 1 像素余量
		if dist < float64(MinDriftDistance)-1.0 || dist > float64(MaxDriftDistance)+1.0 {
			t.Errorf(
				"iteration %d: drift distance = %.1f from center, want [%d, %d]",
				i, dist, MinDriftDistance, MaxDriftDistance,
			)
		}
	}
}

// TestExtractROI_ValidArray 验证正常 ROI 数组提取。
func TestExtractROI_ValidArray(t *testing.T) {
	raw := json.RawMessage(`{"roi": [62, 229, 229, 225], "expected": ["可领取"]}`)
	roi := ExtractROI(raw)

	if roi.X() != 62 || roi.Y() != 229 || roi.Width() != 229 || roi.Height() != 225 {
		t.Errorf("got ROI %v, want {62, 229, 229, 225}", roi)
	}
}

// TestExtractROI_NoROI 验证无 ROI 字段返回零值。
func TestExtractROI_NoROI(t *testing.T) {
	raw := json.RawMessage(`{"expected": ["可领取"]}`)
	roi := ExtractROI(raw)

	if roi != (maa.Rect{}) {
		t.Errorf("got ROI %v, want zero Rect", roi)
	}
}

// TestExtractROI_BooleanROI 验证布尔类型 ROI 优雅降级为零值。
func TestExtractROI_BooleanROI(t *testing.T) {
	raw := json.RawMessage(`{"roi": true}`)
	roi := ExtractROI(raw)

	if roi != (maa.Rect{}) {
		t.Errorf("got ROI %v, want zero Rect for boolean roi", roi)
	}
}

// TestExtractROI_InvalidJSON 验证无效 JSON 返回零值。
func TestExtractROI_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`not valid json`)
	roi := ExtractROI(raw)

	if roi != (maa.Rect{}) {
		t.Errorf("got ROI %v, want zero Rect for invalid JSON", roi)
	}
}

// TestNodeState_VisitHistory 验证环形队列记录行为。
func TestNodeState_VisitHistory(t *testing.T) {
	runner := NewSafeRecognitionRunner()

	// 模拟写入超过队列容量的访问记录
	for i := 0; i < VisitHistorySize+10; i++ {
		runner.mu.Lock()
		runner.visitHistory[runner.visitCursor] = "test_node"
		runner.visitCursor = (runner.visitCursor + 1) % VisitHistorySize
		runner.mu.Unlock()
	}

	// 验证 cursor 回绕正确
	if runner.visitCursor != 10 {
		t.Errorf("visit cursor = %d, want 10 after %d writes", runner.visitCursor, VisitHistorySize+10)
	}

	// 验证队列中所有条目都已被填充
	for i, entry := range runner.visitHistory {
		if entry != "test_node" {
			t.Errorf("visitHistory[%d] = %q, want \"test_node\"", i, entry)
		}
	}
}

// TestNodeState_FailureCountAndReset 验证失败计数累加与删除逻辑。
func TestNodeState_FailureCountAndReset(t *testing.T) {
	runner := NewSafeRecognitionRunner()
	nodeKey := "test_node"

	// 模拟连续失败累加
	for i := 1; i <= DefaultFailThreshold-1; i++ {
		runner.mu.Lock()
		state, exists := runner.nodeStates[nodeKey]
		if !exists {
			state = &NodeState{}
			runner.nodeStates[nodeKey] = state
		}
		state.ConsecutiveFailures++
		runner.mu.Unlock()
	}

	runner.mu.Lock()
	state := runner.nodeStates[nodeKey]
	if state.ConsecutiveFailures != DefaultFailThreshold-1 {
		t.Errorf("failures = %d, want %d", state.ConsecutiveFailures, DefaultFailThreshold-1)
	}
	runner.mu.Unlock()

	// 模拟识别成功，应删除状态
	runner.mu.Lock()
	delete(runner.nodeStates, nodeKey)
	runner.mu.Unlock()

	runner.mu.Lock()
	_, exists := runner.nodeStates[nodeKey]
	runner.mu.Unlock()
	if exists {
		t.Error("node state should be deleted after success")
	}
}
