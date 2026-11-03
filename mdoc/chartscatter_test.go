package mdoc

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
)

// 이슈 itda-hyve#114 단계 D: 산점도 scatter·버블 bubble — 표 꼴 라벨 | x | y [| 크기], x·y 연속 축, 버블 넓이 ∝ 크기.

var (
	scDotRe   = regexp.MustCompile(`<circle cx="([-\d.]+)" cy="([-\d.]+)" r="([-\d.]+)" class="dot (sc|bub) s1 d\d+"><title>([^<]*)</title></circle>`)
	scLegRe   = regexp.MustCompile(`<circle cx="([-\d.]+)" cy="([-\d.]+)" r="([-\d.]+)" class="sw bub s1"></circle><text [^>]*class="tick">([^<]*)</text>`)
	scLabRe   = regexp.MustCompile(`<text x="([-\d.]+)" y="([-\d.]+)" font-size="11" text-anchor="(\w+)" class="val plab d\d+">([^<]*)`)
	scTitleRe = regexp.MustCompile(`<text x="([-\d.]+)" y="([-\d.]+)" font-size="12" text-anchor="(\w+)" class="unit axt">([^<]*)</text>`)
	scLineRe  = regexp.MustCompile(`<line x1="([-\d.]+)" y1="([-\d.]+)" x2="([-\d.]+)" y2="([-\d.]+)" class="gl(?: zero)?"></line>`)
)

type scDot struct {
	cx, cy, r float64
	kind, tip string
}

func scDots(h string) []scDot {
	var out []scDot
	for _, m := range scDotRe.FindAllStringSubmatch(h, -1) {
		out = append(out, scDot{num(m[1]), num(m[2]), num(m[3]), m[4], m[5]})
	}
	return out
}

// scTicks 는 눈금(격자선 + 글자)이다: 가로선은 y 눈금(값, 높이), 세로선은 x 눈금(값, 가로 자리). 글자를 건너뛴 x 눈금은 빠진다.
func scTicks(h string) (xs, ys []axisTick) {
	for _, m := range gridRe.FindAllStringSubmatch(h, -1) {
		if m[2] == m[4] {
			ys = append(ys, axisTick{parseTick(m[5]), num(m[2])})
		} else {
			xs = append(xs, axisTick{parseTick(m[5]), num(m[1])})
		}
	}
	return xs, ys
}

// scPlot 은 그림 영역(격자선의 끝)이다.
func scPlot(h string) (x0, y0, x1, y1 float64) {
	x0, y0, x1, y1 = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, m := range scLineRe.FindAllStringSubmatch(h, -1) {
		x0, x1 = math.Min(x0, math.Min(num(m[1]), num(m[3]))), math.Max(x1, math.Max(num(m[1]), num(m[3])))
		y0, y1 = math.Min(y0, math.Min(num(m[2]), num(m[4]))), math.Max(y1, math.Max(num(m[2]), num(m[4])))
	}
	return
}

func near(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

const scTable = "| 지점 | 매출(억 원) | 이익률 |\n|:--|--:|--:|\n| 본점 | 10 | 4.0% |\n| 강남 | 20 | 8.5% |\n| 판교 | 40 | 6.0% |\n| 부산 | 75 | 12.0% |\n\n: 지점 분포"

const bubTable = "| 제품 | 성장률 | 점유율 | 매출 |\n|:--|--:|--:|--:|\n| 가 | 18% | 12% | 100 |\n| 나 | 9% | 31% | 25 |\n| 다 | 3% | 22% | 4 |\n| 라 | -2% | 8% | 49 |\n\n: 포트폴리오"

func TestChartScatter(t *testing.T) {
	r := render(t, chartSrc("type=scatter", scTable))
	assertSafe(t, r.HTML)
	if len(r.Warnings) != 0 {
		t.Fatalf("경고 %v", r.Warnings)
	}
	h := r.HTML
	for _, want := range []string{`<figure class="chart chart-scatter"`, `class="chart-svg t-scatter"`, `aria-label="지점 분포"`, "<title>본점 · 매출(억 원): 10 · 이익률: 4.0%</title>"} {
		if !strings.Contains(h, want) {
			t.Errorf("%q 없음\n%s", want, h)
		}
	}
	if strings.Contains(h, `class="legend"`) || strings.Contains(h, "<table") {
		t.Error("산점도에 범례·표")
	}
	dots := scDots(h)
	if len(dots) != 4 {
		t.Fatalf("점 %d 개\n%s", len(dots), h)
	}
	// 연속 x: 10·20·40·75 의 가로 간격이 값의 차에 비례한다(항목처럼 같은 간격이 아니다)
	unit := (dots[1].cx - dots[0].cx) / 10
	for i, want := range []float64{10, 20, 40, 75} {
		if got := dots[0].cx + (want-10)*unit; !near(dots[i].cx, got, 0.15) {
			t.Errorf("점 %d cx %.1f, 기대 %.1f(x 간격 비례)", i, dots[i].cx, got)
		}
	}
	// 눈금 격자와 같은 자리: 점의 cx·cy 는 눈금(첫·끝 사이 선형)으로 셈한 자리
	xs, ys := scTicks(h)
	if len(xs) < 2 || len(ys) < 2 {
		t.Fatalf("눈금 x %d · y %d", len(xs), len(ys))
	}
	for i, v := range [][2]float64{{10, 4}, {20, 8.5}, {40, 6}, {75, 12}} {
		if x := yAt(xs, v[0]); !near(dots[i].cx, x, 0.15) {
			t.Errorf("점 %d cx %.1f, 눈금으로 %.1f", i, dots[i].cx, x)
		}
		if y := yAt(ys, v[1]); !near(dots[i].cy, y, 0.15) {
			t.Errorf("점 %d cy %.1f, 눈금으로 %.1f", i, dots[i].cy, y)
		}
	}
	// 축 제목 = 열 머리글: y 는 왼쪽 위, x 는 아래 가운데. y 눈금은 % 가 붙고 x 눈금은 붙지 않는다(열마다 % 추론)
	titles := scTitleRe.FindAllStringSubmatch(h, -1)
	if len(titles) != 2 || titles[0][4] != "이익률" || titles[1][4] != "매출(억 원)" || titles[1][3] != "middle" {
		t.Errorf("축 제목 %v", titles)
	}
	if !strings.Contains(h, `class="tick">12%</text>`) || strings.Contains(h, `text-anchor="middle" class="tick">20%</text>`) {
		t.Errorf("열마다 %% 단위\n%s", h)
	}
	// 점 라벨 넷(겹칠 것이 없다), labels=off 면 없다
	if n := len(scLabRe.FindAllStringSubmatch(h, -1)); n != 4 {
		t.Errorf("점 라벨 %d 개", n)
	}
	if off := render(t, chartSrc("type=scatter labels=off", scTable)).HTML; strings.Contains(off, "plab") || len(scDots(off)) != 4 {
		t.Error("labels=off")
	}
	// 대소문자 무시, table=below 는 그림 뒤에 표
	if r := render(t, chartSrc("type=Scatter table=below", scTable)); !strings.Contains(r.HTML, "chart-scatter") || !strings.Contains(r.HTML, ">본점</td>") || len(r.Warnings) != 0 {
		t.Errorf("type=Scatter table=below: %v", r.Warnings)
	}
	// x 가 0 에서 멀면 0 부터 그리지 않는다(꺾은선의 규칙): 100~130
	far := render(t, chartSrc("type=scatter", "| a | x | y |\n|--|--|--|\n| p | 100 | 1 |\n| q | 130 | 2 |")).HTML
	if xs, _ := scTicks(far); len(xs) == 0 || xs[0].v < 90 {
		t.Errorf("0 에서 먼 x 의 첫 눈금 %v", xs)
	}
}

// 버블: 넓이 ∝ 크기(가장 큰 것의 반지름 bubbleRMax), 큰 것부터 그리고, 크기 범례는 가장 큰 값과 4분의 1 쯤의 보기 좋은 값.
func TestChartBubble(t *testing.T) {
	r := render(t, chartSrc("type=bubble", bubTable))
	assertSafe(t, r.HTML)
	if len(r.Warnings) != 0 {
		t.Fatalf("경고 %v", r.Warnings)
	}
	h := r.HTML
	dots := scDots(h)
	if len(dots) != 4 {
		t.Fatalf("버블 %d 개\n%s", len(dots), h)
	}
	size := map[string]float64{"가": 100, "나": 25, "다": 4, "라": 49}
	var prev = math.Inf(1)
	for _, d := range dots {
		name := strings.SplitN(d.tip, " · ", 2)[0]
		v := size[name]
		if want := bubbleRMax * math.Sqrt(v/100); !near(d.r, want, 0.06) {
			t.Errorf("%s 반지름 %.2f, 기대 %.2f(넓이 ∝ 크기)", name, d.r, want)
		}
		if v > prev {
			t.Errorf("버블을 큰 것부터 그리지 않음: %v 뒤에 %v", prev, v)
		}
		prev = v
		if d.kind != "bub" {
			t.Errorf("클래스 %s", d.kind)
		}
	}
	// 넓이 비율 = 크기 비율(반올림 0.1 안에서)
	for _, a := range dots {
		for _, b := range dots {
			va, vb := size[strings.SplitN(a.tip, " · ", 2)[0]], size[strings.SplitN(b.tip, " · ", 2)[0]]
			if got := a.r * a.r / (b.r * b.r); math.Abs(got-va/vb) > 0.02*va/vb+0.01 {
				t.Errorf("넓이 비 %.3f, 크기 비 %.3f", got, va/vb)
			}
		}
	}
	if !strings.Contains(h, "<title>가 · 성장률: 18% · 점유율: 12% · 매출: 100</title>") {
		t.Errorf("title\n%s", h)
	}
	leg := scLegRe.FindAllStringSubmatch(h, -1)
	if len(leg) != 2 || leg[0][4] != "100" || leg[1][4] != "25" || !near(num(leg[0][3]), bubbleRMax, 0.01) || !near(num(leg[1][3]), bubbleRMax/2, 0.06) {
		t.Errorf("크기 범례 %v", leg)
	}
	if !strings.Contains(h, ">넓이: 매출</text>") {
		t.Error("크기 범례 이름")
	}
	// 보기 좋은 값은 그 값의 자리로: 최대 10 → 2.5(3 으로 반올림하지 않음)
	h2 := render(t, chartSrc("type=bubble", "| a | x | y | s |\n|--|--|--|--|\n| p | 1 | 1 | 10 |\n| q | 2 | 2 | 3 |")).HTML
	if leg := scLegRe.FindAllStringSubmatch(h2, -1); len(leg) != 2 || leg[1][4] != "2.5" {
		t.Errorf("범례 %v", leg)
	}
	// 아주 작은 크기는 최소 반지름(보이게)
	h3 := render(t, chartSrc("type=bubble", "| a | x | y | s |\n|--|--|--|--|\n| p | 1 | 1 | 100000 |\n| q | 2 | 2 | 1 |")).HTML
	if d := scDots(h3); len(d) != 2 || d[1].r != bubbleRMin {
		t.Errorf("작은 버블 %v", d)
	}
}

// 빈 값·0·음수·한 점·같은 좌표·모두 같은 값.
func TestChartScatterValues(t *testing.T) {
	// 한 좌표라도 비면 그 행은 없다(경고 없음). 0·음수 좌표는 그린다
	r := render(t, chartSrc("type=scatter", "| a | x | y |\n|--|--|--|\n| p | 1 | |\n| q | - | 3 |\n| r | 0 | -2 |\n| s | -5 | 0 |"))
	if d := scDots(r.HTML); len(d) != 2 || len(r.Warnings) != 0 || !strings.Contains(r.HTML, "r · x: 0 · y: -2") {
		t.Errorf("빈 좌표: %d 점 %v", len(d), r.Warnings)
	}
	// 0·음수 좌표가 있으면 0 격자선(gl zero)이 가로·세로로
	if n := strings.Count(r.HTML, `class="gl zero"`); n != 2 {
		t.Errorf("0 격자선 %d 개", n)
	}
	// 버블: 빈 크기·0 은 경고 없이 빼고, 음수는 경고하고 뺀다
	r = render(t, chartSrc("type=bubble", "| a | x | y | s |\n|--|--|--|--|\n| p | 1 | 1 | |\n| q | 2 | 2 | 0 |\n| 음수점 | 3 | 3 | -4 |\n| t | 4 | 4 | 9 |"))
	ws := chartWarnings(r)
	if d := scDots(r.HTML); len(d) != 1 || len(ws) != 1 || !strings.Contains(ws[0], `"음수점"`) || !strings.Contains(ws[0], "음수") {
		t.Errorf("버블 크기: %d 점 %v", len(d), ws)
	}
	// 한 점, 같은 좌표 둘(둘 다 그리고 title 도 둘), 모두 같은 값
	for _, tbl := range []string{
		"| a | x | y |\n|--|--|--|\n| 하나 | 5 | 5 |",
		"| a | x | y |\n|--|--|--|\n| 갑 | 3 | 4 |\n| 을 | 3 | 4 |",
		"| a | x | y |\n|--|--|--|\n| 갑 | 0 | 0 |\n| 을 | 0 | 0 |",
		"| a | x | y |\n|--|--|--|\n| 갑 | -7 | -7 |",
	} {
		for _, typ := range []string{"scatter", "bubble"} {
			src := tbl
			if typ == "bubble" {
				src = strings.Replace(strings.Replace(src, "| y |", "| y | s |", 1), "|--|--|--|", "|--|--|--|--|", 1)
				src = regexp.MustCompile(`(?m)^(\| [^|]+ \| [^|]+ \| [^|]+ \|)$`).ReplaceAllString(src, "$1 7 |")
			}
			r := render(t, chartSrc("type="+typ, src))
			rows := strings.Count(src, "\n") - 1
			d := scDots(r.HTML)
			if len(d) != rows || len(r.Warnings) != 0 {
				t.Errorf("%s %q: 점 %d(행 %d) %v", typ, src, len(d), rows, r.Warnings)
				continue
			}
			if len(d) == 2 && (d[0].cx != d[1].cx || d[0].cy != d[1].cy || d[0].tip == d[1].tip) {
				t.Errorf("같은 좌표: %v", d)
			}
			x0, y0, x1, y1 := scPlot(r.HTML)
			for _, p := range d {
				if p.cx < x0-0.1 || p.cx > x1+0.1 || p.cy < y0-0.1 || p.cy > y1+0.1 {
					t.Errorf("%s: 점 %v 가 그림 영역(%v,%v)~(%v,%v) 밖", typ, p, x0, y0, x1, y1)
				}
			}
			// 같은 좌표의 라벨 둘은 서로 다른 자리
			labs := scLabRe.FindAllStringSubmatch(r.HTML, -1)
			if len(labs) == 2 && labs[0][1] == labs[1][1] && labs[0][2] == labs[1][2] {
				t.Errorf("같은 좌표의 라벨이 같은 자리 %v", labs)
			}
		}
	}
}

// 받지 않는 것(표 그대로)과 그리되 경고하는 것.
func TestChartScatterWarnings(t *testing.T) {
	two := "| a | x |\n|--|--|\n| p | 1 |"
	cases := []struct {
		name, src string
		want      []string // 경고마다 들어갈 글자(차례대로)
		drawn     bool
	}{
		{"열이 둘", chartSrc("type=scatter", two), []string{"라벨·x·y 세 열"}, false},
		{"버블 열이 셋", chartSrc("type=bubble", scTable), []string{"라벨·x·y·크기 네 열"}, false},
		{"넷째 열", chartSrc("type=scatter", bubTable), []string{"4 번째 열부터"}, true},
		{"unit", chartSrc(`type=scatter unit="억 원"`, scTable), []string{"unit 을 쓰지 않아"}, true},
		{"ref", chartSrc(`type=scatter ref=10 ref-label="목표"`, scTable), []string{"기준선(ref)을 긋지 않음"}, true},
		{"ref-label 만", chartSrc(`type=bubble ref-label="목표"`, bubTable), []string{"ref-label 은 ref"}, true},
		{"line", chartSrc(`type=scatter line="이익률"`, scTable), []string{"bar 에만"}, false},
		{"axis2", chartSrc(`type=bubble axis2="매출"`, bubTable), []string{"bar·line 에만"}, false},
		{"unit2 만", chartSrc(`type=scatter unit2="%"`, scTable), []string{"unit2 는 axis2"}, true},
		{"읽을 행 없음", chartSrc("type=scatter", "| a | x | y |\n|--|--|--|\n| p | 1 | |\n| q | | 2 |"), []string{"x·y 를 모두 읽을 수 있는 행이 없어"}, false},
		{"버블 크기 모두 0", chartSrc("type=bubble", "| a | x | y | s |\n|--|--|--|--|\n| p | 1 | 1 | 0 |"), []string{"x·y·크기 를 모두"}, false},
		{"읽지 못한 칸", chartSrc("type=scatter", "| a | x | y |\n|--|--|--|\n| p | 1 | 2 |\n| q | 약 3 | 2 |"), []string{`"약 3"`}, true},
		{"% 섞임", chartSrc("type=scatter", "| a | x | 율 |\n|--|--|--|\n| p | 1 | 2% |\n| q | 2 | 3 |"), []string{`y 열(율)`}, true},
		{"점 상한", chartSrc("type=scatter", "| a | x | y |\n|--|--|--|"+strings.Repeat("\n| p | 1 | 2 |", chartMaxPoints+1)), []string{fmt.Sprintf("앞의 %d 개만", chartMaxPoints)}, true},
		{"버블 상한", chartSrc("type=bubble", "| a | x | y | s |\n|--|--|--|--|"+strings.Repeat("\n| p | 1 | 2 | 3 |", chartMaxBubbles+1)), []string{fmt.Sprintf("앞의 %d 개만", chartMaxBubbles)}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := render(t, c.src)
			assertSafe(t, r.HTML)
			ws := chartWarnings(r)
			if len(ws) != len(c.want) || len(r.Warnings) != len(c.want) {
				t.Fatalf("경고 %v, 기대 %v", r.Warnings, c.want)
			}
			for i, w := range c.want {
				if !strings.Contains(ws[i], w) {
					t.Errorf("경고 %q 에 %q 없음", ws[i], w)
				}
			}
			if drawn := strings.Contains(r.HTML, "<svg"); drawn != c.drawn {
				t.Errorf("그림 %v, 기대 %v\n%s", drawn, c.drawn, r.HTML)
			}
			if !c.drawn && !strings.Contains(r.HTML, "<table") && !strings.Contains(r.HTML, `<div class="chart"`) {
				t.Errorf("표 그대로가 아님\n%s", r.HTML)
			}
		})
	}
	// 상한: 앞의 것만 그린다
	h := render(t, chartSrc("type=scatter", "| a | x | y |\n|--|--|--|"+strings.Repeat("\n| p | 1 | 2 |", 250))).HTML
	if n := len(scDots(h)); n != chartMaxPoints {
		t.Errorf("점 %d 개", n)
	}
	// 기준선 경고가 있어도 그림은 기준선 없이 같다
	if a, b := render(t, chartSrc("type=scatter ref=10", scTable)).HTML, render(t, chartSrc("type=scatter", scTable)).HTML; a != b || strings.Contains(a, "ref-line") {
		t.Error("ref 를 무시하지 않음")
	}
}

// 점 라벨: 그림 영역 안이고 서로 겹치지 않으며 다른 점의 가운데를 덮지 않는다. 긴 라벨은 … + title.
func TestChartScatterLabels(t *testing.T) {
	var b strings.Builder
	b.WriteString("| 고객 | 방문 | 구매 | 회원 |\n|--|--|--|--|\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "| 고객 이름이 꽤 긴 사람 %d | %d | %d | %d |\n", i, (i*37)%100, (i*53)%90-20, (i*17)%60+1)
	}
	for _, typ := range []string{"scatter", "bubble"} {
		h := render(t, chartSrc("type="+typ, b.String())).HTML
		x0, y0, _, y1 := scPlot(h)
		labs := scLabRe.FindAllStringSubmatch(h, -1)
		if len(labs) == 0 {
			t.Fatalf("%s: 라벨 없음", typ)
		}
		var boxes [][4]float64
		for _, m := range labs {
			l := valLab{num(m[1]), num(m[2]), m[3], "", m[4]}
			bx := l.box()
			if bx[0] < x0 || bx[2] > cvW || bx[1] < y0-6.1 || bx[3] > y1 {
				t.Errorf("%s: 라벨 %q 칸 %v 가 그림 영역 밖", typ, m[4], bx)
			}
			if !strings.HasSuffix(m[4], "…") && strings.Contains(m[4], "꽤 긴") {
				if textW(m[4], fsVal) > 110 {
					t.Errorf("%s: 긴 라벨을 줄이지 않음 %q", typ, m[4])
				}
			}
			for _, o := range boxes {
				if overlaps(bx, o, 0) {
					t.Errorf("%s: 라벨 %q 가 다른 라벨과 겹침", typ, m[4])
				}
			}
			boxes = append(boxes, bx)
			for _, d := range scDots(h) {
				if inside := bx[0] < d.cx && d.cx < bx[2] && bx[1] < d.cy && d.cy < bx[3]; inside {
					t.Errorf("%s: 라벨 %q 가 점 가운데(%v,%v)를 덮음", typ, m[4], d.cx, d.cy)
				}
			}
		}
		if !strings.Contains(h, "…<title>고객 이름이 꽤 긴 사람") {
			t.Errorf("%s: 줄인 라벨의 title 없음", typ)
		}
	}
}

// 최대 점 수·긴 라벨·큰/작은/음수 값에서 좌표가 유한하고 크기가 음수가 아니며, 점(버블은 원 전체)·라벨·축 제목이 그림 안이다.
func TestChartScatterGeometry(t *testing.T) {
	long := strings.Repeat("아주긴이름", 8)
	tbl := func(typ string, rows int, scale float64, label string) string {
		var b strings.Builder
		b.WriteString("| " + long + " | " + long + " x | " + long + " y |")
		if typ == "bubble" {
			b.WriteString(" " + long + " s |")
		}
		b.WriteString("\n|--|--|--|--|\n")
		for i := 0; i < rows; i++ {
			x := float64((i*37)%100-30) * scale
			y := float64((i*53)%90-20) * scale
			fmt.Fprintf(&b, "| %s%d | %s | %s |", label, i, fmtNum(x, 4), fmtNum(y, 4))
			if typ == "bubble" {
				fmt.Fprintf(&b, " %s |", fmtNum(math.Abs(scale)*float64(i%13+1), 4))
			}
			b.WriteString("\n")
		}
		return b.String()
	}
	attrRe := regexp.MustCompile(` (x|y|x1|y1|x2|y2|cx|cy|width|height|r|rx)="([^"]*)"`)
	viewRe := regexp.MustCompile(`viewBox="0 0 ([\d.]+) ([\d.]+)"`)
	for _, typ := range []string{"scatter", "bubble"} {
		limit := chartMaxPoints
		if typ == "bubble" {
			limit = chartMaxBubbles
		}
		for _, rows := range []int{1, 2, 13, limit} {
			for _, scale := range []float64{1, 1e10, 0.001, -1} {
				for _, label := range []string{"점", long} {
					name := fmt.Sprintf("%s rows=%d scale=%g label=%d", typ, rows, scale, len(label))
					r := render(t, chartSrc("type="+typ, tbl(typ, rows, scale, label)))
					h := r.HTML
					vb := viewRe.FindStringSubmatch(h)
					if vb == nil {
						t.Errorf("%s: 그림 없음 %v", name, r.Warnings)
						continue
					}
					w, hh := num(vb[1]), num(vb[2])
					for _, m := range attrRe.FindAllStringSubmatch(h, -1) {
						v := num(m[2])
						if strings.ContainsAny(m[2], "NaInf") || math.IsNaN(v) || math.IsInf(v, 0) {
							t.Errorf("%s: %s=%q", name, m[1], m[2])
						}
						if (m[1] == "r" || m[1] == "width" || m[1] == "height" || m[1] == "rx") && v < 0 {
							t.Errorf("%s: 음수 크기 %s=%v", name, m[1], v)
						}
					}
					dots := scDots(h)
					if len(dots) != rows {
						t.Errorf("%s: 점 %d 개 %v", name, len(dots), r.Warnings)
					}
					x0, y0, x1, y1 := scPlot(h)
					for _, d := range dots {
						rr := 0.0
						if typ == "bubble" {
							rr = d.r // 버블은 원 전체가 그림 영역 안
						}
						if d.cx-rr < x0-0.15 || d.cx+rr > x1+0.15 || d.cy-rr < y0-0.15 || d.cy+rr > y1+0.15 {
							t.Errorf("%s: 점 (%v,%v) r=%v 가 그림 영역 (%v,%v)~(%v,%v) 밖", name, d.cx, d.cy, d.r, x0, y0, x1, y1)
							break
						}
					}
					for _, m := range scLabRe.FindAllStringSubmatch(h, -1) {
						bx := valLab{num(m[1]), num(m[2]), m[3], "", m[4]}.box()
						if bx[0] < 0 || bx[2] > w || bx[1] < 0 || bx[3] > hh {
							t.Errorf("%s: 라벨 %q 가 그림 밖", name, m[4])
						}
					}
					for _, m := range scTitleRe.FindAllStringSubmatch(h, -1) {
						tw := textW(m[4], fsAxis)
						left := num(m[1]) - tw*map[string]float64{"start": 0, "middle": 0.5, "end": 1}[m[3]]
						if right := left + tw; left < -0.1 || right > w+0.1 {
							t.Errorf("%s: 축 제목 %q 가 그림 밖(%v~%v)", name, m[4], left, right)
						}
					}
				}
			}
		}
	}
}

// 리뷰 R1: (1) x 가 크고 y 가 작으면 가운데 맞춤인 첫 x 눈금 글자가 그림 왼쪽 밖으로 나갔다(-1,000,000,000,000,000 이 -40.8 부터).
// (2) 버블의 크기 범례가 넓으면 y 축 제목(최소 60 을 주던)과 겹쳤다. x·y·크기의 규모가 서로 다른 입력에서 모든 글자가 그림 안이고,
// 머리 줄(y 축 제목·범례 글자·범례 원)이 서로 겹치지 않으며 그림 영역 위에 있다.
func TestChartScatterTextBounds(t *testing.T) {
	textRe := regexp.MustCompile(`<text x="([-\d.]+)" y="([-\d.]+)" font-size="([\d.]+)" text-anchor="(\w+)" class="([^"]+)">([^<]*)`)
	circRe := regexp.MustCompile(`<circle cx="([-\d.]+)" cy="([-\d.]+)" r="([-\d.]+)" class="sw bub s1">`)
	viewRe := regexp.MustCompile(`viewBox="0 0 ([\d.]+) ([\d.]+)"`)
	vals := []string{"-1000000000000000", "1000000000000000", "0.001", "-0.0001", "1", "123456789.125"}
	heads := []string{"세로축이름", "아주긴크기머리글입니다매출액합계아주긴크기머리글입니다매출액합계", "y"}
	for _, typ := range []string{"scatter", "bubble"} {
		for _, xa := range vals {
			for _, ya := range vals {
				for _, head := range heads {
					for _, size := range []string{"1000000000000000.000", "0.5", "250000000000000.125"} {
						tbl := fmt.Sprintf("| a | x | %s | %s |\n|--|--|--|--|\n| p | %s | %s | %s |\n| q | 2 | 2 | 1 |", head, head, xa, ya, size)
						if typ == "scatter" {
							tbl = fmt.Sprintf("| a | x | %s |\n|--|--|--|\n| p | %s | %s |\n| q | 2 | 2 |", head, xa, ya)
						}
						h := render(t, chartSrc("type="+typ, tbl)).HTML
						name := fmt.Sprintf("%s x=%s y=%s head=%d size=%s", typ, xa, ya, len(head), size)
						vb := viewRe.FindStringSubmatch(h)
						if vb == nil {
							t.Fatalf("%s: 그림 없음", name)
						}
						w := num(vb[1])
						_, plotTop, _, _ := scPlot(h)
						var headBoxes [][4]float64
						for _, m := range textRe.FindAllStringSubmatch(h, -1) {
							fs, tw := num(m[3]), textW(m[6], num(m[3]))
							left := num(m[1]) - tw*map[string]float64{"start": 0, "middle": 0.5, "end": 1}[m[4]]
							bx := [4]float64{left, num(m[2]) - fs*0.8, left + tw, num(m[2]) + fs*0.25}
							if bx[0] < -0.1 || bx[2] > w+0.1 {
								t.Errorf("%s: 글자 %q(%s) %.1f~%.1f 가 그림(0~%v) 밖", name, m[6], m[5], bx[0], bx[2], w)
							}
							if (m[5] == "unit axt" || m[5] == "lg" || m[5] == "tick" && m[4] == "start") && bx[1] < plotTop-4 { // 그림 영역 위(머리 줄)의 글자
								headBoxes = append(headBoxes, bx)
								if bx[3] > plotTop+0.1 {
									t.Errorf("%s: 머리 줄 글자 %q 가 그림 영역(%v) 아래로", name, m[6], plotTop)
								}
							}
						}
						for _, m := range circRe.FindAllStringSubmatch(h, -1) {
							cx, cy, r := num(m[1]), num(m[2]), num(m[3])
							headBoxes = append(headBoxes, [4]float64{cx - r, cy - r, cx + r, cy + r})
							if cx-r < -0.1 || cx+r > w+0.1 {
								t.Errorf("%s: 범례 원이 그림 밖", name)
							}
						}
						for i, a := range headBoxes {
							for _, b := range headBoxes[i+1:] {
								if overlaps(a, b, 0) {
									t.Errorf("%s: 머리 줄의 글자·원이 겹침 %v %v", name, a, b)
								}
							}
						}
					}
				}
			}
		}
	}
}
