package clickaway

import (
	"fmt"
	"image"
	"reflect"
	"testing"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

var (
	_ maa.ContextEventSink = (*EvasionSink)(nil)
	_ maa.TaskerEventSink  = (*EvasionSink)(nil)
)

func TestCountsCandidateRoundsAndClearsOnSuccess(t *testing.T) {
	sink := NewEvasionSink()
	detail := maa.NodeNextListDetail{
		TaskID: 1,
		Name:   "温泉",
		List:   []maa.NextItem{{Name: "继续"}, {Name: "领取"}},
	}
	runRound := func(status maa.EventStatus) *stage {
		t.Helper()
		if sink.recordRound(maa.EventStatusStarting, detail) != nil {
			t.Fatal("starting a round must not trigger a check")
		}
		for _, item := range detail.List {
			sink.OnNodeRecognition(nil, maa.EventStatusFailed, maa.NodeRecognitionDetail{
				TaskID: detail.TaskID, Name: item.Name,
			})
		}
		return sink.recordRound(status, detail)
	}

	for range 2 {
		if runRound(maa.EventStatusFailed) != nil {
			t.Fatal("two rounds with two failed candidates each must not trigger")
		}
	}
	if runRound(maa.EventStatusSucceeded) != nil {
		t.Fatal("a successful candidate list must not trigger")
	}
	for range 2 {
		if runRound(maa.EventStatusFailed) != nil {
			t.Fatal("a success must reset the streak")
		}
	}
	current := runRound(maa.EventStatusFailed)
	if current == nil || !sink.beginMove(detail.TaskID, current) {
		t.Fatal("the third failed list should allow one move")
	}
	if sink.beginMove(detail.TaskID, current) {
		t.Fatal("the same stage allowed a second move")
	}
	for range 4 {
		if runRound(maa.EventStatusFailed) != nil {
			t.Fatal("continued failures must not move again")
		}
	}
	sink.OnTaskerTask(nil, maa.EventStatusFailed, maa.TaskerTaskDetail{TaskID: detail.TaskID})
	if _, exists := sink.stages[detail.TaskID]; exists {
		t.Fatal("completed task left stage state behind")
	}
}

func TestCursorCheckRetriesAndStaysWithinItsStage(t *testing.T) {
	sink := NewEvasionSink()
	detail := maa.NodeNextListDetail{TaskID: 1, Name: "parent", List: []maa.NextItem{{Name: "first"}}}
	fail := func(rounds int) *stage {
		var last *stage
		for range rounds {
			last = sink.recordRound(maa.EventStatusFailed, detail)
		}
		return last
	}
	current := fail(3)
	if current == nil {
		t.Fatal("expected eligibility check")
	}
	if current = fail(3); current == nil {
		t.Fatal("a skipped check must allow a later streak")
	}
	detail.List = []maa.NextItem{{Name: "fallback"}}
	sink.recordRound(maa.EventStatusStarting, detail)
	if sink.beginMove(detail.TaskID, current) {
		t.Fatal("an eligibility check from the previous list must not move")
	}
	if fail(2) != nil || fail(1) == nil {
		t.Fatal("changed list must have an independent failure streak")
	}
}

func TestRoundROIs(t *testing.T) {
	bounds := image.Rect(0, 0, 1280, 720)
	nodes := map[string]string{
		"ocr":       `{"recognition":{"type":"OCR","param":{"roi":[30,10,50,40],"roi_offset":[-10,5,25,0]}}}`,
		"legacy":    `{"recognition":"TemplateMatch","roi":[-20,-20,0,0]}`,
		"full":      `{"recognition":"OCR","roi":[0,0,1280,720]}`,
		"default":   `{"recognition":"OCR"}`,
		"no reco":   `{"recognition":{"type":"DirectHit","param":{"roi":[10,10,5,5]}}}`,
		"disabled":  `{"enabled":false,"recognition":"OCR","roi":[10,10,5,5]}`,
		"reference": `{"recognition":"OCR","roi":"previous"}`,
	}
	load := func(name string) (string, error) {
		raw, ok := nodes[name]
		if !ok {
			return "", fmt.Errorf("missing %s", name)
		}
		return raw, nil
	}
	getAnchor := func(name string) (string, error) {
		if name == "legacy anchor" {
			return "legacy", nil
		}
		return "", nil
	}
	list := []maa.NextItem{{Name: "ocr"}, {Name: "legacy anchor", Anchor: true}, {Name: "full"}, {Name: "default"},
		{Name: "no reco"}, {Name: "disabled"}, {Name: "ocr"}}
	rois, err := roundROIs(list, bounds, load, getAnchor)
	want := []image.Rectangle{image.Rect(20, 15, 95, 55), image.Rect(1260, 700, 1280, 720)}
	if err != nil || !reflect.DeepEqual(rois, want) {
		t.Fatalf("round ROIs = %v, %v; want %v", rois, err, want)
	}
	if _, err := roundROIs(append(list, maa.NextItem{Name: "reference"}), bounds, load, getAnchor); err == nil {
		t.Fatal("a dynamic ROI must prevent choosing an unverified target")
	}
}

func TestRetreatTargetAvoidsAllROIs(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 60)
	cursor := image.Pt(44, 30)
	rois := []image.Rectangle{image.Rect(0, 0, 45, 60), image.Rect(45, 0, 80, 60)}
	if !inAnyROI(cursor, rois) || inAnyROI(image.Pt(90, 30), rois) || inAnyROI(cursor, nil) {
		t.Fatal("cursor eligibility must use the union of the effective ROIs")
	}
	target, ok := retreatTarget(cursor, bounds, rois, image.Pt(1, 1))
	if !ok || target != image.Pt(81, 30) {
		t.Fatalf("retreat target = %v, %v; want (81,30)", target, ok)
	}

	rois = append(rois, image.Rect(80, 0, 100, 60))
	if target, ok := retreatTarget(cursor, bounds, rois, image.Point{}); ok {
		t.Fatalf("target %v selected although the ROIs cover the whole image", target)
	}
}
