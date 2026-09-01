package clickaway

import (
	"encoding/json"
	"fmt"
	"image"
	"math"
	"math/rand"
	"sync"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// ============================================================================
// 防遮挡退避策略核心配置（可在此微调参数）
// ============================================================================

// DefaultFailThreshold 连续识别失败次数阈值。
// 当同一个节点连续识别未命中达到该次数时，判定可能存在光标遮挡，
// 触发随机位移以解除遮挡，并删除该节点状态记录。
const DefaultFailThreshold = 3

// 有 ROI 配置时：向 ROI 外缘随机方向移出的距离范围（单位：像素）
const (
	MinEvasionOffset = 25 // 最小移出距离
	MaxEvasionOffset = 60 // 最大移出距离
)

// 无 ROI 配置时：基于屏幕中心做 360° 随机漂移的步长范围（单位：像素）
const (
	MinDriftDistance = 80  // 最小漂移距离
	MaxDriftDistance = 160 // 最大漂移距离
)

// VisitHistorySize 访问历史环形队列容量。
// 记录最近 N 次节点访问名称，当前仅做记录不做分析，
// 供未来扩展钩子检测 A→B→A→B 等循环卡死模式。
const VisitHistorySize = 64

// ============================================================================
// 数据结构
// ============================================================================

// NodeState 记录单个节点的运行期动态状态。
// 当前仅用于防遮挡退避的连续失败计数，
// 后续可扩展为卡死检测、耗时统计等更多维度。
type NodeState struct {
	// ConsecutiveFailures 当前连续识别失败次数（防遮挡退避用）
	ConsecutiveFailures int
}

// SafeRecognitionParam 由编译期转译注入，携带原始识别类型与参数。
type SafeRecognitionParam struct {
	Type  string          `json:"type"`  // 原始识别类型：OCR / TemplateMatch / FeatureMatch
	Param json.RawMessage `json:"param"` // 原始识别参数（透传给底层引擎）
}

// SafeRecognitionRunner 实现 maa.CustomRecognitionRunner 接口，
// 在透传底层原生识别的基础上叠加防遮挡退避能力。
type SafeRecognitionRunner struct {
	mu         sync.Mutex
	nodeStates map[string]*NodeState

	// visitHistory 环形队列，记录最近 N 次节点访问名称。
	// 当前仅做记录，不做任何分析判断。
	visitHistory []string
	visitCursor  int
}

// NewSafeRecognitionRunner 创建防遮挡识别运行器。
func NewSafeRecognitionRunner() *SafeRecognitionRunner {
	return &SafeRecognitionRunner{
		nodeStates:   make(map[string]*NodeState),
		visitHistory: make([]string, VisitHistorySize),
	}
}

// ============================================================================
// 核心执行逻辑
// ============================================================================

// Run 实现 maa.CustomRecognitionRunner 接口。
// 透传调用底层原生识别引擎，并在连续失败达到阈值时触发鼠标退避。
func (r *SafeRecognitionRunner) Run(
	ctx *maa.Context,
	arg *maa.CustomRecognitionArg,
) (*maa.CustomRecognitionResult, bool) {
	// 1. 解析编译期注入的识别参数
	var p SafeRecognitionParam
	if err := json.Unmarshal([]byte(arg.CustomRecognitionParam), &p); err != nil {
		return nil, false
	}

	nodeKey := arg.CurrentTaskName

	// 2. 记录访问历史（环形队列，为未来扩展预留）
	r.mu.Lock()
	r.visitHistory[r.visitCursor] = nodeKey
	r.visitCursor = (r.visitCursor + 1) % VisitHistorySize
	r.mu.Unlock()

	// 3. 调用底层原生 C++ 引擎执行实际识别
	detail, err := runNativeRecognition(ctx, p, arg.Img)

	// 4. 识别成功：删除状态记录，返回原生结果
	if err == nil && detail != nil && detail.Hit {
		r.mu.Lock()
		delete(r.nodeStates, nodeKey)
		r.mu.Unlock()

		return &maa.CustomRecognitionResult{
			Box:    detail.Box,
			Detail: detail.DetailJson,
		}, true
	}

	// 5. 识别失败：累加连续失败计数
	r.mu.Lock()
	state, exists := r.nodeStates[nodeKey]
	if !exists {
		state = &NodeState{}
		r.nodeStates[nodeKey] = state
	}
	state.ConsecutiveFailures++
	failures := state.ConsecutiveFailures

	// 6. 达到阈值：触发防遮挡退避位移
	if failures >= DefaultFailThreshold {
		delete(r.nodeStates, nodeKey)
		r.mu.Unlock()

		roi := ExtractROI(p.Param)
		targetX, targetY := CalculateEvasionTarget(roi)
		moveParam := maa.TouchMoveParam{
			Target: maa.NewTargetRect(maa.Rect{targetX, targetY, 1, 1}),
		}
		_, _ = ctx.RunActionDirect(
			maa.ActionTypeTouchMove,
			moveParam,
			maa.Rect{targetX, targetY, 1, 1},
			nil,
		)
		// 延长休眠时间，等待游戏 UI 渲染管线彻底消除按钮的 Hover 遮挡态
		time.Sleep(200 * time.Millisecond)
	} else {
		r.mu.Unlock()
	}

	return nil, false
}

// ============================================================================
// 底层识别分发
// ============================================================================

// runNativeRecognition 根据识别类型分发调用底层原生 C++ 引擎。
// 支持 OCR、TemplateMatch、FeatureMatch 三种识别类型。
func runNativeRecognition(
	ctx *maa.Context,
	p SafeRecognitionParam,
	img image.Image,
) (*maa.RecognitionDetail, error) {
	switch maa.RecognitionType(p.Type) {
	case maa.RecognitionTypeOCR:
		var param maa.OCRParam
		if err := json.Unmarshal(p.Param, &param); err != nil {
			return nil, fmt.Errorf("unmarshal OCR param: %w", err)
		}
		return ctx.RunRecognitionDirect(maa.RecognitionTypeOCR, param, img)

	case maa.RecognitionTypeTemplateMatch:
		var param maa.TemplateMatchParam
		if err := json.Unmarshal(p.Param, &param); err != nil {
			return nil, fmt.Errorf("unmarshal TemplateMatch param: %w", err)
		}
		return ctx.RunRecognitionDirect(maa.RecognitionTypeTemplateMatch, param, img)

	case maa.RecognitionTypeFeatureMatch:
		var param maa.FeatureMatchParam
		if err := json.Unmarshal(p.Param, &param); err != nil {
			return nil, fmt.Errorf("unmarshal FeatureMatch param: %w", err)
		}
		return ctx.RunRecognitionDirect(maa.RecognitionTypeFeatureMatch, param, img)

	default:
		return nil, fmt.Errorf("unsupported recognition type: %s", p.Type)
	}
}

// ============================================================================
// 退避位移计算
// ============================================================================

// ExtractROI 从原始识别参数 JSON 中提取 ROI 矩形。
// 如果参数中未配置 ROI 或格式不符（如布尔值、字符串引用），返回零值 Rect。
func ExtractROI(paramJSON json.RawMessage) maa.Rect {
	var raw struct {
		ROI json.RawMessage `json:"roi"`
	}
	if err := json.Unmarshal(paramJSON, &raw); err != nil || raw.ROI == nil {
		return maa.Rect{}
	}
	var arr [4]int
	if err := json.Unmarshal(raw.ROI, &arr); err != nil {
		return maa.Rect{}
	}
	return maa.Rect(arr)
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

// CalculateEvasionTarget 计算防遮挡退避的目标坐标，并确保不超过游戏窗口边界 (1280x720)。
//   - 有 ROI：向 ROI 外缘的随机方向（上/下/左/右）移出 MinEvasionOffset~MaxEvasionOffset 像素
//   - 无 ROI：以屏幕中心 (640, 360) 为基准做 360° 随机漂移 MinDriftDistance~MaxDriftDistance 像素
func CalculateEvasionTarget(roi maa.Rect) (int, int) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var targetX, targetY int

	// 有 ROI 配置：随机选一个方向移出 ROI 外缘
	if roi.Width() > 0 && roi.Height() > 0 {
		dir := rng.Intn(4)
		offset := MinEvasionOffset + rng.Intn(MaxEvasionOffset-MinEvasionOffset+1)

		switch dir {
		case 0: // 向右移出
			targetX = roi.X() + roi.Width() + offset
			targetY = roi.Y() + rng.Intn(max(1, roi.Height()))
		case 1: // 向左移出
			targetX = roi.X() - offset
			targetY = roi.Y() + rng.Intn(max(1, roi.Height()))
		case 2: // 向下移出
			targetX = roi.X() + rng.Intn(max(1, roi.Width()))
			targetY = roi.Y() + roi.Height() + offset
		case 3: // 向上移出
			targetX = roi.X() + rng.Intn(max(1, roi.Width()))
			targetY = roi.Y() - offset
		}
	} else {
		// 无 ROI 配置：360° 随机漂移
		angle := rng.Float64() * 2 * math.Pi
		dist := float64(MinDriftDistance + rng.Intn(MaxDriftDistance-MinDriftDistance+1))
		targetX = 640 + int(dist*math.Cos(angle))
		targetY = 360 + int(dist*math.Sin(angle))
	}

	// 钳位以防止 MaaFramework 控制器因坐标超出 1280x720 抛出 Node.Action.Failed 错误
	return clamp(targetX, 10, 1270), clamp(targetY, 10, 710)
}
