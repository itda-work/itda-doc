package mdoc

import (
	"reflect"
	"strings"
	"testing"
)

// 게시용 검사(Options.RequireFigureDesc, itda-work/itda-hyve#845) — 설명 없는 그림·캡션 없는 차트에 figure-no-desc.

// figureKinds 는 일곱 그림 종류마다 그려지는 가장 작은 원문이다. %s 자리에 설명 줄(없으면 빈 줄)을 넣는다.
var figureKinds = []struct{ name, body string }{
	{"flowchart", "flowchart LR\n%s\nA --> B\n"},
	{"gantt", "gantt\ndateFormat YYYY-MM-DD\n%s\nsection 가\n작업 :a1, 2026-01-01, 3d\n"},
	{"timeline", "timeline\n%s\n2025 : 시작\n2026 : 끝\n"},
	{"sequence", "sequenceDiagram\n%s\nA->>B: 안녕\n"},
	{"er", "erDiagram\n%s\nCUSTOMER ||--o{ ORDER : places\n"},
	{"state", "stateDiagram-v2\n%s\n[*] --> 켬\n켬 --> [*]\n"},
	{"classdiag", "classDiagram\n%s\nclass 동물\n동물 <|-- 개\n"},
}

func figureDescWarns(t *testing.T, src string, opts Options) []Warning {
	t.Helper()
	r, err := Render([]byte(src), opts)
	if err != nil {
		t.Fatal(err)
	}
	var out []Warning
	for _, w := range r.Warnings {
		if w.Code == WarnFigureNoDesc {
			out = append(out, w)
		}
		if w.Code == WarnDiagram && strings.Contains(w.Message, "그리지 않고") {
			t.Fatalf("그림을 그리지 못함(시험 원문을 고친다): %v\n%s", w, src)
		}
	}
	return out
}

func TestFigureNoDescMermaid(t *testing.T) {
	descs := []struct{ name, line string }{
		{"한 줄 꼴", "accDescr: 두 단계를 잇는 그림"},
		{"블록 꼴", "accDescr {\n두 단계를\n잇는 그림\n}"},
	}
	for _, k := range figureKinds {
		t.Run(k.name, func(t *testing.T) {
			// 문서: 1 머리 문단, 2 빈 줄, 3 여는 펜스
			none := "머리\n\n```mermaid\n" + strings.Replace(k.body, "%s", "", 1) + "```\n"
			ws := figureDescWarns(t, none, Options{RequireFigureDesc: true})
			if len(ws) != 1 || ws[0].Line != 3 {
				t.Fatalf("accDescr 없는 그림의 경고 = %v, 3번째 줄에 하나여야 함", ws)
			}
			if ws := figureDescWarns(t, none, Options{}); len(ws) != 0 {
				t.Fatalf("옵션을 끄면 경고가 없어야 함: %v", ws)
			}
			// title·accTitle 만으로는 설명이 아니다
			acc := "```mermaid\n" + strings.Replace(k.body, "%s", "accTitle: 제목", 1) + "```\n"
			if ws := figureDescWarns(t, acc, Options{RequireFigureDesc: true}); len(ws) != 1 {
				t.Fatalf("accTitle 만 있는 그림의 경고 = %v, 하나여야 함", ws)
			}
			for _, d := range descs {
				src := "```mermaid\n" + strings.Replace(k.body, "%s", d.line, 1) + "```\n"
				if ws := figureDescWarns(t, src, Options{RequireFigureDesc: true}); len(ws) != 0 {
					t.Fatalf("%s accDescr 가 있는데 경고: %v", d.name, ws)
				}
			}
		})
	}
}

func TestFigureNoDescMermaidTitleOnly(t *testing.T) {
	src := "```mermaid\n---\ntitle: 캡션\n---\nflowchart LR\nA --> B\n```\n"
	if ws := figureDescWarns(t, src, Options{RequireFigureDesc: true}); len(ws) != 1 || ws[0].Line != 1 {
		t.Fatalf("title 만 있는 그림의 경고 = %v, 1번째 줄에 하나여야 함", ws)
	}
}

func TestFigureNoDescNotDrawn(t *testing.T) {
	// 그리지 못한 mermaid·차트에는 figure-no-desc 가 없고 기존 경고만 난다
	src := "```mermaid\nflowchart LR\nA -->\n```\n\n:::chart\n\n글만 있음\n\n:::\n"
	r, err := Render([]byte(src), Options{RequireFigureDesc: true})
	if err != nil {
		t.Fatal(err)
	}
	var diag, chart bool
	for _, w := range r.Warnings {
		switch w.Code {
		case WarnFigureNoDesc:
			t.Fatalf("그리지 못한 블록에 figure-no-desc: %v", w)
		case WarnDiagram:
			diag = true
		case WarnChart:
			chart = true
		}
	}
	if !diag || !chart {
		t.Fatalf("기존 diagram·chart 경고가 있어야 함: %v", r.Warnings)
	}
}

const figureChart = "머리\n\n:::chart{type=bar}\n\n| 분기 | 매출 |\n|:--|--:|\n| 1분기 | 3 |\n| 2분기 | 4 |\n%s\n:::\n"

func TestFigureNoDescChart(t *testing.T) {
	none := strings.Replace(figureChart, "%s", "", 1)
	ws := figureDescWarns(t, none, Options{RequireFigureDesc: true})
	if len(ws) != 1 || ws[0].Line != 3 {
		t.Fatalf("캡션 없는 차트의 경고 = %v, 3번째 줄에 하나여야 함", ws)
	}
	if ws := figureDescWarns(t, none, Options{}); len(ws) != 0 {
		t.Fatalf("옵션을 끄면 경고가 없어야 함: %v", ws)
	}
	with := strings.Replace(figureChart, "%s", "\n: 분기별 매출\n", 1)
	r, err := Render([]byte(with), Options{RequireFigureDesc: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.HTML, `class="chart chart-bar"`) {
		t.Fatalf("차트를 그리지 않음: %s", r.HTML)
	}
	for _, w := range r.Warnings {
		if w.Code == WarnFigureNoDesc {
			t.Fatalf("캡션 있는 차트에 경고: %v", w)
		}
	}
}

func TestFigureNoDescNotes(t *testing.T) {
	src := "본문\n\n:::notes\n\n```mermaid\nflowchart LR\nA --> B\n```\n\n" + strings.Replace(figureChart, "%s", "", 1) + "\n:::\n"
	if ws := figureDescWarns(t, src, Options{RequireFigureDesc: true, DropNotes: true}); len(ws) != 0 {
		t.Fatalf("DropNotes 면 노트 안 그림에 경고가 없어야 함: %v", ws)
	}
	if ws := figureDescWarns(t, src, Options{RequireFigureDesc: true}); len(ws) != 2 {
		t.Fatalf("DropNotes 없이는 노트 안 그림·차트에 경고 둘이어야 함: %v", ws)
	}
}

func TestFigureNoDescCacheIndependent(t *testing.T) {
	// 같은 원문을 켬 → 끔 → 켬 순으로 렌더해도 결과가 각각 같다(diagramCache 가 옵션에 물들지 않는다)
	src := []byte("```mermaid\nflowchart LR\n캐시시험 --> B\n```\n\n" + strings.Replace(figureChart, "%s", "", 1))
	render := func(on bool) Result {
		r, err := Render(src, Options{RequireFigureDesc: on})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	on1, off, on2 := render(true), render(false), render(true)
	if !reflect.DeepEqual(on1, on2) {
		t.Fatal("켬 두 번의 결과가 다름")
	}
	if on1.HTML != off.HTML {
		t.Fatal("옵션이 HTML 을 바꿈")
	}
	n := func(ws []Warning) (c int) {
		for _, w := range ws {
			if w.Code == WarnFigureNoDesc {
				c++
			}
		}
		return
	}
	if n(on1.Warnings) != 2 || n(off.Warnings) != 0 || len(on1.Warnings)-2 != len(off.Warnings) {
		t.Fatalf("경고: 켬 %v / 끔 %v", on1.Warnings, off.Warnings)
	}
}
