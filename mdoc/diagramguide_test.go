package mdoc

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/diagram/classdiag"
	"github.com/itda-work/itda-doc/diagram/er"
	"github.com/itda-work/itda-doc/diagram/flowchart"
	"github.com/itda-work/itda-doc/diagram/gantt"
	"github.com/itda-work/itda-doc/diagram/sequence"
	statediagram "github.com/itda-work/itda-doc/diagram/state"
	"github.com/itda-work/itda-doc/diagram/timeline"
)

// 그림 안내서(spec/diagrams.md)를 코드와 맞춘다 — 상한 표는 상수와, 지원 문법 표의 표기는 경고 없이 그려지는지,
// 쓰지 않는 줄 표의 표기는 경고하고 그림은 그리는지, 미지원 표의 표기는 원문 그대로 두는지, 1장 예제는 경고 없이 그려지는지.
func TestDiagramGuide(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "spec", "diagrams.md"))
	if err != nil {
		t.Fatal(err)
	}
	g := string(b)
	section := func(name string) string {
		i, j := strings.Index(g, "<!-- "+name+":begin -->"), strings.Index(g, "<!-- "+name+":end -->")
		if i < 0 || j < i {
			t.Fatalf("안내서에 %s 표지가 없음", name)
		}
		return g[i:j]
	}
	want := map[string]int{
		"그림 하나의 원문(바이트)": flowchart.MaxSource, "노드 수": flowchart.MaxNodes, "화살표 수": flowchart.MaxEdges,
		"묶음(subgraph) 수": flowchart.MaxSubgraphs, "라벨 하나의 글자 수": flowchart.MaxLabel, "화살표 길이": flowchart.MaxLength,
		"문서 하나의 그림 수": MaxDiagramsPerDoc,
	}
	got := map[string]int{}
	for _, m := range regexp.MustCompile(`(?m)^\| ([^|]+?) \| (\d+) \|`).FindAllStringSubmatch(section("diagram-limits"), -1) {
		n, _ := strconv.Atoi(m[2])
		got[m[1]] = n
	}
	if len(got) != len(want) {
		t.Errorf("상한 표 %v, 코드 %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("상한 %q: 안내서 %d, 코드 %d", k, got[k], v)
		}
	}

	// 표의 둘째 칸 코드들. 칸 안의 \| 는 | 다(GFM)
	codes := func(sec string) [][]string {
		var out [][]string
		re := regexp.MustCompile("``(.+?)``|`([^`]+)`")
		for _, line := range strings.Split(section(sec), "\n") {
			if !strings.HasPrefix(line, "| ") || strings.HasPrefix(line, "| 분류") || strings.HasPrefix(line, "| 표기") {
				continue
			}
			cells := regexp.MustCompile(`[^\\]\|`).Split(line[1:], -1) // 줄 머리의 | 를 떼고, \| 가 아닌 | 에서
			col := cells[0]
			if sec == "diagram-support" {
				col = cells[1]
			}
			var cs []string
			for _, m := range re.FindAllStringSubmatch(col, -1) {
				c := m[1] + m[2]
				cs = append(cs, strings.TrimSpace(strings.ReplaceAll(c, `\|`, "|")))
			}
			out = append(out, cs)
		}
		return out
	}
	block := func(body string) string { return "```mermaid\n" + body + "\n```" }
	n := 0
	for _, row := range codes("diagram-support") {
		switch {
		case len(row) > 0 && strings.HasPrefix(row[0], "subgraph"): // subgraph … end 두 칸
			row = []string{row[0] + "\nX\nend"}
		case len(row) > 0 && row[0] == "---":
			row = []string{"---\ntitle: 제목\n---\ngraph TD\nA"}
		case len(row) > 0 && row[0] == "accDescr { … }":
			row = []string{"accDescr {\n설명\n}"}
		}
		for _, c := range row {
			src := c
			switch {
			case strings.HasPrefix(c, "flowchart") || strings.HasPrefix(c, "graph") || strings.HasPrefix(c, "---"):
				if !strings.Contains(c, "\n") {
					src = c + "\nA --> B"
				}
			case c == "줄바꿈 또는 ;" || c == ";":
				src = "graph TD\nA --> B; B --> C"
			default:
				src = "graph TD\n" + c + "\nA --> B"
			}
			r := render(t, block(src))
			if len(r.Warnings) > 0 || !strings.Contains(r.HTML, "<svg") {
				t.Errorf("지원 문법 %q 를 그리지 못함: %+v", c, r.Warnings)
			}
			n++
		}
	}
	if n < 30 {
		t.Errorf("지원 문법 표에서 표기 %d 개만 읽음", n)
	}
	for _, row := range codes("diagram-ignored") {
		for _, c := range row {
			src := "graph TD\nA --> B\n" + c
			switch {
			case strings.HasPrefix(c, "%%{"):
				src = c + "\ngraph TD\nA --> B"
			case c == "---" || c == "title": // 같은 칸의 config: 가 본다
				continue
			case c == "config:":
				src = "---\nconfig:\n  theme: dark\n---\ngraph TD\nA --> B"
			case strings.HasPrefix(c, "direction"):
				src = "graph TD\nsubgraph s\n" + c + "\nA\nend"
			case strings.HasPrefix(c, "A:::"):
				src = "graph TD\n" + c + " --> B"
			}
			r := render(t, block(src))
			if len(r.Warnings) != 1 || !strings.Contains(r.HTML, "<svg") {
				t.Errorf("쓰지 않는 줄 %q: 경고 %+v, 그림 %v", c, r.Warnings, strings.Contains(r.HTML, "<svg"))
			}
		}
	}
	var ranUns []string
	for _, row := range codes("diagram-unsupported") {
		for _, c := range row {
			ranUns = append(ranUns, c)
			src := "graph TD\n" + c
			switch {
			case strings.Contains(c, "Diagram") || c == "pie" || c == "mindmap":
				src = c
			case strings.HasPrefix(c, "subgraph"):
				src = "graph TD\nsubgraph a\nsubgraph b\nX\nend\nend"
			case c == "A --> 묶음id":
				src = "graph TD\nsubgraph 묶음id\nX\nend\nA --> 묶음id"
			case c == "…":
				continue
			case strings.HasPrefix(c, "acc"): // 접근성 값 — 그릴 노드를 두어 그 줄만 원문인지 본다
				src = "graph TD\n" + c + "\nA"
			}
			r := render(t, block(src))
			if len(r.Warnings) != 1 || strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "language-mermaid") {
				t.Errorf("미지원 %q 가 원문 + 경고가 아님: %+v", c, r.Warnings)
			}
		}
	}
	// 설정 지시 경계(④-1 후속) — 표에서 빠지면 위 실행도 빠지므로 표기가 남아 있는지 본다
	for _, m := range []string{"%%{init: {", "%%{init: {}}%% A --> B", "accTitle: 앞 %%{", "A[글 %%{", "%% 메모 %%{", "%%",
		"A[#copy;]", "A[x#160;y]", "A[두  칸]", "accTitle: a&b", // 엔티티·공백 정책 B(itda-hyve#409 ④-2 후속)
		`A["style x:#35;"];`, `A["classDef x:#35;"];`, "A --> B; classDef c fill:#f00;", // 줄 전처리 경계(같은 후속 수정 1)
		"accTitle:", "accDescr:", "accDescr { 설명 } 뒤"} { // 빈 접근성 값·} 뒤 글(같은 후속 수정 2) // 줄 전처리 경계(같은 후속 수정 1)
		found := false
		for _, c := range ranUns {
			found = found || strings.HasPrefix(c, m) && (m != "%%" || c == m) // 글 없는 주석은 정확히
		}
		if !found {
			t.Errorf("흐름도 미지원 표에 %q 표기가 없음", m)
		}
	}
	// title 표기 표(리뷰 R1): 쓴 것 → 캡션. 칸의 ↵ 는 줄바꿈, \| 는 |
	rows := 0
	for _, line := range strings.Split(section("diagram-title"), "\n") {
		if !strings.HasPrefix(line, "| `title") {
			continue
		}
		cells := regexp.MustCompile(`[^\\]\|`).Split(line[1:], -1)
		var fm []string
		for _, m := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(cells[0], -1) {
			fm = append(fm, strings.ReplaceAll(m[1], `\|`, "|"))
		}
		want := strings.TrimSpace(cells[1])
		r := render(t, block("---\n"+strings.Join(fm, "\n")+"\n---\ngraph TD\nA"))
		if len(r.Warnings) > 0 || !strings.Contains(r.HTML, "<figcaption>"+want+"</figcaption>") {
			t.Errorf("title 표 %q: 캡션이 %q 가 아님 %+v", fm, want, r.Warnings)
		}
		rows++
	}
	if rows != 7 {
		t.Errorf("title 표에서 %d 행만 읽음", rows)
	}
	// 1장 예제
	i := strings.Index(g, "<!-- diagram-example -->")
	ex := g[i:]
	ex = ex[strings.Index(ex, "```mermaid"):]
	ex = ex[:strings.Index(ex, "\n````")]
	r := render(t, ex)
	if len(r.Warnings) > 0 || !strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "<figcaption>구매 결재 흐름</figcaption>") {
		t.Errorf("1장 예제: %+v\n%s", r.Warnings, r.HTML)
	}
}

// 8장 간트 — 지원·무시·미지원 표와 상한 표를 코드와 맞춘다(흐름도 표와 같은 규칙).
// excludeDays 는 안내서 지원 표의 제외 사례마다 「2026-01-05(월) + 5d」 막대의 기대 일수다(제외일 수만큼 끝이 밀린다):
// 주말 → 01-10·11 → 01-12(7일), 월·금 → 금 01-09 → 01-11(6일 — 월 01-12 는 밀린 끝 뒤), 01-06·07 → 01-12(7일).
var excludeDays = map[string]int{
	"excludes weekends":               7,
	"excludes monday, friday":         6,
	"excludes 2026-01-06, 2026-01-07": 7,
}

// ganttBarDays 는 「작업 + 2026-01-20 마일스톤(0d — 밀리지 않는다)」 그림에서 첫 막대의 길이(일)를 SVG 기하로 잰다.
// 범위는 01-05..01-20(15일)에 양쪽 1/40 여백이라 축 폭 620 이 15×1.05 일이다.
func ganttBarDays(t *testing.T, src string) int {
	t.Helper()
	r, err := gantt.Render(src)
	if err != nil {
		t.Fatalf("Render(%q): %v", src, err)
	}
	m := regexp.MustCompile(`<rect class="gt-bar" x="[\d.]+" y="[\d.]+" width="([\d.]+)"`).FindStringSubmatch(r.SVG)
	if m == nil {
		t.Fatalf("막대가 없음\n%s", r.SVG)
	}
	w, _ := strconv.ParseFloat(m[1], 64)
	return int(math.Round(w / 620 * 15 * 1.05))
}

func TestGanttGuide(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "spec", "diagrams.md"))
	if err != nil {
		t.Fatal(err)
	}
	g := string(b)
	section := func(name string) string {
		i, j := strings.Index(g, "<!-- "+name+":begin -->"), strings.Index(g, "<!-- "+name+":end -->")
		if i < 0 || j < i {
			t.Fatalf("안내서에 %s 표지가 없음", name)
		}
		return g[i:j]
	}
	want := map[string]int{
		"작업 수": gantt.MaxTasks, "구역 수": gantt.MaxSections, "날짜 범위(년)": gantt.MaxYears, "눈금 수": gantt.MaxTicks,
	}
	got := map[string]int{}
	for _, m := range regexp.MustCompile(`(?m)^\| ([^|]+?) \| (\d+) \|`).FindAllStringSubmatch(section("gantt-limits"), -1) {
		n, _ := strconv.Atoi(m[2])
		got[m[1]] = n
	}
	if len(got) != len(want) {
		t.Errorf("간트 상한 표 %v, 코드 %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("간트 상한 %q: 안내서 %d, 코드 %d", k, got[k], v)
		}
	}

	// codes 는 표의 「표기」열(헤더 이름으로 찾는다)에서 사례를 뽑는다: 칸을 「 · 」로 나눈 조각마다 백틱 코드가 있으면
	// 그 코드들, 없으면 조각 글 그대로(「같은 id 두 번」 같은 서술 사례). rows 는 표의 행 수다.
	codes := func(sec string) (out []string, rows int) {
		re := regexp.MustCompile("``(.+?)``|`([^`]+)`")
		split := regexp.MustCompile(`[^\\]\|`)
		col := -1
		for _, line := range strings.Split(section(sec), "\n") {
			if !strings.HasPrefix(line, "| ") {
				continue
			}
			cells := split.Split(line[1:], -1)
			if col < 0 { // 헤더 줄
				for k, c := range cells {
					if strings.TrimSpace(c) == "표기" {
						col = k
					}
				}
				if col < 0 {
					t.Fatalf("%s 표에 「표기」 열이 없음: %s", sec, line)
				}
				continue
			}
			rows++
			for _, part := range strings.Split(cells[col], " · ") {
				ms := re.FindAllStringSubmatch(part, -1)
				if len(ms) == 0 {
					if p := strings.TrimSpace(part); p != "" {
						out = append(out, p)
					}
					continue
				}
				for _, m := range ms {
					out = append(out, strings.TrimSpace(strings.ReplaceAll(m[1]+m[2], `\|`, "|")))
				}
			}
		}
		return out, rows
	}
	block := func(body string) string { return "```mermaid\n" + body + "\n```" }
	// 간트 골격 — 날짜 꼴·기준 작업(after·until 참조용 id)
	const ghead = "gantt\ndateFormat YYYY-MM-DD\n기준1 :a1, 2026-01-05, 3d\n기준2 :a2, 2026-01-05, 1d\n"
	endVal := regexp.MustCompile(`^(?:\d{4}-\d{2}-\d{2}|\d+(?:ms|[yMwdhms])|until a1)$`)
	// ran 은 표마다 실제로 실행한 사례, mustHave 는 표에 있어야 하는 주요 표기(앞부분 일치)
	check := func(sec string, ran []string, rows int, mustHave ...string) {
		if len(ran) < rows {
			t.Errorf("%s: 실행한 사례 %d 개 < 표의 행 %d 개", sec, len(ran), rows)
		}
		for _, w := range mustHave {
			ok := false
			for _, c := range ran {
				if strings.HasPrefix(c, w) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s: 표기 %q 를 시험하지 않음(실행 %v)", sec, w, ran)
			}
		}
	}

	sup, rows := codes("gantt-support")
	var ran []string
	for _, c := range sup {
		src := ghead + c + "\n작업 :2026-01-01, 1d"
		switch {
		case c == "gantt":
			src = "gantt\ndateFormat YYYY-MM-DD\n작업 :2026-01-01, 1d"
		case strings.HasPrefix(c, "dateFormat"): // 날짜 꼴마다 작업 날짜도 그 꼴로
			date := map[string]string{
				"dateFormat YYYY-MM-DD": "2026-01-05", "dateFormat YYYY-MM-DD HH:mm": "2026-01-05 09:30",
				"dateFormat YYYY/MM/DD": "2026/01/05", "dateFormat YYYY.MM.DD": "2026.01.05",
				"dateFormat YYYY-MM": "2026-01", "dateFormat YYYY": "2026",
			}[c]
			if date == "" {
				t.Errorf("간트 지원 표의 날짜 꼴 %q 에 시험 날짜가 없음", c)
				continue
			}
			src = "gantt\n" + c + "\n작업 :" + date + ", 1d"
		case strings.HasPrefix(c, "준비 :a1"): // 작업 id 예시 — 기준 작업 id(a1) 와 겹치지 않게 골격을 바꾼다
			src = "gantt\n먼저 :2026-01-01, 1d\n" + c + "\n나중 :after a1, 1d"
		case c == "done" || c == "active" || c == "crit" || c == "milestone":
			src = ghead + "작업 :" + c + ", 2026-01-01, 1d"
		case c == "2026-01-05" || c == "after a1" || c == "after a1 a2": // 시작 자리
			src = ghead + "작업 :" + c + ", 2d"
		case endVal.MatchString(c): // 끝 자리(날짜·기간·until)
			src = ghead + "작업 :2026-01-01, " + c
		case len(c) > 1 && c[0] == '%' && c[1] != '%': // axisFormat 지시자
			src = "gantt\naxisFormat " + c + "\n작업 :2026-01-01, 2d"
		case c == "accDescr { … }":
			src = ghead + "accDescr {\n설명\n}\n작업 :2026-01-01, 1d"
		}
		if strings.HasPrefix(c, "excludes ") { // 제외는 실제 끝 날짜까지 본다(리뷰 2회차 [1] — 요일만 적은 줄이 무효였다)
			days, ok := excludeDays[c]
			if !ok {
				t.Errorf("간트 지원 표의 제외 %q 에 기대 끝 날짜가 없음", c)
				continue
			}
			src = "gantt\ndateFormat YYYY-MM-DD\n" + c + "\n작업 :2026-01-05, 5d\n표 :milestone, 2026-01-20, 0d"
			if got := ganttBarDays(t, src); got != days {
				t.Errorf("간트 제외 %q: 2026-01-05 + 5d 의 막대 %d 일, 기대 %d 일", c, got, days)
			}
		}
		r := render(t, block(src))
		if len(r.Warnings) > 0 || !strings.Contains(r.HTML, "<svg") {
			t.Errorf("간트 지원 문법 %q 를 그리지 못함: %+v", c, r.Warnings)
		}
		if strings.HasPrefix(c, "excludes ") && !strings.Contains(r.HTML, `class="gt-excl"`) {
			t.Errorf("간트 제외 %q: 제외 띠(gt-excl)가 없음", c)
		}
		ran = append(ran, c)
	}
	check("gantt-support", ran, rows, "gantt", "title", "dateFormat YYYY-MM", "axisFormat", "%m월", "tickInterval", "section",
		"준비 :", "milestone", "after a1", "until a1", "excludes weekends", "excludes monday", "inclusiveEndDates", "accDescr {")
	if len(ran) < 40 {
		t.Errorf("간트 지원 문법 표에서 표기 %d 개만 읽음", len(ran))
	}

	ign, rows := codes("gantt-ignored")
	ran = nil
	for _, c := range ign {
		src := ghead + c + "\n작업 :2026-01-01, 1d"
		want := 1
		switch {
		case c == "todayMarker off":
			want = 0
		case c == "---" || c == "title": // config: 사례의 문맥(frontmatter 표지·남는 키) — config: 가 함께 시험한다
			continue
		case c == "config:":
			src = "---\nconfig:\n  theme: dark\n---\ngantt\n작업 :2026-01-01, 1d"
		case strings.HasPrefix(c, "%%{"):
			src = c + "\ngantt\n작업 :2026-01-01, 1d"
		case strings.HasPrefix(c, "todayMarker") || c == "topAxis" || strings.HasPrefix(c, "weekday") || strings.HasPrefix(c, "click"):
		default:
			t.Errorf("간트 무시 표의 표기 %q 에 시험 사례가 없음", c)
			continue
		}
		r := render(t, block(src))
		if len(r.Warnings) != want || !strings.Contains(r.HTML, "<svg") {
			t.Errorf("간트 쓰지 않는 줄 %q: 경고 %+v(기대 %d개), 그림 %v", c, r.Warnings, want, strings.Contains(r.HTML, "<svg"))
		}
		ran = append(ran, c)
	}
	check("gantt-ignored", ran, rows, "todayMarker off", "todayMarker stroke", "topAxis", "weekday", "click", "%%{init", "config:")

	uns, rows := codes("gantt-unsupported")
	ran = nil
	for _, c := range uns {
		var src string
		switch {
		case c == "vert":
			continue // 같은 행의 예시(준비 :vert, …)가 시험한다
		case strings.Contains(c, "%%{"): // 설정 지시 경계(④-1 후속) — 그 줄 뒤에 그릴 수 있는 작업을 둔다
			src = "gantt\n" + c + "\n작업 :2026-01-01, 1d"
		case c == "accDescr {": // 블록 안의 글 없는 주석(④-1 후속 수정 2) — 블록 밖 맨 %% 는 주석이라 그린다
			src = "gantt\naccDescr {\n%%\n}\n작업 :2026-01-01, 1d"
		case c == "%%" || c == "}":
			continue // 같은 행의 accDescr { 사례가 시험한다
		case strings.HasPrefix(c, "준비 :vert"):
			src = ghead + c
		case c == "includes weekends" || c == "weekend friday" || c == "axisFormat %a",
			c == "axisFormat %m#%d" || c == "axisFormat %m;%d" || c == "axisFormat %m#35;%d": // axisFormat 의 #·;(2026-10-31 결정)
			src = ghead + c + "\n작업 :2026-01-01, 1d"
		case c == "excludes 2026-01-06":
			continue // 같은 행의 시각 꼴 예시가 시험한다
		case strings.HasPrefix(c, "dateFormat YYYY-MM-DD HH:mm"):
			src = "gantt\n" + c + "\nexcludes 2026-01-06\n작업 :2026-01-05 09:00, 1d"
		case c == "excludes 공휴일" || c == "excludes ,": // 모르는 낱말·쉼표만(수정 4회차)
			src = "gantt\n" + c + "\n작업 :2026-01-05, 5d"
		case c == "dateFormat YYYY-MM-DD": // 같은 설정 줄이 둘(수정 4회차) — 지원 표와 같은 코드지만 이 행이 시험한다
			src = "gantt\n" + c + "\n" + c + "\n작업 :2026-01-05, 1d"
		case c == "2026/01/01":
			src = ghead + "작업 :2026/01/01, 1d"
		case c == "after x9" || c == "until x9":
			src = ghead + "작업 :2026-01-01, " + c
			if strings.HasPrefix(c, "after") {
				src = ghead + "작업 :" + c + ", 1d"
			}
		case c == "같은 id 두 번":
			src = ghead + "다시 :a1, 2026-01-01, 1d"
		case c == "끝이 시작보다 앞":
			src = ghead + "작업 :2026-01-10, 2026-01-01"
		case c == "1.5d":
			src = ghead + "작업 :2026-01-01, 1.5d"
		case c == "2026-+1-01":
			src = ghead + "작업 :2026-+1-01, 1d"
		case c == "5124096h":
			src = ghead + "작업 :2026-01-01, 5124096h"
		case c == "tickInterval 1001day":
			src = ghead + c + "\n작업 :2026-01-01, 1d"
		case c == "준비 :5d":
			src = "gantt\n" + c
		case c == "날짜 범위 100년 초과":
			src = "gantt\n먼저 :2026-01-01, 1d\n뒤 :2130-06-01, 1d"
		case c == "상한 초과":
			var b2 strings.Builder
			b2.WriteString("gantt\n")
			for i := 0; i <= gantt.MaxTasks; i++ {
				b2.WriteString("작업 :2026-01-01, 1d\n")
			}
			src = b2.String()
		case c == "축 라벨이 겹침":
			src = "gantt\naxisFormat " + strings.Repeat("가", 100) + " %Y\n작업 :1900-01-01, 100y"
		case c == "알 수 없는 문장":
			src = ghead + "이것은 문장"
		case c == "준비 #copy; :2026-01-01, 1d" || c == "두  칸 :2026-01-01, 1d": // 엔티티·공백 정책 B(itda-hyve#409 ④-2 후속)
			src = ghead + c
		case c == "accTitle: a&b", c == "accTitle:", c == "accDescr:", c == "accDescr { 설명 } 뒤":
			src = ghead + c + "\n작업 :2026-01-01, 1d"
		default:
			t.Errorf("간트 미지원 표의 표기 %q 에 시험 사례가 없음", c)
			continue
		}
		r := render(t, block(src))
		if len(r.Warnings) != 1 || strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "language-mermaid") {
			t.Errorf("간트 미지원 %q 가 원문 + 경고가 아님: %+v", c, r.Warnings)
		}
		ran = append(ran, c)
	}
	check("gantt-unsupported", ran, rows, "준비 :vert", "includes", "weekend friday", "2026/01/01", "axisFormat %a",
		"after x9", "until x9", "같은 id 두 번", "1.5d", "준비 :5d", "날짜 범위", "상한 초과", "축 라벨이 겹침",
		"excludes 공휴일", "excludes ,", "dateFormat YYYY-MM-DD",
		"%%{init: {", "%%{init: {}}%% 작업", "accTitle: 앞 %%{", "작업 %%{", "%% 메모 %%{", "accDescr {", // 설정 지시 경계·블록 안의 글 없는 주석(④-1 후속)
		"준비 #copy;", "두  칸", "accTitle: a&b", // 엔티티·공백 정책 B(itda-hyve#409 ④-2 후속)
		"axisFormat %m#%d", "axisFormat %m;%d", "axisFormat %m#35;%d", "accTitle:", "accDescr:", "accDescr { 설명 } 뒤")
	// 8장 예제
	i := strings.Index(g, "<!-- gantt-example -->")
	ex := g[i:]
	ex = ex[strings.Index(ex, "```mermaid"):]
	ex = ex[:strings.Index(ex, "\n````")]
	r := render(t, ex)
	if len(r.Warnings) > 0 || !strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "<figcaption>신규 서비스 출시 일정</figcaption>") {
		t.Errorf("8장 예제: %+v\n%s", r.Warnings, r.HTML)
	}
}

// 9장 타임라인 — 지원·무시·미지원 표와 상한 표를 코드와 맞춘다(흐름도·간트 표와 같은 규칙).
func TestTimelineGuide(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "spec", "diagrams.md"))
	if err != nil {
		t.Fatal(err)
	}
	g := string(b)
	tlSection := func(name string) string {
		i, j := strings.Index(g, "<!-- "+name+":begin -->"), strings.Index(g, "<!-- "+name+":end -->")
		if i < 0 || j < i {
			t.Fatalf("안내서에 %s 표지가 없음", name)
		}
		return g[i:j]
	}
	want := map[string]int{
		"시기 수": timeline.MaxPeriods, "사건 합계": timeline.MaxEvents,
		"시기 하나의 사건 수": timeline.MaxEventsPerPeriod, "구역 수": timeline.MaxSections,
	}
	got := map[string]int{}
	for _, m := range regexp.MustCompile(`(?m)^\| ([^|]+?) \| (\d+) \|`).FindAllStringSubmatch(tlSection("timeline-limits"), -1) {
		n, _ := strconv.Atoi(m[2])
		got[m[1]] = n
	}
	if len(got) != len(want) {
		t.Errorf("타임라인 상한 표 %v, 코드 %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("타임라인 상한 %q: 안내서 %d, 코드 %d", k, got[k], v)
		}
	}

	// tlCodes 는 표의 「표기」열(헤더 이름으로 찾는다)에서 사례를 뽑는다(TestGanttGuide 의 codes 와 같은 규칙).
	tlCodes := func(sec string) (out []string, rows int) {
		re := regexp.MustCompile("``(.+?)``|`([^`]+)`")
		split := regexp.MustCompile(`[^\\]\|`)
		col := -1
		for _, line := range strings.Split(tlSection(sec), "\n") {
			if !strings.HasPrefix(line, "| ") {
				continue
			}
			cells := split.Split(line[1:], -1)
			if col < 0 {
				for k, c := range cells {
					if strings.TrimSpace(c) == "표기" {
						col = k
					}
				}
				if col < 0 {
					t.Fatalf("%s 표에 「표기」 열이 없음: %s", sec, line)
				}
				continue
			}
			rows++
			for _, part := range strings.Split(cells[col], " · ") {
				ms := re.FindAllStringSubmatch(part, -1)
				if len(ms) == 0 {
					if p := strings.TrimSpace(part); p != "" {
						out = append(out, p)
					}
					continue
				}
				for _, m := range ms {
					out = append(out, strings.TrimSpace(strings.ReplaceAll(m[1]+m[2], `\|`, "|")))
				}
			}
		}
		return out, rows
	}
	block := func(body string) string { return "```mermaid\n" + body + "\n```" }
	const head = "timeline\n2026 : 사건\n" // 골격 — 사례를 이 줄 뒤에 얹는다(머리·title 사례는 자기 문맥)
	check := func(sec string, ran []string, rows int, mustHave ...string) {
		if len(ran) < rows {
			t.Errorf("%s: 실행한 사례 %d 개 < 표의 행 %d 개", sec, len(ran), rows)
		}
		for _, w := range mustHave {
			ok := false
			for _, c := range ran {
				if strings.HasPrefix(c, w) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s: 표기 %q 를 시험하지 않음(실행 %v)", sec, w, ran)
			}
		}
	}

	sup, rows := tlCodes("timeline-support")
	var ran []string
	for _, c := range sup {
		src := head + c
		switch {
		case c == "timeline" || c == "timeline LR":
			src = c + "\n2026 : 사건"
		case strings.HasPrefix(c, "title"):
			src = "timeline\n" + c + "\n2026 : 사건"
		case strings.HasPrefix(c, "section"):
			src = "timeline\n" + c + "\n2026 : 사건"
		case c == ": 사건":
			src = head + ": 이어 쓰기"
		case c == "%% 메모":
			src = "timeline\n" + c + "\n2026 : 사건"
		case c == "accTitle: 그림 이름" || c == "accDescr: 설명":
			src = "timeline\n" + c + "\n2026 : 사건"
		case c == "accDescr { … }":
			src = "timeline\naccDescr {\n설명\n}\n2026 : 사건"
		case strings.Contains(c, "<br>"):
			src = "timeline\n2026 : " + c
		case strings.Contains(c, "#"): // 사건 안의 # — 사건 자리에 둔다(지원)
			src = "timeline\n2026 : " + c
		}
		r := render(t, block(src))
		if len(r.Warnings) > 0 || !strings.Contains(r.HTML, "<svg") {
			t.Errorf("타임라인 지원 문법 %q 를 그리지 못함: %+v", c, r.Warnings)
		}
		ran = append(ran, c)
	}
	check("timeline-support", ran, rows, "timeline LR", "title", "section", "2026 :", ": 사건", "첫째 줄<br>", "단계 #35;1", "accDescr {")
	if len(ran) < 10 {
		t.Errorf("타임라인 지원 문법 표에서 표기 %d 개만 읽음", len(ran))
	}

	ign, rows := tlCodes("timeline-ignored")
	ran = nil
	for _, c := range ign {
		src := head + c
		switch {
		case c == "---" || c == "title":
			continue
		case c == "config:":
			src = "---\nconfig:\n  disableMulticolor: true\n---\ntimeline\n2026 : 사건"
		case strings.HasPrefix(c, "%%{"):
			src = c + "\ntimeline\n2026 : 사건"
		}
		r := render(t, block(src))
		if len(r.Warnings) != 1 || !strings.Contains(r.HTML, "<svg") {
			t.Errorf("타임라인 쓰지 않는 줄 %q: 경고 %+v, 그림 %v", c, r.Warnings, strings.Contains(r.HTML, "<svg"))
		}
		ran = append(ran, c)
	}
	check("timeline-ignored", ran, rows, "%%{init", "config:")

	uns, rows := tlCodes("timeline-unsupported")
	ran = nil
	for _, c := range uns {
		var src string
		switch {
		case strings.Contains(c, "%%{"): // 설정 지시 경계(④-1 후속) — 그 줄 뒤에 그릴 수 있는 시기를 둔다
			src = "timeline\n" + c + "\n2027 : 사건"
		case c == "accDescr {": // 블록 안의 글 없는 주석(④-1 후속 수정 2) — 블록 밖 맨 %% 는 주석이라 그린다
			src = "timeline\naccDescr {\n%%\n}\n2027 : 사건"
		case c == "%%" || c == "}":
			continue // 같은 행의 accDescr { 사례가 시험한다
		case c == "timeline TD":
			src = c + "\n2026 : 사건"
		case c == "timeline RL":
			src = c + "\n2026 : 사건"
		case c == "2026 :사건":
			src = "timeline\n2026 :사건"
		case c == "2026 :  : 사건":
			src = "timeline\n2026 :  : 사건"
		case c == "2026 :":
			src = "timeline\n2026 :"
		case c == ": 사건":
			src = "timeline\n: 사건"
		case c == "section":
			src = "timeline\nsection\n2026 : 사건"
		case c == "section a:b":
			src = "timeline\nsection a:b\n2026 : 사건"
		case c == "title":
			src = "timeline\ntitle\n2026 : 사건"
		case c == "accTitle:":
			src = "timeline\naccTitle:\n2026 : 사건"
		case c == "# 메모":
			src = "timeline\n# 메모\n2026 : 사건"
		case c == "2026 : #copy;" || c == "2026 : 두  칸": // 엔티티·공백 정책 B(itda-hyve#409 ④-2 후속)
			src = "timeline\n" + c
		case c == "accTitle: a&b", c == "accDescr:", c == "accDescr { 설명 } 뒤":
			src = "timeline\n" + c + "\n2026 : 사건"
		case c == "2026 # 메모":
			src = "timeline\n2026 # 메모\n2027 : 사건"
		case c == "ﬂ°" || c == "¶ß": // 원문에 이미 있는 내부 표식 문자(리뷰 3회차 [2])
			src = "timeline\n시기 " + c + " B : 사건"
		case c == "상한 초과":
			var b2 strings.Builder
			b2.WriteString("timeline\n")
			for i := 0; i <= timeline.MaxPeriods; i++ {
				fmt.Fprintf(&b2, "%d : 사건\n", 2000+i)
			}
			src = b2.String()
		default:
			t.Errorf("타임라인 미지원 표의 표기 %q 에 시험 사례가 없음", c)
			continue
		}
		r := render(t, block(src))
		if len(r.Warnings) != 1 || strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "language-mermaid") {
			t.Errorf("타임라인 미지원 %q 가 원문 + 경고가 아님: %+v", c, r.Warnings)
		}
		ran = append(ran, c)
	}
	check("timeline-unsupported", ran, rows, "timeline TD", "timeline RL", "2026 :사건", "2026 :", ": 사건", "section a:b", "# 메모", "2026 # 메모", "ﬂ°", "¶ß", "상한 초과",
		"%%{init: {", "%%{init: {}}%% 2026", "accTitle: 앞 %%{", "2026 : 사건 %%{", "%% 메모 %%{", "accDescr {", // 설정 지시 경계·블록 안의 글 없는 주석(④-1 후속)
		"2026 : #copy;", "2026 : 두  칸", "accTitle: a&b", // 엔티티·공백 정책 B(itda-hyve#409 ④-2 후속)
		"accDescr:", "accDescr { 설명 } 뒤")
	// 9장 예제
	i := strings.Index(g, "<!-- timeline-example -->")
	ex := g[i:]
	ex = ex[strings.Index(ex, "```mermaid"):]
	ex = ex[:strings.Index(ex, "\n````")]
	r := render(t, ex)
	if len(r.Warnings) > 0 || !strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "<figcaption>회사 연혁</figcaption>") {
		t.Errorf("9장 예제: %+v\n%s", r.Warnings, r.HTML)
	}
}

// 10장 순서도 — 지원·무시·미지원 표와 상한 표를 코드와 맞춘다(흐름도·간트·타임라인 표와 같은 규칙).
func TestSequenceGuide(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "spec", "diagrams.md"))
	if err != nil {
		t.Fatal(err)
	}
	g := string(b)
	sqSection := func(name string) string {
		i, j := strings.Index(g, "<!-- "+name+":begin -->"), strings.Index(g, "<!-- "+name+":end -->")
		if i < 0 || j < i {
			t.Fatalf("안내서에 %s 표지가 없음", name)
		}
		return g[i:j]
	}
	want := map[string]int{
		"참여자 수": sequence.MaxActors, "메시지·메모 합계": sequence.MaxEvents, "묶음 수": sequence.MaxBlocks,
		"묶음 깊이": sequence.MaxDepth, "box 수": sequence.MaxBoxes, "autonumber 수·번호 자릿수": sequence.MaxNumDigits,
	}
	got := map[string]int{}
	for _, m := range regexp.MustCompile(`(?m)^\| ([^|]+?) \| (\d+) \|`).FindAllStringSubmatch(sqSection("sequence-limits"), -1) {
		n, _ := strconv.Atoi(m[2])
		got[m[1]] = n
	}
	if len(got) != len(want) {
		t.Errorf("순서도 상한 표 %v, 코드 %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("순서도 상한 %q: 안내서 %d, 코드 %d", k, got[k], v)
		}
	}

	// sqCodes 는 표의 「표기」열에서 사례를 뽑는다(TestTimelineGuide 의 tlCodes 와 같은 규칙).
	sqCodes := func(sec string) (out []string, rows int) {
		re := regexp.MustCompile("``(.+?)``|`([^`]+)`")
		split := regexp.MustCompile(`[^\\]\|`)
		col := -1
		for _, line := range strings.Split(sqSection(sec), "\n") {
			if !strings.HasPrefix(line, "| ") {
				continue
			}
			cells := split.Split(line[1:], -1)
			if col < 0 {
				for k, c := range cells {
					if strings.TrimSpace(c) == "표기" {
						col = k
					}
				}
				if col < 0 {
					t.Fatalf("%s 표에 「표기」 열이 없음: %s", sec, line)
				}
				continue
			}
			rows++
			for _, part := range strings.Split(cells[col], " · ") {
				ms := re.FindAllStringSubmatch(part, -1)
				if len(ms) == 0 {
					if p := strings.TrimSpace(part); p != "" {
						out = append(out, p)
					}
					continue
				}
				for _, m := range ms {
					out = append(out, strings.TrimSpace(strings.ReplaceAll(m[1]+m[2], `\|`, "|")))
				}
			}
		}
		return out, rows
	}
	block := func(body string) string { return "```mermaid\n" + body + "\n```" }
	const head = "sequenceDiagram\nU->>S: 기준\n" // 골격 — 사례를 이 줄 뒤에 얹는다(머리·참여자·제목 사례는 자기 문맥)
	check := func(sec string, ran []string, rows int, mustHave ...string) {
		if len(ran) < rows {
			t.Errorf("%s: 실행한 사례 %d 개 < 표의 행 %d 개", sec, len(ran), rows)
		}
		for _, w := range mustHave {
			ok := false
			for _, c := range ran {
				if strings.HasPrefix(c, w) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s: 표기 %q 를 시험하지 않음(실행 %v)", sec, w, ran)
			}
		}
	}

	sup, rows := sqCodes("sequence-support")
	var ran []string
	for _, c := range sup {
		src := head + c
		warn := 0
		switch {
		case c == "sequenceDiagram":
			src = c + "\nU->>S: 기준"
		case strings.HasPrefix(c, "participant") || strings.HasPrefix(c, "actor"):
			src = head + c
		case c == "B-->>-A: 글": // 보낸이 종료 — 먼저 활성을 연다
			src = head + "A->>+B: 열기\n" + c
		case c == "activate A":
			src = head + "A->>A: 만들기\n" + c
		case c == "deactivate A": // 짝 — 먼저 참여자를 만들고 활성을 연다
			src = head + "A->>A: 만들기\nactivate A\n" + c
		case strings.HasPrefix(c, "loop ") || strings.HasPrefix(c, "opt ") || strings.HasPrefix(c, "break ") ||
			strings.HasPrefix(c, "alt ") || strings.HasPrefix(c, "par ") || strings.HasPrefix(c, "critical "):
			src = head + c + "\nU->>S: 안\nend"
		case c == "else 둘째":
			src = head + "alt 첫째\nU->>S: 안\n" + c + "\nend"
		case c == "and 또한":
			src = head + "par 첫째\nU->>S: 안\n" + c + "\nend"
		case c == "option 대안":
			src = head + "critical 첫째\nU->>S: 안\n" + c + "\nend"
		case strings.HasPrefix(c, "rect"):
			src = head + c + "\nU->>S: 안\nend"
			warn = 1 // 색 값은 쓰지 않음(무시 표와 같은 경고)
		case strings.HasPrefix(c, "box"):
			src = "sequenceDiagram\n" + c + "\nparticipant A\nend\nU->>S: 기준"
		case c == "title 제목" || c == "title: 제목" || c == "%% 메모" ||
			c == "accTitle: 그림 이름" || c == "accDescr: 설명":
			src = "sequenceDiagram\n" + c + "\nU->>S: 기준"
		case c == "accDescr { … }":
			src = "sequenceDiagram\naccDescr {\n설명\n}\nU->>S: 기준"
		case strings.Contains(c, "#") || strings.Contains(c, "<br>"): // 메시지 글 자리
			src = "sequenceDiagram\nU->>S: " + c
		}
		r := render(t, block(src))
		if len(r.Warnings) != warn || !strings.Contains(r.HTML, "<svg") {
			t.Errorf("순서도 지원 문법 %q 를 그리지 못함: %+v(경고 기대 %d개)", c, r.Warnings, warn)
		}
		ran = append(ran, c)
	}
	check("sequence-support", ran, rows, "sequenceDiagram", "participant A as", "actor B as", "A->>B: 글", "A-->>B: 글",
		"A->B: 글", "A-xB: 글", "A-)B: 글", "A<<->>B: 글", "A->>A: 글", "A->>+B: 글", "B-->>-A: 글", "Note over A,B: 글",
		"loop 매번", "else 둘째", "and 또한", "option 대안", "rect rgb", "autonumber 10 5", "box 이름", "title: 제목", "비율 #35;")

	ign, rows := sqCodes("sequence-ignored")
	ran = nil
	for _, c := range ign {
		src := head + c
		warn := 1
		switch {
		case c == "---" || c == "title": // 같은 칸의 config: 사례가 본다
			continue
		case c == "config:":
			src = "---\nconfig:\n  theme: dark\n---\nsequenceDiagram\nU->>S: 기준"
			src = "---\nconfig:\n  theme: dark\n---\nsequenceDiagram\nU->>S: 기준"
		case strings.HasPrefix(c, "%%{"):
			src = c + "\nsequenceDiagram\nU->>S: 기준"
		case strings.HasPrefix(c, "rect"):
			src = head + c + "\nU->>S: 안\nend"
		case strings.HasPrefix(c, "box"):
			src = "sequenceDiagram\n" + c + "\nparticipant A\nend\nU->>S: 기준"
		case c == "nowrap:" || c == "wrap:" || c == ":wrap:":
			src = "sequenceDiagram\nU->>S: " + c + " 글"
		case strings.HasPrefix(c, "participant"):
			src = "sequenceDiagram\n" + c + "\nU->>S: 기준"
		case strings.HasPrefix(c, "loop"):
			src = "sequenceDiagram\n" + c + "\nU->>S: 안\nend"
		case strings.HasPrefix(c, "else"):
			src = "sequenceDiagram\nalt 갈림\nU->>S: 안\n" + c + "\nend"
		case strings.HasPrefix(c, "Note"):
			src = "sequenceDiagram\nU->>S: 기준\n" + c
		case c == "links" || c == "properties" || c == "details":
			src = head + c + " A: {\"k\": \"v\"}"
		}
		r := render(t, block(src))
		if len(r.Warnings) != warn || !strings.Contains(r.HTML, "<svg") {
			t.Errorf("순서도 쓰지 않는 줄 %q: 경고 %+v(기대 %d개), 그림 %v", c, r.Warnings, warn, strings.Contains(r.HTML, "<svg"))
		}
		ran = append(ran, c)
	}
	check("sequence-ignored", ran, rows, "%%{init", "config:", "rect rgb", "box grey", "link A") // wrap 은 미지원 표로 옮겼다(itda-hyve#208)

	uns, rows := sqCodes("sequence-unsupported")
	ran = nil
	for _, c := range uns {
		var src string
		switch {
		case c == "else" || c == "alt" || c == "and" || c == "par" || c == "option" || c == "critical":
			src = "sequenceDiagram\n" + c + " 머리\nU->>S: x" // else/and/option 는 자리 오류, 나머지는 닫히지 않음
		case c == "end" || c == "end 뒤에 글":
			src = "sequenceDiagram\nend 뒤에 글"
		case c == "box b":
			src = "sequenceDiagram\nbox b\nU->>S: x\nend"
		case c == "ﬂ°" || c == "¶ß":
			src = "sequenceDiagram\nU->>S: x " + c + " y"
		case c == "상한 초과":
			var b2 strings.Builder
			b2.WriteString("sequenceDiagram\n")
			for i := 0; i <= sequence.MaxActors; i++ {
				fmt.Fprintf(&b2, "P%d->>P%d: x\n", i, i)
			}
			src = b2.String()
		case c == "단계가 커서 번호가 7 자리가 되는 그림":
			src = "sequenceDiagram\nautonumber 999999 999999\nA->>A: x\nA->>A: y"
		case c == "autonumber 10;":
			src = "sequenceDiagram\nautonumber 10; U->>S: x" // ; 로 이은 문장
		case c == "A->>end: 글" || c == "Note over loop: 글":
			src = "sequenceDiagram\n" + c // 참조 자리 예약어
		case c == "wrap:" || c == "nowrap:" || c == ":wrap:":
			src = "sequenceDiagram\nU->>S: " + c + " 글"
		case c == "accDescr {": // 블록 안의 글 없는 주석(④-1 후속 수정 2) — 블록 밖 맨 %% 는 주석이라 그린다
			src = "sequenceDiagram\naccDescr {\n%%\n}\nU->>S: x"
		case c == "%%" || c == "}":
			continue // 같은 행의 accDescr { 사례가 시험한다
		case c == "accTitle:" || c == "accDescr:":
			src = "sequenceDiagram\n" + c + "\nU->>S: x"
		default:
			src = "sequenceDiagram\n" + c
		}
		r := render(t, block(src))
		if len(r.Warnings) != 1 || strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "language-mermaid") {
			t.Errorf("순서도 미지원 %q 가 원문 + 경고가 아님: %+v", c, r.Warnings)
		}
		ran = append(ran, c)
	}
	check("sequence-unsupported", ran, rows, "A-\\B", "A->>()B", "create", "par_over", "participant A@{", "participant A:B", "participant A,B",
		"A->>B", "Note above", "else", "deactivate", "box b", "box rgb(bad)", "#", "title", "autonumber 10x", "autonumber 9999999",
		"단계가 커서", "ﬂ°", "상한 초과", "participant API Server", "A->>B: 첫째;", "participant end", "end가", "accDescr { 설명 }",
		"5->>B", "A%%->>B", "A->>-xB", "rect 무지개", "link A {", "%%{init: {", "autonumber 1.5", "title:제목",
		"%%{init: {}}%% A->>B", "accTitle: 앞 %%{", "A->>B: 글 %%{", "%% 메모 %%{", "accDescr {",
		"A->>B: #copy;", "A->>B: 두  칸", "accTitle: a&b", "accTitle:", "accDescr:") // 엔티티·공백 정책 B(itda-hyve#409 ④-2 후속) · 설정 지시 경계(er ④ 리뷰 1회차)·블록 안의 글 없는 주석(④-1 후속 수정 2)
	// 10장 예제
	i := strings.Index(g, "<!-- sequence-example -->")
	ex := g[i:]
	ex = ex[strings.Index(ex, "```mermaid"):]
	ex = ex[:strings.Index(ex, "\n````")]
	r := render(t, ex)
	if len(r.Warnings) != 0 || !strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "<figcaption>주문 결제 흐름</figcaption>") {
		t.Errorf("10장 예제: %+v\n%s", r.Warnings, r.HTML)
	}
}

// TestERGuide 는 11장(erDiagram)을 코드와 맞춘다 — 상한 표는 상수와, 지원 표의 표기는 경고 없이 그려지는지, 쓰지 않는 줄 표의
// 표기는 경고 하나와 함께 그려지는지, 미지원 표의 표기는 원문 + 경고인지, 11장 예제는 경고 없이 그려지는지(TestSequenceGuide 꼴).
func TestERGuide(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "spec", "diagrams.md"))
	if err != nil {
		t.Fatal(err)
	}
	g := string(b)
	section := func(name string) string {
		i, j := strings.Index(g, "<!-- "+name+":begin -->"), strings.Index(g, "<!-- "+name+":end -->")
		if i < 0 || j < i {
			t.Fatalf("안내서에 %s 표지가 없음", name)
		}
		return g[i:j]
	}
	want := map[string]int{"엔티티 수": er.MaxEntities, "관계 수": er.MaxRelations, "엔티티 하나의 속성": er.MaxAttrsPerEnt, "속성 합계": er.MaxAttrsTotal}
	got := map[string]int{}
	for _, m := range regexp.MustCompile(`(?m)^\| ([^|]+?) \| (\d+) \|`).FindAllStringSubmatch(section("er-limits"), -1) {
		n, _ := strconv.Atoi(m[2])
		got[m[1]] = n
	}
	if len(got) != len(want) {
		t.Errorf("ER 상한 표 %v, 코드 %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("ER 상한 %q: 안내서 %d, 코드 %d", k, got[k], v)
		}
	}

	// codes 는 표의 「표기」열에서 사례를 뽑는다(TestSequenceGuide 의 sqCodes 와 같은 규칙).
	codes := func(sec string) (out []string, rows int) {
		re := regexp.MustCompile("``(.+?)``|`([^`]+)`")
		split := regexp.MustCompile(`[^\\]\|`)
		col := -1
		for _, line := range strings.Split(section(sec), "\n") {
			if !strings.HasPrefix(line, "| ") {
				continue
			}
			cells := split.Split(line[1:], -1)
			if col < 0 {
				for k, c := range cells {
					if strings.TrimSpace(c) == "표기" {
						col = k
					}
				}
				if col < 0 {
					t.Fatalf("%s 표에 「표기」 열이 없음: %s", sec, line)
				}
				continue
			}
			rows++
			for _, part := range strings.Split(cells[col], " · ") {
				ms := re.FindAllStringSubmatch(part, -1)
				if len(ms) == 0 {
					if p := strings.TrimSpace(part); p != "" {
						out = append(out, p)
					}
					continue
				}
				for _, m := range ms {
					out = append(out, strings.TrimSpace(strings.ReplaceAll(m[1]+m[2], `\|`, "|")))
				}
			}
		}
		return out, rows
	}
	block := func(body string) string { return "```mermaid\n" + body + "\n```" }
	check := func(sec string, ran []string, rows int, mustHave ...string) {
		if len(ran) < rows {
			t.Errorf("%s: 실행한 사례 %d 개 < 표의 행 %d 개", sec, len(ran), rows)
		}
		for _, w := range mustHave {
			ok := false
			for _, c := range ran {
				if strings.HasPrefix(c, w) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s: 표기 %q 를 시험하지 않음(실행 %v)", sec, w, ran)
			}
		}
	}
	// isAttr 는 속성 줄 사례다(블록 안에 얹는다).
	isAttr := regexp.MustCompile(`^(?:string|int|decimal|list~|\*int)`).MatchString

	sup, rows := codes("er-support")
	var ran []string
	for _, c := range sup {
		src := "erDiagram\n" + c
		switch {
		case c == "erDiagram":
			src = c + "\nA"
		case c == "고객 {":
			src = "erDiagram\n" + c + "\nstring 이름\n}"
		case c == "}":
			src = "erDiagram\nA {\n" + c
		case isAttr(c):
			src = "erDiagram\nA {\n" + c + "\n}"
		case c == "accDescr { … }":
			src = "erDiagram\naccDescr {\n설명\n}\nA"
		case strings.HasPrefix(c, "%%") || strings.HasPrefix(c, "acc") || strings.HasPrefix(c, "direction"):
			src = "erDiagram\n" + c + "\nA"
		}
		r := render(t, block(src))
		if len(r.Warnings) != 0 || !strings.Contains(r.HTML, "<svg") {
			t.Errorf("ER 지원 문법 %q 를 그리지 못함: %+v", c, r.Warnings)
		}
		ran = append(ran, c)
	}
	check("er-support", ran, rows, "erDiagram", "고객[손님]", `고객["보이는 이름"]`, "고객 {}", "고객 {", "}", "int 번호 PK, FK", `string 메일 UK "`,
		"int? 나이", "A ||--o{ B", "A |o--o| B", "A }|--|{ B", "A }o..o{ B", `A ||--o{ B : ""`, "A ||--o{ A", "direction LR", "A[\"비율 #35;\"]",
		"%% 메모", "accTitle:", "accDescr:", "accDescr {")

	ign, rows := codes("er-ignored")
	ran = nil
	for _, c := range ign {
		src := "erDiagram\nA\n" + c
		switch {
		case c == "---" || c == "title": // 같은 칸의 config: 사례가 본다
			continue
		case c == "config:":
			src = "---\nconfig:\n  theme: dark\n---\nerDiagram\nA"
		case strings.HasPrefix(c, "%%{"):
			src = c + "\nerDiagram\nA"
		}
		r := render(t, block(src))
		if len(r.Warnings) != 1 || !strings.Contains(r.HTML, "<svg") {
			t.Errorf("ER 쓰지 않는 줄 %q: 경고 %+v(기대 1개), 그림 %v", c, r.Warnings, strings.Contains(r.HTML, "<svg"))
		}
		ran = append(ran, c)
	}
	check("er-ignored", ran, rows, "%%{init", "config:", "style A", "classDef", "class A", "A:::")

	uns, rows := codes("er-unsupported")
	ran = nil
	for _, c := range uns {
		src := "erDiagram\n" + c
		switch {
		case isAttr(c):
			src = "erDiagram\nA {\n" + c + "\n}"
		case c == "accTitle:" || c == "accDescr:" || c == "accDescr { 설명 } 뒤":
			src = "erDiagram\n" + c + "\nA"
		case c == "ERDIAGRAM":
			src = c + "\nA"
		case strings.HasPrefix(c, "%%{"):
			src = c + "\nerDiagram\nA"
		case c == "%%":
			src = "erDiagram\n%%\nA"
		case c == "ﬂ°" || c == "¶ß":
			src = "erDiagram\nA[\"x " + c + " y\"]"
		case strings.HasPrefix(c, "ASCII 밖"):
			src = "erDiagram\nA\u00a0||--o{ B : x"
		case c == "상한 초과":
			var b2 strings.Builder
			b2.WriteString("erDiagram\n")
			for i := 0; i <= er.MaxEntities; i++ {
				fmt.Fprintf(&b2, "E%d\n", i)
			}
			src = b2.String()
		}
		r := render(t, block(src))
		if len(r.Warnings) != 1 || strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "language-mermaid") {
			t.Errorf("ER 미지원 %q 가 원문 + 경고가 아님: %+v", c, r.Warnings)
		}
		ran = append(ran, c)
	}
	check("er-unsupported", ran, rows, "A one or more B", "A 1+ to many B", `"고객 정보" ||--o{`, "A { string a }", "A ||.-o{ B", "A ||-.o{ B",
		"A |O--o{ B", "A u--o{ B", "Token", "endpoint", "Classroom", "u-x", "A-", "A--B", "_a", "title", "PK", "A1 ||--o{ B : x;", "A ||--o{ B",
		"A ||--o{ B : x %%", `A["a*b"]`, `A["_기울임_"]`, `A["- 목록"]`, `A["1. 첫째"]`, `A["a<b"]`, `A[""]`, "string a pk", "string pk", "list~int~",
		"*int", `int 번호 PK "설명`, "A ||--o{ B : direction", "title 주문", "subgraph", "ERDIAGRAM", "A {", "}", "%%", "%%{init: {", "ﬂ°", "¶ß",
		"ASCII 밖", "𝔸", "상한 초과", "%%{init: {}}%% A %%{", "accTitle: 앞 %%{", "A %%{", "%% 메모 %%{",
		`A["#copy;"]`, `A["두  칸"]`, "accTitle: a&b", // 엔티티·공백 정책 B(itda-hyve#409 ④-2 후속)
		"accTitle:", "accDescr:", "accDescr { 설명 } 뒤")

	// 11장 예제
	i := strings.Index(g, "<!-- er-example -->")
	ex := g[i:]
	ex = ex[strings.Index(ex, "```mermaid"):]
	ex = ex[:strings.Index(ex, "\n````")]
	r := render(t, ex)
	if len(r.Warnings) != 0 || !strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "<figcaption>주문 모델</figcaption>") ||
		!strings.Contains(r.HTML, ">주문 항목<") || !strings.Contains(r.HTML, ">PK,FK<") {
		t.Errorf("11장 예제: %+v\n%s", r.Warnings, r.HTML)
	}
}

// TestStateGuide 는 12장(stateDiagram)을 코드와 맞춘다 — 상한 표는 상수와, 지원 표의 표기는 경고 없이 그려지는지, 쓰지 않는 줄
// 표의 표기는 경고 하나와 함께 그려지는지, 미지원 표의 표기는 원문 + 경고 하나인지, 12장 예제는 경고 없이 그려지는지(TestERGuide 꼴).
func TestStateGuide(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "spec", "diagrams.md"))
	if err != nil {
		t.Fatal(err)
	}
	g := string(b)
	section := func(name string) string {
		i, j := strings.Index(g, "<!-- "+name+":begin -->"), strings.Index(g, "<!-- "+name+":end -->")
		if i < 0 || j < i {
			t.Fatalf("안내서에 %s 표지가 없음", name)
		}
		return g[i:j]
	}
	want := map[string]int{"상태 수(시작·끝 원 포함)": statediagram.MaxStates, "전이 수": statediagram.MaxTransitions,
		"복합 상태 수": statediagram.MaxComposites, "상태 하나의 설명": statediagram.MaxDescrs, "이름 하나의 글자 수(상태·복합 상태)": statediagram.MaxLabel}
	got := map[string]int{}
	for _, m := range regexp.MustCompile(`(?m)^\| ([^|]+?) \| (\d+) \|`).FindAllStringSubmatch(section("state-limits"), -1) {
		n, _ := strconv.Atoi(m[2])
		got[m[1]] = n
	}
	if len(got) != len(want) {
		t.Errorf("상태도 상한 표 %v, 코드 %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("상태도 상한 %q: 안내서 %d, 코드 %d", k, got[k], v)
		}
	}

	// codes 는 표의 「표기」열에서 사례를 뽑는다(TestERGuide 와 같은 규칙).
	codes := func(sec string) (out []string, rows int) {
		re := regexp.MustCompile("``(.+?)``|`([^`]+)`")
		split := regexp.MustCompile(`[^\\]\|`)
		col := -1
		for _, line := range strings.Split(section(sec), "\n") {
			if !strings.HasPrefix(line, "| ") {
				continue
			}
			cells := split.Split(line[1:], -1)
			if col < 0 {
				for k, c := range cells {
					if strings.TrimSpace(c) == "표기" {
						col = k
					}
				}
				if col < 0 {
					t.Fatalf("%s 표에 「표기」 열이 없음: %s", sec, line)
				}
				continue
			}
			rows++
			for _, part := range strings.Split(cells[col], " · ") {
				ms := re.FindAllStringSubmatch(part, -1)
				if len(ms) == 0 {
					if p := strings.TrimSpace(part); p != "" {
						out = append(out, p)
					}
					continue
				}
				for _, m := range ms {
					out = append(out, strings.TrimSpace(strings.ReplaceAll(m[1]+m[2], `\|`, "|")))
				}
			}
		}
		return out, rows
	}
	block := func(body string) string { return "```mermaid\n" + body + "\n```" }
	check := func(sec string, ran []string, rows int, mustHave ...string) {
		if len(ran) < rows {
			t.Errorf("%s: 실행한 사례 %d 개 < 표의 행 %d 개", sec, len(ran), rows)
		}
		for _, w := range mustHave {
			ok := false
			for _, c := range ran {
				if strings.HasPrefix(c, w) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s: 표기 %q 를 시험하지 않음(실행 %v)", sec, w, ran)
			}
		}
	}

	sup, rows := codes("state-support")
	var ran []string
	for _, c := range sup {
		src := "stateDiagram-v2\n" + c
		switch {
		case strings.HasPrefix(c, "stateDiagram"):
			src = c + "\nA"
		case strings.HasSuffix(c, "{"):
			src = "stateDiagram-v2\n" + c + "\n출고\n}"
		case c == "}":
			src = "stateDiagram-v2\nstate 묶음 {\nA\n" + c
		case c == "accDescr { … }":
			src = "stateDiagram-v2\naccDescr {\n설명\n}\nA"
		case strings.HasPrefix(c, "%%") || strings.HasPrefix(c, "acc") || strings.HasPrefix(c, "direction"):
			src = "stateDiagram-v2\n" + c + "\nA"
		}
		r := render(t, block(src))
		if len(r.Warnings) != 0 || !strings.Contains(r.HTML, "<svg") {
			t.Errorf("상태도 지원 문법 %q 를 그리지 못함: %+v", c, r.Warnings)
		}
		ran = append(ran, c)
	}
	check("state-support", ran, rows, "stateDiagram", "stateDiagram-v2", "접수", "접수 : 주문을 받음", `state "결제 확인" as 결제`, "접수 --> 결제",
		"접수 --> 결제 : 확인", "[*] --> 접수", "완료 --> [*]", "접수 --> 접수", "state 갈림 <<choice>>", "state 갈래 <<fork>>", "state 합류 <<join>>",
		"state 배송 {", `state "배송 중" as 배송 {`, "}", "direction LR", "direction BT", "접수 : 비율 #35;", "accTitle: 번호 #35;1 #+65;", "%% 메모", "%%", "accTitle:", "accDescr:", "accDescr {")

	ign, rows := codes("state-ignored")
	ran = nil
	for _, c := range ign {
		src := "stateDiagram-v2\nA\n" + c
		switch {
		case c == "---" || c == "title": // 같은 칸의 config: 사례가 본다
			continue
		case c == "config:":
			src = "---\nconfig:\n  theme: dark\n---\nstateDiagram-v2\nA"
		case strings.HasPrefix(c, "%%{"):
			src = c + "\nstateDiagram-v2\nA"
		}
		r := render(t, block(src))
		if len(r.Warnings) != 1 || !strings.Contains(r.HTML, "<svg") {
			t.Errorf("상태도 쓰지 않는 줄 %q: 경고 %+v(기대 1개), 그림 %v", c, r.Warnings, strings.Contains(r.HTML, "<svg"))
		}
		ran = append(ran, c)
	}
	check("state-ignored", ran, rows, "%%{init", "config:", "style A", "classDef", "class A", "A:::", "A --> B:::", "click A href", `click A "`,
		"hide empty description")

	uns, rows := codes("state-unsupported")
	ran = nil
	for _, c := range uns {
		src := "stateDiagram-v2\n" + c
		switch {
		case c == "accTitle:" || c == "accDescr:" || c == "accDescr { 설명 } 뒤":
			src = "stateDiagram-v2\n" + c + "\nA"
		case c == "A --> 배송":
			src = "stateDiagram-v2\nstate 배송 {\n출고\n}\n" + c
		case c == "state 안 {":
			src = "stateDiagram-v2\nstate 바깥 {\n" + c + "\nA\n}\n}"
		case c == "--":
			src = "stateDiagram-v2\nstate 묶음 {\nA\n--\nB\n}"
		case c == "direction LR":
			src = "stateDiagram-v2\nstate 묶음 {\n" + c + "\nA\n}"
		case c == "state 갈림 <<choice>>":
			src = "stateDiagram-v2\n갈림 --> A\n" + c
		case c == "갈림 : 설명":
			src = "stateDiagram-v2\nstate 갈림 <<choice>>\n" + c
		case c == "출고":
			src = "stateDiagram-v2\nstate 가 {\n출고\n}\nstate 나 {\n출고\n}"
		case strings.HasPrefix(c, "classDef") || strings.HasPrefix(c, "style"): // 상태가 있어야 「그릴 상태가 없음」과 갈린다
			src = "stateDiagram-v2\nA\n" + c
		case strings.HasPrefix(c, "stateDiagram") || c == "STATEDIAGRAM":
			src = c + "\nA"
		case strings.HasPrefix(c, "%%{"):
			src = c + "\nstateDiagram-v2\nA"
		case c == "ﬂ°" || c == "¶ß":
			src = "stateDiagram-v2\nA : x " + c + " y"
		case strings.HasPrefix(c, "ASCII 밖"):
			src = "stateDiagram-v2\nA\u00a0--> B"
		case strings.HasPrefix(c, "SVG 가 담지 못하는"):
			src = "stateDiagram-v2\nA : x\ufffey"
		case strings.HasPrefix(c, `title: "`):
			src = "---\n" + c + "\n---\nstateDiagram-v2\nA"
		case c == "상한 초과":
			var b2 strings.Builder
			b2.WriteString("stateDiagram-v2\n")
			for i := 0; i <= statediagram.MaxStates; i++ {
				fmt.Fprintf(&b2, "S%d\n", i)
			}
			src = b2.String()
		}
		r := render(t, block(src))
		if len(r.Warnings) != 1 || strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "language-mermaid") {
			t.Errorf("상태도 미지원 %q 가 원문 + 경고가 아님: %+v", c, r.Warnings)
		}
		ran = append(ran, c)
	}
	check("state-unsupported", ran, rows, "note right of A", `note "`, "A --> 배송", "state 안 {", "--", "state 갈래 [[fork]]", "state 갈림 [[choice]]",
		"direction LR", "scale", "state 갈림 <<choice>>", "갈림 : 설명", "출고", "state A", `"결제 확인" --> A`, "A --> B --> C", "A --> B; B --> C", "A; B",
		"A -> B", "A --- B", "[*]", "[*] : 설명", "Clicked", "stateful", "Classroom", "Note2", "Hideout", "Default1", "_a", "a_", "as", "root", "x_start",
		"a_end", "1a", "a-b", "A : *강조*", "A : _기울임_", "A : - 목록", "A : 1. 첫째", "A : a<b", "A : [x]", "A : 값 # 메모", "A : 둘::셋", "A : 끝:",
		"A :", "A --> B :", "A --> B : 방향 direction", "classDef 강조", "style 가", "A : #copy;", "A --> B : #euro;", "A : #hellip;", "A : #AMP;",
		"A : #x41;", "A : #12345678;", "A : #1114111;", "A : #nbsp;", "A : x#160;#160;y", "A --> B : x#8195;y", "A : #32;", "A : #8203;",
		"A : 두  칸", "A --> B : 앞  뒤", "accTitle: a<b", "accDescr: 갑 & 을", "classDef default", "classDef DEFAULT", "stateDiagram-V2", "STATEDIAGRAM", "state 빈 {", "}", "%%{init: {",
		"A --> B %% 메모", "%%{init: {}}%% A", "accTitle: 앞 %%{", "ﬂ°", "¶ß", "ASCII 밖", "SVG 가 담지 못하는", `title: "x\uFFFEy"`, "상한 초과",
		`title: "두  칸"`, `title: "x\ty"`, "accTitle:", "accDescr:", "accDescr { 설명 } 뒤") // 캡션 정책 B 여섯 종류 통일(2026-10-31 결정)

	// 빈 복합 상태(표 밖 문장)
	if r := render(t, block("stateDiagram-v2\nstate 묶음 {\n}")); len(r.Warnings) != 1 || strings.Contains(r.HTML, "<svg") {
		t.Errorf("빈 복합 상태가 원문 + 경고가 아님: %+v", r.Warnings)
	}

	// 12장 예제
	i := strings.Index(g, "<!-- state-example -->")
	ex := g[i:]
	ex = ex[strings.Index(ex, "```mermaid"):]
	ex = ex[:strings.Index(ex, "\n````")]
	r := render(t, ex)
	if len(r.Warnings) != 0 || !strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "<figcaption>주문 처리 상태</figcaption>") ||
		!strings.Contains(r.HTML, ">배송 중<") || !strings.Contains(r.HTML, ">상자에 담기<") || !strings.Contains(r.HTML, `class="st-choice"`) {
		t.Errorf("12장 예제: %+v\n%s", r.Warnings, r.HTML)
	}
}

// TestClassGuide 는 13장(클래스도)의 표를 코드와 맞춘다 — 상한 표는 classdiag 상수와, 지원 표기는 경고 없이 그려지는지, 쓰지 않는 줄은
// 경고 하나에 그림을 그리는지, 미지원 표기는 원문 + 경고인지, 멤버 표의 「같다」 줄은 원본 표시 글을 그대로 그리고 「원문」 줄은
// 원문인지, 13장 예제는 경고 없이 그려지는지(TestStateGuide 와 같은 틀).
func TestClassGuide(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "spec", "diagrams.md"))
	if err != nil {
		t.Fatal(err)
	}
	g := string(b)
	section := func(name string) string {
		i, j := strings.Index(g, "<!-- "+name+":begin -->"), strings.Index(g, "<!-- "+name+":end -->")
		if i < 0 || j < i {
			t.Fatalf("안내서에 %s 표지가 없음", name)
		}
		return g[i:j]
	}
	want := map[string]int{"클래스 수": classdiag.MaxClasses, "관계 수": classdiag.MaxRelations, "클래스 하나의 멤버": classdiag.MaxMembers,
		"이름·멤버·라벨 하나의 글자 수": classdiag.MaxLabel, "양끝 수량 글 하나의 글자 수": classdiag.MaxCard}
	got := map[string]int{}
	for _, m := range regexp.MustCompile(`(?m)^\| ([^|]+?) \| (\d+) \|`).FindAllStringSubmatch(section("class-limits"), -1) {
		n, _ := strconv.Atoi(m[2])
		got[m[1]] = n
	}
	if len(got) != len(want) {
		t.Errorf("클래스도 상한 표 %v, 코드 %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("클래스도 상한 %q: 안내서 %d, 코드 %d", k, got[k], v)
		}
	}

	re := regexp.MustCompile("``(.+?)``|`([^`]+)`")
	split := regexp.MustCompile(`[^\\]\|`)
	// codes 는 표의 「표기」열에서 사례를 뽑는다(TestStateGuide 와 같은 규칙).
	codes := func(sec string) (out []string, rows int) {
		col := -1
		for _, line := range strings.Split(section(sec), "\n") {
			if !strings.HasPrefix(line, "| ") {
				continue
			}
			cells := split.Split(line[1:], -1)
			if col < 0 {
				for k, c := range cells {
					if strings.TrimSpace(c) == "표기" {
						col = k
					}
				}
				if col < 0 {
					t.Fatalf("%s 표에 「표기」 열이 없음: %s", sec, line)
				}
				continue
			}
			rows++
			for _, part := range strings.Split(cells[col], " · ") {
				ms := re.FindAllStringSubmatch(part, -1)
				if len(ms) == 0 {
					if p := strings.TrimSpace(part); p != "" {
						out = append(out, p)
					}
					continue
				}
				for _, m := range ms {
					out = append(out, strings.TrimSpace(strings.ReplaceAll(m[1]+m[2], `\|`, "|")))
				}
			}
		}
		return out, rows
	}
	block := func(body string) string { return "```mermaid\n" + body + "\n```" }
	check := func(sec string, ran []string, rows int, mustHave ...string) {
		if len(ran) < rows {
			t.Errorf("%s: 실행한 사례 %d 개 < 표의 행 %d 개", sec, len(ran), rows)
		}
		for _, w := range mustHave {
			ok := false
			for _, c := range ran {
				if strings.HasPrefix(c, w) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s: 표기 %q 를 시험하지 않음(실행 %v)", sec, w, ran)
			}
		}
	}
	const head = "classDiagram\n"

	sup, rows := codes("class-support")
	var ran []string
	for _, c := range sup {
		src := head + c
		switch {
		case strings.HasPrefix(c, "classDiagram"):
			src = c + "\nclass A"
		case c == "}":
			src = head + "class A {\n+x\n" + c
		case strings.HasSuffix(c, "{"):
			src = head + c + "\n+x\n}"
		case strings.ContainsRune("+-#~", rune(c[0])) || c == "<<interface>>": // 몸 안의 한 줄
			src = head + "class A {\n" + c + "\n}"
		case strings.HasPrefix(c, "<<"):
			src = head + "class 결제수단\n" + c
		case c == "accDescr { … }":
			src = head + "accDescr {\n설명\n}\nclass A"
		case strings.HasPrefix(c, "%%") || strings.HasPrefix(c, "acc") || strings.HasPrefix(c, "direction"):
			src = head + c + "\nclass A"
		}
		r := render(t, block(src))
		if len(r.Warnings) != 0 || !strings.Contains(r.HTML, "<svg") {
			t.Errorf("클래스도 지원 문법 %q 를 그리지 못함: %+v", c, r.Warnings)
		}
		ran = append(ran, c)
	}
	check("class-support", ran, rows, "classDiagram", "classDiagram-v2", "class 주문", "class 주문 {", "}", "class 주문 {}", "class 주문 { +번호 int }",
		"주문 : +번호 int", "+번호 int", "+합계() decimal", "+새주문()$", "+그리기()*", "#내부값", "~찾기", "class 결제수단 <<interface>>",
		"<<interface>> 결제수단", "<<interface>>", "동물 <|-- 개", "주문 *-- 항목", "주문 o-- 상품", "고객 --> 주문", "가 -- 나", "주문 ..> 결제",
		"카드 ..|> 결제", "가 .. 나", "가 <--> 나", "고객 --> 주문 : 주문한다", `고객 "1" --> "*" 주문`, `주문 "1" o-- "0..*" 상품 : 담는다`,
		"노드 --> 노드", "direction LR", "direction BT", "가 --> 나 : 비율 #35;", "주문 : +번호 #35;1", "accTitle: 번호 #35;1 #+65;", "%% 메모", "%%",
		"accTitle:", "accDescr:", "accDescr {", "class 목록~T~", "class 사전~K, V~ {", "목록~T~ <|-- 배열", "목록 : +항목 List~T~",
		"+찾기(키 List~K~) Map~K, V~", "+행렬 Array~Array~int~~")

	ign, rows := codes("class-ignored")
	ran = nil
	for _, c := range ign {
		src := head + "class 주문\nclass 고객\n" + c
		switch {
		case c == "---" || c == "title": // 같은 칸의 config: 사례가 본다
			continue
		case c == "config:":
			src = "---\nconfig:\n  theme: dark\n---\n" + head + "class A"
		case strings.HasPrefix(c, "%%{"):
			src = c + "\n" + head + "class A"
		}
		r := render(t, block(src))
		if len(r.Warnings) != 1 || !strings.Contains(r.HTML, "<svg") {
			t.Errorf("클래스도 쓰지 않는 줄 %q: 경고 %+v(기대 1개), 그림 %v", c, r.Warnings, strings.Contains(r.HTML, "<svg"))
		}
		ran = append(ran, c)
	}
	check("class-ignored", ran, rows, "%%{init", "config:", "classDef hot", "classDef a,b", "style Order", `cssClass "주문,고객"`, "class 주문:::hot",
		"click 주문 href", "click 주문 call", "link 주문", "callback 주문")

	uns, rows := codes("class-unsupported")
	ran = nil
	for _, c := range uns {
		src := head + c
		switch {
		case strings.HasPrefix(c, "namespace"):
			src = head + c + "\nclass A\n}"
		case c == "}":
			src = head + "class A\n" + c
		case strings.HasPrefix(c, "classDef") || strings.HasPrefix(c, "style"): // 클래스가 있어야 「그릴 클래스가 없음」과 갈린다
			src = head + "class 주문\n" + c
		case c == "CLASSDIAGRAM" || strings.HasPrefix(c, "classDiagram"):
			src = c + "\nclass A"
		case strings.HasPrefix(c, "%%{"):
			src = c + "\n" + head + "class A"
		case strings.HasPrefix(c, "accTitle"):
			src = head + c + "\nclass A"
		case c == "ﬂ°" || c == "¶ß":
			src = head + "가 --> 나 : x " + c + " y"
		case strings.HasPrefix(c, "ASCII 밖"):
			src = head + "가 --> 나"
		case strings.HasPrefix(c, "SVG 가 담지 못하는"):
			src = head + "가 --> 나 : x￾y"
		case strings.HasPrefix(c, `title: "`):
			src = "---\n" + c + "\n---\n" + head + "class A"
		case c == "상한 초과":
			var b2 strings.Builder
			b2.WriteString(head)
			for i := 0; i <= classdiag.MaxClasses; i++ {
				fmt.Fprintf(&b2, "class C%d\n", i)
			}
			src = b2.String()
		}
		r := render(t, block(src))
		if len(r.Warnings) != 1 || strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "language-mermaid") {
			t.Errorf("클래스도 미지원 %q 가 원문 + 경고가 아님: %+v", c, r.Warnings)
		}
		ran = append(ran, c)
	}
	check("class-unsupported", ran, rows, "namespace", `note "`, "note for", "class 목록~~", "class 목록~ T~", "class 목록~T-1~",
		"class 목록~List~int~~", "class 목록~T~B", "class 목록~link~", "class 목록~note, T~", "주문 : +쌍 a~T~ b~U~", "주문 : +표 Map~K, V, W~",
		"주문 : +값 x~T~y", "주문 : +값 a ~T~", "주문 : ~T~ x", `click 목록~T~ href "주소"`, "style 목록~T~ fill", `class 주문["`, "class `",
		"가 ()--", "가 --()", "class 주문:::hot {", "class 결제수단 <<interface>> {", "주문 : +합계() : decimal", "주문 : +합계()decimal",
		"주문 : +새주문()$ 주문", "<<interface>> 결제수단", `가 "1" --> 가`, `가 --> "2" 가`, "가 -- > 나", "가 <| -- 나", "가 --oB", "가 -> 나", "가 --- 나",
		"가 --> 나 --> 다", "class 가 나", "가", "Class 가", "Direction LR", "CLASSDIAGRAM", "classDiagram-V2", "class note", "class o", "class 주문class",
		"class 가o", "class accTitle1", "class _a", "class a_", "class a-b", "class 가.나", "class 𝔸", "주문 : +a<b", "주문 : +목록[]", "주문 : +값 *강조*",
		"주문 : 1. 첫째", "주문 : =값", "주문 : _비밀", "주문 : +a:b", "주문 : +a  b", "가 --> 나 : 앞  뒤", "가 --> 나 : *강조*", "가 --> 나 : - 목록",
		"가 --> 나 : a<b", "가 --> 나 : 둘::셋", "가 --> 나 :", `가 "none" --> 나`, "가 --> 나 : #copy;", "주문 : +x#nbsp;y", "가 --> 나; 나 --> 다",
		"가 --> 나 %% 메모", "가 --> 나 : 방향 direction", "classDef 강조", "style 주문 fill", "style Order stroke-width", `link 주문 ""`, `callback 주문 ""`, `click 주문 href ""`, `cssClass "" hot`, `가 "" --> 나`, `link 주문 "주소" ""`, `callback 주문 "f" ""`, `click 주문 href "주소" ""`, `click 주문 call f() ""`, "class 빈 {", "}", "%%{init: {", "ﬂ°", "¶ß",
		"ASCII 밖", "SVG 가 담지 못하는", `title: "두  칸"`, "accTitle: a<b", "상한 초과")

	// 멤버 표 — 입력을 클래스 몸 안의 한 줄로 그린다. 「같다」면 원본 표시 글(괄호 설명을 뗀 것)이 SVG 글 그대로 나온다.
	members := 0
	for _, line := range strings.Split(section("class-members"), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := split.Split(line[1:], -1)
		if len(cells) < 3 {
			t.Fatalf("멤버 표 줄 꼴이 다름: %s", line)
		}
		in := re.FindStringSubmatch(cells[0])
		if in == nil {
			t.Fatalf("멤버 표 입력 칸에 표기가 없음: %s", line)
		}
		input := in[1] + in[2]
		verdict := strings.TrimSpace(cells[2])
		r := render(t, block(head+"class A {\n"+input+"\n}"))
		members++
		switch verdict {
		case "원문":
			if len(r.Warnings) != 1 || strings.Contains(r.HTML, "<svg") {
				t.Errorf("멤버 표 %q: 원문 + 경고가 아님: %+v", input, r.Warnings)
			}
		case "같다":
			disp := re.FindStringSubmatch(cells[1])
			if disp == nil {
				t.Fatalf("멤버 표 원본 표시 글 칸에 표기가 없음: %s", line)
			}
			// SVG 글자는 XML 이스케이프된다(제네릭 표시 글의 <·>)
			esc := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(disp[1] + disp[2])
			if len(r.Warnings) != 0 || !strings.Contains(r.HTML, ">"+esc+"<") {
				t.Errorf("멤버 표 %q: 표시 글 %q 가 그대로 나오지 않음: %+v\n%s", input, disp[1]+disp[2], r.Warnings, r.HTML)
			}
			for mark, cls := range map[string]string{"(밑줄)": "cls-static", "(기울임)": "cls-abstract"} {
				if strings.Contains(cells[1], mark) != strings.Contains(r.HTML, cls) {
					t.Errorf("멤버 표 %q: %s 와 %s 가 맞지 않음", input, mark, cls)
				}
			}
		default:
			t.Errorf("멤버 표 %q: 「이 판」 칸 %q 는 같다·원문 가운데 하나", input, verdict)
		}
	}
	if members < 10 {
		t.Errorf("멤버 표 줄 %d 개 — 표지를 확인한다", members)
	}

	// 13장 예제
	i := strings.Index(g, "<!-- class-example -->")
	ex := g[i:]
	ex = ex[strings.Index(ex, "```mermaid"):]
	ex = ex[:strings.Index(ex, "\n````")]
	r := render(t, ex)
	if len(r.Warnings) != 0 || !strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "<figcaption>주문 클래스 모델</figcaption>") ||
		!strings.Contains(r.HTML, ">+합계() : decimal<") || !strings.Contains(r.HTML, ">«interface»<") || !strings.Contains(r.HTML, ">주문한다<") {
		t.Errorf("13장 예제: %+v\n%s", r.Warnings, r.HTML)
	}
}
