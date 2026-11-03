package mdoc

import (
	"math"
	"regexp"
	"strings"
	"testing"
)

func chartSrc(attrs, table string) string {
	return ":::chart{" + attrs + "}\n" + table + "\n:::\n"
}

const twoSeries = "| 분기 | 2025 | 2026 |\n|:--|--:|--:|\n| 1분기 | 31.0 | 38.2 |\n| 2분기 | 35.4 | 41.5 |\n| 3분기 | 39.1 | 44.9 |\n\n: 분기별 매출"

// 종류마다 그림 하나(figure.chart.chart-종류 > svg + figcaption), 표는 나오지 않는다(table=below 가 아니면).
func TestChartTypes(t *testing.T) {
	marks := map[string]string{"bar": `class="col `, "hbar": `class="col `, "stacked": `class="seg `, "line": `class="line `,
		"area": `class="area `, "pie": `class="slice `, "donut": `class="slice `}
	for typ, mark := range marks {
		t.Run(typ, func(t *testing.T) {
			r := render(t, chartSrc("type="+typ+` unit="억 원"`, twoSeries))
			assertSafe(t, r.HTML)
			if len(r.Warnings) != 0 {
				t.Errorf("경고 %+v", r.Warnings)
			}
			for _, want := range []string{`<figure class="chart chart-` + typ + `" data-line="1">`, `<svg class="chart-svg t-` + typ + `"`,
				`role="img" aria-label="분기별 매출"`, "<figcaption>분기별 매출</figcaption>", mark} {
				if !strings.Contains(r.HTML, want) {
					t.Errorf("%q 없음\n%s", want, r.HTML)
				}
			}
			if strings.Contains(r.HTML, "<table") {
				t.Error("표가 함께 나옴")
			}
			// 값은 <title> 로도 싣는다(마우스를 올리면 보인다)
			if !strings.Contains(r.HTML, "38.2 억 원</title>") && !strings.Contains(r.HTML, "1분기: 31.0 억 원 (29%)</title>") {
				t.Errorf("값 title 없음\n%s", r.HTML)
			}
		})
	}
	// 막대 수 = 값이 있는 칸 수, 원 차트는 첫 계열만
	r := render(t, chartSrc("type=bar", twoSeries))
	if n := strings.Count(r.HTML, `class="col `); n != 6 {
		t.Errorf("막대 %d 개", n)
	}
	r = render(t, chartSrc("type=pie", twoSeries))
	if n := strings.Count(r.HTML, `class="slice `); n != 3 || !strings.Contains(r.HTML, "1분기: 31.0") || strings.Contains(r.HTML, "38.2") {
		t.Errorf("원 차트 조각 %d 개 — 첫 계열만이어야\n%s", n, r.HTML)
	}
	// 범례: 계열이 둘 이상일 때만
	if !strings.Contains(render(t, chartSrc("type=bar", twoSeries)).HTML, `class="legend"`) {
		t.Error("두 계열인데 범례 없음")
	}
	one := "| a | 값 |\n|--|--|\n| x | 1 |\n| y | 2 |"
	if strings.Contains(render(t, chartSrc("type=bar", one)).HTML, `class="legend"`) {
		t.Error("한 계열인데 범례 있음")
	}
}

func TestChartOptions(t *testing.T) {
	r := render(t, chartSrc("type=bar labels=off", twoSeries))
	if strings.Contains(r.HTML, `class="val`) {
		t.Error("labels=off 인데 값 글자가 있음")
	}
	r = render(t, chartSrc("type=bar table=below #c1 .wide", twoSeries))
	i, j := strings.Index(r.HTML, "</figure>"), strings.Index(r.HTML, "<table")
	if i < 0 || j < i {
		t.Fatalf("table=below: 그림 뒤에 표가 있어야 함\n%s", r.HTML)
	}
	if strings.Contains(r.HTML[j:], "<caption>") {
		t.Error("표 캡션이 두 번(그림과 표)")
	}
	if !strings.Contains(r.HTML, `<figure class="chart chart-bar wide" id="c1"`) {
		t.Errorf("id·class\n%s", r.HTML[:120])
	}
	// % 값만 있으면 단위는 % 이고 눈금에 % 가 붙는다
	r = render(t, chartSrc("type=line", "| 월 | 율 |\n|--|--|\n| 1월 | 8% |\n| 2월 | 12.5% |"))
	if !strings.Contains(r.HTML, ">12.5%</text>") || !strings.Contains(r.HTML, `class="tick">10%</text>`) {
		t.Errorf("%% 단위\n%s", r.HTML)
	}
	// 음수 막대는 neg, 0 기준선
	r = render(t, chartSrc("type=bar", "| a | 값 |\n|--|--|\n| x | -3 |\n| y | 5 |"))
	if !strings.Contains(r.HTML, `class="col s1 d0 neg"`) || !strings.Contains(r.HTML, `class="gl zero"`) {
		t.Errorf("음수\n%s", r.HTML)
	}
	// 빈 칸은 그리지 않는다(선은 끊긴다)
	r = render(t, chartSrc("type=line", "| a | 값 |\n|--|--|\n| x | 1 |\n| y |  |\n| z | 3 |\n| w | 4 |"))
	if m := regexp.MustCompile(`d="([^"]*)" class="line`).FindStringSubmatch(r.HTML); m == nil || strings.Count(m[1], "M") != 2 {
		t.Errorf("빈 칸에서 선이 끊겨야 함: %v", m)
	}
}

func TestChartNumbers(t *testing.T) {
	cases := []struct {
		in, unit string
		v        float64
		dec      int
		pct, ok  bool
		empty    bool
	}{
		{"1,234.5", "", 1234.5, 1, false, true, false},
		{"-3", "", -3, 0, false, true, false},
		{"−2.25", "", -2.25, 2, false, true, false},
		{"+4", "", 4, 0, false, true, false},
		{"12%", "", 12, 0, true, true, false},
		{" 12.5 % ", "", 12.5, 1, true, true, false},
		{".5", "", 0.5, 1, false, true, false},
		{"38.2억 원", "억 원", 38.2, 1, false, true, false},
		{"38.2 억 원", "억 원", 38.2, 1, false, true, false},
		{"1,000,000", "", 1e6, 0, false, true, false},
		{"", "", 0, 0, false, false, true},
		{"-", "", 0, 0, false, false, true},
		{"—", "", 0, 0, false, false, true},
		{"N/A", "", 0, 0, false, false, true},
		{"12,34", "", 0, 0, false, false, false},
		{"1e9", "", 0, 0, false, false, false},
		{"abc", "", 0, 0, false, false, false},
		{"3억", "", 0, 0, false, false, false},
		{"--3", "", 0, 0, false, false, false},
		{"1.2.3", "", 0, 0, false, false, false},
		{"9999999999999999999", "", 0, 0, false, false, false},
		{"Infinity", "", 0, 0, false, false, false},
		{"0x10", "", 0, 0, false, false, false},
	}
	for _, c := range cases {
		v, dec, pct, ok, empty := parseChartNumber(c.in, c.unit)
		if v != c.v || dec != c.dec || pct != c.pct || ok != c.ok || empty != c.empty {
			t.Errorf("%q → %v %d %v %v %v, 기대 %v %d %v %v %v", c.in, v, dec, pct, ok, empty, c.v, c.dec, c.pct, c.ok, c.empty)
		}
	}
	for _, c := range []struct {
		v   float64
		dec int
		out string
	}{{1234.5, 1, "1,234.5"}, {-3, 0, "-3"}, {1e6, 0, "1,000,000"}, {0.004, 2, "0.00"}, {-0.001, 1, "0.0"}, {999, 2, "999.00"}} {
		if got := fmtNum(c.v, c.dec); got != c.out {
			t.Errorf("fmtNum(%v, %d) = %q, 기대 %q", c.v, c.dec, got, c.out)
		}
	}
	for _, c := range []struct{ lo, hi, tlo, thi, step float64 }{
		{0, 44.9, 0, 50, 10}, {-2.5, 13.1, -5, 15, 5}, {0, 2210, 0, 2500, 500}, {0, 1, 0, 1, 0.2}, {0, 0, 0, 1, 0.2}, {-11, 18.4, -15, 20, 5}, {0, 7, 0, 8, 2},
	} {
		tlo, thi, step, _ := niceTicks(c.lo, c.hi)
		if tlo != c.tlo || thi != c.thi || step != c.step {
			t.Errorf("niceTicks(%v, %v) = %v %v %v, 기대 %v %v %v", c.lo, c.hi, tlo, thi, step, c.tlo, c.thi, c.step)
		}
	}
}

func TestChartWarnings(t *testing.T) {
	cases := []struct {
		name, src, want string
		drawn           bool
	}{
		{"표 없음", ":::chart{type=bar}\n그냥 글\n:::\n", "표가 없어", false},
		{"계열 없음", chartSrc("", "| 항목 |\n|--|\n| a |"), "계열", false},
		{"숫자 없음", chartSrc("", "| a | b |\n|--|--|\n| x | 없음 |"), "읽을 수 있는 숫자가 없어", false},
		{"읽지 못한 칸", chartSrc("", "| a | b |\n|--|--|\n| x | 1 |\n| y | 많음 |"), `"많음" 를 숫자로 읽지 못해`, true},
		{"모르는 종류", chartSrc("type=radar", "| a | b |\n|--|--|\n| x | 1 |"), "radar", false},
		{"모양 속성", chartSrc("color=red", "| a | b |\n|--|--|\n| x | 1 |"), "color", true},
		{"계열이 많음", chartSrc("", "| a | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 |\n|--|--|--|--|--|--|--|--|--|--|\n| x | 1 | 1 | 1 | 1 | 1 | 1 | 1 | 1 | 1 |"), "앞의 8 개만", true},
		{"원 음수", chartSrc("type=pie", "| a | b |\n|--|--|\n| x | 1 |\n| y | -2 |"), "음수", true},
		{"원 조각이 많음", chartSrc("type=pie", "| a | b |\n|--|--|"+strings.Repeat("\n| x | 1 |", 10)), "기타", true},
		{"항목이 많음", chartSrc("type=hbar", "| a | b |\n|--|--|"+strings.Repeat("\n| x | 1 |", 31)), "앞의 30 개만", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := render(t, c.src)
			assertSafe(t, r.HTML)
			found := false
			for _, w := range r.Warnings {
				if w.Code == WarnChart && strings.Contains(w.Message, c.want) {
					found = true
				}
			}
			if !found {
				t.Errorf("경고 %q 없음: %+v", c.want, r.Warnings)
			}
			if drawn := strings.Contains(r.HTML, "<svg"); drawn != c.drawn {
				t.Errorf("그림 %v, 기대 %v\n%s", drawn, c.drawn, r.HTML)
			}
			if !c.drawn && !strings.Contains(r.HTML, `<div class="chart"`) {
				t.Errorf("그리지 못하면 안의 내용을 그대로 내야 함\n%s", r.HTML)
			}
		})
	}
	// 읽지 못한 칸의 경고는 그 칸의 원문 줄이다
	r := render(t, "글\n\n"+chartSrc("", "| a | b |\n|--|--|\n| x | 1 |\n| y | 많음 |"))
	for _, w := range r.Warnings {
		if w.Code == WarnChart && w.Line != 7 {
			t.Errorf("경고 줄 %d, 기대 7", w.Line)
		}
	}
	// 원 조각이 많으면 앞의 7개 + 기타
	r = render(t, chartSrc("type=pie", "| a | b |\n|--|--|"+strings.Repeat("\n| x | 1 |", 10)))
	if n := strings.Count(r.HTML, `class="slice `); n != 8 || !strings.Contains(r.HTML, ">기타</text>") {
		t.Errorf("조각 %d", n)
	}
}

// 칸·캡션·속성에 넣은 HTML·따옴표·url(·javascript: 가 SVG 를 깨거나 속성·요소로 새지 않는다.
var chartAttacks = []string{
	chartSrc(`type=bar unit="\"><script>alert(1)</script>"`, twoSeries),
	chartSrc(`type=bar unit="url(https://evil.example/x)"`, "| a | b |\n|--|--|\n| x | 1 |"),
	chartSrc(`type="bar\" onload=\"alert(1)"`, "| a | b |\n|--|--|\n| x | 1 |"),
	chartSrc(`type=bar style="fill:url(https://evil.example)"`, "| a | b |\n|--|--|\n| x | 1 |"),
	chartSrc(`type=bar onclick=alert(1)`, "| a | b |\n|--|--|\n| x | 1 |"),
	chartSrc(`type=bar #a"b`, "| a | b |\n|--|--|\n| x | 1 |"),
	chartSrc("type=bar", "| a | <script>alert(1)</script> |\n|--|--|\n| <img src=x onerror=alert(1)> | 1 |\n| </text></svg><script>alert(1)</script> | 2 |"),
	chartSrc("type=hbar", "| a | \"' onmouseover=\"alert(1) |\n|--|--|\n| javascript:alert(1) | 1 |\n| url(https://evil.example) | 2 |"),
	chartSrc("type=pie", "| a | b |\n|--|--|\n| <svg onload=alert(1)> | 1 |\n| ]]><script>x</script> | 2 |\n\n: <script>alert(1)</script> \"><img src=x onerror=alert(1)>"),
	chartSrc("type=donut", "| a | b |\n|--|--|\n| [링크](javascript:alert(1)) | 1 |\n| ![x](https://evil.example/p.png) | 2 |"),
	chartSrc("type=line", "| a | b |\n|--|--|\n| x | 1 |\n| y | 2 |\n\nTable: </figcaption><foreignObject><iframe src=x></iframe>"),
	chartSrc("type=stacked", "| a | <style>*{}</style> | `code` |\n|--|--|--|\n| **굵게** | 1 | 2 |"),
	// 기준선(itda-hyve#114): 라벨·값에 넣은 HTML·따옴표·url(
	chartSrc(`type=bar ref=1 ref-label="</text></svg><script>alert(1)</script>"`, twoSeries),
	chartSrc(`type=hbar ref=1 ref-label="\" onmouseover=\"alert(1)"`, twoSeries),
	chartSrc(`type=line ref="url(https://evil.example)" ref-label="<img src=x onerror=alert(1)>"`, twoSeries),
	chartSrc(`type=stacked ref="\"><script>alert(1)</script>"`, twoSeries),
	chartSrc(`type=area ref=-1 ref-label="javascript:alert(1)" unit="<style>"`, twoSeries),
	// 모르는 type(itda-hyve#114): 표로 남는다 — 그 표도 안전해야 한다
	chartSrc(`type="<script>alert(1)</script>" ref=1`, "| a | <img src=x onerror=alert(1)> |\n|--|--|\n| x | 1 |"),
	// 혼합(itda-hyve#114 단계 B): 선 계열 이름·범례·점 title 에 넣은 HTML, 이름이 맞지 않아 표로 남을 때의 속성(data-line 과 겹치지 않음)
	chartSrc(`type=bar line="</text></svg><script>alert(1)</script>"`, "| a | b | </text></svg><script>alert(1)</script> |\n|--|--|--|\n| x | 1 | 2 |"),
	chartSrc(`line="\" onmouseover=\"alert(1)"`, "| a | b | \" onmouseover=\"alert(1) |\n|--|--|--|\n| <img src=x onerror=alert(1)> | 1 | 2 |"),
	chartSrc(`line="\"><script>alert(1)</script>"`, twoSeries),
	chartSrc(`type=stacked line="2026" ref-label="<style>"`, "| a | <style>*{}</style> | 2026 |\n|--|--|--|\n| x | 1 | 2 |"),
	chartSrc(`line="2026" ref=40 ref-label="</text><script>alert(1)</script>" unit="url(https://evil.example)"`, twoSeries),
	// 보조 축(itda-hyve#114 단계 C): 오른쪽 축 계열 이름(범례 "(오른쪽 축)")·unit2(단위 줄·title)에 넣은 HTML, 받지 않는 조합의 표, unit2 만
	chartSrc(`axis2="</text></svg><script>alert(1)</script>" unit2="\"><script>alert(1)</script>"`, "| a | b | </text></svg><script>alert(1)</script> |\n|--|--|--|\n| x | 1 | 2 |"),
	chartSrc(`type=line axis2="\" onmouseover=\"alert(1)" unit2="url(https://evil.example)"`, "| a | b | \" onmouseover=\"alert(1) |\n|--|--|--|\n| <img src=x onerror=alert(1)> | 1 | 2 |"),
	chartSrc(`axis2="2026" unit2="</text><script>alert(1)</script>" unit="<style>"`, twoSeries),
	chartSrc(`type=stacked axis2="2026" unit2="<style>*{}</style>"`, twoSeries),
	chartSrc(`unit2="<img src=x onerror=alert(1)>"`, twoSeries),
	chartSrc(`axis2="2026" ref="\"><script>alert(1)</script>" line="\"><img src=x>"`, twoSeries),
	// 산점도·버블(itda-hyve#114 단계 D): 점 라벨(라벨 글자·줄인 라벨의 title)·축 제목(열 머리글)·크기 범례 이름·점 title 에 넣은 HTML, 받지 않는 조합의 표
	chartSrc("type=scatter", "| a | </text></svg><script>alert(1)</script> | \" onmouseover=\"alert(1) |\n|--|--|--|\n| <img src=x onerror=alert(1)> | 1 | 2 |\n| javascript:alert(1) "+strings.Repeat("길게", 40)+" | 3 | 4 |"),
	chartSrc("type=bubble", "| a | x | y | <style>*{}</style><svg onload=alert(1)> |\n|--|--|--|--|\n| </text><foreignObject><iframe src=x></iframe> | 1 | 2 | 3 |\n| url(https://evil.example) | 2 | 3 | 4 |\n\n: <script>alert(1)</script>"),
	chartSrc(`type=scatter unit="\"><script>alert(1)</script>" ref="<img src=x>" ref-label="</text><script>x</script>"`, "| a | x | y |\n|--|--|--|\n| p | 1 | 2 |"),
	chartSrc(`type=bubble line="<script>" axis2="\" onclick=\"alert(1)"`, "| a | x | y | s |\n|--|--|--|--|\n| <script>alert(1)</script> | 1 | 2 | 3 |"),
	chartSrc("type=scatter", "| a | x | y |\n|--|--|--|\n| <svg onload=alert(1)> | 약 1 | 2% |\n| <b>굵게</b> | 3 | 4% |"),
}

func TestChartSecurity(t *testing.T) {
	for i, src := range chartAttacks {
		for _, opts := range []Options{{}, {AllowRemoteImages: true}} {
			r := render(t, src, opts)
			if opts.AllowRemoteImages {
				r.HTML = strings.NewReplacer(`src="https://`, `src="x-`).Replace(r.HTML)
			}
			assertSafe(t, r.HTML)
			// 그리지 않은 컨테이너는 속성을 data-* 로 낸다(data-onload 는 이벤트 속성이 아니다 — 이름만 지우고 본다)
			low := dataAttrName.ReplaceAllString(strings.ToLower(r.HTML), " data=")
			for _, bad := range []string{"<script", "<iframe", "<style", "<img", "<foreignobject", "onload=", "onerror=", "onclick=", "onmouseover=\""} {
				if strings.Contains(low, bad) {
					t.Errorf("%d: 결과에 %q\n%s", i, bad, r.HTML)
				}
			}
			if !strings.Contains(r.HTML, "<svg") && !strings.Contains(r.HTML, ":::chart{") && !unknownType(r) {
				// 여는 줄의 속성을 못 읽은 것(글자로 남는다)이나 모르는 type(표로 남는다)이 아니면 차트를 그려야 한다
				t.Errorf("%d: 차트가 그려지지 않음(공격 글자 때문에 거부할 까닭은 없다)\n%s", i, r.HTML)
			}
			if n := strings.Count(r.HTML, "<svg"); n != strings.Count(r.HTML, "</svg>") {
				t.Errorf("%d: svg 짝이 맞지 않음", i)
			}
		}
	}
}

var dataAttrName = regexp.MustCompile(`\sdata-[a-z0-9-]+=`)

var svgNumAttr = regexp.MustCompile(` (?:x|y|x1|y1|x2|y2|cx|cy|r|rx|width|height|d|viewBox)="([^"]*)"`)

func unknownType(r Result) bool {
	for _, w := range r.Warnings {
		if w.Code == WarnChart && strings.Contains(w.Message, "차트를 그리지 않고 표로 둠") {
			return true
		}
	}
	return false
}

// 차트 문법으로도 어떤 입력에 패닉하지 않고 결과가 안전하다(FuzzRender 의 시드에도 넣는다).
func FuzzChart(f *testing.F) {
	for _, s := range chartAttacks {
		f.Add(s)
	}
	for _, typ := range []string{"bar", "hbar", "stacked", "line", "area", "pie", "donut"} {
		f.Add(chartSrc("type="+typ, twoSeries))
		f.Add(chartSrc("type="+typ, "| a | b |\n|--|--|\n| x | 0 |\n| y | 0 |"))
		f.Add(chartSrc("type="+typ, "| a | b | c |\n|--|--|--|\n| x | -1e3 |  |\n| y | 1,000,000,000 | 0.0001 |"))
		f.Add(chartSrc("type="+typ+` ref=40 ref-label="목표"`, twoSeries))
		f.Add(chartSrc("type="+typ+` ref=-999999999999999 ref-label="`+strings.Repeat("긴", 80)+`"`, "| a | b |\n|--|--|\n| x | 0 |"))
		f.Add(chartSrc("type="+typ+" ref=0.001", "| a | b |\n|--|--|\n| x | 0.001 |"))
	}
	f.Add(chartSrc("type=radar ref=1", twoSeries))
	f.Add(chartSrc(`line="2026"`, twoSeries))
	f.Add(chartSrc(`type=bar line="b" ref=0 labels=off`, "| a | b | c |\n|--|--|--|\n| x | -1e3 |  |\n| y | 1,000,000,000 | 0.0001 |"))
	f.Add(chartSrc(`line="c"`, "| a | b | c |\n|--|--|--|\n| x | | 5 |"))
	f.Add(chartSrc(`type=hbar line="2026"`, twoSeries))
	f.Add(chartSrc(`axis2="2026" unit2="%"`, twoSeries))
	f.Add(chartSrc(`type=line axis2="c" unit="억 원" unit2="건"`, "| a | b | c |\n|--|--|--|\n| x | -1e3 |  |\n| y | 1,000,000,000 | 0.0001 |"))
	f.Add(chartSrc(`axis2="c" labels=off`, "| a | b | c |\n|--|--|--|\n| x | | 0.00001 |\n| y | -999999999999999 | -0.5 |"))
	f.Add(chartSrc(`axis2="b"`, "| a | b | c |\n|--|--|--|\n| x | 5 | - |"))
	f.Add(chartSrc(`type=line axis2="b" ref=3`, "| a | b | c |\n|--|--|--|\n| x | 5% | 7 |\n| y | 6 | 8% |"))
	f.Add(chartSrc(`type=area axis2="b" unit2="%"`, twoSeries))
	f.Add(chartSrc("type=scatter", "| a | x | y |\n|--|--|--|\n| p | 0 | 0 |\n| q | 0 | 0 |"))
	f.Add(chartSrc("type=scatter labels=off", "| a | x | y | z |\n|--|--|--|--|\n| p | -999999999999999 | 0.0001 | 1 |\n| | 1e3 | |"))
	f.Add(chartSrc("type=bubble", "| a | x | y | s |\n|--|--|--|--|\n| p | 1 | 1 | 0.000001 |\n| q | 1 | 1 | 999999999999999 |\n| r | -5 | 5% | -1 |"))
	f.Add(chartSrc(`type=bubble ref=3 unit="%"`, "| a | x | y | s |\n|--|--|--|--|\n| p | 5% | 5 | 0 |"))
	f.Add(chartSrc("type=bubble", "| a | x | y |\n|--|--|--|\n| p | 1 | 2 |"))
	f.Fuzz(func(t *testing.T, src string) {
		r, err := Render([]byte(src), Options{})
		if err != nil {
			t.Fatal(err)
		}
		assertSafe(t, r.HTML)
		// 좌표(SVG 속성 값)만 본다 — 본문 글자 "NaN" 은 좌표가 아니다(퍼즈가 찾은 거짓 양성)
		for _, m := range svgNumAttr.FindAllStringSubmatch(r.HTML, -1) {
			if strings.Contains(m[1], "NaN") || strings.Contains(m[1], "Inf") {
				t.Errorf("좌표에 NaN·Inf\n%s", r.HTML)
			}
		}
	})
}

// hbar 의 음수 값 글자(막대 왼쪽 끝에 오른쪽 맞춤)는 항목 이름(오른쪽 맞춤, 끝 x = labW)과 6 이상 떨어진다 — 전에는
// 그림 시작 x0 = labW+10 에서 바로 자라 "대구·경북-3.6%" 처럼 붙었다(공문 PDF 실측).
func TestChartHBarNegativeLabels(t *testing.T) {
	catRe := regexp.MustCompile(`<text x="([-\d.]+)" y="[-\d.]+" font-size="12" text-anchor="end" class="cat">`)
	valRe := regexp.MustCompile(`<text x="([-\d.]+)" y="[-\d.]+" font-size="11" text-anchor="end" class="val[^"]*">([^<]*)</text>`)
	long := "광주·전남·전북 공공 부문 아주 긴 이름"
	for _, tbl := range []string{
		"| 지역 | 증감 |\n|--|--:|\n| 서울·수도권 기업 고객 부문 | 18.4% |\n| 대구·경북 | -3.6% |",
		"| 지역 | 증감 |\n|--|--:|\n| " + long + " | -11.0% |\n| 대전 | 4.9% |\n| 대구 | -0.1% |",
		"| 지역 | 증감 |\n|--|--:|\n| 가 | -1,234,567.125 |\n| 나 | -0.001 |",
		"| 지역 | 2025 | 2026 |\n|--|--:|--:|\n| " + long + " | -3.6 | 12.5 |\n| 부산 | -40 | -0.5 |",
		"| 지역 | 증감 |\n|--|--:|\n| 가 | -5 |\n| 나 | -3 |", // 모두 음수 — 0 이 오른쪽 끝
	} {
		for _, attrs := range []string{"type=hbar", "type=hbar ref=-2", `type=hbar unit="억 원"`} {
			h := render(t, chartSrc(attrs, tbl)).HTML
			cats := catRe.FindAllStringSubmatch(h, -1)
			if len(cats) == 0 {
				t.Fatalf("항목 글자 없음\n%s", h)
			}
			catEnd := 0.0
			for _, c := range cats {
				catEnd = math.Max(catEnd, num(c[1]))
			}
			vals := valRe.FindAllStringSubmatch(h, -1)
			if len(vals) == 0 {
				t.Errorf("%s: 음수 값 글자 없음\n%s", attrs, tbl)
			}
			for _, v := range vals {
				if left := num(v[1]) - textW(v[2], fsVal); left < catEnd+6-0.1 {
					t.Errorf("%s: 값 %q 왼쪽 끝 %.1f 가 항목 이름 끝 %.1f 에 붙음\n%s", attrs, v[2], left, catEnd, tbl)
				}
			}
		}
	}
	// 자리는 모자랄 때만 민다: -4.9(축 끝 -5 에 가까움)는 밀고, labels=off 면 값 글자가 없어 밀지 않는다
	off := render(t, chartSrc("type=hbar labels=off", "| a | b |\n|--|--:|\n| 가 | -4.9 |\n| 나 | 18 |")).HTML
	on := render(t, chartSrc("type=hbar", "| a | b |\n|--|--:|\n| 가 | -4.9 |\n| 나 | 18 |")).HTML
	first := regexp.MustCompile(`class="grid"><line x1="([-\d.]+)"`)
	if a, b := num(first.FindStringSubmatch(off)[1]), num(first.FindStringSubmatch(on)[1]); a >= b {
		t.Errorf("labels=off 의 그림 시작 %v 이 값 글자가 있을 때 %v 보다 오른쪽", a, b)
	}
}
