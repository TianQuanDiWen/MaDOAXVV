package ocrlog

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"testing"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

func TestFormatFocus_SinglePassNonRecursive(t *testing.T) {
	template := "重复0: {0}, 重排1: {1}, 再引0: {0}, 超出: {5}"
	results := []string{
		"Result{1}_With_$Dollar", // 含有 {1} 和 $，严禁二次替换
		"Result1_Safe",
	}

	formatted := formatFocus(template, results)
	expected := "重复0: Result{1}_With_$Dollar, 重排1: Result1_Safe, 再引0: Result{1}_With_$Dollar, 超出: 未识别"

	if formatted != expected {
		t.Fatalf("formatFocus mismatch.\nGot:  %q\nWant: %q", formatted, expected)
	}
}

func TestSortReadingOrder(t *testing.T) {
	items := []*maa.OCRResult{
		{Box: maa.Rect{200, 12, 100, 20}, Text: "World"},
		{Box: maa.Rect{50, 10, 80, 20}, Text: "Hello"},
		{Box: maa.Rect{300, 62, 90, 20}, Text: "Maa"},
		{Box: maa.Rect{80, 58, 70, 20}, Text: "Go"},
	}

	sorted := sortReadingOrder(items)
	if len(sorted) != 4 {
		t.Fatalf("expected 4 items, got %d", len(sorted))
	}

	expectedOrder := []string{"Hello", "World", "Go", "Maa"}
	for i, want := range expectedOrder {
		if sorted[i].Text != want {
			t.Errorf("item %d mismatch: got %q, want %q", i, sorted[i].Text, want)
		}
	}
}

func TestSortReadingOrder_AdjacentLines(t *testing.T) {
	// 用户复现用例：
	// 三个文本框高度均为 20，Y 分别为 0、20、40，X 分别为 200、100、0
	// 修复后按从上到下的阅读顺序返回“第一行、第二行、第三行”，避免因行高累加导致逆序混排
	items := []*maa.OCRResult{
		{Box: maa.Rect{200, 0, 80, 20}, Text: "Line1"},
		{Box: maa.Rect{100, 20, 80, 20}, Text: "Line2"},
		{Box: maa.Rect{0, 40, 80, 20}, Text: "Line3"},
	}

	sorted := sortReadingOrder(items)
	if len(sorted) != 3 {
		t.Fatalf("expected 3 items, got %d", len(sorted))
	}

	expectedOrder := []string{"Line1", "Line2", "Line3"}
	for i, want := range expectedOrder {
		if sorted[i].Text != want {
			t.Errorf("item %d mismatch: got %q, want %q (reading order inverted!)", i, sorted[i].Text, want)
		}
	}
}

func TestUnrecognizedFallback(t *testing.T) {
	runner := NewRunner()

	// 模拟 OCR 接口返回错误或空列表
	runner.ocrFunc = func(ctx *maa.Context, ocrParam *maa.OCRParam, img image.Image) ([]*maa.OCRResult, error) {
		return nil, fmt.Errorf("simulated ocr failure")
	}

	res := runner.recognizeRegion(nil, "TestNode", 0, json.RawMessage(`{"roi": [0,0,10,10]}`), nil)
	if res != "未识别" {
		t.Fatalf("expected '未识别', got %q", res)
	}
}

func TestFilter_NoBypass(t *testing.T) {
	runner := NewRunner()

	// 1. threshold 未达标或未命中时，OCR 识别层返回 nil/empty，严格返回“未识别”
	runner.ocrFunc = func(ctx *maa.Context, ocrParam *maa.OCRParam, img image.Image) ([]*maa.OCRResult, error) {
		return nil, nil
	}

	res := runner.recognizeRegion(nil, "TestNode", 0, json.RawMessage(`{"roi": [0,0,10,10], "threshold": 0.9}`), nil)
	if res != "未识别" {
		t.Fatalf("expected '未识别' when filter not passed, got %q", res)
	}

	// 2. 命中过滤条件时，正常返回识别到的文本
	runner.ocrFunc = func(ctx *maa.Context, ocrParam *maa.OCRParam, img image.Image) ([]*maa.OCRResult, error) {
		return []*maa.OCRResult{
			{Box: maa.Rect{0, 0, 10, 10}, Text: "MatchedText", Score: 0.95},
		}, nil
	}

	resMatched := runner.recognizeRegion(nil, "TestNode", 0, json.RawMessage(`{"roi": [0,0,10,10], "threshold": 0.9}`), nil)
	if resMatched != "MatchedText" {
		t.Fatalf("expected 'MatchedText', got %q", resMatched)
	}
}

func TestSharedTargetNode_Mixed_OneExplicitOneInherited(t *testing.T) {
	// 用户复现用例（一方自带模板、另一方继承共享模板）：
	// 共享输出节点配置 Value: {0}。
	// OCR_A 自带 Score: {0}，识别后将输出节点覆写为 Score: 100；
	// OCR_B 没有自身模板，仍须成功继承 Value: {0}，输出 Value: 200，绝不能丢失 Value: 前缀！
	runner := NewRunner()
	taskID := int64(4001)
	sharedLogNodeName := "LogNode"

	sharedLogNode := maa.NewNode(sharedLogNodeName).SetFocus("Value: {0}")

	// 1. OCR_A 执行：自带 Score: {0}
	ocrA := maa.NewNode("OCR_A").SetFocus("Score: {0}")
	tplA := runner.resolveTemplate(taskID, "OCR_A", sharedLogNodeName, ocrA, sharedLogNode, 1)
	if tplA != "Score: {0}" {
		t.Fatalf("OCR_A template mismatch: got %q, want 'Score: {0}'", tplA)
	}

	// 模拟 OCR_A 识别出 100，覆写 sharedLogNode 的 focus 为 "Score: 100"
	outputA := formatFocus(tplA, []string{"100"})
	if outputA != "Score: 100" {
		t.Fatalf("outputA mismatch: %q", outputA)
	}
	sharedLogNode.SetFocus(outputA)

	// 2. OCR_B 执行：没有自身模板，目标节点虽然已被 OCR_A 覆写为 "Score: 100"，但必须能继承原始 "Value: {0}"
	ocrB := maa.NewNode("OCR_B") // 无 focus
	tplB := runner.resolveTemplate(taskID, "OCR_B", sharedLogNodeName, ocrB, sharedLogNode, 1)
	if tplB != "Value: {0}" {
		t.Fatalf("OCR_B template mismatch: got %q, want 'Value: {0}' (lost prefix or contaminated by OCR_A)", tplB)
	}

	// OCR_B 识别出 200，格式化输出必须为 "Value: 200"
	outputB := formatFocus(tplB, []string{"200"})
	if outputB != "Value: 200" {
		t.Fatalf("outputB mismatch: got %q, want 'Value: 200'", outputB)
	}
	sharedLogNode.SetFocus(outputB)

	// 3. 自循环/重试验证：OCR_A 与 OCR_B 再次执行依然使用各自正确的原始模板
	tplARetry := runner.resolveTemplate(taskID, "OCR_A", sharedLogNodeName, ocrA, sharedLogNode, 1)
	if tplARetry != "Score: {0}" {
		t.Fatalf("OCR_A retry expected 'Score: {0}', got %q", tplARetry)
	}

	tplBRetry := runner.resolveTemplate(taskID, "OCR_B", sharedLogNodeName, ocrB, sharedLogNode, 1)
	if tplBRetry != "Value: {0}" {
		t.Fatalf("OCR_B retry expected 'Value: {0}', got %q", tplBRetry)
	}
}

func TestSharedTargetNode_BothInherit_NoCrosstalk(t *testing.T) {
	// 两个 OCR 节点均未配置自身的 focus，共用一个带模板的输出节点 LogNode (Value: {0})。
	// OCR_A 识别后覆写 LogNode 为 Value: 100；
	// 后续 OCR_B 执行时必须读取到 LogNode 覆写前的原始模板 Value: {0}，绝不能缓存 Value: 100！
	runner := NewRunner()
	taskID := int64(4002)
	sharedLogNodeName := "LogNode"

	sharedLogNode := maa.NewNode(sharedLogNodeName).SetFocus("Value: {0}")

	// OCR_A 节点（无自身 focus）
	ocrA := maa.NewNode("OCR_A")
	tplA := runner.resolveTemplate(taskID, "OCR_A", sharedLogNodeName, ocrA, sharedLogNode, 1)
	if tplA != "Value: {0}" {
		t.Fatalf("OCR_A expected 'Value: {0}', got %q", tplA)
	}

	outputA := formatFocus(tplA, []string{"100"})
	sharedLogNode.SetFocus(outputA)

	// OCR_B 节点（同样无自身 focus）
	ocrB := maa.NewNode("OCR_B")
	tplB := runner.resolveTemplate(taskID, "OCR_B", sharedLogNodeName, ocrB, sharedLogNode, 1)
	if tplB != "Value: {0}" {
		t.Fatalf("OCR_B expected pristine 'Value: {0}' from shared target, but got contaminated %q", tplB)
	}

	outputB := formatFocus(tplB, []string{"200"})
	if outputB != "Value: 200" {
		t.Fatalf("outputB mismatch: got %q, want 'Value: 200'", outputB)
	}
}

func TestTemplateCache_DefaultTemplatePreserved(t *testing.T) {
	runner := NewRunner()
	taskID := int64(2001)
	nodeName := "TestNode"

	// 第 1 次执行：节点未配置 focus，模板应由 regionCount=2 生成默认模板 "{0} {1}"
	currNode1 := maa.NewNode(nodeName)
	tpl1 := runner.resolveTemplate(taskID, nodeName, "", currNode1, nil, 2)
	if tpl1 != "{0} {1}" {
		t.Fatalf("run 1 expected default template '{0} {1}', got %q", tpl1)
	}

	// 第 2 次执行：节点已被覆写为上次运行结果 "100 200"
	currNode2 := maa.NewNode(nodeName).SetFocus("100 200")
	tpl2 := runner.resolveTemplate(taskID, nodeName, "", currNode2, nil, 2)
	if tpl2 != "{0} {1}" {
		t.Fatalf("run 2 expected cached default template '{0} {1}', got %q", tpl2)
	}

	newOutput := formatFocus(tpl2, []string{"300", "400"})
	if newOutput != "300 400" {
		t.Fatalf("expected '300 400', got %q", newOutput)
	}
}

func TestTemplateCache_OCRTextContainsPlaceholder(t *testing.T) {
	runner := NewRunner()
	taskID := int64(2002)
	nodeName := "TestNode"

	currNode1 := maa.NewNode(nodeName).SetFocus("原始: {0}")
	tpl1 := runner.resolveTemplate(taskID, nodeName, "", currNode1, nil, 1)
	if tpl1 != "原始: {0}" {
		t.Fatalf("run 1 expected '原始: {0}', got %q", tpl1)
	}

	output1 := formatFocus(tpl1, []string{"Hello {1} World"})
	if output1 != "原始: Hello {1} World" {
		t.Fatalf("output1 mismatch: %q", output1)
	}

	currNode2 := maa.NewNode(nodeName).SetFocus(output1)
	tpl2 := runner.resolveTemplate(taskID, nodeName, "", currNode2, nil, 1)
	if tpl2 != "原始: {0}" {
		t.Fatalf("run 2 should still return cached '原始: {0}', but got corrupted %q", tpl2)
	}

	output2 := formatFocus(tpl2, []string{"NewVal"})
	if output2 != "原始: NewVal" {
		t.Fatalf("output2 mismatch: %q", output2)
	}
}

func TestTaskIsolation_NewStaticFocus(t *testing.T) {
	runner := NewRunner()
	nodeName := "ScoreNode"

	// Task 1
	nodeTask1 := maa.NewNode(nodeName).SetFocus("OldTask1: {0}")
	tpl1 := runner.resolveTemplate(301, nodeName, "", nodeTask1, nil, 1)
	if tpl1 != "OldTask1: {0}" {
		t.Fatalf("task 1 expected 'OldTask1: {0}', got %q", tpl1)
	}

	// Task 2
	nodeTask2 := maa.NewNode(nodeName).SetFocus("NewTask2: {0}")
	tpl2 := runner.resolveTemplate(302, nodeName, "", nodeTask2, nil, 1)
	if tpl2 != "NewTask2: {0}" {
		t.Fatalf("task 2 expected 'NewTask2: {0}', got %q", tpl2)
	}
}

func TestParseParam_ErrorDiagnostics(t *testing.T) {
	_, err := parseParam(`not a valid json`)
	if err == nil {
		t.Fatalf("expected error on invalid json, got nil")
	}

	p, err := parseParam(`{"regions": [{"roi": [10, 10, 50, 20]}]}`)
	if err != nil || len(p.Regions) != 1 {
		t.Fatalf("expected 1 region, got err=%v, len=%d", err, len(p.Regions))
	}
}

func TestEndToEndMaaFramework_OCRLog(t *testing.T) {
	libDir, _ := filepath.Abs("../../../deps/bin")
	if _, err := os.Stat(libDir); err != nil {
		libDir, _ = filepath.Abs("../../../install/maafw")
	}
	logDir, _ := filepath.Abs("../../../cache/maa-logs/test")
	_ = os.MkdirAll(logDir, 0755)

	err := maa.Init(maa.WithLibDir(libDir), maa.WithLogDir(logDir))
	if err != nil {
		t.Skipf("MaaFramework not initialized (library not found at %s): %v", libDir, err)
		return
	}
	defer maa.Release()

	tasker, err := maa.NewTasker()
	if err != nil {
		t.Fatalf("NewTasker failed: %v", err)
	}
	defer tasker.Destroy()

	ctrl, err := maa.NewBlankController()
	if err != nil {
		t.Fatalf("NewBlankController failed: %v", err)
	}
	defer ctrl.Destroy()
	if !ctrl.PostConnect().Wait().Success() {
		t.Fatalf("ctrl.PostConnect failed")
	}

	res, err := maa.NewResource()
	if err != nil {
		t.Fatalf("NewResource failed: %v", err)
	}
	defer res.Destroy()

	runner := NewRunner()
	// 直接在导出的 OCR 结果层注入测试数据，无需使用 unsafe.Pointer
	runner.ocrFunc = func(ctx *maa.Context, ocrParam *maa.OCRParam, img image.Image) ([]*maa.OCRResult, error) {
		box, _ := ocrParam.ROI.AsRect()
		return []*maa.OCRResult{
			{Box: box, Text: fmt.Sprintf("Score_%d", box.X()), Score: 0.99},
		}, nil
	}

	if err := res.RegisterCustomAction("OCRLog", runner); err != nil {
		t.Fatalf("RegisterCustomAction failed: %v", err)
	}
	if err := tasker.BindResource(res); err != nil {
		t.Fatalf("BindResource failed: %v", err)
	}
	if err := tasker.BindController(ctrl); err != nil {
		t.Fatalf("BindController failed: %v", err)
	}

	var capturedOutputFocus any
	tasker.OnNodeActionInContext(func(ctx *maa.Context, status maa.EventStatus, detail maa.NodeActionDetail) {
		if detail.Name == "LogOutputNode" && status == maa.EventStatusSucceeded {
			capturedOutputFocus = detail.Focus
		}
	})

	pipeline := maa.NewPipeline()
	execNode := maa.NewNode("ExecNode").
		SetRecognition(maa.RecDirectHit()).
		SetFocus("得分: {0}, 评级: {1}").
		SetAction(maa.ActCustom(maa.CustomActionParam{
			CustomAction: "OCRLog",
			CustomActionParam: `{
				"regions": [
					{"roi": [100, 100, 200, 40]},
					{"roi": [400, 100, 150, 40]}
				]
			}`,
		})).
		AddNext("LogOutputNode")

	logNode := maa.NewNode("LogOutputNode").
		SetRecognition(maa.RecDirectHit()).
		SetAction(maa.ActDoNothing())

	pipeline.AddNode(execNode)
	pipeline.AddNode(logNode)

	job := tasker.PostTask(execNode.Name, pipeline)
	if !job.Wait().Success() {
		t.Fatalf("task execution failed")
	}

	t.Logf("Captured LogOutputNode Focus: %v", capturedOutputFocus)
	expectedFocus := "得分: Score_100, 评级: Score_400"
	if capturedOutputFocus != expectedFocus {
		t.Fatalf("LogOutputNode Focus mismatch. Got %v, want %q", capturedOutputFocus, expectedFocus)
	}
}

func TestResolveTemplate_MPEMapFocus(t *testing.T) {
	runner := NewRunner()
	taskID := int64(5001)

	// MPE 生成的 map[string]any 格式 focus
	node := maa.NewNode("MPENode").SetFocus(map[string]any{
		"Node.Recognition.Starting":  "场次：{0}",
		"Node.Recognition.Succeeded": "场次：{0}",
	})

	tpl := runner.resolveTemplate(taskID, "MPENode", "", node, nil, 1)
	if tpl != "场次：{0}" {
		t.Fatalf("expected '场次：{0}' from MPE map focus, got %q", tpl)
	}

	// 覆写结构测试
	overridden := buildOverrideFocus(node.Focus, "场次：1")
	m, ok := overridden.(map[string]any)
	if !ok || m["Node.Recognition.Succeeded"] != "场次：1" {
		t.Fatalf("expected map[string]any with updated values, got %v", overridden)
	}
}



