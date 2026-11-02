package mdoc

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 이슈 #114 단계 B: 혼합 차트 — :::chart{type=bar line="계열 이름"} 은 그 계열을 선으로, 나머지를 묶은 막대로 그린다.

var (
	comboColRe  = regexp.MustCompile(`<rect x="([-\d.]+)" y="([-\d.]+)" width="([-\d.]+)" height="([-\d.]+)" rx="[-\d.]+" class="col (s\d) d(\d+)[^"]*"><title>([^<]*)</title></rect>`)
	comboDotRe  = regexp.MustCompile(`<circle cx="([-\d.]+)" cy="([-\d.]+)" r="[-\d.]+" class="dot (?:cl )?(s\d) d\d+"><title>([^<]*)</title></circle>`)
	comboPathRe = regexp.MustCompile(`<path d="([^"]*)" class="line (?:cl )?(s\d|halo)" pathLength="1"></path>`)
	comboValRe  = regexp.MustCompile(`<text x="([-\d.]+)" y="([-\d.]+)" font-size="11" text-anchor="(\w+)" class="val[^"]*">([^<]*)</text>`)
)

const comboTable = "| 분기 | 매출 | 계획 | 비용 |\n|:--|--:|--:|--:|\n| 1분기 | 31.0 | 33 | 20 |\n| 2분기 | 35.4 | 36 | 21 |\n| 3분기 | 44.9 | 40 | 25 |\n\n: 매출과 계획"

func comboVals(html string) []valLab {
	var out []valLab
	for _, m := range comboValRe.FindAllStringSubmatch(html, -1) {
		out = append(out, valLab{x: num(m[1]), y: num(m[2]), anchor: m[3], t: m[4]})
	}
	return out
}

func TestChartCombo(t *testing.T) {
	for _, attrs := range []string{`type=bar line="계획"`, `line="계획"`, `type=BAR line=계획`} {
		r := render(t, chartSrc(attrs, comboTable))
		assertSafe(t, r.HTML)
		if len(r.Warnings) != 0 {
			t.Fatalf("%s: 경고 %v", attrs, r.Warnings)
		}
		if !strings.Contains(r.HTML, `<figure class="chart chart-bar"`) {
			t.Errorf("%s: bar 그림이 아님", attrs)
		}
		// 막대는 선 계열을 뺀 두 계열(색은 열 차례: 매출 s1·비용 s3), 선은 s2
		cols := comboColRe.FindAllStringSubmatch(r.HTML, -1)
		if len(cols) != 6 {
			t.Fatalf("%s: 막대 %d 개", attrs, len(cols))
		}
		for _, c := range cols {
			if c[5] == "s2" || strings.Contains(c[7], "계획") {
				t.Errorf("%s: 선 계열이 막대로 그려짐 %v", attrs, c[7])
			}
		}
		paths := comboPathRe.FindAllStringSubmatch(r.HTML, -1)
		if len(paths) != 2 || paths[0][2] != "halo" || paths[1][2] != "s2" || paths[0][1] != paths[1][1] {
			t.Fatalf("%s: 선 %v", attrs, paths)
		}
		dots := comboDotRe.FindAllStringSubmatch(r.HTML, -1)
		if len(dots) != 3 {
			t.Fatalf("%s: 점 %d 개", attrs, len(dots))
		}
		tk := ticks(r.HTML, false)
		for i, d := range dots {
			if d[3] != "s2" || !strings.HasPrefix(d[4], fmt.Sprintf("%d분기 · 계획: ", i+1)) {
				t.Errorf("%s: 점 %v", attrs, d)
			}
			// 점은 그 항목의 막대 묶음 가운데에 놓인다
			a, b := cols[2*i], cols[2*i+1]
			if mid := (num(a[1]) + num(b[1]) + num(b[3])) / 2; math.Abs(mid-num(d[1])) > 0.11 {
				t.Errorf("%s: 점 x %v, 막대 묶음 가운데 %v", attrs, d[1], mid)
			}
		}
		// 값 40 의 점은 눈금 40 의 높이
		if num(dots[2][2]) != tk["40"] {
			t.Errorf("%s: 점 y %v, 눈금 40 의 y %v", attrs, dots[2][2], tk["40"])
		}
		// 선은 막대 뒤, 점은 선 뒤, 값 글자는 모두 그 뒤(.vals)
		pi, di, vi := strings.Index(r.HTML, `class="line halo"`), strings.Index(r.HTML, `class="dot `), strings.Index(r.HTML, `<g class="vals">`)
		ci := strings.LastIndex(r.HTML, `class="col `)
		if !(ci < pi && pi < di && di < vi) || strings.Contains(r.HTML[:vi], `class="val `) {
			t.Errorf("%s: 차례 col=%d line=%d dot=%d vals=%d", attrs, ci, pi, di, vi)
		}
		// 선·점·범례 표식에 표지 cl(공문 theme.css 가 진하게 칠한다), 막대·테두리에는 없다
		if n := strings.Count(r.HTML, `class="line cl s2"`) + strings.Count(r.HTML, `class="dot cl s2 `) + strings.Count(r.HTML, `class="sw cl s2"`) + strings.Count(r.HTML, `class="sw pt cl s2"`); n != 1+3+2 {
			t.Errorf("%s: 표지 cl %d 개", attrs, n)
		}
		if strings.Contains(r.HTML, `class="col cl`) || strings.Contains(r.HTML, `class="line halo cl`) {
			t.Errorf("%s: 막대·테두리에 cl", attrs)
		}
		// 범례: 막대 계열은 사각형, 선 계열은 선(가는 사각형) + 점
		for _, want := range []string{`class="sw s1"`, `class="sw s3"`, `<rect x="503" y="6.8" width="16" height="2.4" rx="1.2" class="sw cl s2">`, `class="sw pt cl s2"`} {
			if !strings.Contains(r.HTML, want) {
				t.Errorf("%s: 범례에 %q 없음", attrs, want)
			}
		}
	}

	// 선 계열 값이 막대보다 크면 축이 넓어진다. 막대가 있으니 0 부터
	r := render(t, chartSrc(`line="계획"`, "| 분기 | 매출 | 계획 |\n|--|--|--|\n| 1분기 | 31 | 120 |\n| 2분기 | 35 | 90 |"))
	tk := ticks(r.HTML, false)
	if _, ok := tk["120"]; !ok {
		t.Errorf("선 계열 120 이 축에 없음 %v", tk)
	}
	if _, ok := tk["0"]; !ok {
		t.Errorf("0 부터가 아님 %v", tk)
	}
	r = render(t, chartSrc(`line="계획"`, "| 분기 | 매출 | 계획 |\n|--|--|--|\n| 1분기 | 310 | 300 |\n| 2분기 | 350 | 320 |"))
	if _, ok := ticks(r.HTML, false)["0"]; !ok {
		t.Errorf("값이 0 에서 멀어도 막대가 있으면 0 부터 %v", ticks(r.HTML, false))
	}
	// 음수·0
	r = render(t, chartSrc(`line="율"`, "| 분기 | 매출 | 율 |\n|--|--|--|\n| 1분기 | -5 | 0 |\n| 2분기 | 8 | -3 |"))
	tk = ticks(r.HTML, false)
	dots := comboDotRe.FindAllStringSubmatch(r.HTML, -1)
	if len(dots) != 2 || num(dots[0][2]) != tk["0.0"] || len(r.Warnings) != 0 {
		t.Errorf("0 의 점 %v, 눈금 %v, 경고 %v", dots, tk, r.Warnings)
	}

	// 기준선과 함께: 기준선 뒤에 값 글자, 선의 끝 값 글자도 그림 영역 안
	r = render(t, chartSrc(`line="계획" ref=38 ref-label="목표"`, comboTable))
	rl := findRef(t, r.HTML)
	if rl == nil || len(r.Warnings) != 0 {
		t.Fatalf("기준선 %v %v", rl, r.Warnings)
	}
	if strings.Index(r.HTML, `<g class="vals">`) < strings.Index(r.HTML, `<g class="ref">`) {
		t.Error("값 글자가 기준선 앞")
	}
	for _, l := range comboVals(r.HTML) {
		if right := textRight(l.x, l.anchor, l.t); right > rl.x2+0.1 {
			t.Errorf("값 %q 오른쪽 끝 %.1f 가 그림 끝 %.1f 너머", l.t, right, rl.x2)
		}
	}

	// labels=off: 값 글자 없음
	if h := render(t, chartSrc(`line="계획" labels=off`, comboTable)).HTML; strings.Contains(h, `class="val `) {
		t.Error("labels=off 인데 값 글자")
	}

	// line 이 없는 차트(기존 7종)에는 선 테두리·범례 점·.vals 가 없다 — SVG 가 그대로(골든)
	for _, typ := range []string{"bar", "hbar", "stacked", "line", "area"} {
		if h := render(t, chartSrc("type="+typ, comboTable)).HTML; strings.Contains(h, "halo") || strings.Contains(h, "sw pt") || strings.Contains(h, `class="vals"`) || regexp.MustCompile(`class="[^"]*\bcl\b`).MatchString(h) {
			t.Errorf("%s: 혼합 차트의 표지가 새어 나옴", typ)
		}
	}
}

// 이름은 공백을 하나로 줄여 맞추고, 열 차례가 바뀌어도 같은 계열을 선으로 그린다. 숫자 이름을 열 번호로 읽지 않는다.
func TestChartComboNames(t *testing.T) {
	a := render(t, chartSrc(`line="  영업   이익률 "`, "| 분기 | 매출 |  영업  이익률  |\n|--|--|--|\n| 1분기 | 31 | 8.2 |\n| 2분기 | 35 | 9.4 |"))
	if len(a.Warnings) != 0 || len(comboDotRe.FindAllString(a.HTML, -1)) != 2 {
		t.Errorf("공백 정규화: %v", a.Warnings)
	}
	// 열 재배열: 선은 같은 점(같은 좌표)을 지나고 색만 열 차례를 따른다
	b := render(t, chartSrc(`line="영업 이익률"`, "| 분기 | 영업 이익률 | 매출 |\n|--|--|--|\n| 1분기 | 8.2 | 31 |\n| 2분기 | 9.4 | 35 |"))
	pa, pb := comboPathRe.FindAllStringSubmatch(a.HTML, -1), comboPathRe.FindAllStringSubmatch(b.HTML, -1)
	if len(pa) != 2 || len(pb) != 2 || pa[1][1] != pb[1][1] || pa[1][2] != "s2" || pb[1][2] != "s1" {
		t.Errorf("열 재배열: %v / %v", pa, pb)
	}
	ca, cb := comboColRe.FindAllStringSubmatch(a.HTML, -1), comboColRe.FindAllStringSubmatch(b.HTML, -1)
	for i := range ca {
		if ca[i][1] != cb[i][1] || ca[i][4] != cb[i][4] || ca[i][7] != cb[i][7] {
			t.Errorf("열 재배열: 막대 %v / %v", ca[i], cb[i])
		}
	}
	// 숫자 이름: 머리 칸 "2026" 을 이름으로 맞춘다. line=1 은 첫 계열이 아니다(없는 이름)
	r := render(t, chartSrc(`line=2026`, twoSeries))
	if p := comboPathRe.FindAllStringSubmatch(r.HTML, -1); len(r.Warnings) != 0 || len(p) != 2 || p[1][2] != "s2" {
		t.Errorf("line=2026: %v %v", p, r.Warnings)
	}
	if r := render(t, chartSrc(`line=1`, twoSeries)); len(chartWarnings(r)) != 1 || strings.Contains(r.HTML, "<svg") {
		t.Errorf("line=1 은 열 번호가 아니다: %v", r.Warnings)
	}
}

func TestChartComboRejects(t *testing.T) {
	nine := "| 항목 |"
	for i := 1; i <= 9; i++ {
		nine += fmt.Sprintf(" c%d |", i)
	}
	nine += "\n|--|" + strings.Repeat("--|", 9) + "\n| x |" + strings.Repeat(" 1 |", 9)
	cases := []struct {
		name, attrs, table, want string
	}{
		{"stacked", `type=stacked line="2026"`, twoSeries, "bar 에만"},
		{"hbar", `type=hbar line="2026"`, twoSeries, "type=hbar"},
		{"line", `type=line line="2026"`, twoSeries, "type=line"},
		{"area", `type=area line="2026"`, twoSeries, "type=area"},
		{"pie", `type=pie line="2026"`, twoSeries, "type=pie"},
		{"donut", `type=donut line="2026"`, twoSeries, "type=donut"},
		{"빈 이름", `line=""`, twoSeries, "비어"},
		{"공백 이름", `line="   "`, twoSeries, "비어"},
		{"없는 이름", `line="2027"`, twoSeries, `"2027" 와 같은 이름의 계열이 없어`},
		{"대소문자", `line="Profit"`, "| a | 매출 | profit |\n|--|--|--|\n| x | 1 | 2 |", `"Profit"`},
		{"중복", `line="계획"`, "| a | 계획 | 매출 | 계획 |\n|--|--|--|--|\n| x | 1 | 2 | 3 |", "2 개"},
		{"공백만 다른 중복", `line="영업 이익"`, "| a | 영업 이익 | 영업  이익 |\n|--|--|--|\n| x | 1 | 2 |", "2 개"},
		{"첫 열", `line="분기"`, twoSeries, "첫 열"},
		{"상한 밖", `line="c9"`, nine, "상한(8) 밖"},
		{"막대 없음", `line="값"`, "| a | 값 |\n|--|--|\n| x | 1 |\n| y | 2 |", "막대로 그릴 계열이 없어"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := render(t, chartSrc(c.attrs, c.table))
			assertSafe(t, r.HTML)
			ws := chartWarnings(r)
			if len(ws) != 1 || len(r.Warnings) != 1 || !strings.Contains(ws[0], c.want) || !strings.Contains(ws[0], "표로 둠") {
				t.Errorf("경고 %v, 기대 %q 하나", r.Warnings, c.want)
			}
			if strings.Contains(r.HTML, "<svg") || !strings.Contains(r.HTML, "<table") {
				t.Errorf("차트 없이 표 그대로여야\n%s", r.HTML)
			}
			// 컨테이너의 data-line 은 원문 줄 번호 하나뿐(line="…" 이 data-line 으로 새지 않는다)
			if m := regexp.MustCompile(`data-line="([^"]*)"`).FindAllStringSubmatch(r.HTML, -1); len(m) == 0 || m[0][1] != "1" || strings.Count(r.HTML[:strings.Index(r.HTML, ">")], "data-line") != 1 {
				t.Errorf("data-line %v", m)
			}
		})
	}
	// 상한 안의 계열이면 계열 상한 경고와 함께 그린다
	r := render(t, chartSrc(`line="c2"`, nine))
	if ws := chartWarnings(r); len(ws) != 1 || !strings.Contains(ws[0], "앞의 8 개만") || !strings.Contains(r.HTML, "<svg") {
		t.Errorf("상한 안: %v", r.Warnings)
	}
	// 모르는 type 이 먼저다(경고 하나 — 그 종류의 속성일 수 있다)
	if r := render(t, chartSrc(`type=radar line="2026"`, twoSeries)); len(r.Warnings) != 1 || !unknownType(r) {
		t.Errorf("모르는 type: %v", r.Warnings)
	}
}

// 빈 값·한 점·같은 좌표: 선은 빈 값에서 끊기고, 계열이 모두 비어도 막대는 그린다.
func TestChartComboValues(t *testing.T) {
	r := render(t, chartSrc(`line="율"`, "| 월 | 매출 | 율 |\n|--|--|--|\n| 1월 | 3 | 5 |\n| 2월 | 4 | 6 |\n| 3월 | 5 | - |\n| 4월 | 6 | 8 |\n| 5월 | 7 | 9 |"))
	p := comboPathRe.FindAllStringSubmatch(r.HTML, -1)
	if len(p) != 2 || strings.Count(p[1][1], "M") != 2 || len(comboDotRe.FindAllString(r.HTML, -1)) != 4 || len(r.Warnings) != 0 {
		t.Errorf("빈 값에서 끊김: %v %v", p, r.Warnings)
	}
	r = render(t, chartSrc(`line="율"`, "| 월 | 매출 | 율 |\n|--|--|--|\n| 1월 | 3 | |\n| 2월 | 4 | — |"))
	if len(comboPathRe.FindAllString(r.HTML, -1)) != 0 || len(comboColRe.FindAllString(r.HTML, -1)) != 2 || len(r.Warnings) != 0 {
		t.Errorf("선 계열이 모두 빈 값: %v", r.Warnings)
	}
	r = render(t, chartSrc(`line="율"`, "| 월 | 매출 | 율 |\n|--|--|--|\n| 1월 | 3 | 7 |"))
	p = comboPathRe.FindAllStringSubmatch(r.HTML, -1)
	d := comboDotRe.FindAllStringSubmatch(r.HTML, -1)
	if len(p) != 2 || strings.Contains(p[1][1], "L") || len(d) != 1 || !strings.Contains(r.HTML, `>7.0</text>`) && !strings.Contains(r.HTML, `>7</text>`) {
		t.Errorf("한 점: %v %v", p, d)
	}
	// 같은 좌표(선 값 = 막대 값): 점이 막대 끝에 놓이고 경고 없음
	r = render(t, chartSrc(`line="율"`, "| 월 | 매출 | 율 |\n|--|--|--|\n| 1월 | 5 | 5 |\n| 2월 | 5 | 5 |"))
	d = comboDotRe.FindAllStringSubmatch(r.HTML, -1)
	c := comboColRe.FindAllStringSubmatch(r.HTML, -1)
	if len(d) != 2 || len(c) != 2 || d[0][2] != c[0][2] || len(r.Warnings) != 0 {
		t.Errorf("같은 좌표: 점 %v 막대 %v", d, c)
	}
	// % 단위: 칸이 모두 % 면 단위 %, 값 글자에 %
	r = render(t, chartSrc(`line="율"`, "| 월 | 점유 | 율 |\n|--|--|--|\n| 1월 | 30% | 12.5% |\n| 2월 | 31% | 13% |"))
	if !strings.Contains(r.HTML, "율: 13.0%</title>") {
		t.Errorf("%%: %s", r.HTML)
	}
}

// 값 글자는 막대가 먼저다: 선의 값 글자는 막대 값 글자와 겹치지도 붙지도(comboGap) 않는 단계(모든 점 → 마지막 점 → 없음)만 쓴다.
func TestChartComboLabels(t *testing.T) {
	tbl := func(rows int, bar, line func(i int) int) string {
		var b strings.Builder
		b.WriteString("| 월 | 매출 | 계획 | 비용 |\n|--|--|--|--|\n")
		for i := 1; i <= rows; i++ {
			fmt.Fprintf(&b, "| %d월 | %d | %d | %d |\n", i, bar(i), line(i), bar(i)/2)
		}
		return b.String()
	}
	// 선이 막대보다 훨씬 위: 막대 값 글자와 겹치지 않아 모든 점에 값
	far := render(t, chartSrc(`line="계획"`, tbl(6, func(i int) int { return 10 + i }, func(i int) int { return 100 + i*7 }))).HTML
	if n := len(comboVals(far)); n != 6*2+6 {
		t.Errorf("떨어진 선: 값 글자 %d 개(막대 12 + 선 6)", n)
	}
	// 선이 막대 끝 바로 위: 마지막 점이나 없음으로 줄인다
	near := render(t, chartSrc(`line="계획"`, tbl(6, func(i int) int { return 50 + i }, func(i int) int { return 52 + i }))).HTML
	if n := len(comboVals(near)); n < 12 || n > 13 {
		t.Errorf("붙은 선: 값 글자 %d 개", n)
	}
	// 여러 표에서: 어떤 값 글자 둘도 겹치지 않는다(막대 묶음 안 이웃 막대의 글자는 기존 규칙대로 자리를 잰다)
	for name, h := range map[string]string{"far": far, "near": near,
		"zigzag": render(t, chartSrc(`line="계획"`, tbl(12, func(i int) int { return 40 + i%3*10 }, func(i int) int { return 30 + i%2*25 }))).HTML,
		"neg":    render(t, chartSrc(`line="계획"`, tbl(5, func(i int) int { return i*10 - 25 }, func(i int) int { return 20 - i*9 }))).HTML,
		"한 계열":   render(t, chartSrc(`line="계획"`, "| 월 | 매출 | 계획 |\n|--|--|--|\n| 1 | 10 | 11 |\n| 2 | 20 | 19 |\n| 3 | 15 | 40 |")).HTML,
	} {
		ls := comboVals(h)
		centers := map[string]bool{}
		for _, c := range comboColRe.FindAllStringSubmatch(h, -1) {
			centers[f1(num(c[1])+num(c[3])/2)] = true
		}
		isBar := func(l valLab) bool { return l.anchor == "middle" && centers[f1(l.x)] }
		for i := range ls {
			for j := i + 1; j < len(ls); j++ {
				// 막대 글자끼리의 자리는 기존 규칙(막대 너비)이 정한다 — 여기서는 선 글자와의 겹침만 본다
				if overlaps(ls[i].box(), ls[j].box(), comboGap-0.2) && !(isBar(ls[i]) && isBar(ls[j])) {
					t.Errorf("%s: 값 글자 %q(%v,%v) 와 %q(%v,%v) 가 겹침", name, ls[i].t, ls[i].x, ls[i].y, ls[j].t, ls[j].x, ls[j].y)
				}
			}
		}
	}
}

// 최대 행·계열·긴 이름·기준선과 함께: 좌표가 유한하고 크기가 음수가 아니며, 선과 점은 그림 안이다.
func TestChartComboGeometry(t *testing.T) {
	longCat := strings.Repeat("아주긴항목이름", 6)
	attrRe := regexp.MustCompile(` (x|y|x1|y1|x2|y2|cx|cy|width|height|r|rx)="([^"]*)"`)
	for _, rows := range []int{1, 2, 12, 30, 60} {
		for _, series := range []int{2, 3, 8} {
			for _, lineAt := range []int{1, series} {
				for _, ref := range []string{"", "ref=0", "ref=999 ref-label=\"" + longCat + "\"", "ref=-5000"} {
					var b strings.Builder
					b.WriteString("| 항목 |")
					for s := 1; s <= series; s++ {
						fmt.Fprintf(&b, " 계열 %d %s |", s, strings.Repeat("이름", s))
					}
					b.WriteString("\n|--|" + strings.Repeat("--|", series) + "\n")
					for i := 0; i < rows; i++ {
						fmt.Fprintf(&b, "| %s%d |", longCat, i)
						for s := 0; s < series; s++ {
							fmt.Fprintf(&b, " %d |", (i*37+s*101)%1000-300)
						}
						b.WriteString("\n")
					}
					name := fmt.Sprintf("rows=%d series=%d line=%d %s", rows, series, lineAt, ref)
					attrs := fmt.Sprintf(`line="계열 %d %s" %s`, lineAt, strings.Repeat("이름", lineAt), ref)
					r := render(t, chartSrc(attrs, b.String()))
					if len(r.Warnings) != 0 {
						t.Errorf("%s: 경고 %v", name, r.Warnings)
					}
					if strings.Contains(r.HTML, "NaN") || strings.Contains(r.HTML, "Inf") {
						t.Errorf("%s: NaN·Inf", name)
					}
					for _, m := range attrRe.FindAllStringSubmatch(r.HTML, -1) {
						v, err := strconv.ParseFloat(m[2], 64)
						if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
							t.Errorf("%s: %s=%q", name, m[1], m[2])
						} else if (m[1] == "width" || m[1] == "height" || m[1] == "r" || m[1] == "rx") && v < 0 {
							t.Errorf("%s: 음수 크기 %s=%v", name, m[1], v)
						}
					}
					dots := comboDotRe.FindAllStringSubmatch(r.HTML, -1)
					if len(dots) != rows {
						t.Errorf("%s: 점 %d 개", name, len(dots))
					}
					for _, d := range dots {
						if x, y := num(d[1]), num(d[2]); x < 0 || x > cvW || y < 0 || y > 300 {
							t.Errorf("%s: 점 (%v,%v) 가 그림 밖", name, x, y)
						}
					}
					if n := len(comboColRe.FindAllString(r.HTML, -1)); n != rows*(series-1) {
						t.Errorf("%s: 막대 %d 개", name, n)
					}
				}
			}
		}
	}
}

// 음수 막대의 값 글자(막대 아래)는 그림 영역(가장 아래 격자선) 안이다 — 축 끝까지 자란 막대면 전에는 항목 이름 줄로 내려가
// "-2.5%3월" 처럼 겹쳤다(혼합 차트 좁은 단 실측, bar 도 같다).
func TestChartBarNegativeLabelRoom(t *testing.T) {
	for _, tbl := range []string{
		"| 월 | 본점 |\n|--|--|\n| 1월 | 8.2% |\n| 2월 | -2.5% |\n| 3월 | 4% |",
		"| 월 | 본점 | 지점 |\n|--|--|--|\n| 1월 | 8.2% | 6.1% |\n| 2월 | -2.5% | 1.2% |",
		"| 월 | 값 |\n|--|--|\n| 1월 | -5 |\n| 2월 | -10 |",
		"| 월 | 값 |\n|--|--|\n| 1월 | -100 |\n| 2월 | 900 |",
		"| 월 | 값 | 평균 |\n|--|--|--|\n| 1월 | -40 | 3 |\n| 2월 | 60 | 10 |",
	} {
		for _, attrs := range []string{"type=bar", `type=bar unit="억 원"`, "type=bar ref=1", `line="평균"`} {
			r := render(t, chartSrc(attrs, tbl))
			if len(r.Warnings) != 0 {
				if strings.Contains(attrs, "line") {
					continue // 평균 열이 없는 표
				}
				if ws := chartWarnings(r); !(len(ws) == 1 && strings.Contains(attrs, "unit=") && strings.Contains(ws[0], "% 를 붙인 칸")) {
					t.Fatalf("%s: %v", attrs, r.Warnings) // % 칸에 unit="억 원" 은 단위 혼합 경고(단계 C)뿐이어야 — 그림은 그대로 본다
				}
			}
			bottom := 0.0
			for _, y := range ticks(r.HTML, false) {
				bottom = math.Max(bottom, y)
			}
			n := 0
			for _, l := range comboVals(r.HTML) {
				if strings.HasPrefix(l.t, "-") {
					n++
					if l.y > bottom+0.1 {
						t.Errorf("%s: 값 %q 의 y %.1f 가 그림 아래 끝 %.1f 너머\n%s", attrs, l.t, l.y, bottom, tbl)
					}
				}
			}
			if n == 0 {
				t.Errorf("%s: 음수 값 글자 없음\n%s", attrs, tbl)
			}
		}
	}
}

// 리뷰 R1: 선의 값 글자는 그림 안(왼쪽 0·오른쪽 viewBox 끝·아래 끝 격자선 위 — 항목 이름 줄과 겹치지 않게)이고, 선 값 글자끼리도
// 겹치지 않는다. (1) 축 최솟값에 있는 선 점의 값이 아래로 놓여 "1월" 과 같은 자리에 나왔다, (2) 12항목의 900,000,000 이 서로
// 겹치고 오른쪽 끝이 606.9 로 넘쳤다.
func TestChartComboLineLabelBounds(t *testing.T) {
	catRe := regexp.MustCompile(`<text x="([-\d.]+)" y="([-\d.]+)" font-size="12" text-anchor="middle" class="cat">([^<]*)</text>`)
	rows := func(n int, bar, line func(i int) string) string {
		var b strings.Builder
		b.WriteString("| 월 | 매출 | 계획 |\n|--|--|--|\n")
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&b, "| %d월 | %s | %s |\n", i, bar(i), line(i))
		}
		return b.String()
	}
	c := func(s string) func(int) string { return func(int) string { return s } }
	tables := map[string]string{
		"R1-1 최솟값의 선":  "| 월 | 매출 | 계획 |\n|--|--|--|\n| 1월 | 10 | -10 |\n| 2월 | 20 | 0 |",
		"R1-2 긴 숫자 12": rows(12, c("1"), c("900000000")),
		"긴 숫자 12 오름":   rows(12, c("1"), func(i int) string { return strconv.Itoa(900000000 + i*1000000) }),
		"골짜기 최솟값":      rows(5, c("30"), func(i int) string { return strconv.Itoa((i - 3) * (i - 3) * 10) }),
		"음수 선만":        rows(4, c("5"), func(i int) string { return strconv.Itoa(-10 * i) }),
	}
	for name, tbl := range tables {
		for _, attrs := range []string{`line="계획"`, `line="계획" ref=1`, `line="계획" ref=950000000 ref-label="연간 목표치"`, `line="계획" unit="억 원"`} {
			h := render(t, chartSrc(attrs, tbl)).HTML
			bottom := 0.0
			for _, y := range ticks(h, false) {
				bottom = math.Max(bottom, y)
			}
			var cats [][4]float64
			for _, m := range catRe.FindAllStringSubmatch(h, -1) {
				cats = append(cats, valLab{x: num(m[1]), y: num(m[2]), anchor: "middle", t: m[3]}.box())
			}
			centers := map[string]bool{}
			for _, m := range comboColRe.FindAllStringSubmatch(h, -1) {
				centers[f1(num(m[1])+num(m[3])/2)] = true
			}
			var line []valLab
			for _, l := range comboVals(h) {
				b := l.box()
				if b[0] < -0.1 || b[2] > cvW+0.1 || l.y > bottom+0.1 {
					t.Errorf("%s %s: 값 %q 칸 %v 가 그림(0~%v, 아래 %v) 밖", name, attrs, l.t, b, cvW, bottom)
				}
				for _, cb := range cats {
					if overlaps(b, cb, 0) {
						t.Errorf("%s %s: 값 %q 가 항목 이름과 겹침", name, attrs, l.t)
					}
				}
				if !(l.anchor == "middle" && centers[f1(l.x)]) {
					line = append(line, l)
				}
			}
			for i := range line {
				for j := i + 1; j < len(line); j++ {
					if overlaps(line[i].box(), line[j].box(), 0) {
						t.Errorf("%s %s: 선 값 %q(%v) 와 %q(%v) 가 겹침", name, attrs, line[i].t, line[i].x, line[j].t, line[j].x)
					}
				}
			}
		}
	}
	// 같은 데이터의 꺾은선(type=line)도 최솟값의 값 글자를 항목 이름 줄로 내리지 않는다
	h := render(t, chartSrc("type=line", "| 월 | 계획 |\n|--|--|\n| 1월 | -10 |\n| 2월 | 0 |")).HTML
	if len(comboVals(h)) != 2 {
		t.Fatalf("type=line: 값 글자 %d 개", len(comboVals(h)))
	}
	for _, l := range comboVals(h) {
		if bottom := ticks(h, false)["-10"]; l.y > bottom+0.1 {
			t.Errorf("type=line: 값 %q y %v 가 그림 아래 끝 %v 너머", l.t, l.y, bottom)
		}
	}
}
