package mdoc

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// itda-hyve#475 그림 자리 — 펜스 줄 ratio·width·height 의 가림(parseDiagramSize)과 문서 자리(figure 속성·경고 줄).
func TestDiagramSizeParse(t *testing.T) {
	cases := []struct {
		info  string
		want  diagramSize
		warns []string // 경고 문구의 앞부분(포함 검사)
	}{
		{info: "mermaid", want: diagramSize{}},
		{info: "mermaid ratio=16:9", want: diagramSize{ratioW: 16, ratioH: 9}},
		{info: "mermaid width=50%", want: diagramSize{width: 50}},
		{info: "mermaid ratio=4:3 width=40%", want: diagramSize{ratioW: 4, ratioH: 3, width: 40}},
		{info: "mermaid ratio=4:3 height=70%", want: diagramSize{ratioW: 4, ratioH: 3, height: 70}},
		{info: "mermaid width=30% height=90%", want: diagramSize{width: 30, height: 90}},
		// 셋 다 — ratio 를 뗀다(경고 한 번)
		{info: "mermaid ratio=16:9 width=50% height=60%", want: diagramSize{width: 50, height: 60},
			warns: []string{"ratio 를 안 씀"}},
		// 값 경계: 10~100 까지
		{info: "mermaid width=9%", want: diagramSize{}, warns: []string{"width=9%"}},
		{info: "mermaid width=10%", want: diagramSize{width: 10}},
		{info: "mermaid width=100%", want: diagramSize{width: 100}},
		{info: "mermaid width=101%", want: diagramSize{}, warns: []string{"width=101%"}},
		{info: "mermaid height=0%", want: diagramSize{}, warns: []string{"height=0%"}},
		{info: "mermaid width=50", want: diagramSize{}, warns: []string{"% 필수"}}, // % 없음
		{info: "mermaid width=50.5%", want: diagramSize{}, warns: []string{"width=50.5%"}},
		// 비율 경계: 1/10~10 까지(W·H 은 1~99)
		{info: "mermaid ratio=1:10", want: diagramSize{ratioW: 1, ratioH: 10}},
		{info: "mermaid ratio=1:11", want: diagramSize{}, warns: []string{"ratio=1:11"}},
		{info: "mermaid ratio=11:1", want: diagramSize{}, warns: []string{"ratio=11:1"}},
		{info: "mermaid ratio=10:1", want: diagramSize{ratioW: 10, ratioH: 1}},
		{info: "mermaid ratio=0:5", want: diagramSize{}, warns: []string{"ratio=0:5"}},
		{info: "mermaid ratio=100:5", want: diagramSize{}, warns: []string{"ratio=100:5"}},
		{info: "mermaid ratio=16:9:9", want: diagramSize{}, warns: []string{"ratio=16:9:9"}},
		{info: "mermaid ratio=abc", want: diagramSize{}, warns: []string{"ratio=abc"}},
		// 낱말 경계 — mywidth= 는 잡지 않는다(낱말 전체가 속성 꼴이어야 한다)
		{info: "mermaid mywidth=50%", want: diagramSize{}},
		// 같은 속성이 여럿이면 앞의 것이 이긴다
		{info: "mermaid width=50% width=70%", want: diagramSize{width: 50}},
		// title= 과 함께 쓸 수 있다
		{info: `mermaid title="a.md" ratio=16:9`, want: diagramSize{ratioW: 16, ratioH: 9}},
		// 리뷰 1회차 [1] 회귀 — 다른 속성의 따옴표 값 안은 낱말이 아니므로 크기 속성이 아니다
		{info: `mermaid title="report width=50% draft.md"`, want: diagramSize{}},
		{info: `mermaid title="report width=50% draft.md" width=70%`, want: diagramSize{width: 70}},
		{info: `mermaid title="a width=50%" width=70%`, want: diagramSize{width: 70}},
		// 리뷰 2회차 [1] 회귀 — 따옴표 없는 꼴만 받는다(값에 따옴표가 든 낱말은 경고 후 무시)
		{info: `mermaid ratio="16:9"`, want: diagramSize{}, warns: []string{`ratio="16:9"`}},
		{info: `mermaid ratio='1:1'`, want: diagramSize{}, warns: []string{`ratio='1:1'`}},
		{info: `mermaid height="60%"`, want: diagramSize{}, warns: []string{`height="60%"`}},
		{info: `mermaid width=50%"junk"`, want: diagramSize{}, warns: []string{`width=50%"junk"`}},
		{info: `mermaid width=50%'junk'`, want: diagramSize{}, warns: []string{`width=50%'junk'`}},
		{info: `mermaid width="50%"junk`, want: diagramSize{}, warns: []string{`width="50%"junk`}},
		{info: `mermaid ratio=16:9"x"`, want: diagramSize{}, warns: []string{`ratio=16:9"x"`}},
		// 낱말 경계는 unicode.IsSpace 다 — 전각 공백·탭으로 나뉜 속성도 읽는다
		{info: "mermaid　width=50%", want: diagramSize{width: 50}},
		{info: "mermaid\twidth=50%", want: diagramSize{width: 50}},
		// 닫히지 않은 따옴표는 줄 끌까지가 그 속성의 값이라 뒤의 크기 낱말을 읽지 않는다(경고도 없다)
		{info: `mermaid title="a width=50%`, want: diagramSize{}},
		// 크기 낱말 안에서 닫히지 않은 따옴표 — 낱말은 다음 공백에서 끊긴다: width 는 틀린 값, 뒤 height 는 산다
		{info: `mermaid width="50% height=60%`, want: diagramSize{height: 60}, warns: []string{`width="50%`}},
	}
	for _, c := range cases {
		got, warns := parseDiagramSize(c.info)
		if got != c.want {
			t.Errorf("parseDiagramSize(%q) = %+v, 기대 %+v", c.info, got, c.want)
		}
		if len(warns) != len(c.warns) {
			t.Errorf("parseDiagramSize(%q) 경고 %d 개(%v), 기대 %d 개", c.info, len(warns), warns, len(c.warns))
			continue
		}
		for i, w := range c.warns {
			if !strings.Contains(warns[i], w) {
				t.Errorf("parseDiagramSize(%q) 경고 %d 가 %q 를 담지 않음: %q", c.info, i, w, warns[i])
			}
		}
	}
}

// 문서 자리 — 조합 표의 각 줄이 펜스 줄 → figure 의 data-diagram-* 속성이 된다. 속성이 없으면 출력에 아무것도 남지 않는다.
func TestDiagramSizeFence(t *testing.T) {
	body := "flowchart LR\n  A[시작] --> B[끝]"
	cases := []syntaxCase{
		{name: "속성 없음 — 출력 불변", src: "```mermaid\n" + body + "\n```",
			want: []string{`<figure class="diagram" data-line="1">`}, not: []string{"data-diagram"}},
		{name: "ratio", src: "```mermaid ratio=16:9\n" + body + "\n```",
			want: []string{`<figure class="diagram" data-line="1" data-diagram-ratio="16:9">`}, not: []string{"data-diagram-width", "data-diagram-height"}},
		{name: "width", src: "```mermaid width=50%\n" + body + "\n```",
			want: []string{` data-diagram-width="50%"`}},
		{name: "height", src: "```mermaid height=60%\n" + body + "\n```",
			want: []string{` data-diagram-height="60%"`}},
		{name: "ratio+width", src: "```mermaid ratio=4:3 width=40%\n" + body + "\n```",
			want: []string{` data-diagram-ratio="4:3" data-diagram-width="40%"`}},
		{name: "ratio+height", src: "```mermaid ratio=4:3 height=70%\n" + body + "\n```",
			want: []string{` data-diagram-ratio="4:3" data-diagram-height="70%"`}},
		{name: "width+height", src: "```mermaid width=30% height=90%\n" + body + "\n```",
			want: []string{` data-diagram-width="30%" data-diagram-height="90%"`}},
		{name: "셋 다 — ratio 를 뗀다", src: "```mermaid ratio=16:9 width=50% height=60%\n" + body + "\n```",
			want: []string{` data-diagram-width="50%" data-diagram-height="60%"`}, not: []string{`data-diagram-ratio="16:9"`}, warn: WarnDiagram},
		{name: "틀린 ratio 값 — 경고하고 그림", src: "```mermaid ratio=abc\n" + body + "\n```",
			want: []string{"<svg"}, not: []string{"data-diagram"}, warn: WarnDiagram},
		{name: "따옴표 값 — 경고하고 무시", src: "```mermaid ratio=\"16:9\" height=\"60%\"\n" + body + "\n```",
			want: []string{"<svg"}, not: []string{"data-diagram"}, warn: WarnDiagram},
		{name: "줄임 파일명 표기와 함께", src: "```mermaid:flow.md ratio=16:9\n" + body + "\n```",
			want: []string{` data-diagram-ratio="16:9"`}},
	}
	runCases(t, cases)

	// 경고 줄 번호 — 펜스 줄. 문단 뒤 3째 줄.
	r := render(t, "앞\n\n```mermaid ratio=x\n"+body+"\n```\n")
	if len(r.Warnings) != 1 || r.Warnings[0].Line != 3 || r.Warnings[0].Code != WarnDiagram ||
		!strings.Contains(r.Warnings[0].Message, "ratio=x") {
		t.Errorf("경고가 펜스 줄(3) 에 없음: %+v", r.Warnings)
	}
	// 값 여럿이 틀리면 값마다 경고
	r = render(t, "```mermaid ratio=x width=5 height=abc\n"+body+"\n```")
	if len(r.Warnings) != 3 {
		t.Errorf("틀린 값 3개에 경고가 3개여야 함: %+v", r.Warnings)
	}

	// 그림을 그리지 못하면(코드 블록으로 남으면) 속성도 자리 경고도 없다 — 그림 실패 경고만.
	r = render(t, "```mermaid ratio=16:9 width=5\n    pie\n      \"a\" : 1\n```")
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0].Message, "그리지 않고") {
		t.Errorf("그림 실패 경고만 있어야 함: %+v", r.Warnings)
	}
	contains(t, r.HTML, "language-mermaid")
	notContains(t, r.HTML, "data-diagram")
}

// 그림 안내서 6장의 그림 자리 절(itda-hyve#475) — 조합 표의 각 줄(쓴 것)이 그대로 data-diagram-* 속성이 되는지,
// 속성 없는 줄은 아무 속성도 내지 않는지, 예제가 경고 없이 그려지는지. 기대 속성은 parseDiagramSize 로 다시 얻어
// 표와 코드가 같은 규칙을 말하는지를 본다(안내서가 코드를 따라가는지, 값 표기의 검증은 TestDiagramSizeParse).
func TestDiagramSizeGuide(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "spec", "diagrams.md"))
	if err != nil {
		t.Fatal(err)
	}
	g := string(b)
	i, j := strings.Index(g, "<!-- diagram-size:begin -->"), strings.Index(g, "<!-- diagram-size:end -->")
	if i < 0 || j < i {
		t.Fatal("안내서에 diagram-size 표지가 없음")
	}
	rows := 0
	for _, line := range strings.Split(g[i:j], "\n") {
		if !strings.HasPrefix(line, "| ") || strings.HasPrefix(line, "| 쓴 것") || strings.HasPrefix(line, "|---") {
			continue
		}
		rows++
		cell := strings.SplitN(line[1:], "|", 2)[0]
		var codes []string
		for _, m := range backtickRe.FindAllStringSubmatch(cell, -1) {
			codes = append(codes, m[1]+m[2])
		}
		c := strings.Join(codes, " ")
		info := "mermaid"
		if c != "" {
			info += " " + c
		}
		r := render(t, "```"+info+"\nflowchart LR\n  A[시작] --> B[끝]\n```")
		if !strings.Contains(r.HTML, "<svg") {
			t.Errorf("그림 자리 %q: 그림이 안 그려짐: %+v", c, r.Warnings)
			continue
		}
		sz, _ := parseDiagramSize(c)
		want, not := sz.attrsHTML()
		contains(t, r.HTML, want...)
		notContains(t, r.HTML, not...)
		allThree := strings.Count(c, "ratio=") > 0 && strings.Contains(c, "width=") && strings.Contains(c, "height=")
		if allThree && len(r.Warnings) != 1 {
			t.Errorf("그림 자리 %q: 셋 다 경고가 1개여야 함: %+v", c, r.Warnings)
		}
		if !allThree && len(r.Warnings) > 0 {
			t.Errorf("그림 자리 %q: 경고가 없어야 함: %+v", c, r.Warnings)
		}
	}
	if rows != 8 {
		t.Errorf("그림 자리 조합 표에서 %d 행을 읽음(기대 8)", rows)
	}
	// 절의 예제 — 경고 없이 그려지고 값이 속성으로 나간다
	i = strings.Index(g, "<!-- diagram-size-example -->")
	ex := g[i:]
	ex = ex[strings.Index(ex, "```mermaid"):]
	ex = ex[:strings.Index(ex, "\n````")]
	r := render(t, ex)
	if len(r.Warnings) > 0 || !strings.Contains(r.HTML, "<svg") ||
		!strings.Contains(r.HTML, ` data-diagram-ratio="16:9" data-diagram-height="60%"`) {
		t.Errorf("그림 자리 예제: %+v\n%s", r.Warnings, r.HTML)
	}
}

var backtickRe = regexp.MustCompile("``(.+?)``|`([^`]+)`")

// attrsHTML 은 diagramSize 가 내야 할 속성(있어야 할 것·없어야 할 것)을 검사 문자열로 준다.
func (sz diagramSize) attrsHTML() (want, not []string) {
	if sz.ratioW != 0 {
		want = append(want, ` data-diagram-ratio="`+strconv.Itoa(sz.ratioW)+":"+strconv.Itoa(sz.ratioH)+`"`)
	} else {
		not = append(not, "data-diagram-ratio=")
	}
	if sz.width != 0 {
		want = append(want, ` data-diagram-width="`+strconv.Itoa(sz.width)+`%"`)
	} else {
		not = append(not, "data-diagram-width=")
	}
	if sz.height != 0 {
		want = append(want, ` data-diagram-height="`+strconv.Itoa(sz.height)+`%"`)
	} else {
		not = append(not, "data-diagram-height=")
	}
	return want, not
}
