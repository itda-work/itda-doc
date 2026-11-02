package mdoc

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 이슈 #114 단계 C: 보조 축 axis2·unit2, 그 앞의 단위 혼합 경고와 표시 정밀도.

var (
	tick2Re   = regexp.MustCompile(`<text x="([-\d.]+)" y="([-\d.]+)" font-size="12" text-anchor="start" class="tick ax2">([^<]*)</text>`)
	gridEndRe = regexp.MustCompile(`class="grid"><line x1="[-\d.]+" y1="[-\d.]+" x2="([-\d.]+)"`)
	anyDotRe  = regexp.MustCompile(`<circle cx="([-\d.]+)" cy="([-\d.]+)" r="[-\d.]+" class="dot (?:cl )?(s\d) d\d+"><title>([^<]*)</title></circle>`)
)

const axis2Table = "| 분기 | 매출 | 이익률 |\n|:--|--:|--:|\n| 1분기 | 31.0 | 8.2% |\n| 2분기 | 35.4 | 9.4% |\n| 3분기 | 44.9 | 11.0% |\n| 4분기 | 52.0 | 12.6% |\n\n: 매출과 이익률"

// axisTick 은 눈금 하나다(글자에서 단위 꼬리를 뗀 값과 격자선 높이).
type axisTick struct{ v, y float64 }

func parseTick(t string) float64 {
	v, _ := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSuffix(t, "%"), ",", ""), 64)
	return v
}

// leftTicks·rightTicks 는 아래에서 위로 늘어선 눈금이다. 오른쪽 눈금의 y 는 글자 기준선(격자선 + 4)에서 4 를 뺀 격자선 높이다.
func leftTicks(h string) []axisTick {
	var out []axisTick
	for _, m := range gridRe.FindAllStringSubmatch(h, -1) {
		out = append(out, axisTick{parseTick(m[5]), num(m[2])})
	}
	return out
}

func rightTicks(h string) []axisTick {
	var out []axisTick
	for _, m := range tick2Re.FindAllStringSubmatch(h, -1) {
		out = append(out, axisTick{parseTick(m[3]), num(m[2]) - 4})
	}
	return out
}

// yAt 은 눈금으로 값 v 의 높이를 셈한다(첫·끝 눈금 사이 선형).
func yAt(ts []axisTick, v float64) float64 {
	a, b := ts[0], ts[len(ts)-1]
	return a.y + (v-a.v)/(b.v-a.v)*(b.y-a.y)
}

func TestChartAxis2(t *testing.T) {
	r := render(t, chartSrc(`type=bar unit="억 원" axis2="이익률"`, axis2Table))
	assertSafe(t, r.HTML)
	if len(r.Warnings) != 0 {
		t.Fatalf("경고 %v", r.Warnings)
	}
	h := r.HTML
	lt, rt := leftTicks(h), rightTicks(h)
	if len(lt) < 3 || len(rt) != len(lt) {
		t.Fatalf("눈금 왼쪽 %d · 오른쪽 %d — 칸 수가 같아야", len(lt), len(rt))
	}
	// 오른쪽 눈금은 왼쪽 격자선 높이에, % 가 붙고 간격이 고르다
	for k := range rt {
		if math.Abs(rt[k].y-lt[k].y) > 0.11 {
			t.Errorf("오른쪽 눈금 %d 의 높이 %v ≠ 격자선 %v", k, rt[k].y, lt[k].y)
		}
		if k > 1 && math.Abs((rt[k].v-rt[k-1].v)-(rt[1].v-rt[0].v)) > 1e-9 {
			t.Errorf("오른쪽 눈금 간격이 고르지 않음 %v", rt)
		}
	}
	if m := tick2Re.FindAllStringSubmatch(h, -1); !strings.HasSuffix(m[0][3], "%") {
		t.Errorf("오른쪽 눈금에 %% 가 없음 %v", m[0])
	}
	// 오른쪽 축은 값 범위를 덮고, 왼쪽(막대)은 0 부터. 이익률은 0 에서 멀어 오른쪽 축은 0 부터가 아니다(선의 규칙)
	if lt[0].v != 0 || rt[0].v > 8.2 || rt[len(rt)-1].v < 12.6 || rt[0].v == 0 {
		t.Errorf("범위 왼쪽 %v 오른쪽 %v", lt, rt)
	}
	// 막대는 왼쪽 축, 점은 오른쪽 축의 높이
	cols := comboColRe.FindAllStringSubmatch(h, -1)
	if len(cols) != 4 {
		t.Fatalf("막대 %d 개", len(cols))
	}
	for _, c := range cols {
		if c[5] != "s1" || !strings.HasSuffix(c[7], " 억 원") {
			t.Errorf("막대 %v", c)
		}
	}
	if y := num(cols[3][2]); math.Abs(y-yAt(lt, 52)) > 0.11 {
		t.Errorf("막대 52 의 y %v, 왼쪽 축으로 %v", y, yAt(lt, 52))
	}
	dots := comboDotRe.FindAllStringSubmatch(h, -1)
	want := []float64{8.2, 9.4, 11.0, 12.6}
	if len(dots) != 4 {
		t.Fatalf("점 %d 개", len(dots))
	}
	for i, d := range dots {
		if math.Abs(num(d[2])-yAt(rt, want[i])) > 0.11 {
			t.Errorf("점 %v 의 y %v, 오른쪽 축으로 %v", want[i], d[2], yAt(rt, want[i]))
		}
		if d[3] != "s2" || !strings.HasSuffix(d[4], "%") || strings.Contains(d[4], "억 원") {
			t.Errorf("점 %v — 오른쪽 축 단위(%%)여야", d)
		}
	}
	// 값 글자: 막대는 왼쪽 단위(% 없음), 선은 오른쪽 단위(%)
	var pct, plain int
	for _, l := range comboVals(h) {
		if strings.HasSuffix(l.t, "%") {
			pct++
		} else {
			plain++
		}
	}
	if pct == 0 || plain != 4 {
		t.Errorf("값 글자 %% %d · 일반 %d", pct, plain)
	}
	// 범례에 오른쪽 축 계열을 글자로도 적는다, 단위 줄은 왼쪽 단위만(% 는 눈금에)
	for _, want := range []string{">이익률 (오른쪽 축)</text>", ">매출</text>", `class="unit">단위: 억 원</text>`, `class="sw pt cl s2"`} {
		if !strings.Contains(h, want) {
			t.Errorf("%q 없음", want)
		}
	}
	// 그림 영역(격자선 끝 x2)은 오른쪽 눈금 글자 앞에서 끝나고, 눈금 글자는 그림 안
	x2 := num(gridEndRe.FindStringSubmatch(h)[1])
	for _, m := range tick2Re.FindAllStringSubmatch(h, -1) {
		if x := num(m[1]); x < x2+5.9 || x+textW(m[3], fsAxis) > cvW-3.9 {
			t.Errorf("오른쪽 눈금 %q x %v (그림 끝 %v)", m[3], x, x2)
		}
	}
	for _, l := range comboVals(h) {
		if textRight(l.x, l.anchor, l.t) > x2+0.1 {
			t.Errorf("값 %q 가 그림 끝 %v 너머(오른쪽 눈금과 겹침)", l.t, x2)
		}
	}

	// line 을 함께 적으면 같은 계열이어야 한다(같은 결과)
	if r2 := render(t, chartSrc(`type=bar unit="억 원" line="이익률" axis2="이익률"`, axis2Table)); len(r2.Warnings) != 0 || r2.HTML != h {
		t.Errorf("line 과 axis2 가 같은 계열: %v", r2.Warnings)
	}
	// axis2 없는 같은 표는 오른쪽 축이 없다(기존 그림)
	if strings.Contains(render(t, chartSrc(`type=bar unit="억 원" line="이익률"`, axis2Table)).HTML, "ax2") {
		t.Error("axis2 없이 오른쪽 축")
	}
	for _, typ := range []string{"bar", "hbar", "stacked", "line", "area", "pie", "donut"} {
		if h := render(t, chartSrc("type="+typ, twoSeries)).HTML; strings.Contains(h, "ax2") || strings.Contains(h, "오른쪽 축") {
			t.Errorf("%s: 보조 축 표지가 새어 나옴", typ)
		}
	}
}

// type=line 의 보조 축: 오른쪽 축 계열만 오른쪽 눈금으로, 나머지 선은 왼쪽 눈금으로. 단위 줄에 오른쪽 단위(% 아님).
func TestChartAxis2Line(t *testing.T) {
	tbl := "| 월 | 방문 | 체험 | 전환 |\n|--|--|--|--|\n| 1월 | 1,240 | 310 | 3.1 |\n| 2월 | 1,385 | 352 | 2.6 |\n| 3월 | 1,302 | 340 | 4.4 |"
	r := render(t, chartSrc(`type=line unit="명" axis2="전환" unit2="건"`, tbl))
	if len(r.Warnings) != 0 {
		t.Fatalf("경고 %v", r.Warnings)
	}
	h := r.HTML
	lt, rt := leftTicks(h), rightTicks(h)
	dots := anyDotRe.FindAllStringSubmatch(h, -1)
	if len(dots) != 9 || len(rt) != len(lt) {
		t.Fatalf("점 %d · 눈금 %d/%d", len(dots), len(lt), len(rt))
	}
	vals := map[string][]float64{"s1": {1240, 1385, 1302}, "s2": {310, 352, 340}, "s3": {3.1, 2.6, 4.4}}
	seen := map[string]int{}
	for _, d := range dots {
		ts, unit := lt, " 명"
		if d[3] == "s3" {
			ts, unit = rt, " 건"
		}
		v := vals[d[3]][seen[d[3]]]
		seen[d[3]]++
		if math.Abs(num(d[2])-yAt(ts, v)) > 0.11 {
			t.Errorf("%s %v 의 y %v, 기대 %v", d[3], v, d[2], yAt(ts, v))
		}
		if !strings.HasSuffix(d[4], unit) {
			t.Errorf("점 %v 의 단위가 %q 가 아님", d[4], unit)
		}
	}
	if !strings.Contains(h, `class="unit">단위: 명 · 오른쪽 축 단위: 건</text>`) || !strings.Contains(h, ">전환 (오른쪽 축)</text>") {
		t.Errorf("단위 줄·범례\n%s", h)
	}
	// 0 포함은 축마다 선의 규칙: 왼쪽(310~1,385)은 0 에 가까워 0 부터, 오른쪽(2.6~4.4)은 멀어 0 부터가 아니다
	if lt[0].v != 0 || rt[0].v == 0 || rt[0].v > 2.6 {
		t.Errorf("0 포함 왼쪽 %v 오른쪽 %v", lt, rt)
	}
	// 축별 소수 자리: 왼쪽은 정수, 오른쪽은 한 자리
	if !strings.Contains(h, ">4.4</text>") || strings.Contains(h, "1,302.0") {
		t.Errorf("축별 소수 자리\n%s", h)
	}
}

// 두 축이 모두 0 을 지나면 0 이 같은 격자선이다(막대와 선의 0 이 같은 높이). 그러려면 오른쪽 선이 그림의 절반도 못 쓰면 맞추지 않는다.
func TestChartAxis2Zero(t *testing.T) {
	cases := []struct {
		name, tbl string
		aligned   bool
	}{
		{"둘 다 음수 있음", "| 월 | 손익 | 율 |\n|--|--|--|\n| 1월 | -20 | -5% |\n| 2월 | 40 | 10% |\n| 3월 | 15 | 3% |", true},
		{"선은 0 부터, 막대는 아래로", "| 월 | 손익 | 율 |\n|--|--|--|\n| 1월 | -10 | 1% |\n| 2월 | 40 | 10% |", true},
		{"막대 음수 칸이 넓어짐", "| 월 | 손익 | 율 |\n|--|--|--|\n| 1월 | -10 | -2% |\n| 2월 | 40 | 8% |", true},
		// 막대의 0 이 맨 아래 격자선인데 선은 0 아래로 간다 — 같은 격자선에 맞출 수 없다(왼쪽 축을 늘리지 않는다)
		{"막대 0 이 바닥, 선은 음수", "| 월 | 손익 | 율 |\n|--|--|--|\n| 1월 | 10 | -1% |\n| 2월 | 40 | -9% |", false},
		{"맞추면 선이 납작", "| 월 | 손익 | 율 |\n|--|--|--|\n| 1월 | -40 | 1% |\n| 2월 | 10 | 10% |", false},
	}
	for _, c := range cases {
		h := render(t, chartSrc(`axis2="율"`, c.tbl)).HTML
		lt, rt := leftTicks(h), rightTicks(h)
		if len(lt) != len(rt) || len(lt) < 2 {
			t.Fatalf("%s: 눈금 %v %v", c.name, lt, rt)
		}
		zl, zr := -1, -1
		for k := range lt {
			if lt[k].v == 0 {
				zl = k
			}
			if rt[k].v == 0 {
				zr = k
			}
		}
		if (zl == zr) != c.aligned || zl < 0 {
			t.Errorf("%s: 0 의 칸 왼쪽 %d 오른쪽 %d, 맞춤 기대 %v (%v / %v)", c.name, zl, zr, c.aligned, lt, rt)
		}
		// 오른쪽 축은 값을 덮는다
		for _, d := range comboDotRe.FindAllStringSubmatch(h, -1) {
			if y := num(d[2]); y > lt[0].y+0.1 || y < lt[len(lt)-1].y-0.1 {
				t.Errorf("%s: 점 y %v 가 그림 밖", c.name, y)
			}
		}
	}
}

func TestChartAlignTicks(t *testing.T) {
	for _, n := range []int{1, 3, 4, 5, 6, 9} {
		for k0 := -1; k0 <= n; k0++ {
			for _, r := range [][2]float64{{0, 0}, {0, 1}, {8.2, 12.6}, {-5, 10}, {-9, -1}, {0, 1e15}, {-1e15, 3}, {0.0001, 0.0004}, {-3, 0}, {7, 7}, {-7, -7}} {
				tlo, step, dec := alignTicks(n, k0, r[0], r[1])
				thi := tlo + float64(n)*step
				lo, hi := r[0], r[1]
				if math.IsNaN(tlo) || math.IsInf(tlo, 0) || !(step > 0) || dec < 0 || dec > 6 {
					t.Fatalf("n=%d k0=%d %v: %v %v %d", n, k0, r, tlo, step, dec)
				}
				if tlo > math.Min(lo, 0)+step*1e-6 && lo != hi && tlo > lo+step*1e-6 || thi < hi-step*1e-6 {
					t.Errorf("n=%d k0=%d %v: 눈금 %v~%v 가 값을 덮지 않음", n, k0, r, tlo, thi)
				}
				if math.Abs(tlo/step-math.Round(tlo/step)) > 1e-6 {
					t.Errorf("n=%d k0=%d %v: 시작 %v 가 간격 %v 의 배수가 아님", n, k0, r, tlo, step)
				}
			}
		}
	}
	// 맞출 수 있으면 0 이 k0 칸
	if tlo, step, _ := alignTicks(6, 2, -5, 10); tlo != -2*step {
		t.Errorf("0 이 2 칸이 아님: %v %v", tlo, step)
	}
}

// 받지 않는 조합은 경고 하나 + 차트 없이 표 그대로(line 과 같은 원칙).
func TestChartAxis2Rejects(t *testing.T) {
	nine := "| 항목 |"
	for i := 1; i <= 9; i++ {
		nine += fmt.Sprintf(" c%d |", i)
	}
	nine += "\n|--|" + strings.Repeat("--|", 9) + "\n| x |" + strings.Repeat(" 1 |", 9)
	three := "| a | 매출 | 계획 | 율 |\n|--|--|--|--|\n| x | 1 | 2 | 3% |"
	cases := []struct {
		name, attrs, table, want string
	}{
		{"stacked", `type=stacked axis2="2026"`, twoSeries, "type=stacked"},
		{"hbar", `type=hbar axis2="2026"`, twoSeries, "type=hbar"},
		{"area", `type=area axis2="2026"`, twoSeries, "type=area"},
		{"pie", `type=pie axis2="2026"`, twoSeries, "type=pie"},
		{"donut", `type=donut axis2="2026"`, twoSeries, "type=donut"},
		{"빈 이름", `axis2=""`, twoSeries, "비어"},
		{"없는 이름", `axis2="2027"`, twoSeries, `axis2 "2027" 와 같은 이름의 계열이 없어`},
		{"숫자는 열 번호가 아님", `axis2=1`, twoSeries, `axis2 "1" 와 같은 이름`},
		{"중복", `axis2="율"`, "| a | 율 | 매출 | 율 |\n|--|--|--|--|\n| x | 1 | 2 | 3 |", "2 개"},
		{"첫 열", `axis2="분기"`, twoSeries, "첫 열"},
		{"상한 밖", `axis2="c9"`, nine, "상한(8) 밖"},
		{"왼쪽이 빔", `axis2="값"`, "| a | 값 |\n|--|--|\n| x | 1 |", "왼쪽 축에 그릴 계열이 없어"},
		{"line 과 다른 계열", `line="계획" axis2="율"`, three, `axis2 "율" 와 line "계획" 가 다른 계열`},
		{"line 이름이 틀림", `line="없음" axis2="율"`, three, `line "없음"`},
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
		})
	}
	// 상한 안이면 계열 상한 경고와 함께 그린다. line 과 같은 이름이면 받는다
	if r := render(t, chartSrc(`axis2="c2"`, nine)); len(chartWarnings(r)) != 1 || !strings.Contains(r.HTML, "ax2") {
		t.Errorf("상한 안: %v", r.Warnings)
	}
	// 모르는 type 이 먼저(경고 하나)
	if r := render(t, chartSrc(`type=radar axis2="2026" unit2="%"`, twoSeries)); len(r.Warnings) != 1 || !unknownType(r) {
		t.Errorf("모르는 type: %v", r.Warnings)
	}
}

// 그리되 경고하는 것: unit2 만, 보조 축과 기준선, 오른쪽 축 계열에 숫자가 없음.
func TestChartAxis2Warnings(t *testing.T) {
	// unit2 만: 무시하고 그린다(오른쪽 축 없음)
	r := render(t, chartSrc(`unit2="%"`, twoSeries))
	if ws := chartWarnings(r); len(ws) != 1 || !strings.Contains(ws[0], "unit2 는 axis2") || !strings.Contains(r.HTML, "<svg") || strings.Contains(r.HTML, "ax2") {
		t.Errorf("unit2 만: %v", r.Warnings)
	}
	if h := r.HTML; h != render(t, chartSrc(``, twoSeries)).HTML {
		t.Error("무시한 unit2 가 그림을 바꿈")
	}
	// 기준선과 함께: 기준선 없이 그린다
	r = render(t, chartSrc(`axis2="이익률" ref=40 ref-label="목표"`, axis2Table))
	if ws := chartWarnings(r); len(ws) != 1 || !strings.Contains(ws[0], "기준선(ref)을 긋지 않음") || findRef(t, r.HTML) != nil || !strings.Contains(r.HTML, "ax2") {
		t.Errorf("기준선과 함께: %v", r.Warnings)
	}
	// 오른쪽 축 계열에 숫자가 없음: 오른쪽 축 없이(bar 에서는 빈 선), 왼쪽은 그대로
	r = render(t, chartSrc(`axis2="율" unit2="%"`, "| 월 | 매출 | 율 |\n|--|--|--|\n| 1월 | 3 | - |\n| 2월 | 4 | |"))
	if ws := chartWarnings(r); len(ws) != 1 || !strings.Contains(ws[0], "오른쪽 축 없이") || strings.Contains(r.HTML, "ax2") || strings.Contains(r.HTML, "오른쪽 축") ||
		len(comboColRe.FindAllString(r.HTML, -1)) != 2 {
		t.Errorf("빈 오른쪽 축: %v\n%s", r.Warnings, r.HTML)
	}
	// 왼쪽 축에 숫자가 없음(막대가 모두 빈 값): 경고 없이 0 부터의 왼쪽 축, 선은 오른쪽 축
	r = render(t, chartSrc(`axis2="율"`, "| 월 | 매출 | 율 |\n|--|--|--|\n| 1월 | - | 3% |\n| 2월 | | 5% |"))
	if len(r.Warnings) != 0 || len(rightTicks(r.HTML)) == 0 || len(comboDotRe.FindAllString(r.HTML, -1)) != 2 || len(comboColRe.FindAllString(r.HTML, -1)) != 0 {
		t.Errorf("빈 왼쪽 축: %v", r.Warnings)
	}
}

// 단위 혼합: % 와 일반 숫자가 섞이면 경고한다(축마다). 단위 % 추론도 축마다다.
func TestChartUnitMix(t *testing.T) {
	cases := []struct {
		name, attrs, tbl, want string // want 가 비면 경고 없음
		unit                   string // 오른쪽 축 눈금에 % 가 붙는지(axis2 일 때) 또는 왼쪽 눈금
	}{
		{"섞임", "", "| a | b |\n|--|--|\n| x | 12% |\n| y | 30 |", `% 를 붙인 칸("12%")과 붙이지 않은 칸("30")이 섞여`, ""},
		{"두 계열에 걸쳐 섞임", "", "| a | b | c |\n|--|--|--|\n| x | 12 | 3% |", `("3%")과 붙이지 않은 칸("12")`, ""},
		{"모두 %", "", "| a | b |\n|--|--|\n| x | 12% |\n| y | 30% |", "", "%"},
		{"unit=% 면 섞여도 같음", `unit="%"`, "| a | b |\n|--|--|\n| x | 12% |\n| y | 30 |", "", "%"},
		{"다른 단위에 %", `unit="억 원"`, "| a | b |\n|--|--|\n| x | 12% |\n| y | 30 |", `unit="억 원" 인데 값에 % 를 붙인 칸("12%")`, ""},
		{"축마다 따로", `axis2="율"`, "| a | 매출 | 율 |\n|--|--|--|\n| x | 12 | 3% |\n| y | 30 | 4% |", "", "%"},
		{"오른쪽 축에서 섞임", `axis2="율"`, "| a | 매출 | 율 |\n|--|--|--|\n| x | 12 | 3% |\n| y | 30 | 4 |", `오른쪽 축의 값에 % 를 붙인 칸("3%")과 붙이지 않은 칸("4")`, ""},
		{"왼쪽 축에서 섞임", `axis2="율"`, "| a | 매출 | 율 |\n|--|--|--|\n| x | 12% | 3% |\n| y | 30 | 4% |", `왼쪽 축의 값에`, "%"},
		{"unit2 가 다른 단위", `axis2="율" unit2="건"`, "| a | 매출 | 율 |\n|--|--|--|\n| x | 12 | 3% |", `unit2="건" 인데 오른쪽 축의 값에 % 를 붙인 칸("3%")`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := render(t, chartSrc(c.attrs, c.tbl))
			ws := chartWarnings(r)
			if c.want == "" && len(r.Warnings) != 0 || c.want != "" && (len(ws) != 1 || !strings.Contains(ws[0], c.want)) {
				t.Errorf("경고 %v, 기대 %q", r.Warnings, c.want)
			}
			if !strings.Contains(r.HTML, "<svg") {
				t.Fatal("그림이 없음 — 단위 혼합은 그리되 경고한다")
			}
			tk := regexp.MustCompile(`class="tick">([^<]*)</text>`).FindAllStringSubmatch(r.HTML, -1)
			if strings.Contains(c.attrs, "axis2") {
				tk = tick2Re.FindAllStringSubmatch(r.HTML, -1)
				for i := range tk {
					tk[i][1] = tk[i][3]
				}
			}
			if got := strings.HasSuffix(tk[len(tk)-1][1], "%"); got != (c.unit == "%") {
				t.Errorf("눈금 %q 의 %% %v, 기대 %q", tk[len(tk)-1][1], got, c.unit)
			}
		})
	}
}

// 표시 정밀도: 0 이 아닌 값이 표의 소수 자리(3 까지)로 0 이 되면 그 값만 첫 유효 숫자까지(6 까지) 적는다.
func TestChartPrecision(t *testing.T) {
	for _, c := range []struct {
		v   float64
		dec int
		out string
	}{{0.0004, 3, "0.0004"}, {-0.00004, 3, "-0.00004"}, {0.00049, 1, "0.0005"}, {1.5, 3, "1.500"}, {0, 3, "0.000"}, {0.0000001, 3, "0.000000"}, {1234.6, 0, "1,235"}, {0.05, 1, "0.1"}} {
		if got := fmtVal(c.v, c.dec); got != c.out {
			t.Errorf("fmtVal(%v, %d) = %q, 기대 %q", c.v, c.dec, got, c.out)
		}
	}
	// 0.0004 는 "0.000" 이 아니라 "0.0004"(값 글자·title), 다른 값은 세 자리 그대로
	r := render(t, chartSrc("type=bar", "| a | 율 |\n|--|--|\n| x | 0.0004 |\n| y | 0.125 |"))
	if !strings.Contains(r.HTML, ">0.0004</text>") || !strings.Contains(r.HTML, "x: 0.0004</title>") || !strings.Contains(r.HTML, ">0.125</text>") || strings.Contains(r.HTML, `class="val d0">0.000<`) {
		t.Errorf("0.0004\n%s", r.HTML)
	}
	// 보조 축의 값도, 기준선 라벨도
	r = render(t, chartSrc(`axis2="율"`, "| a | 매출 | 율 |\n|--|--|--|\n| x | 3 | 0.00002 |\n| y | 4 | 0.5 |"))
	if !strings.Contains(r.HTML, "율: 0.00002</title>") {
		t.Errorf("보조 축 정밀도\n%s", r.HTML)
	}
	r = render(t, chartSrc("type=bar ref=0.0001", "| a | 율 |\n|--|--|\n| x | 0.5 |"))
	if rl := findRef(t, r.HTML); rl == nil || rl.label != "0.0001" {
		t.Errorf("기준선 라벨 %+v", rl)
	}
}

// 최대 행·계열·긴 이름·긴 숫자: 좌표가 유한하고 크기가 음수가 아니며, 점·오른쪽 눈금·값 글자는 그림 안이다.
func TestChartAxis2Geometry(t *testing.T) {
	longCat := strings.Repeat("아주긴항목이름", 6)
	attrRe := regexp.MustCompile(` (x|y|x1|y1|x2|y2|cx|cy|width|height|r|rx)="([^"]*)"`)
	for _, typ := range []string{"bar", "line"} {
		for _, rows := range []int{1, 2, 12, 30, 60} {
			for _, series := range []int{2, 3, 8} {
				for _, at := range []int{1, series} {
					for _, scale := range []string{"1", "1e12", "0.0001", "-1"} {
						var b strings.Builder
						b.WriteString("| 항목 |")
						for s := 1; s <= series; s++ {
							fmt.Fprintf(&b, " 계열 %d %s |", s, strings.Repeat("이름", s))
						}
						b.WriteString("\n|--|" + strings.Repeat("--|", series) + "\n")
						f, _ := strconv.ParseFloat(scale, 64)
						for i := 0; i < rows; i++ {
							fmt.Fprintf(&b, "| %s%d |", longCat, i)
							for s := 0; s < series; s++ {
								v := float64((i*37+s*101)%1000 - 300)
								if s+1 == at {
									v *= f
								}
								fmt.Fprintf(&b, " %s |", strconv.FormatFloat(v, 'f', -1, 64))
							}
							b.WriteString("\n")
						}
						name := fmt.Sprintf("%s rows=%d series=%d at=%d scale=%s", typ, rows, series, at, scale)
						r := render(t, chartSrc(fmt.Sprintf(`type=%s axis2="계열 %d %s" unit2="%%"`, typ, at, strings.Repeat("이름", at)), b.String()))
						if len(r.Warnings) != 0 {
							t.Errorf("%s: 경고 %v", name, r.Warnings)
							continue
						}
						for _, m := range attrRe.FindAllStringSubmatch(r.HTML, -1) {
							v, err := strconv.ParseFloat(m[2], 64)
							if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
								t.Errorf("%s: %s=%q", name, m[1], m[2])
							} else if (m[1] == "width" || m[1] == "height" || m[1] == "r" || m[1] == "rx") && v < 0 {
								t.Errorf("%s: 음수 크기 %s=%v", name, m[1], v)
							}
						}
						lt, rt := leftTicks(r.HTML), rightTicks(r.HTML)
						if len(rt) != len(lt) || len(rt) < 2 {
							t.Errorf("%s: 눈금 %d/%d", name, len(lt), len(rt))
							continue
						}
						top, bottom := lt[len(lt)-1].y, lt[0].y
						for _, d := range anyDotRe.FindAllStringSubmatch(r.HTML, -1) {
							if x, y := num(d[1]), num(d[2]); x < 0 || x > cvW || y < top-0.1 || y > bottom+0.1 {
								t.Errorf("%s: 점 (%v,%v) 가 그림(%v~%v) 밖", name, x, y, top, bottom)
							}
						}
						for _, m := range tick2Re.FindAllStringSubmatch(r.HTML, -1) {
							if x := num(m[1]); x+textW(m[3], fsAxis) > cvW+0.1 || x < cvW/2 {
								t.Errorf("%s: 오른쪽 눈금 %q 가 그림 밖(x %v)", name, m[3], x)
							}
						}
						// 값 글자는 그림 영역(격자선 끝 x2) 안 — 오른쪽 눈금 글자(x2+6 부터)와 겹치지 않게
						x2 := num(gridEndRe.FindStringSubmatch(r.HTML)[1])
						for _, l := range comboVals(r.HTML) {
							if bx := l.box(); bx[0] < -0.1 || bx[2] > x2+0.1 {
								t.Errorf("%s: 값 %q 칸 %v 가 그림(0~%v) 밖", name, l.t, bx, x2)
							}
						}
					}
				}
			}
		}
	}
}

// 리뷰 R1: 단위 줄("단위: … · 오른쪽 축 단위: …")과 범례(이름 뒤 "(오른쪽 축)")가 함께 길면 첫 범례가 단위 줄 위에 겹쳐 놓였다.
// 첫 항목도 단위 곁에 들지 않으면 범례는 다음 줄부터다. 단위·범례 글자와 표식이 서로 겹치지 않고, 그림(가장 위 격자선)은 그 아래다.
func TestChartAxis2Header(t *testing.T) {
	unitRe := regexp.MustCompile(`<text x="([-\d.]+)" y="([-\d.]+)" font-size="12" text-anchor="start" class="unit">([^<]*)</text>`)
	lgRe := regexp.MustCompile(`<text x="([-\d.]+)" y="([-\d.]+)" font-size="12" text-anchor="start" class="lg">([^<]*)</text>`)
	swRe := regexp.MustCompile(`<rect x="([-\d.]+)" y="([-\d.]+)" width="([-\d.]+)" height="[-\d.]+" rx="[-\d.]+" class="sw `)
	long := "인구 10만 명당 연령 표준화 발생률"
	for _, typ := range []string{"bar", "line"} {
		for _, first := range []bool{true, false} {
			for _, units := range [][2]string{{"백만 미국 달러(구매력 기준)", "인구 10만 명당 발생 건수"}, {"억 원", "%"}, {"", "건"}, {strings.Repeat("아주 긴 단위", 12), strings.Repeat("더 긴 단위", 12)}} {
				for _, name := range []string{long, "율"} {
					hdr := "| a | " + name + " | 보건 지출 |"
					if !first {
						hdr = "| a | 보건 지출 | " + name + " |"
					}
					tbl := hdr + "\n|--|--|--|\n| x | 3 | 100 |\n| y | 5 | 200 |"
					attrs := fmt.Sprintf(`type=%s unit="%s" axis2="%s" unit2="%s"`, typ, units[0], name, units[1])
					r := render(t, chartSrc(attrs, tbl))
					if len(r.Warnings) != 0 {
						t.Fatalf("%s: %v", attrs, r.Warnings)
					}
					type box struct {
						what string
						b    [4]float64
					}
					var boxes []box
					for _, m := range unitRe.FindAllStringSubmatch(r.HTML, -1) {
						x, y := num(m[1]), num(m[2])
						boxes = append(boxes, box{"단위 " + m[3], [4]float64{x, y - 10, x + textW(m[3], fsAxis), y + 3}})
					}
					for _, m := range lgRe.FindAllStringSubmatch(r.HTML, -1) {
						x, y := num(m[1]), num(m[2])
						boxes = append(boxes, box{"범례 " + m[3], [4]float64{x, y - 10, x + textW(m[3], fsLegend), y + 3}})
					}
					for _, m := range swRe.FindAllStringSubmatch(r.HTML, -1) {
						x, y := num(m[1]), num(m[2])
						boxes = append(boxes, box{"표식", [4]float64{x, y, x + num(m[3]), y + 2}})
					}
					name := fmt.Sprintf("%s first=%v %v %s", typ, first, units, name)
					bottom := 0.0
					for i, a := range boxes {
						if a.b[0] < -0.1 || a.b[2] > cvW+0.1 {
							t.Errorf("%s: %s 가 그림 밖 %v", name, a.what, a.b)
						}
						bottom = math.Max(bottom, a.b[3])
						for _, c := range boxes[i+1:] {
							if overlaps(a.b, c.b, 0) && !(a.what == "표식" || c.what == "표식") || overlaps(a.b, c.b, 0) && strings.HasPrefix(a.what+c.what, "단위") {
								t.Errorf("%s: %q 와 %q 가 겹침", name, a.what, c.what)
							}
						}
					}
					if lt := leftTicks(r.HTML); lt[len(lt)-1].y-12 < bottom-0.1 {
						t.Errorf("%s: 가장 위 눈금 %v 이 머리 줄 아래 끝 %v 위", name, lt[len(lt)-1].y, bottom)
					}
				}
			}
		}
	}
}

// 리뷰 R1: 도넛 가운데 합계도 표시 정밀도를 따른다(0.0001 + 0.0002 가 "0.000" 이었다).
func TestChartPrecisionDonutTotal(t *testing.T) {
	h := render(t, chartSrc("type=donut", "| a | b |\n|--|--|\n| x | 0.0001 |\n| y | 0.0002 |")).HTML
	if !strings.Contains(h, `class="total">0.0003</text>`) || !strings.Contains(h, ">0.0001</text>") {
		t.Errorf("도넛 합계\n%s", h)
	}
	// 단위는 그대로 가운데 아래에. 0 으로 보이지 않는 합계는 표의 자리 그대로(2.0001 → 2.000 — 늘리는 것은 0 이 되는 값만)
	h = render(t, chartSrc(`type=donut unit="건"`, "| a | b |\n|--|--|\n| x | 0.0001 |\n| y | 2 |")).HTML
	if !strings.Contains(h, `class="total">2.000</text>`) || !strings.Contains(h, ">합계 · 건</text>") {
		t.Errorf("도넛 합계 단위\n%s", h)
	}
}
