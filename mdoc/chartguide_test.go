package mdoc

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// 차트 안내서(spec/charts.md)를 코드와 맞춰 본다 — 종류 표·상한 표는 상수와 양방향으로, 예제 md 는 렌더로,
// 링크·그림은 파일로. 표지(<!-- chart-types:begin --> 등) 안만 계약이다 — 본문 문장과 경고 문구는 묶지 않는다.

var guideDir = filepath.Join("..", "spec")

func readGuide(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(guideDir, "charts.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// marked 는 <!-- name:begin --> 와 <!-- name:end --> 사이다.
func marked(t *testing.T, g, name string) string {
	t.Helper()
	i, j := strings.Index(g, "<!-- "+name+":begin -->"), strings.Index(g, "<!-- "+name+":end -->")
	if i < 0 || j < i {
		t.Fatalf("안내서에 %s 표지가 없음", name)
	}
	return g[i:j]
}

func TestChartGuideTypes(t *testing.T) {
	doc := map[string]bool{}
	row := regexp.MustCompile("^\\| `([a-z]+)` \\|")
	ver := regexp.MustCompile(`\| (\d+\.\d+\.\d+) \|$`) // 판 열: 그 종류를 처음 그린 판
	for line := range strings.SplitSeq(marked(t, readGuide(t), "chart-types"), "\n") {
		if m := row.FindStringSubmatch(line); m != nil {
			doc[m[1]] = true
			if !ver.MatchString(line) {
				t.Errorf("안내서 종류 표의 %s 에 판(0.11.0 꼴)이 없음: %s", m[1], line)
			}
		}
	}
	for k := range chartTypes {
		if !doc[k] {
			t.Errorf("안내서 종류 표에 %s 가 없음", k)
		}
	}
	for k := range doc {
		if !chartTypes[k] {
			t.Errorf("안내서 종류 표의 %s 를 코드가 모름", k)
		}
	}
}

// 2장의 속성 표(#id·.class 말고)를 실제 파서와 맞춘다: 표의 "대표 표기" 를 Render 에 넣어 파서가 받고(모르는 속성 경고 없음)
// 효과가 있는지(그 표기를 뺀 것과 결과가 다름) 본다. 경고 문구의 속성 목록(chartAttrList)도 표와 같아야 한다.
func TestChartGuideAttrs(t *testing.T) {
	doc := map[string]string{} // 속성 → 대표 표기
	row := regexp.MustCompile("^\\| `([a-z][a-z0-9-]*)` \\| [^|]+ \\| `([^`]+)` \\|")
	for line := range strings.SplitSeq(marked(t, readGuide(t), "chart-attrs"), "\n") {
		if m := row.FindStringSubmatch(line); m != nil {
			doc[m[1]] = m[2]
		} else if strings.HasPrefix(line, "| `") && !strings.HasPrefix(line, "| `#") && !strings.HasPrefix(line, "| `.") {
			t.Errorf("안내서 속성 표의 줄을 읽지 못함(대표 표기 칸): %s", line)
		}
	}
	for k, rep := range doc {
		if !strings.HasPrefix(rep, k+"=") {
			t.Errorf("%s 의 대표 표기 %q 가 그 속성이 아님", k, rep)
			continue
		}
		base, tbl := "type=bar", twoSeries
		switch k {
		case "type":
			base = ""
		case "ref-label":
			base = "type=bar ref=40" // 라벨은 기준선과 함께만
		case "line":
			tbl = strings.Replace(twoSeries, "| 2026 |", "| 계획 |", 1) // 대표 표기가 가리키는 계열 이름
		case "axis2":
			tbl = strings.Replace(twoSeries, "| 2026 |", "| 이익률 |", 1)
		case "unit2":
			base, tbl = `type=bar axis2="이익률"`, strings.Replace(twoSeries, "| 2026 |", "| 이익률 |", 1) // 단위는 보조 축과 함께만
		}
		src := func(attrs string) string { return chartSrc(attrs, tbl) }
		with := render(t, src(strings.TrimSpace(base+" "+rep)))
		for _, w := range with.Warnings {
			t.Errorf("%s: 대표 표기 %q 에 경고 %q — 파서가 받지 않음", k, rep, w.Message)
		}
		if without := render(t, src(base)); without.HTML == with.HTML {
			t.Errorf("%s: 대표 표기 %q 가 결과를 바꾸지 않음", k, rep)
		}
	}
	code := strings.Split(chartAttrList, "·")
	for _, k := range code {
		if _, ok := doc[k]; !ok {
			t.Errorf("안내서 속성 표에 %s 가 없음(경고 문구의 속성 목록에는 있음)", k)
		}
	}
	for k := range doc {
		if !slices.Contains(code, k) {
			t.Errorf("안내서 속성 표의 %s 가 경고 문구의 속성 목록(chartAttrList)에 없음", k)
		}
	}
}

func TestChartGuideLimits(t *testing.T) {
	code := map[string]int{
		"계열(둘째 열부터)":    chartMaxSeries,
		"항목(행)":         chartMaxCats,
		"항목(행), `hbar`": chartMaxHBar,
		"원 조각":          chartMaxSlices,
		"점, `scatter`":  chartMaxPoints,
		"점, `bubble`":   chartMaxBubbles,
	}
	doc := map[string]int{}
	row := regexp.MustCompile(`^\| ([^|]+?) \| (\d+) \|`)
	for line := range strings.SplitSeq(marked(t, readGuide(t), "chart-limits"), "\n") {
		if m := row.FindStringSubmatch(line); m != nil {
			n, _ := strconv.Atoi(m[2])
			doc[m[1]] = n
		}
	}
	for k, v := range code {
		if dv, ok := doc[k]; !ok {
			t.Errorf("안내서 상한 표에 %q 가 없음", k)
		} else if dv != v {
			t.Errorf("%s 상한: 안내서 %d ≠ 코드 %d", k, dv, v)
		}
	}
	for k := range doc {
		if _, ok := code[k]; !ok {
			t.Errorf("안내서 상한 표의 %q 를 테스트가 모름(코드 상수와 이어 줄 것)", k)
		}
	}
}

type guideExample struct {
	src  string
	warn []string // 경고 예제가 낼 경고의 대상(칸·항목 글자) — 원인마다 하나
}

// guideExamples 는 <!-- chart-example --> 바로 다음의 ```markdown 블록들이다. 경고 예제는 <!-- chart-example warn="환불|약 5" --> 로
// 경고마다 그 대상 글자를 적는다(문구 전체가 아니라 경고가 짚는 칸·항목만 계약이다).
func guideExamples(t *testing.T, g string) []guideExample {
	t.Helper()
	re := regexp.MustCompile("(?s)<!-- chart-example(?: warn=\"([^\"]+)\")? -->\n```markdown\n(.*?)\n```\n")
	var out []guideExample
	for _, m := range re.FindAllStringSubmatch(g, -1) {
		ex := guideExample{src: m[2] + "\n"}
		if m[1] != "" {
			ex.warn = strings.Split(m[1], "|")
		}
		out = append(out, ex)
	}
	if n := strings.Count(g, "<!-- chart-example"); n != len(out) {
		t.Fatalf("예제 표지 %d 개 중 %d 개만 ```markdown 블록이 바로 뒤따름", n, len(out))
	}
	return out
}

// 예제 md 는 경고 없이 그려지고(경고 예제는 chart 경고와 함께), 일곱 종류가 모두 나온다. 경고 아닌 예제는 견본
// spec/samples/charts-guide.md 에 그대로 들어 있어야 한다 — 그림과 호스트 앱의 화면 검사가 그 견본을 쓴다.
func TestChartGuideExamples(t *testing.T) {
	g := readGuide(t)
	sample, err := os.ReadFile(filepath.Join(guideDir, "samples", "charts-guide.md"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	figRe := regexp.MustCompile(`<figure class="chart chart-([a-z]+)`)
	exs := guideExamples(t, g)
	if len(exs) < len(chartTypes) {
		t.Fatalf("예제가 %d 개뿐", len(exs))
	}
	for i, ex := range exs {
		res, err := Render([]byte(ex.src), Options{})
		if err != nil {
			t.Fatalf("예제 %d: %v", i+1, err)
		}
		if len(ex.warn) > 0 {
			// 경고는 적은 대상마다 정확히 하나씩, 그 밖의 경고는 없다 — 원인 하나가 사라져도 남은 경고로 통과하지 않게
			if len(res.Warnings) != len(ex.warn) {
				t.Errorf("경고 예제 %d: 경고 %d 개(적은 원인 %d 개) %v", i+1, len(res.Warnings), len(ex.warn), res.Warnings)
			}
			for _, target := range ex.warn {
				n := 0
				for _, w := range res.Warnings {
					if w.Code == WarnChart && strings.Contains(w.Message, `"`+target+`"`) {
						n++
					}
				}
				if n != 1 {
					t.Errorf("경고 예제 %d: %q 를 짚는 chart 경고가 %d 개 %v", i+1, target, n, res.Warnings)
				}
			}
			continue
		}
		if len(res.Warnings) > 0 {
			t.Errorf("예제 %d 의 경고 %v:\n%s", i+1, res.Warnings, ex.src)
		}
		figs := figRe.FindAllStringSubmatch(res.HTML, -1)
		if len(figs) != 1 {
			t.Errorf("예제 %d 의 차트 %d 개:\n%s", i+1, len(figs), ex.src)
		}
		for _, f := range figs {
			seen[f[1]] = true
		}
		if strings.Contains(res.HTML, `class="ref-line"`) {
			seen["ref"] = true // 기준선(3.8)
		}
		if strings.Contains(res.HTML, `class="line halo"`) {
			seen["combo"] = true // 막대 + 선(3.9)
		}
		if strings.Contains(res.HTML, `class="tick ax2"`) {
			seen["axis2"] = true // 보조 축(3.10)
		}
		if !strings.Contains(string(sample), ex.src) {
			t.Errorf("예제 %d 가 견본 spec/samples/charts-guide.md 에 그대로 없음:\n%s", i+1, ex.src)
		}
	}
	for k := range chartTypes {
		if !seen[k] {
			t.Errorf("예제에 %s 차트가 없음", k)
		}
	}
	if !seen["ref"] {
		t.Error("예제에 기준선(ref)이 없음")
	}
	if !seen["combo"] {
		t.Error("예제에 막대 + 선(line)이 없음")
	}
	if !seen["axis2"] {
		t.Error("예제에 보조 축(axis2)이 없음")
	}
	// 견본 자체도 경고 없이 그려진다
	if res, err := Render(sample, Options{}); err != nil || len(res.Warnings) > 0 {
		t.Errorf("견본 렌더: %v %v", err, res.Warnings)
	}
}

// 상대 링크·그림은 파일이 있고(제목 조각 #… 은 그 문서의 제목과 맞고), 그림은 대체 글이 있다. 이슈 링크는 이 저장소의 이슈 주소다.
// 거꾸로 README·FEATURES·테마 안내서는 이 안내서로 잇는다.
func TestChartGuideLinks(t *testing.T) {
	g := readGuide(t)
	// 코드 블록·인라인 코드 안의 글자는 링크가 아니다(인라인 코드로 적은 "`[링크](…)`" 같은 글)
	prose := regexp.MustCompile("(?s)```.*?```").ReplaceAllString(g, "")
	prose = regexp.MustCompile("`[^`\n]*`").ReplaceAllString(prose, "")
	link := regexp.MustCompile(`(!?)\[([^\]]*)\]\(([^)\s]+)\)`)
	issue := regexp.MustCompile(`^https://github\.com/itda-work/itda-(hyve|doc)/issues/\d+$`)
	images := 0
	for _, m := range link.FindAllStringSubmatch(prose, -1) {
		img, alt, target := m[1] == "!", m[2], m[3]
		if strings.HasPrefix(target, "http") {
			if strings.Contains(target, "/issues") && strings.Contains(target, "github.com/itda-work/") && !issue.MatchString(target) {
				t.Errorf("이슈 링크 꼴이 틀림: %s", target)
			}
			continue
		}
		path, frag, _ := strings.Cut(target, "#")
		full := filepath.Join(guideDir, filepath.FromSlash(path))
		b, err := os.ReadFile(full)
		if err != nil {
			t.Errorf("링크 %s: %v", target, err)
			continue
		}
		if frag != "" && !slices.Contains(headingSlugs(string(b)), frag) {
			t.Errorf("링크 %s: %s 에 그 제목이 없음", target, path)
		}
		if img {
			images++
			if strings.TrimSpace(alt) == "" {
				t.Errorf("그림 %s 에 대체 글이 없음", target)
			}
		}
	}
	if images < 9 {
		t.Errorf("그림이 %d 개뿐(한눈에·일곱 종류·숫자·다른 뷰어)", images)
	}
	for _, f := range []string{"../README.md", "themes.md"} {
		b, err := os.ReadFile(filepath.Join(guideDir, filepath.FromSlash(f)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "charts.md)") {
			t.Errorf("%s 에 차트 안내서 링크가 없음", f)
		}
	}
}

// headingSlugs 는 GitHub 이 제목에 붙이는 조각 이름들이다(소문자, 글자·숫자·_·- 만 남기고 공백은 -).
func headingSlugs(md string) []string {
	md = regexp.MustCompile("(?s)```.*?```").ReplaceAllString(md, "")
	var out []string
	for line := range strings.SplitSeq(md, "\n") {
		h := strings.TrimLeft(line, "#")
		if h == line || !strings.HasPrefix(h, " ") {
			continue
		}
		var b strings.Builder
		for _, r := range strings.ToLower(strings.TrimSpace(h)) {
			switch {
			case unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' || r == '-':
				b.WriteRune(r)
			case r == ' ':
				b.WriteByte('-')
			}
		}
		out = append(out, b.String())
	}
	return out
}
