package clickaway

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"slices"
	"sync"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// DefaultFailThreshold 是检查鼠标遮挡前所需的连续整轮识别失败次数。
const DefaultFailThreshold = 3

// Windows 构建提供鼠标坐标读取函数，其余平台跳过退避。
var readCursorGeometry func(*maa.Context) (cursorGeometry, error)

type cursorGeometry struct {
	bounds image.Rectangle
	margin image.Point
	cursor func() (image.Point, bool)
}

// stage 保存当前任务的父节点、候选列表及对应的退避状态。
type stage struct {
	name      string
	list      []maa.NextItem
	failures  int
	attempted bool
}

// EvasionSink 按候选列表的整轮结果计数，并在鼠标可能遮挡识别时尝试退避。
type EvasionSink struct {
	mu     sync.Mutex
	stages map[uint64]*stage
}

// NewEvasionSink 创建防遮挡事件监听器。
func NewEvasionSink() *EvasionSink {
	return &EvasionSink{stages: make(map[uint64]*stage)}
}

// OnNodeNextList 处理候选列表事件。候选全部失败算一轮，任一候选命中即清零。
func (s *EvasionSink) OnNodeNextList(ctx *maa.Context, status maa.EventStatus, detail maa.NodeNextListDetail) {
	if readCursorGeometry == nil {
		return
	}
	current := s.recordRound(status, detail)
	if current == nil || ctx == nil {
		return
	}

	log := slog.With("component", "clickaway", "task_id", detail.TaskID, "stage", detail.Name,
		"failed_rounds", DefaultFailThreshold)
	geometry, err := readCursorGeometry(ctx)
	if err != nil {
		log.Debug("skipped cursor retreat", "reason", err)
		return
	}
	rois, err := roundROIs(detail.List, geometry.bounds, ctx.GetNodeJSON, ctx.GetAnchor)
	if err != nil {
		log.Debug("skipped cursor retreat", "reason", err)
		return
	}
	cursor, ok := geometry.cursor()
	if !ok {
		log.Debug("skipped cursor retreat", "reason", "cursor position cannot be mapped to recognition image")
		return
	}
	if !inAnyROI(cursor, rois) {
		log.Debug("skipped cursor retreat", "reason", "cursor outside effective ROIs", "roi_count", len(rois))
		return
	}
	target, ok := retreatTarget(cursor, geometry.bounds, rois, geometry.margin)
	if !ok {
		log.Debug("skipped cursor retreat", "reason", "no reachable point outside all ROIs")
		return
	}
	if !s.beginMove(detail.TaskID, current) {
		return
	}

	box := maa.Rect{target.X, target.Y, 1, 1}
	action, err := ctx.RunActionDirect(maa.ActionTypeTouchMove, maa.TouchMoveParam{
		Target: maa.NewTargetRect(box),
	}, box, nil)
	log = log.With("cursor", cursor, "target", target, "roi_count", len(rois))
	switch {
	case err != nil:
		log.Warn("cursor retreat failed", "error", err)
	case action == nil:
		log.Warn("cursor retreat failed", "reason", "missing action result")
	case !action.Success:
		log.Warn("cursor retreat failed", "action_id", action.ID)
	default:
		log.Info("cursor retreat completed", "action_id", action.ID)
	}
}

// recordRound 达到阈值时返回当前识别阶段供退避检查。
// 若检查后没有下发移动，后续失败可以重新累计。
func (s *EvasionSink) recordRound(status maa.EventStatus, detail maa.NodeNextListDetail) *stage {
	if detail.TaskID == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(detail.List) == 0 {
		delete(s.stages, detail.TaskID)
		return nil
	}

	current := s.stages[detail.TaskID]
	switch status {
	case maa.EventStatusSucceeded:
		delete(s.stages, detail.TaskID)
	case maa.EventStatusStarting, maa.EventStatusFailed:
		if current == nil || current.name != detail.Name || !slices.Equal(current.list, detail.List) {
			current = &stage{name: detail.Name, list: slices.Clone(detail.List)}
			s.stages[detail.TaskID] = current
		}
		if status == maa.EventStatusFailed && !current.attempted {
			current.failures++
			if current.failures == DefaultFailThreshold {
				current.failures = 0
				return current
			}
		}
	}
	return nil
}

// beginMove 在锁内确认阶段未变并登记移动尝试，实际动作在锁外执行。
func (s *EvasionSink) beginMove(taskID uint64, current *stage) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stages[taskID] != current || current.attempted {
		return false
	}
	current.attempted = true
	return true
}

// OnTaskerTask 在任务开始和结束时清理该任务的退避状态。
func (s *EvasionSink) OnTaskerTask(_ *maa.Tasker, _ maa.EventStatus, detail maa.TaskerTaskDetail) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.stages, detail.TaskID)
}

// 其余回调只用于满足 ContextEventSink 接口，单节点事件不参与计数。
func (s *EvasionSink) OnNodePipelineNode(*maa.Context, maa.EventStatus, maa.NodePipelineNodeDetail) {}
func (s *EvasionSink) OnNodeRecognitionNode(*maa.Context, maa.EventStatus, maa.NodeRecognitionNodeDetail) {
}
func (s *EvasionSink) OnNodeActionNode(*maa.Context, maa.EventStatus, maa.NodeActionNodeDetail)   {}
func (s *EvasionSink) OnNodeRecognition(*maa.Context, maa.EventStatus, maa.NodeRecognitionDetail) {}
func (s *EvasionSink) OnNodeAction(*maa.Context, maa.EventStatus, maa.NodeActionDetail)           {}

// roundROIs 汇总本轮所有候选的有效识别区域。区域无法确定时跳过本轮退避。
func roundROIs(list []maa.NextItem, bounds image.Rectangle, nodeJSON, getAnchor func(string) (string, error)) ([]image.Rectangle, error) {
	var rois []image.Rectangle
	for _, item := range list {
		name := item.Name
		if item.Anchor {
			resolved, err := getAnchor(item.Name)
			if err != nil {
				return nil, fmt.Errorf("resolve anchor %q: %w", item.Name, err)
			}
			if resolved == "" {
				continue
			}
			name = resolved
		}
		raw, err := nodeJSON(name)
		if err != nil {
			return nil, fmt.Errorf("read node %q: %w", name, err)
		}
		roi, err := nodeROI(raw, bounds)
		if err != nil {
			return nil, fmt.Errorf("resolve ROI for %q: %w", name, err)
		}
		if !roi.Empty() && !slices.Contains(rois, roi) {
			rois = append(rois, roi)
		}
	}
	return rois, nil
}

// nodeROI 解析节点的静态识别区域。全屏、未配置区域和不执行识别的节点返回空区域。
func nodeROI(raw string, bounds image.Rectangle) (image.Rectangle, error) {
	var node struct {
		Enabled     *bool           `json:"enabled"`
		Recognition json.RawMessage `json:"recognition"`
		ROI         json.RawMessage `json:"roi"`
		Offset      json.RawMessage `json:"roi_offset"`
	}
	if err := json.Unmarshal([]byte(raw), &node); err != nil {
		return image.Rectangle{}, err
	}
	if node.Enabled != nil && !*node.Enabled {
		return image.Rectangle{}, nil
	}

	kind := "DirectHit"
	roi, offset := node.ROI, node.Offset
	if len(node.Recognition) > 0 {
		if node.Recognition[0] == '"' {
			if err := json.Unmarshal(node.Recognition, &kind); err != nil {
				return image.Rectangle{}, err
			}
		} else {
			var rec struct {
				Type  string `json:"type"`
				Param struct {
					ROI    json.RawMessage `json:"roi"`
					Offset json.RawMessage `json:"roi_offset"`
				} `json:"param"`
			}
			if err := json.Unmarshal(node.Recognition, &rec); err != nil {
				return image.Rectangle{}, err
			}
			kind = rec.Type
			if len(rec.Param.ROI) != 0 {
				roi = rec.Param.ROI
			}
			if len(rec.Param.Offset) != 0 {
				offset = rec.Param.Offset
			}
		}
	}
	switch kind {
	case "", "DirectHit":
		return image.Rectangle{}, nil
	case "TemplateMatch", "FeatureMatch", "ColorMatch", "OCR", "NeuralNetworkClassify", "NeuralNetworkDetect":
	default:
		return image.Rectangle{}, errors.New("recognition has no verifiable static ROI")
	}

	values, err := readRect(roi)
	if err != nil {
		return image.Rectangle{}, err
	}
	delta, err := readRect(offset)
	if err != nil {
		return image.Rectangle{}, err
	}
	// 按识别图尺寸解析 ROI，再将 roi_offset 逐项加到位置和尺寸上。
	x, w := resolveAxis(values[0], values[2], bounds.Dx())
	y, h := resolveAxis(values[1], values[3], bounds.Dy())
	x, y, w, h = x+delta[0], y+delta[1], w+delta[2], h+delta[3]
	if w <= 0 || h <= 0 {
		return image.Rectangle{}, errors.New("ROI has nonpositive dimensions after applying offset")
	}
	result := image.Rect(x, y, x+w, y+h).Intersect(bounds)
	if result == bounds || result.Empty() {
		return image.Rectangle{}, nil
	}
	return result, nil
}

func readRect(raw json.RawMessage) ([4]int, error) {
	var values []int32
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return [4]int{}, nil
	}
	if err := json.Unmarshal(raw, &values); err != nil || len(values) != 4 {
		return [4]int{}, errors.New("ROI is not a static rectangle")
	}
	return [4]int{int(values[0]), int(values[1]), int(values[2]), int(values[3])}, nil
}

func resolveAxis(position, size, limit int) (int, int) {
	if position < 0 {
		position += limit
	}
	switch {
	case size == 0:
		size = limit - position
	case size < 0:
		position += size
		size = -size
	}
	return position, size
}

func inAnyROI(p image.Point, rois []image.Rectangle) bool {
	return slices.ContainsFunc(rois, p.In)
}

// retreatTarget 用各 ROI 的边界划分识别图，从可用区域中选离鼠标最近的像素。
func retreatTarget(cursor image.Point, bounds image.Rectangle, rois []image.Rectangle, margin image.Point) (image.Point, bool) {
	xs := []int{cursor.X}
	ys := []int{cursor.Y}
	excluded := make([]image.Rectangle, 0, len(rois))
	for _, r := range rois {
		r.Min = r.Min.Sub(margin)
		r.Max = r.Max.Add(margin)
		excluded = append(excluded, r)
		xs = append(xs, r.Min.X-1, r.Max.X)
		ys = append(ys, r.Min.Y-1, r.Max.Y)
	}

	var best image.Point
	bestDistance := int64(-1)
	for _, x := range xs {
		for _, y := range ys {
			p := image.Pt(x, y)
			if !p.In(bounds) || inAnyROI(p, excluded) {
				continue
			}
			dx, dy := int64(x-cursor.X), int64(y-cursor.Y)
			if distance := dx*dx + dy*dy; bestDistance < 0 || distance < bestDistance {
				best, bestDistance = p, distance
			}
		}
	}
	return best, bestDistance >= 0
}
