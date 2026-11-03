package mdoc

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 이슈 itda-hyve#114 단계 A: 모르는 type 은 표로 남기고, 기준선 ref·ref-label 을 긋는다.

var (
	refLineRe = regexp.MustCompile(`<line x1="([-\d.]+)" y1="([-\d.]+)" x2="([-\d.]+)" y2="([-\d.]+)" class="ref-line"><title>([^<]*)</title></line><text x="([-\d.]+)" y="([-\d.]+)" font-size="11" text-anchor="(\w+)" class="ref-lab">([^<]*)</text>`)
	gridRe    = regexp.MustCompile(`<line x1="([-\d.]+)" y1="([-\d.]+)" x2="([-\d.]+)" y2="([-\d.]+)" class="gl(?: zero)?"></line><text [^>]*class="tick">([^<]*)</text>`)
)

type refLine struct {
	x1, y1, x2, y2 float64
	tip, anchor    string
	lx, ly         float64
	label          string
}

func num(s string) float64 { f, _ := strconv.ParseFloat(s, 64); return f }

func findRef(t *testing.T, html string) *refLine {
	t.Helper()
	ms := refLineRe.FindAllStringSubmatch(html, -1)
	switch len(ms) {
	case 0:
		return nil
	case 1:
	default:
		t.Fatalf("기준선이 %d 개", len(ms))
	}
	m := ms[0]
	return &refLine{num(m[1]), num(m[2]), num(m[3]), num(m[4]), m[5], m[8], num(m[6]), num(m[7]), m[9]}
}

// ticks 는 눈금 글자 → 격자선 좌표(세로형은 y, hbar 는 x)다.
func ticks(html string, horizontal bool) map[string]float64 {
	out := map[string]float64{}
	for _, m := range gridRe.FindAllStringSubmatch(html, -1) {
		if horizontal {
			out[m[5]] = num(m[1])
		} else {
			out[m[5]] = num(m[2])
		}
	}
	return out
}

func chartWarnings(r Result) []string {
	var out []string
	for _, w := range r.Warnings {
		if w.Code == WarnChart {
			out = append(out, w.Message)
		}
	}
	return out
}

func TestChartUnknownTypeKeepsTable(t *testing.T) {
	// 새 판의 종류와 그 종류의 속성: 표 그대로, 경고는 type 하나뿐
	src := chartSrc(`type=heatmap x="매출" size=3`, "| 지점 | 매출 | 이익 |\n|--|--|--|\n| 본점 | 120 | 14 |\n| 강남 | 98 | 9 |\n\n: 지점 분포")
	r := render(t, src)
	assertSafe(t, r.HTML)
	if strings.Contains(r.HTML, "<svg") {
		t.Errorf("모르는 type 을 그림\n%s", r.HTML)
	}
	for _, want := range []string{`<div class="chart"`, "<table", "<td>본점</td>", "<td>120</td>", "<td>9</td>"} {
		if !strings.Contains(r.HTML, want) {
			t.Errorf("%q 없음 — 표를 그대로 내야 함\n%s", want, r.HTML)
		}
	}
	ws := chartWarnings(r)
	if len(ws) != 1 || len(r.Warnings) != 1 || !strings.Contains(ws[0], `"heatmap"`) || !strings.Contains(ws[0], "표로 둠") || !strings.Contains(ws[0], chartTypeList) {
		t.Errorf("경고 %v", r.Warnings)
	}
	// 대소문자는 무시하고 아는 종류는 그린다. 빈 type 도 모르는 종류다
	if r := render(t, chartSrc("type=LINE", twoSeries)); !strings.Contains(r.HTML, `chart-line`) || len(r.Warnings) != 0 {
		t.Errorf("type=LINE: %v", r.Warnings)
	}
	if r := render(t, chartSrc(`type=""`, twoSeries)); strings.Contains(r.HTML, "<svg") || len(chartWarnings(r)) != 1 {
		t.Errorf(`type="": %v`, r.Warnings)
	}
	// 모르는 type 이어도 표가 없으면(내용 그대로) 경고는 type 하나
	if r := render(t, ":::chart{type=radar}\n그냥 글\n:::\n"); len(r.Warnings) != 1 || !strings.Contains(r.HTML, "그냥 글") {
		t.Errorf("표 없음 + 모르는 type: %v", r.Warnings)
	}
}

func TestChartRef(t *testing.T) {
	// 세로 막대: 가로 점선이 눈금 40 의 격자선과 같은 높이, 그림 영역(첫 격자선 x1 ~ x2) 전체, 라벨은 오른쪽 밖
	r := render(t, chartSrc(`type=bar unit="억 원" ref=40 ref-label="목표"`, twoSeries))
	assertSafe(t, r.HTML)
	if len(r.Warnings) != 0 {
		t.Errorf("경고 %v", r.Warnings)
	}
	ref := findRef(t, r.HTML)
	if ref == nil {
		t.Fatalf("기준선 없음\n%s", r.HTML)
	}
	tk := ticks(r.HTML, false)
	if y, ok := tk["40"]; !ok || y != ref.y1 || ref.y1 != ref.y2 {
		t.Errorf("기준선 y %v, 눈금 40 의 y %v(%v)", ref.y1, y, ok)
	}
	g := gridRe.FindStringSubmatch(r.HTML)
	if ref.x1 != num(g[1]) || ref.x2 != num(g[3]) {
		t.Errorf("기준선 x %v~%v, 격자 %v~%v", ref.x1, ref.x2, g[1], g[3])
	}
	if ref.label != "목표 40.0" || ref.tip != "목표: 40.0 억 원" || ref.anchor != "start" || ref.lx <= ref.x2 {
		t.Errorf("라벨 %+v", ref)
	}
	if ref.lx+textW(ref.label, fsVal) > cvW {
		t.Errorf("라벨이 오른쪽으로 넘침 %+v", ref)
	}
	// 막대는 기준선 라벨 자리를 침범하지 않는다
	for _, m := range regexp.MustCompile(`<rect x="([-\d.]+)" y="[-\d.]+" width="([-\d.]+)"[^>]*class="col`).FindAllStringSubmatch(r.HTML, -1) {
		if num(m[1])+num(m[2]) > ref.x2 {
			t.Errorf("막대 %v+%v 가 기준선 끝 %v 너머", m[1], m[2], ref.x2)
		}
	}

	// 점선 아래에 종이색 테두리 선, 값 글자는 기준선 뒤(.vals)에 — 점선이 글자를 가로지르지 않게. 기준선이 없으면 .vals 가 없다
	for _, typ := range []string{"bar", "hbar", "stacked", "line", "area"} {
		h := render(t, chartSrc("type="+typ+" ref=40", twoSeries)).HTML
		ri, vi, hi := strings.Index(h, `<g class="ref">`), strings.Index(h, `<g class="vals">`), strings.Index(h, `class="ref-halo"`)
		if ri < 0 || vi < ri || hi < ri || hi > strings.Index(h, `class="ref-line"`) {
			t.Errorf("%s: 기준선·테두리·값 글자 차례 ref=%d halo=%d vals=%d", typ, ri, hi, vi)
		}
		if strings.Contains(h[:ri], `class="val`) {
			t.Errorf("%s: 기준선 앞에 값 글자", typ)
		}
		if h := render(t, chartSrc("type="+typ, twoSeries)).HTML; strings.Contains(h, `class="vals"`) || strings.Contains(h, "ref-") {
			t.Errorf("%s: 기준선 없는데 vals·ref", typ)
		}
	}

	// 라벨이 없으면 값만, title 은 "기준선"
	r = render(t, chartSrc("type=bar ref=40", twoSeries))
	if ref := findRef(t, r.HTML); ref == nil || ref.label != "40.0" || ref.tip != "기준선: 40.0" {
		t.Errorf("라벨 없음: %+v", ref)
	}
	// ref 의 소수 자리가 더 길면 그 자리로 적는다
	if ref := findRef(t, render(t, chartSrc("type=bar ref=40.25", twoSeries)).HTML); ref == nil || ref.label != "40.25" {
		t.Errorf("소수 자리: %+v", ref)
	}
	// 단위 꼬리·천 단위·유니코드 빼기
	r = render(t, chartSrc(`type=bar unit="억 원" ref="1,000억 원"`, "| a | b |\n|--|--|\n| x | 400 |\n| y | 900 |"))
	if ref := findRef(t, r.HTML); ref == nil || ref.label != "1,000" || len(r.Warnings) != 0 {
		t.Errorf("단위 꼬리: %+v %v", ref, r.Warnings)
	}
	if ref := findRef(t, render(t, chartSrc(`type=bar ref="−2"`, "| a | b |\n|--|--|\n| x | -4 |\n| y | 5 |")).HTML); ref == nil || ref.label != "-2" {
		t.Errorf("음수: %+v", ref)
	}

	// 범위 밖 기준선은 축을 넓힌다(위로·아래로). 막대는 여전히 0 에서
	r = render(t, chartSrc("type=bar ref=120", twoSeries))
	tk = ticks(r.HTML, false)
	if _, ok := tk["120"]; !ok {
		t.Errorf("ref=120: 눈금이 120 까지 가지 않음 %v", tk)
	}
	if ref := findRef(t, r.HTML); ref == nil || ref.y1 != tk["120"] {
		t.Errorf("ref=120 의 y")
	}
	r = render(t, chartSrc("type=bar ref=-20", twoSeries))
	tk = ticks(r.HTML, false)
	if _, ok := tk["-20"]; !ok || !strings.Contains(r.HTML, `class="gl zero"`) {
		t.Errorf("ref=-20: 눈금 %v", tk)
	}
	// 0 기준선과 같은 자리
	r = render(t, chartSrc("type=bar ref=0", "| a | b |\n|--|--|\n| x | -4 |\n| y | 5 |"))
	if ref := findRef(t, r.HTML); ref == nil || ref.y1 != ticks(r.HTML, false)["0"] {
		t.Errorf("ref=0: %+v", ref)
	}

	// 꺾은선은 0 에서 먼 값이면 0 부터 그리지 않는다 — 기준선도 범위에 넣은 뒤 같은 규칙
	line := "| 월 | 값 |\n|--|--|\n| 1월 | 82 |\n| 2월 | 95 |\n| 3월 | 100 |"
	tk = ticks(render(t, chartSrc("type=line ref=70", line)).HTML, false)
	if _, ok := tk["0"]; ok {
		t.Errorf("line ref=70: 0 부터 그림 %v", tk)
	}
	if _, ok := tk["70"]; !ok {
		t.Errorf("line ref=70: 70 눈금 없음 %v", tk)
	}
	tk = ticks(render(t, chartSrc("type=line ref=40", line)).HTML, false)
	if _, ok := tk["0"]; !ok {
		t.Errorf("line ref=40: 범위가 넓어져 0 부터여야 %v", tk)
	}
	// % 차트: 라벨에 % 가 붙는다
	r = render(t, chartSrc(`type=line ref="10%" ref-label="목표"`, "| 월 | 율 |\n|--|--|\n| 1월 | 8% |\n| 2월 | 12.5% |"))
	if ref := findRef(t, r.HTML); ref == nil || ref.label != "목표 10.0%" || ref.tip != "목표: 10.0%" {
		t.Errorf("%%: %+v", ref)
	}

	// 쌓은 막대: 합계 축에 긋는다
	r = render(t, chartSrc("type=stacked ref=80", twoSeries))
	if ref := findRef(t, r.HTML); ref == nil || ref.y1 != ticks(r.HTML, false)["80"] {
		t.Errorf("stacked: %+v %v", ref, ticks(r.HTML, false))
	}
	// 면
	if ref := findRef(t, render(t, chartSrc("type=area ref=30", twoSeries)).HTML); ref == nil {
		t.Error("area: 기준선 없음")
	}

	// 가로 막대: 세로선이 눈금 40 의 x, 막대 영역의 위에서 아래까지, 라벨은 막대 영역 위
	r = render(t, chartSrc(`type=hbar ref=40 ref-label="목표"`, twoSeries))
	ref = findRef(t, r.HTML)
	if ref == nil {
		t.Fatalf("hbar 기준선 없음\n%s", r.HTML)
	}
	htk := ticks(r.HTML, true)
	g = gridRe.FindStringSubmatch(r.HTML)
	if ref.x1 != htk["40"] || ref.x1 != ref.x2 || ref.y2 != num(g[4]) || ref.y1 > num(g[2]) {
		t.Errorf("hbar 기준선 %+v, 눈금 40 x %v, 격자 y %v~%v", ref, htk["40"], g[2], g[4])
	}
	if ref.ly >= ref.y1 || ref.anchor != "middle" {
		t.Errorf("hbar 라벨은 선 위에 %+v", ref)
	}
	// 가장 위 막대는 라벨 줄 아래
	if m := regexp.MustCompile(`<rect x="[-\d.]+" y="([-\d.]+)"[^>]*class="col`).FindStringSubmatch(r.HTML); num(m[1]) <= ref.ly {
		t.Errorf("hbar 막대 y %v 가 라벨 %v 위", m[1], ref.ly)
	}
	// 오른쪽 끝 기준선: 라벨이 그림 안에 머문다
	r = render(t, chartSrc(`type=hbar ref=1000 ref-label="연간 목표치"`, twoSeries))
	if ref := findRef(t, r.HTML); ref == nil || ref.lx+textW(ref.label, fsVal)/2 > cvW || ref.lx-textW(ref.label, fsVal)/2 < 0 {
		t.Errorf("hbar 오른쪽 끝 라벨 %+v", ref)
	}

	// 긴 라벨은 줄이고 title 에 전체
	long := strings.Repeat("아주 긴 기준선 이름 ", 8)
	r = render(t, chartSrc(fmt.Sprintf(`type=bar ref=40 ref-label="%s"`, long), twoSeries))
	if ref := findRef(t, r.HTML); ref == nil || !strings.HasSuffix(ref.label, "…") || !strings.Contains(ref.tip, strings.TrimSpace(long)) || ref.lx+textW(ref.label, fsVal) > cvW {
		t.Errorf("긴 라벨: %+v", ref)
	}
}

func TestChartRefWarnings(t *testing.T) {
	cases := []struct {
		name, attrs, want string
	}{
		{"숫자 아님", `type=bar ref="목표치"`, `"목표치" 를 숫자로 읽지 못해`},
		{"지수", `type=bar ref=4e1`, `"4e1"`},
		{"빈 값", `type=bar ref=""`, "비어"},
		{"원", `type=pie ref=10`, "원·도넛"},
		{"도넛", `type=donut ref=10 ref-label="목표"`, "원·도넛"},
		{"라벨만", `type=bar ref-label="목표"`, "ref-label 은 ref"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := render(t, chartSrc(c.attrs, twoSeries))
			assertSafe(t, r.HTML)
			ws := chartWarnings(r)
			if len(ws) != 1 || len(r.Warnings) != 1 || !strings.Contains(ws[0], c.want) {
				t.Errorf("경고 %v, 기대 %q 하나", r.Warnings, c.want)
			}
			if !strings.Contains(r.HTML, "<svg") {
				t.Error("기준선을 못 읽어도 차트는 그려야 함")
			}
			if strings.Contains(r.HTML, "ref-line") {
				t.Error("못 읽은 기준선을 그음")
			}
		})
	}
	// 숫자가 없어 차트를 그리지 않으면 기준선 경고는 내지 않는다
	if r := render(t, chartSrc(`type=bar ref="x"`, "| a | b |\n|--|--|\n| x | 없음 |")); len(chartWarnings(r)) != 2 {
		t.Errorf("숫자 없음: %v", r.Warnings)
	}
}

// 큰 표·긴 라벨·기준선 위치를 바꿔 가며: 모든 좌표가 유한하고, 크기(width·height·r)가 음수가 아니며, 기준선이 그림 안이다.
func TestChartGeometryBounds(t *testing.T) {
	longCat := strings.Repeat("아주긴항목이름", 6)
	big := func(rows, series int, cat string) string {
		var b strings.Builder
		b.WriteString("| 항목 |")
		for s := 0; s < series; s++ {
			fmt.Fprintf(&b, " 계열 %d %s |", s+1, strings.Repeat("이름", s))
		}
		b.WriteString("\n|--|" + strings.Repeat("--|", series) + "\n")
		for i := 0; i < rows; i++ {
			fmt.Fprintf(&b, "| %s%d |", cat, i)
			for s := 0; s < series; s++ {
				fmt.Fprintf(&b, " %d |", (i*37+s*101)%1000-300)
			}
			b.WriteString("\n")
		}
		return b.String()
	}
	attrRe := regexp.MustCompile(` (x|y|x1|y1|x2|y2|cx|cy|width|height|r|rx)="([^"]*)"`)
	viewRe := regexp.MustCompile(`viewBox="0 0 ([\d.]+) ([\d.]+)"`)
	for _, typ := range []string{"bar", "hbar", "stacked", "line", "area"} {
		for _, rows := range []int{1, 2, 30, 60} {
			for _, series := range []int{1, 8} {
				for _, ref := range []string{"", "ref=0", "ref=999 ref-label=\"" + longCat + "\"", "ref=-5000", "ref=1e15", "ref=0.001"} {
					src := chartSrc("type="+typ+" "+ref, big(rows, series, longCat))
					r := render(t, src)
					name := fmt.Sprintf("%s rows=%d series=%d %s", typ, rows, series, ref)
					if strings.Contains(r.HTML, "NaN") || strings.Contains(r.HTML, "Inf") {
						t.Errorf("%s: NaN·Inf", name)
					}
					vb := viewRe.FindStringSubmatch(r.HTML)
					if vb == nil {
						t.Errorf("%s: 그림 없음 %v", name, r.Warnings)
						continue
					}
					w, h := num(vb[1]), num(vb[2])
					for _, m := range attrRe.FindAllStringSubmatch(r.HTML, -1) {
						v, err := strconv.ParseFloat(m[2], 64)
						if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
							t.Errorf("%s: %s=%q", name, m[1], m[2])
							continue
						}
						switch m[1] {
						case "width", "height", "r", "rx":
							if v < 0 {
								t.Errorf("%s: 음수 크기 %s=%v", name, m[1], v)
							}
						}
					}
					if rl := findRef(t, r.HTML); rl != nil {
						for _, v := range []float64{rl.x1, rl.x2, rl.lx} {
							if v < 0 || v > w {
								t.Errorf("%s: 기준선 x %v 가 그림(0~%v) 밖", name, v, w)
							}
						}
						for _, v := range []float64{rl.y1, rl.y2, rl.ly} {
							if v < 0 || v > h {
								t.Errorf("%s: 기준선 y %v 가 그림(0~%v) 밖", name, v, h)
							}
						}
					} else if ref != "" && !strings.Contains(ref, "1e15") {
						t.Errorf("%s: 기준선 없음", name)
					}
				}
			}
		}
	}
}

// 기준선이 있으면 값 글자(선·면의 끝 값 포함)의 오른쪽 끝은 그림 영역 끝(기준선 x2) 안이다 — 라벨은 그 오른쪽 밖(x2+6)이다.
// 리뷰 R1: 12항목 한 계열의 긴 숫자에서 마지막 값 "890,000,000"(가운데 맞춤)이 "목표 885,000,000" 위로 9 넘어왔다.
func TestChartRefLabelClearOfValues(t *testing.T) {
	valRe := regexp.MustCompile(`<text x="([-\d.]+)" y="([-\d.]+)" font-size="11" text-anchor="(\w+)" class="val[^"]*">([^<]*)</text>`)
	series := func(rows, ns int, f func(i, s int) int) string {
		var b strings.Builder
		b.WriteString("| 월 |" + strings.Repeat(" 계열 |", ns) + "\n|--|" + strings.Repeat("--|", ns) + "\n")
		for i := 1; i <= rows; i++ {
			fmt.Fprintf(&b, "| %d |", i)
			for s := 0; s < ns; s++ {
				fmt.Fprintf(&b, " %d |", f(i, s))
			}
			b.WriteString("\n")
		}
		return b.String()
	}
	tables := map[string]string{
		"리뷰 R1(내려감)": series(12, 1, func(i, _ int) int { return 1000000000 - (i-1)*10000000 }),
		"오름":         series(12, 1, func(i, _ int) int { return 100000000 + i*10000000 }),
		"봉우리":        series(12, 1, func(i, _ int) int { return 900000000 - (i-11)*(i-11)*1000000 }),
		"두 계열":       series(8, 2, func(i, s int) int { return 2000000 + i*1000 - s*300000 }),
		"60항목":       series(60, 1, func(i, _ int) int { return 123456789 + i*1000 }),
		"한 점":        series(1, 1, func(int, int) int { return 987654321 }),
	}
	for name, tbl := range tables {
		for _, typ := range []string{"bar", "stacked", "line", "area"} {
			for _, ref := range []string{"ref=885000000", `ref=890000000 ref-label="목표"`, "ref=0", "ref=2001000"} {
				h := render(t, chartSrc("type="+typ+" "+ref, tbl)).HTML
				rl := findRef(t, h)
				if rl == nil {
					t.Fatalf("%s %s %s: 기준선 없음", name, typ, ref)
				}
				for _, m := range valRe.FindAllStringSubmatch(h, -1) {
					if right := textRight(num(m[1]), m[3], m[4]); right > rl.x2+0.1 {
						t.Errorf("%s %s %s: 값 %q 오른쪽 끝 %.1f 가 그림 끝 %.1f(기준선 라벨 %.1f~) 너머", name, typ, ref, m[4], right, rl.x2, rl.lx)
					}
				}
			}
		}
	}
}
