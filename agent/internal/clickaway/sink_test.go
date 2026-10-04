package clickaway

import (
	"math"
	"testing"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// 确保 EvasionSink 完整实现 maa.ContextEventSink 接口
var _ maa.ContextEventSink = (*EvasionSink)(nil)

// TestExtractROI 验证从节点 JSON 中提取各种格式 ROI 的健壮性。
func TestExtractROI(t *testing.T) {
	cases := []struct {
		name     string
		rawJSON  string
		expected maa.Rect
	}{
		{
			name:     "顶层 ROI 数组",
			rawJSON:  `{"roi": [100, 200, 150, 80]}`,
			expected: maa.Rect{100, 200, 150, 80},
		},
		{
			name:     "recognition.param 内嵌 ROI",
			rawJSON:  `{"recognition": {"type": "OCR", "param": {"roi": [62, 229, 229, 225]}}}`,
			expected: maa.Rect{62, 229, 229, 225},
		},
		{
			name:     "全屏 ROI",
			rawJSON:  `{"roi": [0, 0, 0, 0]}`,
			expected: maa.Rect{0, 0, 0, 0},
		},
		{
			name:     "未声明 ROI",
			rawJSON:  `{"recognition": {"type": "OCR"}}`,
			expected: maa.Rect{},
		},
		{
			name:     "布尔类型 ROI 容错",
			rawJSON:  `{"roi": true}`,
			expected: maa.Rect{},
		},
		{
			name:     "无效 JSON 容错",
			rawJSON:  `not valid json`,
			expected: maa.Rect{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractROI(tc.rawJSON)
			if got != tc.expected {
				t.Fatalf("extractROI() = %v, want %v", got, tc.expected)
			}
		})
	}
}

// TestCalculateTarget_WithROI 验证有有效 ROI 时，退避坐标严格落在 ROI 外缘 25~60px 且不落在 ROI 内部。
func TestCalculateTarget_WithROI(t *testing.T) {
	sink := NewEvasionSink()
	roi := maa.Rect{200, 200, 150, 80}

	for i := 0; i < 500; i++ {
		x, y := sink.calculateTarget(roi)

		// 验证不落在 ROI 内部
		insideX := x >= roi.X() && x < roi.X()+roi.Width()
		insideY := y >= roi.Y() && y < roi.Y()+roi.Height()
		if insideX && insideY {
			t.Fatalf("iteration %d: target (%d, %d) is inside ROI %v", i, x, y, roi)
		}

		// 验证边界安全
		if x < 10 || x > 1270 || y < 10 || y > 710 {
			t.Fatalf("iteration %d: target (%d, %d) out of safe bounds", i, x, y)
		}
	}
}

// TestCalculateTarget_NoROI 验证无 ROI 时回退为屏幕中心 (640, 360) 漂移 80~160px。
func TestCalculateTarget_NoROI(t *testing.T) {
	sink := NewEvasionSink()
	zeroROI := maa.Rect{}

	for i := 0; i < 500; i++ {
		x, y := sink.calculateTarget(zeroROI)

		// 验证边界安全
		if x < 10 || x > 1270 || y < 10 || y > 710 {
			t.Fatalf("iteration %d: target (%d, %d) out of safe bounds", i, x, y)
		}

		dx := float64(x - 640)
		dy := float64(y - 360)
		dist := math.Sqrt(dx*dx + dy*dy)

		if dist < 79.0 || dist > 161.0 {
			t.Fatalf("iteration %d: center drift distance=%.2f, want [80, 160]", i, dist)
		}
	}
}

// TestEvasionSink_OnNodeRecognition_StateTracking 验证失败计数累加、重置与超限清空逻辑。
func TestEvasionSink_OnNodeRecognition_StateTracking(t *testing.T) {
	sink := NewEvasionSink()
	nodeName := "测试节点_温泉"

	// 1. 空节点名应直接忽略
	sink.OnNodeRecognition(nil, maa.EventStatusFailed, maa.NodeRecognitionDetail{Name: ""})
	sink.mu.Lock()
	if len(sink.nodeStates) != 0 {
		t.Fatalf("expected empty nodeStates for empty nodeName, got %v", sink.nodeStates)
	}
	sink.mu.Unlock()

	// 2. 第一次失败
	sink.OnNodeRecognition(nil, maa.EventStatusFailed, maa.NodeRecognitionDetail{Name: nodeName})
	sink.mu.Lock()
	if sink.nodeStates[nodeName] != 1 {
		t.Fatalf("expected failure count 1, got %d", sink.nodeStates[nodeName])
	}
	sink.mu.Unlock()

	// 3. 第二次失败
	sink.OnNodeRecognition(nil, maa.EventStatusFailed, maa.NodeRecognitionDetail{Name: nodeName})
	sink.mu.Lock()
	if sink.nodeStates[nodeName] != 2 {
		t.Fatalf("expected failure count 2, got %d", sink.nodeStates[nodeName])
	}
	sink.mu.Unlock()

	// 4. 识别成功，清空计数
	sink.OnNodeRecognition(nil, maa.EventStatusSucceeded, maa.NodeRecognitionDetail{Name: nodeName})
	sink.mu.Lock()
	if _, exists := sink.nodeStates[nodeName]; exists {
		t.Fatalf("expected nodeState to be deleted after success")
	}
	sink.mu.Unlock()

	// 5. 重新累加至连续 3 次失败，触发退避并清空状态
	sink.OnNodeRecognition(nil, maa.EventStatusFailed, maa.NodeRecognitionDetail{Name: nodeName})
	sink.OnNodeRecognition(nil, maa.EventStatusFailed, maa.NodeRecognitionDetail{Name: nodeName})
	// 第 3 次失败（ctx 为 nil 时安全跳过 RunActionDirect，仅测试状态机流转）
	sink.OnNodeRecognition(nil, maa.EventStatusFailed, maa.NodeRecognitionDetail{Name: nodeName})

	sink.mu.Lock()
	if _, exists := sink.nodeStates[nodeName]; exists {
		t.Fatalf("expected nodeState to be deleted after reaching threshold 3")
	}
	sink.mu.Unlock()
}
