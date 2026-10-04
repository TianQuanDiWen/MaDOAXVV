package clickaway

import (
	"encoding/json"
	"math"
	"math/rand"
	"sync"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// DefaultFailThreshold 连续识别失败次数阈值。
// 当同一个节点连续识别未命中达到该次数时，判定可能存在光标遮挡，
// 触发就近向 ROI 外缘或屏幕中心移出以解除遮挡，并清空该节点状态记录。
const DefaultFailThreshold = 3

// 有 ROI 配置时：向 ROI 外缘随机方向移出的距离范围（单位：像素）
const (
	MinEvasionOffset = 25
	MaxEvasionOffset = 60
)

// 无 ROI 配置时：基于屏幕中心做 360° 随机漂移的步长范围（单位：像素）
const (
	MinDriftDistance = 80
	MaxDriftDistance = 160
)

// EvasionSink 实现 maa.ContextEventSink 接口，作为旁路监听器统计连续失败并在达到阈值时执行退避位移。
type EvasionSink struct {
	mu         sync.Mutex
	nodeStates map[string]int
	roiCache   map[string]maa.Rect
	rng        *rand.Rand
}

// NewEvasionSink 创建防遮挡退避事件监听器。
func NewEvasionSink() *EvasionSink {
	return &EvasionSink{
		nodeStates: make(map[string]int),
		roiCache:   make(map[string]maa.Rect),
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// OnNodeRecognition 监听节点识别事件。
func (s *EvasionSink) OnNodeRecognition(ctx *maa.Context, status maa.EventStatus, detail maa.NodeRecognitionDetail) {
	nodeName := detail.Name
	if nodeName == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. 识别成功：清空失败计数
	if status == maa.EventStatusSucceeded {
		delete(s.nodeStates, nodeName)
		return
	}

	// 2. 识别失败：累加计数
	if status == maa.EventStatusFailed {
		s.nodeStates[nodeName]++
		if s.nodeStates[nodeName] >= DefaultFailThreshold {
			delete(s.nodeStates, nodeName)

			// 连续失败达到阈值：获取节点 ROI 并计算退避坐标（优先移出 ROI，保底中心漂移）
			roi := s.getROI(ctx, nodeName)
			targetX, targetY := s.calculateTarget(roi)

			// 直接在当前上下文下发鼠标位移动作
			if ctx != nil {
				_, _ = ctx.RunActionDirect(
					maa.ActionTypeTouchMove,
					maa.TouchMoveParam{
						Target: maa.NewTargetRect(maa.Rect{targetX, targetY, 1, 1}),
					},
					maa.Rect{targetX, targetY, 1, 1},
					nil,
				)
			}
			// 等待游戏 UI 渲染管线消除按钮的 Hover 遮挡态
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// 实现 ContextEventSink 接口的其余 5 个方法（留空）
func (s *EvasionSink) OnNodePipelineNode(*maa.Context, maa.EventStatus, maa.NodePipelineNodeDetail)       {}
func (s *EvasionSink) OnNodeRecognitionNode(*maa.Context, maa.EventStatus, maa.NodeRecognitionNodeDetail) {}
func (s *EvasionSink) OnNodeActionNode(*maa.Context, maa.EventStatus, maa.NodeActionNodeDetail)          {}
func (s *EvasionSink) OnNodeNextList(*maa.Context, maa.EventStatus, maa.NodeNextListDetail)              {}
func (s *EvasionSink) OnNodeAction(*maa.Context, maa.EventStatus, maa.NodeActionDetail)                  {}

// getROI 惰性获取并缓存节点的 ROI 定义（内存命中 < 1 微秒，未命中查 C++ 配置 < 0.2 毫秒）。
func (s *EvasionSink) getROI(ctx *maa.Context, nodeName string) maa.Rect {
	if roi, ok := s.roiCache[nodeName]; ok {
		return roi
	}
	if ctx == nil {
		return maa.Rect{}
	}

	rawJSON, err := ctx.GetNodeJSON(nodeName)
	if err != nil || rawJSON == "" {
		s.roiCache[nodeName] = maa.Rect{}
		return maa.Rect{}
	}

	roi := extractROI(rawJSON)
	s.roiCache[nodeName] = roi
	return roi
}

// extractROI 从节点 JSON 中解析 ROI 矩形。
// 支持顶层 "roi" 或 "recognition.param.roi"，格式须为 [x, y, w, h]。
func extractROI(rawJSON string) maa.Rect {
	var raw struct {
		ROI         json.RawMessage `json:"roi"`
		Recognition *struct {
			Param json.RawMessage `json:"param"`
		} `json:"recognition"`
	}
	if err := json.Unmarshal([]byte(rawJSON), &raw); err != nil {
		return maa.Rect{}
	}

	roiBytes := raw.ROI
	if len(roiBytes) == 0 && raw.Recognition != nil && len(raw.Recognition.Param) > 0 {
		var param struct {
			ROI json.RawMessage `json:"roi"`
		}
		if err := json.Unmarshal(raw.Recognition.Param, &param); err == nil {
			roiBytes = param.ROI
		}
	}

	if len(roiBytes) == 0 {
		return maa.Rect{}
	}

	var arr [4]int
	if err := json.Unmarshal(roiBytes, &arr); err != nil {
		return maa.Rect{}
	}
	return maa.Rect(arr)
}

// calculateTarget 计算防遮挡退避的目标坐标，并确保不超过游戏窗口边界 (1280x720)。
//   - 有有效 ROI：向 ROI 外缘随机方向（上/下/左/右）移出 25~60 像素
//   - 无 ROI 或全屏：以屏幕中心 (640, 360) 为基准做 360° 随机漂移 80~160 像素
func (s *EvasionSink) calculateTarget(roi maa.Rect) (int, int) {
	var targetX, targetY int

	if roi.Width() > 0 && roi.Height() > 0 {
		dir := s.rng.Intn(4)
		offset := MinEvasionOffset + s.rng.Intn(MaxEvasionOffset-MinEvasionOffset+1)

		switch dir {
		case 0: // 向右移出
			targetX = roi.X() + roi.Width() + offset
			targetY = roi.Y() + s.rng.Intn(max(1, roi.Height()))
		case 1: // 向左移出
			targetX = roi.X() - offset
			targetY = roi.Y() + s.rng.Intn(max(1, roi.Height()))
		case 2: // 向下移出
			targetX = roi.X() + s.rng.Intn(max(1, roi.Width()))
			targetY = roi.Y() + roi.Height() + offset
		case 3: // 向上移出
			targetX = roi.X() + s.rng.Intn(max(1, roi.Width()))
			targetY = roi.Y() - offset
		}
	} else {
		// 无 ROI 或全屏：以屏幕中心 (640, 360) 为基准 360° 随机漂移
		angle := s.rng.Float64() * 2 * math.Pi
		dist := float64(MinDriftDistance + s.rng.Intn(MaxDriftDistance-MinDriftDistance+1))
		targetX = 640 + int(math.Round(dist*math.Cos(angle)))
		targetY = 360 + int(math.Round(dist*math.Sin(angle)))
	}

	clampedX := clamp(targetX, 10, 1270)
	clampedY := clamp(targetY, 10, 710)

	// 如果贴边导致钳位后仍落在 ROI 内部，保底退化为屏幕中心漂移
	if roi.Width() > 0 && roi.Height() > 0 &&
		clampedX >= roi.X() && clampedX < roi.X()+roi.Width() &&
		clampedY >= roi.Y() && clampedY < roi.Y()+roi.Height() {
		return s.calculateTarget(maa.Rect{})
	}

	return clampedX, clampedY
}

func clamp(val, minVal, maxVal int) int {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}
