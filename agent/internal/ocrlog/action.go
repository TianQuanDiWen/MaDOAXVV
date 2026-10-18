package ocrlog

import (
	"encoding/json"
	"fmt"
	"image"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// Param 对应流水线节点中 custom_action_param 的 JSON 配置。
type Param struct {
	TargetNode string            `json:"target_node,omitempty"`
	Regions    []json.RawMessage `json:"regions"`
}

var placeholderRe = regexp.MustCompile(`\{(\d+)\}`)

// Runner 实现 maa.CustomActionRunner 接口，提供多区域 OCR 识别并格式化写入原生 focus 的能力。
type Runner struct {
	mu              sync.Mutex
	nodeTemplates   map[int64]map[string]string // taskID -> nodeName -> template
	targetTemplates map[int64]map[string]string // taskID -> targetNodeName -> pristine template
	// ocrFunc 为 OCR 结果识别方法，未设置时默认调用 defaultRecognizeOCR 复用框架能力
	ocrFunc func(ctx *maa.Context, ocrParam *maa.OCRParam, img image.Image) ([]*maa.OCRResult, error)
}

// NewRunner 创建一个 OCRLog 自定义动作运行器。
func NewRunner() *Runner {
	return &Runner{
		nodeTemplates:   make(map[int64]map[string]string),
		targetTemplates: make(map[int64]map[string]string),
	}
}

// Run 执行 OCR 识别、阅读顺序重组与 focus 模板填充。
func (r *Runner) Run(ctx *maa.Context, arg *maa.CustomActionArg) bool {
	if arg == nil {
		return true
	}

	p, err := parseParam(arg.CustomActionParam)
	if err != nil {
		fmt.Printf("[OCRLog] 节点 %q custom_action_param 参数解析失败: %v (raw: %q)\n", arg.CurrentTaskName, err, arg.CustomActionParam)
	}

	var currNode *maa.Node
	if ctx != nil && arg.CurrentTaskName != "" {
		currNode, _ = ctx.GetNode(arg.CurrentTaskName)
	}

	// 目标输出节点：优先采用配置的 target_node；若未配置则默认取当前节点的首个 next 节点；最后回退到当前节点
	targetNodeName := p.TargetNode
	if targetNodeName == "" {
		if currNode != nil && len(currNode.Next) > 0 && currNode.Next[0].Name != "" {
			targetNodeName = currNode.Next[0].Name
		} else {
			targetNodeName = arg.CurrentTaskName
		}
	}

	var targetNode *maa.Node
	if ctx != nil && targetNodeName != "" && targetNodeName != arg.CurrentTaskName {
		targetNode, _ = ctx.GetNode(targetNodeName)
	}

	template := r.resolveTemplate(arg.TaskID, arg.CurrentTaskName, targetNodeName, currNode, targetNode, len(p.Regions))

	// 一次截图供所有区域识别，复用框架底层缓存或控制器截图
	img, _ := r.getImage(ctx)

	results := make([]string, len(p.Regions))
	for i, reg := range p.Regions {
		results[i] = r.recognizeRegion(ctx, arg.CurrentTaskName, i, reg, img)
	}

	formattedFocus := formatFocus(template, results)

	fmt.Printf("[OCRLog] 节点 %q 识别完成 -> 结果: %s (目标输出节点: %q)\n", arg.CurrentTaskName, formattedFocus, targetNodeName)

	var currOrigFocus, targetOrigFocus any
	if currNode != nil {
		currOrigFocus = currNode.Focus
	}
	if targetNode != nil {
		targetOrigFocus = targetNode.Focus
	}

	// 同时覆写当前节点与后续输出节点的 focus，保留节点原有的结构（兼容 string 或 MPE map）
	override := map[string]any{
		arg.CurrentTaskName: map[string]any{
			"focus": buildOverrideFocus(currOrigFocus, formattedFocus),
		},
	}
	if targetNodeName != "" && targetNodeName != arg.CurrentTaskName {
		override[targetNodeName] = map[string]any{
			"focus": buildOverrideFocus(targetOrigFocus, formattedFocus),
		}
	}

	if ctx != nil {
		if err := ctx.OverridePipeline(override); err != nil {
			fmt.Printf("[OCRLog] 节点 %q 覆写流水线 focus 失败: %v\n", arg.CurrentTaskName, err)
		}
	}

	return true
}

func parseParam(paramStr string) (Param, error) {
	var p Param
	trimmed := strings.TrimSpace(paramStr)
	if trimmed == "" {
		return p, nil
	}

	// 若传入的参数是经过二次转义的 JSON 字符串（首尾带双引号），先解包出原始 JSON
	if strings.HasPrefix(trimmed, "\"") && strings.HasSuffix(trimmed, "\"") {
		var unquoted string
		if err := json.Unmarshal([]byte(trimmed), &unquoted); err == nil {
			trimmed = strings.TrimSpace(unquoted)
		} else {
			return p, fmt.Errorf("unquote custom_action_param string: %w", err)
		}
	}

	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal([]byte(trimmed), &p.Regions); err != nil {
			return p, fmt.Errorf("unmarshal regions array: %w", err)
		}
	} else if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal([]byte(trimmed), &p); err != nil {
			return p, fmt.Errorf("unmarshal param object: %w", err)
		}
	} else {
		return p, fmt.Errorf("invalid json format: expected '{' or '['")
	}
	return p, nil
}

func (r *Runner) getImage(ctx *maa.Context) (image.Image, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	tasker := ctx.GetTasker()
	if tasker == nil {
		return nil, fmt.Errorf("tasker is nil")
	}
	ctrl := tasker.GetController()
	if ctrl == nil {
		return nil, fmt.Errorf("controller is nil")
	}

	if img, err := ctrl.CacheImage(); err == nil && img != nil {
		return img, nil
	}

	job := ctrl.PostScreencap()
	if job != nil && job.Wait().Success() {
		return ctrl.CacheImage()
	}
	return nil, fmt.Errorf("screencap failed")
}

func (r *Runner) recognizeRegion(ctx *maa.Context, nodeName string, regionIdx int, regionRaw json.RawMessage, img image.Image) string {
	var ocrParam maa.OCRParam
	if err := json.Unmarshal(regionRaw, &ocrParam); err != nil {
		fmt.Printf("[OCRLog] 节点 %q 区域 #%d 参数反序列化失败: %v (raw: %s)\n", nodeName, regionIdx, err, string(regionRaw))
		return "未识别"
	}

	fn := r.ocrFunc
	if fn == nil {
		fn = defaultRecognizeOCR
	}

	items, err := fn(ctx, &ocrParam, img)
	if err != nil || len(items) == 0 {
		return "未识别"
	}

	sorted := sortReadingOrder(items)
	var parts []string
	for _, it := range sorted {
		parts = append(parts, strings.TrimSpace(it.Text))
	}
	res := strings.Join(parts, " ")
	if res == "" {
		return "未识别"
	}
	return res
}

func defaultRecognizeOCR(ctx *maa.Context, ocrParam *maa.OCRParam, img image.Image) ([]*maa.OCRResult, error) {
	if ctx == nil || img == nil {
		return nil, nil
	}
	recoDetail, err := ctx.RunRecognitionDirect(maa.RecognitionTypeOCR, ocrParam, img)
	if err != nil || recoDetail == nil || recoDetail.Results == nil || !recoDetail.Hit || len(recoDetail.Results.Filtered) == 0 {
		return nil, err
	}

	var items []*maa.OCRResult
	for _, c := range recoDetail.Results.Filtered {
		if c != nil {
			if ocr, ok := c.AsOCR(); ok && ocr != nil && strings.TrimSpace(ocr.Text) != "" {
				items = append(items, ocr)
			}
		}
	}
	return items, nil
}

// formatFocus 一次性将识别结果填入模板的 {0}、{1}... 占位符。
// 单遍扫描替换，绝不对已填入的识别文本进行二次替换，支持重排与重复引用。
func formatFocus(template string, results []string) string {
	return placeholderRe.ReplaceAllStringFunc(template, func(match string) string {
		idx, err := strconv.Atoi(match[1 : len(match)-1])
		if err != nil || idx < 0 || idx >= len(results) {
			return "未识别"
		}
		return results[idx]
	})
}

// extractFocusTemplate 从节点的 focus 属性中提取文本模板。
// 兼容原生 string 标量以及 MPE 生成的事件监听字典（如 {"Node.Recognition.Succeeded": "..."}）。
func extractFocusTemplate(focus any) string {
	if focus == nil {
		return ""
	}
	switch v := focus.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]any:
		// 优先查找含有占位符的配置
		for _, val := range v {
			if s, ok := val.(string); ok && placeholderRe.MatchString(s) {
				return strings.TrimSpace(s)
			}
		}
		// 按常用成功事件查找
		for _, key := range []string{"Node.Action.Succeeded", "Node.Recognition.Succeeded", "Node.PipelineNode.Succeeded"} {
			if val, ok := v[key]; ok {
				if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
		// 取任意非空字符串
		for _, val := range v {
			if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	case map[string]string:
		for _, s := range v {
			if placeholderRe.MatchString(s) {
				return strings.TrimSpace(s)
			}
		}
		for _, key := range []string{"Node.Action.Succeeded", "Node.Recognition.Succeeded", "Node.PipelineNode.Succeeded"} {
			if s, ok := v[key]; ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
		for _, s := range v {
			if strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

// buildOverrideFocus 构建覆写流水线时的 focus 结构。
// 若节点原本配置为 MPE 事件字典，则保留字典结构并将每个事件对应的值更新为格式化后的结果；若原本为字符串则返回字符串。
func buildOverrideFocus(origFocus any, formatted string) any {
	if origMap, ok := origFocus.(map[string]any); ok && len(origMap) > 0 {
		newMap := make(map[string]any, len(origMap))
		for k := range origMap {
			newMap[k] = formatted
		}
		return newMap
	}
	if origStrMap, ok := origFocus.(map[string]string); ok && len(origStrMap) > 0 {
		newMap := make(map[string]any, len(origStrMap))
		for k := range origStrMap {
			newMap[k] = formatted
		}
		return newMap
	}
	return formatted
}

// isSameLine 判断待聚类文本框 b 与基准行框 base 是否属于同一行。
// 要求纵向重叠高度占较矮文本框高度的 50% 以上，严格防止紧邻多行文字因容差累加而被错误合并。
func isSameLine(b, base maa.Rect) bool {
	top := max(b.Y(), base.Y())
	bottom := min(b.Y()+b.Height(), base.Y()+base.Height())
	overlap := bottom - top
	if overlap <= 0 {
		return false
	}
	minH := min(b.Height(), base.Height())
	if minH <= 0 {
		return false
	}
	return float64(overlap) >= 0.5*float64(minH)
}

// sortReadingOrder 按从上到下、从左到右的阅读顺序对 OCR 结果进行稳定排序。
// 利用基准锚点框的纵向重叠比例聚类为行，各行内按水平 X 升序排序。
func sortReadingOrder(items []*maa.OCRResult) []*maa.OCRResult {
	if len(items) <= 1 {
		return items
	}

	sorted := make([]*maa.OCRResult, len(items))
	copy(sorted, items)
	// 首先按 Y 升序排（从上到下），Y 相同则按 X 升序排
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Box.Y() != sorted[j].Box.Y() {
			return sorted[i].Box.Y() < sorted[j].Box.Y()
		}
		return sorted[i].Box.X() < sorted[j].Box.X()
	})

	type line struct {
		base  maa.Rect
		items []*maa.OCRResult
	}
	var lines []*line

	for _, item := range sorted {
		b := item.Box
		matchedIdx := -1

		for idx, l := range lines {
			if isSameLine(b, l.base) {
				matchedIdx = idx
				break
			}
		}

		if matchedIdx >= 0 {
			lines[matchedIdx].items = append(lines[matchedIdx].items, item)
		} else {
			lines = append(lines, &line{
				base:  b,
				items: []*maa.OCRResult{item},
			})
		}
	}

	var result []*maa.OCRResult
	for _, l := range lines {
		// 行内按水平 X 坐标从左到右排序
		sort.SliceStable(l.items, func(i, j int) bool {
			return l.items[i].Box.X() < l.items[j].Box.X()
		})
		result = append(result, l.items...)
	}
	return result
}

// resolveTemplate 负责解析并缓存当前 OCR 节点在当前任务下的原始模板。
// 规则：
// 1. 在第一次覆写目标节点前记录目标节点的初始原始模板，不依赖当前节点是否自带模板。
// 2. 同任务内重复执行时，优先命中当前 OCR 节点已缓存的原始模板。
// 3. 首次执行：优先读取当前节点自身的静态 focus 配置（兼容 string 或 MPE 字典）。
// 4. 若当前节点未配置 focus，从目标输出节点继承已固化的原始模板缓存。
// 5. 若节点均未配置 focus，自动生成默认模板（如 "{0} {1}"）。
// 6. 各 OCR 节点独立维护自身模板缓存，互不污染。
func (r *Runner) resolveTemplate(taskID int64, currTask, targetTask string, currNode, targetNode *maa.Node, regionCount int) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 无论当前节点是否自带 focus，只要存在目标输出节点，第一时间固化并记录目标节点的初始原始模板
	if targetTask != "" && targetTask != currTask {
		if _, recorded := r.getTargetTemplateLocked(taskID, targetTask); !recorded {
			var origTpl string
			if targetNode != nil {
				origTpl = extractFocusTemplate(targetNode.Focus)
			}
			r.saveTargetTemplateLocked(taskID, targetTask, origTpl)
		}
	}

	// 1. 同一任务内重复执行时，优先使用当前 OCR 节点已确定的模板缓存
	if tpl := r.getNodeTemplateLocked(taskID, currTask); tpl != "" {
		return tpl
	}

	// 2. 首次执行：优先读取当前节点自身的静态 focus 配置
	var template string
	if currNode != nil {
		template = extractFocusTemplate(currNode.Focus)
	}

	// 3. 若当前节点未配置 focus，从目标输出节点继承已固化的原始模板
	if template == "" && targetTask != "" {
		if tpl, recorded := r.getTargetTemplateLocked(taskID, targetTask); recorded && tpl != "" {
			template = tpl
		}
	}

	// 4. 若节点均未配置 focus，自动生成默认模板（如 "{0} {1}"）
	if template == "" {
		template = defaultTemplate(regionCount)
	}

	// 5. 缓存当前 OCR 节点的原始模板
	r.saveNodeTemplateLocked(taskID, currTask, template)
	return template
}

func (r *Runner) saveNodeTemplateLocked(taskID int64, node string, tpl string) {
	r.ensureTaskLocked(taskID)
	r.nodeTemplates[taskID][node] = tpl
}

func (r *Runner) getNodeTemplateLocked(taskID int64, node string) string {
	if r.nodeTemplates != nil {
		if m, ok := r.nodeTemplates[taskID]; ok {
			return m[node]
		}
	}
	return ""
}

func (r *Runner) saveTargetTemplateLocked(taskID int64, targetNode string, tpl string) {
	r.ensureTaskLocked(taskID)
	r.targetTemplates[taskID][targetNode] = tpl
}

func (r *Runner) getTargetTemplateLocked(taskID int64, targetNode string) (string, bool) {
	if r.targetTemplates != nil {
		if m, ok := r.targetTemplates[taskID]; ok {
			tpl, exists := m[targetNode]
			return tpl, exists
		}
	}
	return "", false
}

func (r *Runner) ensureTaskLocked(taskID int64) {
	if r.nodeTemplates == nil {
		r.nodeTemplates = make(map[int64]map[string]string)
	}
	if r.targetTemplates == nil {
		r.targetTemplates = make(map[int64]map[string]string)
	}
	if len(r.nodeTemplates) >= 32 {
		for id := range r.nodeTemplates {
			if id != taskID {
				delete(r.nodeTemplates, id)
				delete(r.targetTemplates, id)
			}
		}
	}
	if _, ok := r.nodeTemplates[taskID]; !ok {
		r.nodeTemplates[taskID] = make(map[string]string)
	}
	if _, ok := r.targetTemplates[taskID]; !ok {
		r.targetTemplates[taskID] = make(map[string]string)
	}
}

func defaultTemplate(count int) string {
	if count <= 0 {
		return ""
	}
	items := make([]string, count)
	for i := 0; i < count; i++ {
		items[i] = fmt.Sprintf("{%d}", i)
	}
	return strings.Join(items, " ")
}
