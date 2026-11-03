package gantt

import (
	"errors"
	"fmt"
	"html"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/flowchart"
)

// mermaid 그대로의 예제 문법 — 모르는 것은 *Error(줄 번호), 쓰지 않는 것은 Issue(경고)다.

// errAt 는 원문을 그려 *Error 의 줄·메시지 일부를 견준다(Render — 원문 상한 검사까지 지나간다).
func errAt(t *testing.T, src string, line int, msg string) {
	t.Helper()
	_, err := Render(src)
	var de *diagram.Error
	if !errors.As(err, &de) {
		t.Fatalf("Render(%q) 오류가 *Error 가 아님: %v", src, err)
	}
	if de.Line != line || !strings.Contains(de.Msg, msg) {
		t.Errorf("Render 오류 %d:%q, 기대 %d 에 %q", de.Line, de.Msg, line, msg)
	}
}

// drawn 은 원문을 그려 SVG·경고를 얻는다.
func drawn(t *testing.T, src string) (string, []diagram.Issue) {
	t.Helper()
	r, err := Render(src)
	if err != nil {
		t.Fatalf("Render(%q) 실패: %v", src, err)
	}
	return r.SVG, r.Issues
}

func TestParseStatements(t *testing.T) {
	// dateFormat 여섯 꼴 — 같은 그림이 다른 꼴로 읽힌다
	for _, c := range []struct{ fmt, date string }{
		{"YYYY-MM-DD", "2026-03-05"},
		{"YYYY-MM-DD HH:mm", "2026-03-05 09:30"},
		{"YYYY/MM/DD", "2026/03/05"},
		{"YYYY.MM.DD", "2026.03.05"},
		{"YYYY-MM", "2026-03"},
		{"YYYY", "2026"},
	} {
		svg, issues := drawn(t, "gantt\ndateFormat "+c.fmt+"\n작업 :"+c.date+", 5d")
		if len(issues) != 0 {
			t.Errorf("dateFormat %s: 경고 %+v", c.fmt, issues)
		}
		if !strings.Contains(svg, ">작업<") {
			t.Errorf("dateFormat %s: 작업 이름이 없음\n%s", c.fmt, svg)
		}
	}
	// 기본 dateFormat 은 YYYY-MM-DD
	drawn(t, "gantt\n작업 :2026-03-05, 5d")

	// axisFormat 지시자 — %Y %y %m %d %e %H %M 과 글자(한글 포함)
	for _, af := range []string{"%Y-%m-%d", "%y/%m/%d", "%m월 %d일", "%Y.%m.%d %H:%M", "%e"} {
		svg, issues := drawn(t, "gantt\naxisFormat "+af+"\ndateFormat YYYY-MM-DD HH:mm\n작업 :2026-03-05 09:30, 2h")
		if len(issues) != 0 {
			t.Errorf("axisFormat %s: 경고 %+v", af, issues)
		}
		if !strings.Contains(svg, "alab") {
			t.Errorf("axisFormat %s: 축 라벨이 없음", af)
		}
	}

	// tickInterval
	for _, ti := range []string{"1day", "2week", "1month"} {
		dur := map[string]string{"1day": "6d", "2week": "10w", "1month": "6M"}[ti] // 라벨이 겹치지 않는 범위
		_, issues := drawn(t, "gantt\ntickInterval "+ti+"\ndateFormat YYYY-MM-DD\n작업 :2026-01-01, "+dur)
		if len(issues) != 0 {
			t.Errorf("tickInterval %s: 경고 %+v", ti, issues)
		}
	}

	// section — 구역 없는 앞작업, 구역 안 작업
	svg, _ := drawn(t, "gantt\n밖의 작업 :2026-01-01, 2d\nsection 구역\n안의 작업 :2026-01-03, 2d")
	for _, s := range []string{">구역<", ">밖의 작업<", ">안의 작업<"} {
		if !strings.Contains(svg, s) {
			t.Errorf("section: %s 없음\n%s", s, svg)
		}
	}

	// 작업 메타 개수 — 1개(끝)·2개(시작·끝)·3개(id·시작·끝)
	drawn(t, "gantt\n둘 :2026-01-05, 3d\n하나 :5d")              // 1개: 앞 작업 끝에서 시작
	drawn(t, "gantt\n둘 :a2, 2026-01-05, 3d\n셋 :after a2, 2d") // 3개: id
	drawn(t, "gantt\n하나 :2026-01-05, 2026-01-10")             // 2개: 날짜·날짜

	// 태그 — 클래스로 나온다
	svg, _ = drawn(t, "gantt\na :done, 2026-01-01, 1d\nb :active, 2026-01-02, 1d\nc :crit, 2026-01-03, 1d\nd :milestone, 2026-01-04, 0d")
	for _, cls := range []string{`class="gt-bar gt-done"`, `class="gt-bar gt-active"`, `class="gt-bar gt-crit"`, `class="gt-ms"`} {
		if !strings.Contains(svg, cls) {
			t.Errorf("태그 %s 없음\n%s", cls, svg)
		}
	}

	// after 여럿(가장 늦은 끝)·until 여럿(가장 이른 시작) — until 은 끝 자리·값 하나일 때도 온다
	drawn(t, "gantt\na :a1, 2026-01-01, 2d\nb :b1, 2026-01-05, 2d\nc :c1, after a1 b1, 3d\nd :after b1, until c1")
	drawn(t, "gantt\na :a1, 2026-01-01, 2d\nz :2026-01-01, until a1 a1")
	drawn(t, "gantt\na :a1, 2026-01-10, 1d\nb :2026-01-01, 1d\nz :until a1") // 값 하나 until: 시작은 앞 작업(b)의 끝

	// 기간 단위마다(정수) — y M w d h m s ms
	for _, u := range []string{"y", "M", "w", "d", "h", "m", "s", "ms"} {
		_, issues := drawn(t, "gantt\n작업 :2026-01-01, 3"+u)
		if len(issues) != 0 {
			t.Errorf("기간 3%s: 경고 %+v", u, issues)
		}
	}

	// excludes 세 꼴
	drawn(t, "gantt\nexcludes weekends\n작업 :2026-01-05, 10d")                      // 토·일
	drawn(t, "gantt\nexcludes monday, friday\n작업 :2026-01-05, 10d")                // 요일 이름(쉼표)
	drawn(t, "gantt\nexcludes 2026-01-06\n작업 :2026-01-05, 10d")                    // 날짜
	drawn(t, "gantt\nexcludes weekends\nexcludes 2026-01-06\n작업 :2026-01-05, 10d") // 여러 줄 합침

	// inclusiveEndDates — 날짜 끝에 최소 단위만큼
	_, _ = drawn(t, "gantt\ninclusiveEndDates\n작업 :2026-01-05, 2026-01-10")

	// 접근성·frontmatter 캡션
	svg, issues := drawn(t, "---\ntitle: 간트 캡션\n---\ngantt\naccTitle: 접근 이름\naccDescr: 설명\n작업 :2026-01-01, 1d")
	if len(issues) != 0 {
		t.Errorf("접근성: 경고 %+v", issues)
	}
	if !strings.Contains(svg, `aria-label="접근 이름"`) || !strings.Contains(svg, "<title>설명</title>") {
		t.Errorf("접근성 글자가 없음\n%s", svg)
	}

	// 그림 안 제목(title 줄)
	svg, _ = drawn(t, "gantt\ntitle 분기 로드맵\n작업 :2026-01-01, 5d")
	if !strings.Contains(svg, `class="gt-title"`) || !strings.Contains(svg, ">분기 로드맵<") {
		t.Errorf("그림 안 제목이 없음\n%s", svg)
	}
}

func TestParseIgnored(t *testing.T) {
	// 쓰지 않는 줄 — 그림은 그리고 경고 한 번씩. todayMarker off 는 경고 없음
	cases := []struct {
		src  string
		n    int
		frag string
	}{
		{"gantt\ntodayMarker off\n작업 :2026-01-01, 1d", 0, ""},
		{"gantt\ntodayMarker stroke:red\n작업 :2026-01-01, 1d", 1, "todayMarker"},
		{"gantt\ntopAxis\n작업 :2026-01-01, 1d", 1, "topAxis"},
		{"gantt\nweekday monday\n작업 :2026-01-01, 1d", 1, "weekday"},
		{"gantt\nclick a1 \"https://x.example\"\n작업 :2026-01-01, 1d", 1, "click"},
		{"%%{init: {\"theme\":\"dark\"}}%%\ngantt\n작업 :2026-01-01, 1d", 1, "설정 지시"},
		{"---\nconfig:\n  theme: dark\n---\ngantt\n작업 :2026-01-01, 1d", 1, "frontmatter 의 설정(config)"},
	}
	for _, c := range cases {
		r, err := Render(c.src)
		if err != nil {
			t.Fatalf("Render(%q) 실패: %v", c.src, err)
		}
		if len(r.Issues) != c.n {
			t.Errorf("%q: 경고 %d개(기대 %d): %+v", c.src, len(r.Issues), c.n, r.Issues)
		}
		if c.n > 0 && !strings.Contains(r.Issues[0].Msg, c.frag) {
			t.Errorf("%q: 경고 문구 %q 에 %q 없음", c.src, r.Issues[0].Msg, c.frag)
		}
		if !strings.Contains(r.SVG, "<svg") {
			t.Errorf("%q: 그림이 없음", c.src)
		}
	}
}

func TestParseUnsupported(t *testing.T) {
	// 미지원 — 그리지 않고 *Error(줄 번호)
	errAt(t, "gantt\n작업 :vert, 2026-01-01, 1d", 2, "vert")
	errAt(t, "gantt\nincludes weekends\n작업 :2026-01-01, 1d", 2, "includes")
	errAt(t, "gantt\nexcludes weekends\nweekend friday\n작업 :2026-01-01, 1d", 3, "weekend")
	errAt(t, "gantt\n작업 :2026/01/01, 1d", 2, "dateFormat")                      // 꼴과 맞지 않는 날짜
	errAt(t, "gantt\ndateFormat YYYY-MM-DD\n작업 :2026-02-30, 1d", 3, "맞지 않는 날짜") // 없는 날
	errAt(t, "gantt\ndateFormat X\n작업 :2026-01-01, 1d", 2, "dateFormat 토큰")
	errAt(t, "gantt\naxisFormat %b\n작업 :2026-01-01, 1d", 2, "axisFormat 지시자")
	errAt(t, "gantt\ntickInterval 1hour\n작업 :2026-01-01, 1d", 2, "tickInterval 단위")
	errAt(t, "gantt\n알 수 없는 문장", 2, "알 수 없는 문장")
	errAt(t, "gantt\n작업 :after 없는id, 1d", 2, "모름")
	errAt(t, "gantt\na :x1, 2026-01-01, 1d\nb :x1, 2026-01-02, 1d", 3, "겹침") // 같은 id 두 번
	errAt(t, "gantt\n작업 :2026-01-10, 2026-01-05", 2, "끝이 시작보다 앞")
	errAt(t, "gantt\na :2026-01-01, 1d\nb :2150-01-01, 1d", 3, "100년") // 범위 초과
	errAt(t, "gantt\n작업 :5d", 2, "첫 작업의 시작")
	errAt(t, "gantt\n작업 :2026-01-01, 1.5d", 2, "1.5d") // 소수 기간은 이 판이 안 받는다
	errAt(t, "gantt\n작업 :2026-01-01, 3dX", 2, "3dX")
	errAt(t, "gantt\n작업 :2026-01-01, 1d, 남는값", 2, "맞지 않는 날짜") // 값 3개로 id·시작·끝으로 읽다 시작(1d)에서 실패
	errAt(t, "gantt\n작업 :", 2, "시작·끝이 없음")
	errAt(t, "gantt\nsection", 2, "section 이름이 없음")
	errAt(t, "gantt\ndateFormat YYYY-MM-DD\ndateFormat YYYY-MM-DD\n작업 :2026-01-01, 1d", 3, "둘 이상")
	errAt(t, "gantt\ninclusiveEndDates now\n작업 :2026-01-01, 1d", 2, "뒤에 글이")
	// 모르는 excludes 낱말·쉼표만 있는 값은 그 줄에서 원문(리뷰 3회차 집중 확인 — 조용히 무효가 아니라 오류)
	errAt(t, "gantt\nexcludes 공휴일\n작업 :2026-01-01, 1d", 2, "dateFormat 꼴 날짜가 아님")
	errAt(t, "gantt\nexcludes weekends, 공휴일\n작업 :2026-01-01, 1d", 2, "dateFormat 꼴 날짜가 아님")
	errAt(t, "gantt\nexcludes ,\n작업 :2026-01-01, 1d", 2, "excludes 값이 없음")
	errAt(t, "gantt\n작업 :2026-01-05, 1d\nexcludes 공휴일", 3, "dateFormat 꼴 날짜가 아님") // 줄 번호는 그 excludes 줄
	// frontmatter 규칙(공통) — YAML 오류의 줄은 frontmatter 시작(원문 1줄), 글 아닌 title 은 그 키의 줄
	errAt(t, "---\ntitle: [가\n---\ngantt\n작업 :2026-01-01, 1d", 1, "YAML")
	errAt(t, "---\ntitle:\n  a: b\n---\ngantt\n작업 :2026-01-01, 1d", 2, "title 이 글이 아님")
	// 길이 초과도 title 키 줄(리뷰 R4 P3 — 여러 줄 제목·별칭에서 값 줄이 아니라 키 줄을 보고한다)
	errAt(t, "---\ntitle:\n  >-\n    "+strings.Repeat("가", MaxLabel+1)+"\n---\ngantt\n작업 :2026-01-01, 1d", 2, "라벨이 200 자를 넘음")
	errAt(t, "---\nbase: &t "+strings.Repeat("가", MaxLabel+1)+"\ntitle: *t\n---\ngantt\n작업 :2026-01-01, 1d", 3, "라벨이 200 자를 넘음")
	// 빈 구역은 그리지 않고 경고
	r, err := Render("gantt\nsection 빈 구역\nsection 실제 구역\n작업 :2026-01-01, 1d")
	if err != nil || len(r.Issues) != 1 || !strings.Contains(r.Issues[0].Msg, "빈 section") {
		t.Errorf("빈 구역: %+v %v", r.Issues, err)
	}
}

func TestLimits(t *testing.T) {
	// 작업 150 까지 그린다, 151 부터 원문
	many := func(n int) string {
		var b strings.Builder
		b.WriteString("gantt\n")
		for i := 0; i < n; i++ {
			b.WriteString("작업" + strconv.Itoa(i) + " :2026-01-01, 1d\n")
		}
		return b.String()
	}
	if _, err := Render(many(MaxTasks)); err != nil {
		t.Errorf("작업 %d 개: %v", MaxTasks, err)
	}
	errAt(t, many(MaxTasks+1), MaxTasks+2, "작업이 150 개를 넘음")
	// 구역 30
	secs := func(n int) string {
		var b strings.Builder
		b.WriteString("gantt\n")
		for i := 0; i < n; i++ {
			b.WriteString("section 구역" + strconv.Itoa(i) + "\n작업 :2026-01-01, 1d\n")
		}
		return b.String()
	}
	if _, err := Render(secs(MaxSections)); err != nil {
		t.Errorf("구역 %d 개: %v", MaxSections, err)
	}
	errAt(t, secs(MaxSections+1), 2*MaxSections+2, "section 이 30 개를 넘음")
	// 원문 16KB
	big := "gantt\n" + strings.Repeat("%% ", 17000)
	errAt(t, big, 0, "16KB")
	// 라벨 200자
	errAt(t, "gantt\n"+strings.Repeat("가", MaxLabel+1)+" :2026-01-01, 1d", 2, "라벨이 200 자를 넘음")
}

func TestDeterminism(t *testing.T) {
	src := "gantt\ntitle 일정\ndateFormat YYYY-MM-DD\nexcludes weekends\nsection A\na :done, a1, 2026-01-05, 4d\nb :b1, after a1, 5d\nsection B\nc :crit, active, after b1, 1w\nd :milestone, 2026-02-01, 0d"
	r1, err := Render(src)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		r2, _ := Render(src)
		if r1.SVG != r2.SVG {
			t.Fatalf("%d번째 렌더가 다름", i)
		}
	}
	if r1.Caption != "" {
		t.Errorf("캡션 %q", r1.Caption)
	}
}

// SVG 안 요소·속성은 흐름도와 같은 허용 목록 — 여기서는 요소 집합만 본다(mdoc 의 security_test 가 전체를 본다).
func TestSVGShape(t *testing.T) {
	svg, _ := drawn(t, "gantt\nsection 구역\na :2026-01-01, 5d\nb :milestone, 2026-01-08, 0d")
	for _, bad := range []string{"style", "href", "url(", " id=", "onclick"} {
		if strings.Contains(svg, bad) {
			t.Errorf("SVG 에 %q\n%s", bad, svg)
		}
	}
	re := regexp.MustCompile(`<([a-z]+)`)
	allowed := map[string]bool{"svg": true, "g": true, "rect": true, "path": true, "line": true, "circle": true, "text": true, "tspan": true, "title": true}
	for _, m := range re.FindAllStringSubmatch(svg, -1) {
		if !allowed[m[1]] {
			t.Errorf("허용 밖 요소 <%s>", m[1])
		}
	}
}

// 제외 날짜 — 끝 미루기(Mermaid checkTaskDates 와 같은 알고리즘)와 띠.
func TestExcludes(t *testing.T) {
	// 2026-01-05(월) + 5d = 01-10(토). 제외일(10 토)마다 끝이 하루씩 밀린다: 10→11, 11(일)→12 — 최종 01-12(월)
	p := newTestParser()
	if err := p.parse("gantt\nexcludes weekends\na :2026-01-05, 5d"); err != nil {
		t.Fatal(err)
	}
	end := p.c.Tasks[0].End
	if got := end.Format("2006-01-02"); got != "2026-01-12" {
		t.Errorf("주말 제외 뒤 끝 %s, 기대 2026-01-12", got)
	}
	// 제외가 없으면 그대로
	p2 := newTestParser()
	if err := p2.parse("gantt\na :2026-01-05, 5d"); err != nil {
		t.Fatal(err)
	}
	if got := p2.c.Tasks[0].End.Format("2006-01-02"); got != "2026-01-10" {
		t.Errorf("제외 없음 끝 %s, 기대 2026-01-10", got)
	}
	// 띠가 그린다
	svg, _ := drawn(t, "gantt\nexcludes weekends\na :2026-01-05, 15d")
	if !strings.Contains(svg, `class="gt-excl"`) {
		t.Errorf("제외 띠가 없음\n%s", svg)
	}
	// 요일 이름·날짜 제외 띠
	svg, _ = drawn(t, "gantt\nexcludes friday, 2026-01-06\na :2026-01-05, 10d")
	if !strings.Contains(svg, `class="gt-excl"`) {
		t.Errorf("요일·날짜 제외 띠가 없음\n%s", svg)
	}
	// 제외가 없으면 띠도 없다
	svg, _ = drawn(t, "gantt\na :2026-01-05, 10d")
	if strings.Contains(svg, `class="gt-excl"`) {
		t.Errorf("제외 없는데 띠가 있음")
	}
}

// inclusiveEndDates — dateFormat 최소 단위만큼 끝을 포함한다.
func TestInclusiveEnd(t *testing.T) {
	p := newTestParser()
	if err := p.parse("gantt\ninclusiveEndDates\na :2026-01-05, 2026-01-09"); err != nil {
		t.Fatal(err)
	}
	if got := p.c.Tasks[0].End.Format("2006-01-02"); got != "2026-01-10" {
		t.Errorf("포함 끝 %s, 기대 2026-01-10", got)
	}
	// 월 단위 꼴이면 한 달
	p3 := newTestParser()
	if err := p3.parse("gantt\ndateFormat YYYY-MM\ninclusiveEndDates\na :2026-01, 2026-02"); err != nil {
		t.Fatal(err)
	}
	if got := p3.c.Tasks[0].End.Format("2006-01"); got != "2026-03" {
		t.Errorf("월 꼴 포함 끝 %s, 기대 2026-03", got)
	}
}

func newTestParser() *parser {
	return &parser{c: &chart{}, byID: map[string]int{}, said: map[string]bool{},
		df: mustDateFormat(), af: defaultAxisFmt(), ex: &excludes{dates: map[string]bool{}, none: true}}
}

// after·until 계산 — 여럿이면 가장 늦은 끝·가장 이른 시작.
func TestAfterUntil(t *testing.T) {
	p := newTestParser()
	if err := p.parse("gantt\na :a1, 2026-01-01, 2d\nb :b1, 2026-01-05, 2d\nc :c1, after a1 b1, 1d\nd :2026-01-01, until c1 b1"); err != nil {
		t.Fatal(err)
	}
	c, d := p.c.Tasks[2], p.c.Tasks[3]
	if c.Start.Format("2006-01-02") != "2026-01-07" {
		t.Errorf("after 여럿 시작 %s, 기대 2026-01-07", c.Start.Format("2006-01-02"))
	}
	if d.End.Format("2006-01-02") != "2026-01-05" { // min(시작 c=01-07, 시작 b1=01-05)
		t.Errorf("until 여럿 끝 %s, 기대 2026-01-05", d.End.Format("2006-01-02"))
	}
}

// 마일스톤 마름모는 작업 시작·끝의 가운데(시작 + 기간/2).
func TestMilestone(t *testing.T) {
	p := newTestParser()
	if err := p.parse("gantt\nm :milestone, 2026-01-05, 4d"); err != nil {
		t.Fatal(err)
	}
	mid := p.c.Tasks[0].Start.Add(p.c.Tasks[0].End.Sub(p.c.Tasks[0].Start) / 2)
	if got := mid.Format("2006-01-02 15:04"); got != "2026-01-07 00:00" {
		t.Errorf("마일스톤 가운데 %s, 기대 2026-01-07", got)
	}
}

// axisInfo 는 SVG 의 축 라벨 글자(차례대로)·격자선 x·라벨 x 다.
func axisInfo(svg string) (labs []string, gridX, labX []float64) {
	for _, m := range regexp.MustCompile(`<text class="gt-alab" x="([\d.]+)"[^>]*>([^<]*)</text>`).FindAllStringSubmatch(svg, -1) {
		v, _ := strconv.ParseFloat(m[1], 64)
		labs, labX = append(labs, m[2]), append(labX, v)
	}
	for _, m := range regexp.MustCompile(`<line class="gt-grid" x1="([\d.]+)"`).FindAllStringSubmatch(svg, -1) {
		v, _ := strconv.ParseFloat(m[1], 64)
		gridX = append(gridX, v)
	}
	return
}

// 눈금 사다리(수정 2회차) — d3(Mermaid) 꼴: 목표 눈금 수에 가장 가까운 간격, 첫 눈금은 실제 범위 시작(모든 작업 중 가장 이른 시작)을
// 간격 경계로 올린 시각, 격자선 수 = 라벨 수. 범위마다 눈금 열을 그대로 고정한다(결정성 포함).
func TestTickLadder(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
		issue     string // 기대 경고 일부(없으면 경고 0건)
	}{
		{"2개월(주)", "gantt\na :2026-01-05, 8w",
			[]string{"2026-01-11", "2026-01-18", "2026-01-25", "2026-02-01", "2026-02-08", "2026-02-15", "2026-02-22", "2026-03-01"}, ""},
		// 9주 — 1주 라벨이 겹쳐 2주(d3 timeWeek.every(2) 자리 — 1970 기준 일요일부터 짝수 주)
		{"9주(2주)", "gantt\nexcludes weekends\nsection 준비\n조사 :done, 2026-01-05, 4d\n수립 :p1, 2026-01-12, 5d\nsection 개발\n설계 :active, after p1, 2w\n구현 :crit, 2026-02-02, 3w\nsection 출시\n시험 :2026-02-23, 1w\n출시 :milestone, 2026-03-02, 0d",
			[]string{"2026-01-11", "2026-01-25", "2026-02-08", "2026-02-22"}, ""},
		{"10개월(달)", "gantt\ndateFormat YYYY-MM\naxisFormat %Y.%m\nsection 상반기\n파일럿 :2026-01, 2M\nsection 하반기\n확산 :2026-07, 4M",
			[]string{"2026.01", "2026.02", "2026.03", "2026.04", "2026.05", "2026.06", "2026.07", "2026.08", "2026.09", "2026.10", "2026.11"}, ""},
		{"3년(년)", "gantt\na :2026-03-01, 3y",
			[]string{"2027-01-01", "2028-01-01", "2029-01-01"}, ""},
		{"3년·짧은 라벨(3개월)", "gantt\naxisFormat %y.%m\na :2026-03-01, 3y",
			[]string{"26.04", "26.07", "26.10", "27.01", "27.04", "27.07", "27.10", "28.01", "28.04", "28.07", "28.10", "29.01"}, ""},
		{"5일(일)", "gantt\na :2026-01-05, 5d",
			[]string{"2026-01-05", "2026-01-06", "2026-01-07", "2026-01-08", "2026-01-09", "2026-01-10"}, ""},
		{"3시간(15분)", "gantt\ndateFormat YYYY-MM-DD HH:mm\naxisFormat %H:%M\na :2026-01-05 09:00, 3h",
			[]string{"09:00", "09:15", "09:30", "09:45", "10:00", "10:15", "10:30", "10:45", "11:00", "11:15", "11:30", "11:45", "12:00"}, ""},
		{"이틀(6시간)", "gantt\ndateFormat YYYY-MM-DD HH:mm\naxisFormat %d일 %H시\na :2026-01-05 10:00, 2d",
			[]string{"05일 12시", "05일 18시", "06일 00시", "06일 06시", "06일 12시", "06일 18시", "07일 00시", "07일 06시"}, ""},
		// tickInterval 명시는 그 간격 그대로(일 간격은 그 달의 1·3·5… 일 — d3 timeDay.every(2) 와 같다)
		{"tickInterval 2day", "gantt\ntickInterval 2day\na :2026-01-05, 10d",
			[]string{"2026-01-05", "2026-01-07", "2026-01-09", "2026-01-11", "2026-01-13", "2026-01-15"}, ""},
		{"tickInterval 1month", "gantt\ntickInterval 1month\na :2026-01-15, 100d",
			[]string{"2026-02-01", "2026-03-01", "2026-04-01"}, ""},
		// 라벨이 겹치는 tickInterval 은 경고하고 자동(5주 → 주)
		{"tickInterval 겹침", "gantt\ntickInterval 1day\na :2026-01-05, 5w",
			[]string{"2026-01-11", "2026-01-18", "2026-01-25", "2026-02-01", "2026-02-08"}, "라벨이 겹쳐"},
		// 400개를 넘는 tickInterval 은 경고하고 자동
		{"tickInterval 상한", "gantt\ntickInterval 1day\na :2026-01-01, 900d",
			[]string{"2026-01-01", "2027-01-01", "2028-01-01"}, "400 개를 넘어"},
	}
	for _, c := range cases {
		r1, err := Render(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		r2, _ := Render(c.src)
		if r1.SVG != r2.SVG {
			t.Errorf("%s: 두 번 그린 결과가 다름", c.name)
		}
		labs, gx, lx := axisInfo(r1.SVG)
		if strings.Join(labs, " | ") != strings.Join(c.want, " | ") {
			t.Errorf("%s: 눈금\n  %v\n기대\n  %v", c.name, labs, c.want)
		}
		if len(gx) != len(labs) {
			t.Errorf("%s: 격자선 %d개 ≠ 라벨 %d개", c.name, len(gx), len(labs))
		}
		for i := range gx { // 라벨은 제 눈금 바로 아래(왼쪽 끝 당김만 허용)
			if i < len(lx) && lx[i] != gx[i] && lx[i] < gx[i] {
				t.Errorf("%s: 라벨 %d x=%v 가 눈금 x=%v 와 다름", c.name, i, lx[i], gx[i])
			}
		}
		switch {
		case c.issue == "" && len(r1.Issues) != 0:
			t.Errorf("%s: 경고 %+v", c.name, r1.Issues)
		case c.issue != "" && (len(r1.Issues) != 1 || !strings.Contains(r1.Issues[0].Msg, c.issue) || r1.Issues[0].Line != 2):
			t.Errorf("%s: 경고 %+v, 기대 2줄에 %q", c.name, r1.Issues, c.issue)
		}
	}
	// 첫 눈금 ≥ 실제 범위 시작: 무작위 범위에서 눈금 시각이 rlo..rhi 안
	rng := rand.New(rand.NewSource(409))
	for i := 0; i < 300; i++ {
		lo := time.Date(2000+rng.Intn(50), time.Month(1+rng.Intn(12)), 1+rng.Intn(28), rng.Intn(24), rng.Intn(60), 0, 0, time.UTC)
		hi := lo.Add(time.Duration(rng.Int63n(int64(80 * 365 * 24 * time.Hour))))
		for _, r := range ladder(true, true) {
			ts, err := genTicks(lo, hi, r)
			if err != nil {
				t.Fatalf("genTicks %s: %v", r, err)
			}
			if len(ts) > MaxTicks+1 {
				t.Fatalf("genTicks %s: %d 개 — MaxTicks+1 을 넘음", r, len(ts))
			}
			for k, x := range ts {
				if x.Before(lo) || x.After(hi) || (k > 0 && !x.After(ts[k-1])) {
					t.Fatalf("genTicks %s %v..%v: 눈금 %v 가 범위 밖이거나 앞으로 가지 않음", r, lo, hi, x)
				}
			}
			if len(ts) > 0 && !alignFloor(ts[0], r).Equal(ts[0]) {
				t.Fatalf("genTicks %s: 첫 눈금 %v 가 간격 경계가 아님", r, ts[0])
			}
		}
	}
}

// 리뷰 1회차 발견 [1]~[8]의 재현 입력 — 그리지 않거나(원문 + 줄 번호), 바르게 그린다.
func TestReviewR1(t *testing.T) {
	// [1] 큰 tickInterval 은 읽을 때 거절, 눈금 생성기는 나아가지 않으면 오류, 많아야 MaxTicks+1 개
	errAt(t, "gantt\ntickInterval 144115188075855872day\na :2026-01-01, 1d", 2, "1000 을 넘음")
	errAt(t, "gantt\ntickInterval 1001day\na :2026-01-01, 1d", 2, "1000 을 넘음")
	errAt(t, "gantt\ntickInterval +2day\na :2026-01-01, 1d", 2, "숫자+단위가 아님")
	if _, err := Render("gantt\ntickInterval 1000day\na :2026-01-01, 1d"); err != nil {
		t.Errorf("tickInterval 1000day: %v", err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, r := range []rung{{tickDay, 144115188075855872}, {tickYear, 1 << 62}, {tickMonth, 1 << 62}} {
		if _, err := genTicks(base, base.AddDate(1, 0, 0), r); err == nil {
			t.Errorf("genTicks %v: 나아가지 않는데 오류가 없음", r)
		}
	}
	if ts, err := genTicks(base, base.AddDate(MaxYears, 0, 0), rung{tickMinute, 1}); err != nil || len(ts) != MaxTicks+1 {
		t.Errorf("100년 분 눈금: %d 개 %v — MaxTicks+1 에서 멈춰야 한다", len(ts), err)
	}
	// [2] 기간 곱셈 넘침 — 곱하기 전에 단위 상한
	for _, d := range []string{"5124096h", "101y", "1201M", "5219w", "36526d", "876601h", "52596001m", "3155760001s",
		"3155760000001ms", "99999999999999999999d", "9223372036854775807ms"} {
		errAt(t, "gantt\na :2026-01-01, "+d, 2, "100년을 넘음")
	}
	drawn(t, "gantt\na :2026-01-01, 100y")
	drawn(t, "gantt\na :2026-01-01, 876000h")
	errAt(t, "gantt\na :2026-01-01, +5d", 2, "맞지 않는 날짜")
	// [3] inclusiveEndDates 는 꼴의 가장 작은 단위(분)
	p := newTestParser()
	if err := p.parse("gantt\ndateFormat YYYY-MM-DD HH:mm\ninclusiveEndDates\na :2026-01-01 09:00, 2026-01-01 09:30\nb :1h"); err != nil {
		t.Fatal(err)
	}
	if got := p.c.Tasks[0].End.Format("15:04"); got != "09:31" {
		t.Errorf("분 꼴 포함 끝 %s, 기대 09:31", got)
	}
	if got := p.c.Tasks[1].Start.Format("15:04"); got != "09:31" {
		t.Errorf("뒤 작업 시작 %s, 기대 09:31", got)
	}
	// [4] axisFormat 의 한글 글자 — 실제 출력 문자열
	svg, _ := drawn(t, "gantt\naxisFormat %m월 %d일\na :2026-01-05, 3d")
	if labs, _, _ := axisInfo(svg); len(labs) == 0 || labs[0] != "01월 05일" {
		t.Errorf("한글 축 라벨 %q, 기대 첫 라벨 01월 05일", labs)
	}
	if a, _ := parseAxisFormat("주 %e일 · %H시"); a.format(base.Add(9*time.Hour)) != "주 1일 · 09시" {
		t.Errorf("axisFormat 글자 보존: %q", a.format(base.Add(9*time.Hour)))
	}
	// [5] 제외 띠는 자정에서 시작하고 viewBox 안 — 일 눈금(격자선) x 와 같은 자리
	svg, _ = drawn(t, "gantt\nexcludes weekends\na :2026-01-05, 5d")
	vw := viewW(t, svg)
	_, gx, _ := axisInfo(svg)
	onGrid := map[string]bool{}
	for _, g := range gx {
		onGrid[strconv.FormatFloat(g, 'f', -1, 64)] = true
	}
	ex := regexp.MustCompile(`<rect class="gt-excl" x="([\d.]+)" y="[\d.]+" width="([\d.]+)"`).FindAllStringSubmatch(svg, -1)
	// 01-04(일)은 왼쪽 여백 안에서 그림 시작(축 왼쪽 끝 x=20)으로 잘리고, 01-10(토)·01-11(일)은 자정 눈금에서 하루 폭
	if len(ex) != 3 || ex[0][1] != "20" {
		t.Fatalf("제외 띠 %d 개, 기대 3(01-04 일 — 잘림·01-10 토·01-11 일)\n%s", len(ex), svg)
	}
	for k, m := range ex {
		x, _ := strconv.ParseFloat(m[1], 64)
		w, _ := strconv.ParseFloat(m[2], 64)
		if x+w > vw+0.05 || (k > 0 && !onGrid[m[1]]) || (k > 0 && m[2] != ex[1][2]) {
			t.Errorf("제외 띠 x=%v w=%v — 자정 눈금 위가 아니거나 viewBox(%v) 밖\n%s", x, w, vw, svg)
		}
	}
	// 시각 꼴의 날짜 제외는 그리지 않는다(원문) — weekends 는 그린다
	errAt(t, "gantt\nexcludes 2026-01-06\ndateFormat YYYY-MM-DD HH:mm\na :2026-01-05 09:00, 3d", 2, "시각이 있는 dateFormat")
	drawn(t, "gantt\ndateFormat YYYY-MM-DD HH:mm\nexcludes weekends\na :2026-01-05 09:00, 3d")
	// [6] 긴 이름(구역·작업 200자) — 글자가 viewBox 안(속성 시험의 검사 함수로)
	long := "gantt\nsection " + strings.Repeat("가", MaxLabel) + "\n" + strings.Repeat("나", MaxLabel) + " :milestone, 2026-01-01, 0d"
	svg, _ = drawn(t, long)
	checkLayout(t, "긴 이름", svg)
	// [8] 고정 자릿수는 숫자만, 값 범위, 같은 토큰 두 번
	errAt(t, "gantt\na :2026-+1-01, 1d", 2, "맞지 않는 날짜")
	errAt(t, "gantt\na :+026-01-01, 1d", 2, "맞지 않는 날짜")
	errAt(t, "gantt\na :2026-01- 1, 1d", 2, "맞지 않는 날짜")
	errAt(t, "gantt\na :2026-13-01, 1d", 2, "맞지 않는 날짜")
	errAt(t, "gantt\na :2026-01-32, 1d", 2, "맞지 않는 날짜")
	errAt(t, "gantt\ndateFormat YYYY-MM-DD HH:mm\na :2026-01-01 24:00, 1h", 3, "맞지 않는 날짜")
	errAt(t, "gantt\ndateFormat YYYY-MM-DD HH:mm\na :2026-01-01 09:60, 1h", 3, "맞지 않는 날짜")
	errAt(t, "gantt\ndateFormat YYYY-MM-MM\na :2026-01-01, 1d", 2, "두 번")
}

func viewW(t *testing.T, svg string) float64 {
	t.Helper()
	m := regexp.MustCompile(`viewBox="0 0 ([\d.]+) ([\d.]+)"`).FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("viewBox 없음")
	}
	v, _ := strconv.ParseFloat(m[1], 64)
	return v
}

// 클래스 충돌(수정 1회차) — 간트 SVG 의 클래스는 모두 gt- 접두사를 단다. 호스트 앱 창 틀 CSS 의 .bar { height: 46px } 같은
// 앱 도구 막대 규칙이 <rect class="bar"> 의 기하 속성에 걸려 막대가 3.5배로 그려지는 충돌을 막는다.
// .gt- 선택자는 base.css 의 .diagram-svg 아래에만 있어야 하고, 흐름도 클래스(node·edge … 접두사 없음)도
// 지금 web/*.css(share.css 등 — 창 틀 CSS 가 이 폴더에 있으면 그것까지)와 겹치지 않음을 같은 시험이 고정한다.
func TestClassCollisions(t *testing.T) {
	svg, _ := drawn(t, "gantt\ndateFormat YYYY-MM-DD\ntitle 제목\nsection 구역\na :done, 2026-01-05, 4d\nb :active, 2026-01-12, 5d\nc :crit, 2026-01-19, 1w\nd :milestone, 2026-01-26, 0d")
	classWords := func(svg string) map[string]bool {
		out := map[string]bool{}
		for _, m := range regexp.MustCompile(`class="([^"]*)"`).FindAllStringSubmatch(svg, -1) {
			for _, w := range strings.Fields(m[1]) {
				out[w] = true
			}
		}
		return out
	}
	for w := range classWords(svg) {
		if w != "diagram-svg" && !strings.HasPrefix(w, "gt-") {
			t.Errorf("간트 클래스 %q 에 gt- 접두사가 없음", w)
		}
	}

	files, err := filepath.Glob("../../web/*.css")
	if err != nil || len(files) == 0 {
		t.Fatalf("dist css 를 못 찾음: %v", err)
	}
	css := map[string][]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s 읽기 실패: %v", f, err)
		}
		css[filepath.Base(f)] = strings.Split(string(b), "\n")
	}
	for name, lines := range css {
		for i, line := range lines {
			if strings.Contains(line, ".gt-") && !strings.Contains(line, ".diagram-svg") {
				t.Errorf("%s:%d — .gt- 선택자가 .diagram-svg 문맥 밖", name, i+1)
			}
		}
	}

	// 흐름도 — 접두사 없는 클래스가 앱 css(창 틀 app.css 가 있으면 그것·share.css)의 선택자와 겹치면 실패
	fr, err := flowchart.Render("flowchart LR\n  A[시작] --> B{갈림}\n  B -->|예| C((고리))\n  B -->|아니오| A\n  subgraph S[묶음]\n    C\n  end")
	if err != nil {
		t.Fatalf("흐름도 렌더 실패: %v", err)
	}
	appCSS := css["app.css"]
	if len(appCSS) == 0 {
		css["share.css"] = css["share.css"] // app.css 가 없는 배포에서도 share.css 검사는 돈다
	}
	for w := range classWords(fr.SVG) {
		if w == "diagram-svg" {
			continue
		}
		for _, name := range []string{"app.css", "share.css"} {
			for i, line := range css[name] {
				for _, m := range regexp.MustCompile(`\.`+regexp.QuoteMeta(w)).FindAllStringIndex(line, -1) {
					end := m[1]
					if end < len(line) {
						c := line[end]
						if c == '-' || c == '_' || (c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
							continue // 더 긴 이름의 일부다
						}
					}
					t.Errorf("흐름도 클래스 .%s 가 %s:%d 의 선택자와 겹침: %s", w, name, i+1, strings.TrimSpace(line))
				}
			}
		}
	}
}

// 속성 시험 — 같지 않은 무작위 그림 수십 회: 막대가 행 안·축 범위 안, 글자(추정 폭·높이)가 viewBox 안, 구역 띠가 서로
// 겹치지 않고, 작업 이름끼리·구역 이름끼리·축 라벨끼리 겹치지 않는다. 긴 이름(한글 200자)·긴 구역 이름도 섞는다.
// 시드를 고정해 무작위도 결정적으로 만든다(오늘 날짜·난수 의존 금지는 원문→출력의 규칙이다).
func TestLayoutProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(2026))
	longName := func(r rune) string { return strings.Repeat(string(r), 1+rng.Intn(MaxLabel)) }
	ran, longSeen := 0, 0
	for round := 0; round < 60; round++ {
		var b strings.Builder
		b.WriteString("gantt\ndateFormat YYYY-MM-DD\n")
		if rng.Intn(2) == 0 {
			b.WriteString("excludes weekends\n")
		}
		if rng.Intn(3) == 0 {
			b.WriteString("title 제목 " + strconv.Itoa(round) + "\n")
		}
		nTask := 2 + rng.Intn(MaxTasks-2)
		day := rng.Intn(90)
		nSec := 0
		for i := 0; i < nTask; i++ {
			if i > 0 && nSec < MaxSections-5 && rng.Intn(4) == 0 {
				name := "구역 " + strconv.Itoa(i)
				if rng.Intn(5) == 0 {
					name = longName('가')
				}
				b.WriteString("section " + name + "\n")
				nSec++
			}
			day += rng.Intn(10)
			name := "작업" + strconv.Itoa(i)
			switch rng.Intn(8) {
			case 0:
				name = longName('나')
			case 1:
				name = "긴 이름 " + strings.Repeat("낱말 ", rng.Intn(30)) + strconv.Itoa(i)
			}
			start := time.Date(2026, 1, 1+day, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
			switch rng.Intn(4) {
			case 0:
				fmt.Fprintf(&b, "%s :done, %s, %dd\n", name, start, 1+rng.Intn(30))
			case 1:
				fmt.Fprintf(&b, "%s :crit, %s, %dd\n", name, start, 1+rng.Intn(30))
			case 2:
				fmt.Fprintf(&b, "%s :milestone, %s, 0d\n", name, start)
			default:
				fmt.Fprintf(&b, "%s :%s, %dd\n", name, start, 1+rng.Intn(30))
			}
		}
		src := b.String()
		if len(src) > MaxSource { // 긴 이름이 많으면 원문 상한을 넘는다 — 그 회차는 건너뛴다(아래에서 실행 수를 단언)
			continue
		}
		r, err := Render(src)
		if err != nil {
			t.Fatalf("round %d: %v\n%s", round, err, src)
		}
		checkLayout(t, "round "+strconv.Itoa(round), r.SVG)
		ran++
		if strings.Contains(r.SVG, "<tspan") && strings.Count(r.SVG, "<tspan") > nTask {
			longSeen++
		}
	}
	if ran < 30 || longSeen < 10 {
		t.Errorf("실행 %d 회·여러 줄 이름 섞인 회차 %d — 표본이 모자람", ran, longSeen)
	}
}

// box 는 글자 한 줄의 추정 경계다(폭은 diagram.TextW, 높이는 글자 크기 — 기준선 위 0.85·아래 0.25).
type box struct {
	cls        string
	x0, y0, x1 float64
	y1         float64
	s          string
}

func (a box) overlaps(b box) bool {
	return a.x0 < b.x1-0.05 && b.x0 < a.x1-0.05 && a.y0 < b.y1-0.05 && b.y0 < a.y1-0.05
}

// textBoxes 는 SVG 의 <text>·<tspan> 줄마다 추정 경계다(줄 하나에 상자 하나, 부모 text 의 class·font-size·anchor 를 잇는다).
func textBoxes(t *testing.T, svg string) []box {
	t.Helper()
	var out []box
	attr := func(s, k string) string {
		m := regexp.MustCompile(`\b` + k + `="([^"]*)"`).FindStringSubmatch(s)
		if m == nil {
			return ""
		}
		return m[1]
	}
	mk := func(cls, anchor, fs, xs, ys, content string) box {
		f, _ := strconv.ParseFloat(fs, 64)
		x, _ := strconv.ParseFloat(xs, 64)
		y, _ := strconv.ParseFloat(ys, 64)
		content = html.UnescapeString(content)
		w := diagram.TextW(content, f)
		x0 := x
		switch anchor {
		case "middle":
			x0 = x - w/2
		case "end":
			x0 = x - w
		}
		return box{cls: cls, x0: x0, x1: x0 + w, y0: y - f*0.85, y1: y + f*0.25, s: content}
	}
	for _, m := range regexp.MustCompile(`<text ([^>]*)>(.*?)</text>`).FindAllStringSubmatch(svg, -1) {
		cls, anchor, fs := attr(m[1], "class"), attr(m[1], "text-anchor"), attr(m[1], "font-size")
		if !strings.Contains(m[2], "<tspan") {
			out = append(out, mk(cls, anchor, fs, attr(m[1], "x"), attr(m[1], "y"), m[2]))
			continue
		}
		for _, ts := range regexp.MustCompile(`<tspan ([^>]*)>(.*?)</tspan>`).FindAllStringSubmatch(m[2], -1) {
			out = append(out, mk(cls, anchor, fs, attr(ts[1], "x"), attr(ts[1], "y"), ts[2]))
		}
	}
	return out
}

// checkLayout 은 SVG 하나의 배치 성질을 본다(요소가 viewBox 안, 글자 경계가 viewBox 안, 겹침 없음).
func checkLayout(t *testing.T, name, svg string) {
	t.Helper()
	m := regexp.MustCompile(`viewBox="0 0 ([\d.]+) ([\d.]+)"`).FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("%s: viewBox 없음", name)
	}
	vw, _ := strconv.ParseFloat(m[1], 64)
	vh, _ := strconv.ParseFloat(m[2], 64)
	f := func(el el, k string) float64 {
		v, err := strconv.ParseFloat(el.attrs[k], 64)
		if err != nil {
			t.Fatalf("%s: <%s> 의 %s=%q 이 숫자가 아님", name, el.tag, k, el.attrs[k])
		}
		return v
	}
	nums := regexp.MustCompile(`-?[\d.]+`)
	var bands []struct{ y, h float64 }
	for _, e := range splitEls(svg) {
		switch e.tag {
		case "svg", "title", "g", "text", "tspan":
		case "rect":
			x, y := f(e, "x"), f(e, "y")
			w, h := f(e, "width"), f(e, "height")
			if x < -0.05 || y < -0.05 || w < 0 || h < 0 || x+w > vw+0.05 || y+h > vh+0.05 {
				t.Fatalf("%s: <rect class=%q> 가 viewBox 밖 x=%v y=%v w=%v h=%v\n%s", name, e.attrs["class"], x, y, w, h, svg)
			}
			if e.attrs["class"] == "gt-band" {
				for _, b2 := range bands {
					if y < b2.y+b2.h-0.05 && b2.y < y+h-0.05 {
						t.Fatalf("%s: 구역 띠 겹침\n%s", name, svg)
					}
				}
				bands = append(bands, struct{ y, h float64 }{y, h})
			}
		case "path": // 마름모 — 꼭짓점(x y 쌍)이 viewBox 안
			fs := nums.FindAllString(strings.ReplaceAll(strings.ReplaceAll(e.attrs["d"], "M", " "), "L", " "), -1)
			for k, ns := range fs {
				v, _ := strconv.ParseFloat(ns, 64)
				lim := vw
				if k%2 == 1 {
					lim = vh
				}
				if v < -0.05 || v > lim+0.05 {
					t.Fatalf("%s: path 좌표 %v 밖\n%s", name, v, svg)
				}
			}
		case "line":
			for _, k := range []string{"x1", "x2", "y1", "y2"} {
				v, lim := f(e, k), vw
				if k[0] == 'y' {
					lim = vh
				}
				if v < -0.05 || v > lim+0.05 {
					t.Fatalf("%s: line %s=%v 밖\n%s", name, k, v, svg)
				}
			}
		default:
			t.Fatalf("%s: 모르는 요소 <%s>", name, e.tag)
		}
	}
	boxes := textBoxes(t, svg)
	if len(boxes) == 0 {
		t.Fatalf("%s: 글자가 없음", name)
	}
	for _, b := range boxes {
		if b.x0 < -0.05 || b.y0 < -0.05 || b.x1 > vw+0.05 || b.y1 > vh+0.05 {
			t.Fatalf("%s: 글자 %q(%s) 경계 [%.1f,%.1f]-[%.1f,%.1f] 가 viewBox %vx%v 밖\n%s", name, b.s, b.cls, b.x0, b.y0, b.x1, b.y1, vw, vh, svg)
		}
	}
	// 같은 종류 글자끼리 겹침 없음(작업 이름·구역 이름·축 라벨)
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i], boxes[j]
			ka, kb := strings.Fields(a.cls)[0], strings.Fields(b.cls)[0]
			if ka != kb || ka == "gt-title" {
				continue
			}
			if a.overlaps(b) {
				t.Fatalf("%s: 글자 %q 와 %q(%s)가 겹침\n%s", name, a.s, b.s, ka, svg)
			}
		}
	}
}

type el struct {
	tag   string
	attrs map[string]string
}

func splitEls(svg string) []el {
	var out []el
	dec := func(s string) el {
		s = strings.TrimPrefix(s, "<")
		i := strings.IndexByte(s, ' ')
		if i < 0 {
			i = strings.IndexAny(s, ">/")
			if i < 0 {
				i = len(s)
			}
		}
		e := el{tag: s[:i], attrs: map[string]string{}}
		for _, m := range regexp.MustCompile(`([a-zA-Z0-9-]+)="([^"]*)"`).FindAllStringSubmatch(s, -1) {
			e.attrs[m[1]] = m[2]
		}
		return e
	}
	for _, part := range strings.Split(svg, "<") {
		if part == "" || strings.HasPrefix(part, "?") || strings.HasPrefix(part, "/") {
			continue
		}
		out = append(out, dec(part))
	}
	return out
}

// ---- 리뷰 2회차 발견 [1]~[4]와 극단 날짜 속성 시험(수정 3회차)

// civilDays 는 1970-01-01 부터 센 일수다(Howard Hinnant days_from_civil — time 패키지의 Duration·Unix 와 독립인 정수 계산).
func civilDays(y, m, d int) int {
	if m <= 2 {
		y--
	}
	era := y / 400
	if y < 0 && y%400 != 0 {
		era--
	}
	yoe := y - era*400
	doy := (153*((m+9)%12)+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

func modFloor(a, n int) int {
	r := a % n
	if r < 0 {
		r += n
	}
	return r
}

// onRung 은 시각 t 가 간격 r 의 눈금 자리인지 독립 계산으로 본다(alignFloor 를 쓰지 않는다): 분·시·일·달은 상위 단위 안
// 필드가 n 의 배수, 주는 일요일 0시이고 1969-12-28(일)부터 센 주 번호가 n 의 배수, 년은 1월 1일 0시이고 연도가 n 의 배수.
func onRung(t time.Time, r rung) bool {
	y, mo, d := t.Date()
	h, mi, s := t.Clock()
	if s != 0 || t.Nanosecond() != 0 {
		return false
	}
	switch r.u {
	case tickMinute:
		return mi%r.n == 0
	case tickHour:
		return mi == 0 && h%r.n == 0
	}
	if h != 0 || mi != 0 {
		return false
	}
	switch r.u {
	case tickDay:
		return (d-1)%r.n == 0
	case tickWeek:
		days := civilDays(y, int(mo), d)
		return modFloor(days+4, 7) == 0 && modFloor((days-civilDays(1969, 12, 28))/7, r.n) == 0 // 1970-01-01 은 목요일(4)
	case tickMonth:
		return d == 1 && (int(mo)-1)%r.n == 0
	default:
		return d == 1 && mo == 1 && modFloor(y, r.n) == 0
	}
}

// stepOf 는 간격 r 의 한 걸음(다음 눈금은 이보다 멀지 않다 — 일·달은 상위 단위 경계에서 더 가깝게 다시 센다).
func stepOf(t time.Time, r rung) time.Time {
	switch r.u {
	case tickMinute:
		return t.Add(time.Duration(r.n) * time.Minute)
	case tickHour:
		return t.Add(time.Duration(r.n) * time.Hour)
	case tickDay:
		return t.AddDate(0, 0, r.n)
	case tickWeek:
		return t.AddDate(0, 0, 7*r.n)
	case tickMonth:
		return t.AddDate(0, r.n, 0)
	default:
		return t.AddDate(r.n, 0, 0)
	}
}

// ticksOnSomeRung 은 눈금 열이 후보 간격 하나의 자리에 모두 서고, 첫 눈금이 rlo 에서·마지막이 rhi 까지 한 걸음 안이며
// 이웃 눈금 사이가 한 걸음 이하인지(빠진 눈금 없음) 본다. 맞는 간격을 돌려준다.
func ticksOnSomeRung(ts []time.Time, rlo, rhi time.Time, cands []rung) (rung, bool) {
next:
	for _, r := range cands {
		if stepOf(rlo, r).Before(ts[0]) || !stepOf(ts[len(ts)-1], r).After(rhi) {
			continue
		}
		for k, x := range ts {
			if !onRung(x, r) || (k > 0 && stepOf(ts[k-1], r).Before(x)) {
				continue next
			}
		}
		return r, true
	}
	return rung{}, false
}

func TestReviewR2(t *testing.T) {
	// [1] 요일 이름만 적은 제외도 켜진다 — 실제 끝 날짜와 띠
	for _, c := range []struct{ src, end string }{
		{"gantt\nexcludes friday\na :2026-01-05, 5d", "2026-01-11"},                   // 금 01-09 → 하루
		{"gantt\nexcludes tuesday, friday\na :2026-01-05, 5d", "2026-01-12"},          // 화 01-06·금 01-09
		{"gantt\nexcludes tuesday\nexcludes friday\na :2026-01-05, 5d", "2026-01-12"}, // 여러 줄 합침
		{"gantt\nexcludes Friday\na :2026-01-05, 5d", "2026-01-11"},                   // 대소문자
		{"gantt\na :2026-01-05, 5d", "2026-01-10"},                                    // 제외 없음
	} {
		p := newTestParser()
		if err := p.parse(c.src); err != nil {
			t.Fatal(err)
		}
		if got := p.c.Tasks[0].End.Format("2006-01-02"); got != c.end {
			t.Errorf("%q: 끝 %s, 기대 %s", c.src, got, c.end)
		}
		svg, _ := drawn(t, c.src)
		if has := strings.Contains(svg, `class="gt-excl"`); has != strings.Contains(c.src, "excludes") {
			t.Errorf("%q: 제외 띠 %v", c.src, has)
		}
	}

	// [2] 주 번호는 epoch 에서 먼 해에도 맞다 — 1500·0001·2500·9999 년의 2주 눈금(명시·자동)이 모두 짝수 주(독립 계산)
	for _, y := range []string{"1500", "0001", "2500", "9999", "1969", "2026"} {
		for _, head := range []string{"tickInterval 2week\n", ""} {
			src := "gantt\n" + head + "a :" + y + "-01-05, 12w"
			c, _, err := Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := drawSVG(c); err != nil {
				t.Fatal(err)
			}
			if len(c.ticks) < 2 {
				t.Fatalf("%q: 눈금 %d 개", src, len(c.ticks))
			}
			for k, x := range c.ticks {
				days := civilDays(x.Year(), int(x.Month()), x.Day())
				if modFloor(days+4, 7) != 0 || modFloor((days-civilDays(1969, 12, 28))/7, 2) != 0 {
					t.Errorf("%q: 눈금 %s 가 짝수 주의 일요일이 아님(주 번호 %d)", src, x.Format("2006-01-02"), (days-civilDays(1969, 12, 28))/7)
				}
				if k > 0 && x.Sub(c.ticks[k-1]) != 14*24*time.Hour {
					t.Errorf("%q: 눈금 간격 %v — 2주가 아님", src, x.Sub(c.ticks[k-1]))
				}
			}
		}
	}

	// [3] 사다리 마지막 칸까지 라벨이 겹치면 원문 + 경고(axisFormat 줄, 없으면 머리 줄)
	longAxis := "axisFormat " + strings.Repeat("가", 100) + " %Y"
	errAt(t, "gantt\n"+longAxis+"\na :1900-01-01, 100y", 2, "축 라벨이 겹쳐 그리지 않음")
	errAt(t, "gantt\ndateFormat YYYY\n"+longAxis+"\na :1900, 100y", 3, "axisFormat 을 줄인다")
	errAt(t, "gantt\naxisFormat "+strings.Repeat("나", 190)+" %Y\na :0100-01-01, 100y", 2, "축 라벨이 겹쳐") // 0100·0200 두 눈금
	drawn(t, "gantt\n"+longAxis+"\na :1900-01-01, 10y")                                               // 10년이면 눈금 하나(겹칠 것이 없다) — 그린다
	c, _, err := Parse("%% 메모\ngantt\na :1900-01-01, 100y")
	if err != nil {
		t.Fatal(err)
	}
	rlo, rhi := timeRange(c.Tasks)
	_, _, err = chooseTicks(c, rlo, rhi, func([]time.Time) bool { return false })
	var de *Error
	if !errors.As(err, &de) || de.Line != 2 || !strings.Contains(de.Msg, "축 라벨이 겹쳐") {
		t.Errorf("axisFormat 없는 그림의 겹침 오류 %v — 머리 줄(2)이어야 한다", err)
	}

	// [4] 연도 0000 의 after — 첫 참조 작업 값으로 시작한 최댓값
	p := newTestParser()
	if err := p.parse("gantt\na :a, 0000-01-01, 1d\nb :after a, 1d\nc :c, 0000-12-30, 1d\nd :d, 0000-06-01, 1d\ne :after d c, 1d\nf :f, 0000-12-31, 2d\ng :after d f, 1d\nh :after d, 1d"); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[int]string{1: "0000-01-02", 4: "0000-12-31", 6: "0001-01-02", 7: "0000-06-02"} {
		if got := p.c.Tasks[k].Start.Format("2006-01-02"); got != want {
			t.Errorf("작업 %s 시작 %s, 기대 %s", p.c.Tasks[k].Name, got, want)
		}
	}
	drawn(t, "gantt\na :a, 0000-01-01, 1d\nb :after a, 1d")
}

// 리뷰 3회차 발견 [1] — 설정 줄의 적용 순서. 설정(dateFormat·excludes·inclusiveEndDates)이 작업 뒤에 와도
// 그림 전체에 적용된다(두 단계 계산 — parse.go computeTasks): 리뷰 재현 표의 네 줄 순서 각각의 실제 끝이 설정을
// 앞에 둔 입력과 같고, 제외 띠와 막대도 같은 설정을 따른다(설정 줄 위치만 다른 입력의 SVG 바이트 같음).
func TestReviewR3(t *testing.T) {
	// 재현 표 — excludes friday: 앞(01-11)·뒤(예전엔 01-10) 순서가 같은 끝을 낸다
	front := "gantt\nexcludes friday\na :2026-01-05, 5d"
	back := "gantt\na :2026-01-05, 5d\nexcludes friday"
	for _, src := range []string{front, back} {
		p := newTestParser()
		if err := p.parse(src); err != nil {
			t.Fatal(err)
		}
		if got := p.c.Tasks[0].End.Format("2006-01-02"); got != "2026-01-11" {
			t.Errorf("%q: 끝 %s, 기대 2026-01-11(설정이 앞에 있을 때와 같음)", src, got)
		}
		svg, issues := drawn(t, src)
		if len(issues) != 0 {
			t.Errorf("%q: 경고 %+v", src, issues)
		}
		if n := strings.Count(svg, `class="gt-excl"`); n != 1 {
			t.Errorf("%q: 제외 띠 %d 개, 기대 1(금요일 01-09)\n%s", src, n, svg)
		}
	}
	// 재현 표 — inclusiveEndDates: 앞(01-11)·뒤(예전엔 01-10) 순서가 같은 끝을 낸다
	for _, src := range []string{
		"gantt\ninclusiveEndDates\na :2026-01-05, 2026-01-10",
		"gantt\na :2026-01-05, 2026-01-10\ninclusiveEndDates",
	} {
		p := newTestParser()
		if err := p.parse(src); err != nil {
			t.Fatal(err)
		}
		if got := p.c.Tasks[0].End.Format("2006-01-02"); got != "2026-01-11" {
			t.Errorf("%q: 포함 끝 %s, 기대 2026-01-11", src, got)
		}
	}
	// 설정 줄 위치만 다른 두 입력의 SVG 는 바이트까지 같다(띠·막대가 같은 설정)
	svgF, _ := drawn(t, front)
	svgB, _ := drawn(t, back)
	if svgF != svgB {
		t.Errorf("excludes 줄 위치만 다른 두 입력의 SVG 가 다름\n%s\n%s", svgF, svgB)
	}
	// 작업 사이에 excludes 줄이 여럿 섞여도 합쳐져 전체에 적용된다 — 전부 앞으로 옮긴 입력과 끝·SVG 가 같다
	// (b 는 값 하나 작업 — 시작이 2단계에서 계산된 a 의 끝이다)
	mixed := "gantt\na :2026-01-05, 5d\nexcludes tuesday\nb :3d\nexcludes friday\nc :3d"
	frontAll := "gantt\nexcludes tuesday\nexcludes friday\na :2026-01-05, 5d\nb :3d\nc :3d"
	pm, pf := newTestParser(), newTestParser()
	if err := pm.parse(mixed); err != nil {
		t.Fatal(err)
	}
	if err := pf.parse(frontAll); err != nil {
		t.Fatal(err)
	}
	if got := pm.c.Tasks[0].End.Format("2006-01-02"); got != "2026-01-12" {
		t.Errorf("섞은 입력의 a 끝 %s, 기대 2026-01-12(화 01-06·금 01-09 제외로 2일 밀림)", got)
	}
	for k := range pm.c.Tasks {
		if !pm.c.Tasks[k].End.Equal(pf.c.Tasks[k].End) {
			t.Errorf("작업 %s: 섞은 끝 %v, 앞으로 옮긴 끝 %v", pm.c.Tasks[k].Name, pm.c.Tasks[k].End, pf.c.Tasks[k].End)
		}
	}
	sm, _ := drawn(t, mixed)
	sf2, _ := drawn(t, frontAll)
	if sm != sf2 {
		t.Errorf("excludes 줄을 앞으로 옮긴 입력의 SVG 가 다름")
	}
	// 작업 뒤의 dateFormat — 그 꼴로 앞 작업 날짜를 읽는다(기본 꼴로는 읽히지 않아 예전엔 원문이던 값)
	p := newTestParser()
	if err := p.parse("gantt\na :2026/01/05, 5d\ndateFormat YYYY/MM/DD"); err != nil {
		t.Fatal(err)
	}
	if got := p.c.Tasks[0].End.Format("2006-01-02"); got != "2026-01-10" {
		t.Errorf("작업 뒤 dateFormat: 끝 %s, 기대 2026-01-10", got)
	}
	// after 연쇄가 설정 뒤에 와도 같은 결과(참조 끝도 2단계 차례)
	chainBack := "gantt\na :a1, 2026-01-01, 2d\nb :b1, after a1, 3d\nexcludes weekends\ndateFormat YYYY-MM-DD"
	chainFront := "gantt\ndateFormat YYYY-MM-DD\nexcludes weekends\na :a1, 2026-01-01, 2d\nb :b1, after a1, 3d"
	pc := newTestParser()
	if err := pc.parse(chainBack); err != nil {
		t.Fatal(err)
	}
	if got := pc.c.Tasks[1].Start.Format("2006-01-02"); got != "2026-01-05" {
		t.Errorf("after 연쇄(설정 뒤): b 시작 %s, 기대 2026-01-05(a 의 밀린 끝)", got)
	}
	sb2, _ := drawn(t, chainBack)
	sf3, _ := drawn(t, chainFront)
	if sb2 != sf3 {
		t.Errorf("after 연쇄: 설정 줄 위치만 다른 두 입력의 SVG 가 다름")
	}
}

// 극단 날짜 속성 시험(수정 3회차, itda-hyve#208 접근 방식 — 상한을 비껴 가는 숫자·날짜 입력을 한 시험으로 닫는다).
// 시드 고정 300판: 시작 연도 0001~9999, 범위 1분~100년, dateFormat 6꼴 × axisFormat(긴 글자 포함) × tickInterval 명시 ×
// excludes 네 꼴 × after/until 연쇄. 4회차부터 설정 줄(dateFormat·axisFormat·tickInterval·excludes)을 작업 앞·사이·뒤에
// 무작위로 섞고(같은 시드면 같은 입력), 설정을 모두 머리 바로 뒤로 옮긴 입력과 성패·SVG 바이트가 같음을 본다(두 단계 계산).
// 결과가 *Error 이거나 아니면 눈금 ≤ MaxTicks·엄격 증가·첫 눈금 ≥ 범위 시작·간격 경계 정렬(독립 계산)·격자선 수 = 라벨 수·
// 라벨 안 겹침·모든 rect/text 가 viewBox 안·같은 입력 두 번 바이트 같음. 벽시계는 정지 감지(판당 2초·전체 60초)로만
// 본다(리뷰 3회차 의견 3 — 종료성은 결정적 한계가 지킨다: 눈금 ≤ MaxTicks·genTicks 의 비전진 오류·pushEnd 반복 상한).
func TestExtremeDates(t *testing.T) {
	rng := rand.New(rand.NewSource(40903))
	dfs := []string{"YYYY-MM-DD", "YYYY-MM-DD HH:mm", "YYYY/MM/DD", "YYYY.MM.DD", "YYYY-MM", "YYYY"}
	afs := []string{"", "%Y-%m-%d", "%Y", "%y.%m", "%H:%M", "%d일 %H시", "%m월 %d일", "LONG"}
	exs := []string{"", "weekends", "friday", "monday, saturday", "DATE", "weekends, tuesday, DATE"}
	ticksKinds := []string{"day", "week", "month"}
	var drew, failed, longDrew, longFailed, explicit, farYears, empty int
	var slowest time.Duration
	all0 := time.Now()
	for round := 0; round < 300; round++ {
		df := dfs[rng.Intn(len(dfs))]
		dfp, err := parseDateFormat(df)
		if err != nil {
			t.Fatal(err)
		}
		year := 1 + rng.Intn(9999)
		// 경계 판(20판마다 하나): 긴 축 라벨 × 100년 칸 둘(x00 년 1월 1일부터 100년) — 사다리 마지막 칸까지 겹치는 입력
		boundary := round%20 == 0
		if boundary {
			year = 100 * (1 + rng.Intn(98))
		}
		if year < 300 || year > 2262 {
			farYears++ // time.Duration(±292년) 밖 — 리뷰 2회차 [2] 의 영역
		}
		st := time.Date(year, time.Month(1+rng.Intn(12)), 1+rng.Intn(28), rng.Intn(24), rng.Intn(60), 0, 0, time.UTC)
		if boundary {
			st = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		start := dfp.fmtString(st) // 꼴이 담는 만큼만 적는다(YYYY-MM 이면 달까지)
		// 범위: 1분 ~ 100년(로그 균등) — 꼴에 맞는 단위로 작업마다 나눈다
		total := math.Exp(rng.Float64() * math.Log(float64(MaxYears*525960))) // 분
		nTask := 1 + rng.Intn(6)
		if boundary {
			total, nTask = float64(MaxYears*525960), 1
		}
		per := total / float64(nTask)
		durOf := func(min float64) string {
			switch {
			case min >= 2*525960:
				return strconv.Itoa(int(min/525960)) + "y"
			case min >= 60*1440 && rng.Intn(2) == 0:
				return strconv.Itoa(int(min/43830)) + "M"
			case min >= 14*1440 && rng.Intn(2) == 0:
				return strconv.Itoa(int(min/10080)) + "w"
			case min >= 1440:
				return strconv.Itoa(int(min/1440)) + "d"
			case min >= 60:
				return strconv.Itoa(int(min/60)) + "h"
			default:
				return strconv.Itoa(max(1, int(min))) + "m"
			}
		}
		// 설정 줄(cfgs)과 몸통(구역·작업 줄)을 따로 만들고, 설정 줄은 머리 바로 뒤(작업 앞)~작업 뒤 어디에든 무작위로
		// 끼운다 — 설정 줄이 작업 사이·뒤에 있어도 결과는 같아야 한다(두 단계 계산).
		cfgs := []string{"dateFormat " + df}
		af := afs[rng.Intn(len(afs))]
		if boundary {
			af = "LONG"
		}
		isLong := af == "LONG"
		if isLong {
			af = strings.Repeat("가", 100+rng.Intn(101)) + " %Y"
		}
		if af != "" {
			cfgs = append(cfgs, "axisFormat "+af)
		}
		if rng.Intn(3) == 0 && !boundary {
			n := int(math.Exp(rng.Float64() * math.Log(1000)))
			cfgs = append(cfgs, fmt.Sprintf("tickInterval %d%s", max(1, n), ticksKinds[rng.Intn(3)]))
			explicit++
		}
		if ex := exs[rng.Intn(len(exs))]; ex != "" && !boundary { // 경계 판은 제외 없이(끝이 밀려 100년을 넘지 않게)
			cfgs = append(cfgs, "excludes "+strings.ReplaceAll(ex, "DATE", dfp.fmtString(st.AddDate(0, 0, 2))))
		}
		tags := []string{"", "done, ", "active, ", "crit, ", "done, crit, "}
		var body []string
		body = append(body, fmt.Sprintf("작업0 :%sa0, %s, %s", tags[rng.Intn(len(tags))], start, durOf(per)))
		for k := 1; k < nTask; k++ {
			if k%3 == 0 && rng.Intn(2) == 0 {
				body = append(body, "section 구역 "+strconv.Itoa(k))
			}
			switch rng.Intn(4) {
			case 0:
				body = append(body, fmt.Sprintf("작업%d :%sa%d, after a%d, %s", k, tags[rng.Intn(len(tags))], k, k-1, durOf(per)))
			case 1:
				body = append(body, fmt.Sprintf("작업%d :a%d, after a0 a%d, %s", k, k, k-1, durOf(per)))
			case 2: // 값 하나 — 바로 앞 작업의 끝에서. id 는 다음 작업이 참조하도록 3개 꼴로 바꿀 수 없으니 after 로 이어 준다
				body = append(body, fmt.Sprintf("작업%d :a%d, after a%d, %s", k, k, k-1, durOf(per)),
					fmt.Sprintf("이음%d :%s", k, durOf(per/4)))
			default:
				body = append(body, fmt.Sprintf("점%d :milestone, a%d, after a%d, 0d", k, k, k-1))
			}
		}
		if nTask > 1 && rng.Intn(2) == 0 {
			body = append(body, fmt.Sprintf("마감 :%s, until a1 a%d", start, nTask-1))
		}
		mixed := append([]string{"gantt"}, body...)
		for _, cfg := range cfgs {
			pos := 1 + rng.Intn(len(mixed)) // 머리 바로 뒤 ~ 끝(작업 뒤) — 작업 사이 포함
			mixed = append(mixed, "")
			copy(mixed[pos+1:], mixed[pos:])
			mixed[pos] = cfg
		}
		src := strings.Join(mixed, "\n")
		srcFront := strings.Join(append(append([]string{"gantt"}, cfgs...), body...), "\n")

		t0 := time.Now()
		r, err := Render(src)
		el := time.Since(t0)
		if el > slowest {
			slowest = el
		}
		if el > 2*time.Second {
			t.Fatalf("round %d: Render %v — 판당 2초를 넘음(정지 감지)\n%s", round, el, src)
		}
		if time.Since(all0) > 60*time.Second {
			t.Fatalf("300판이 60초를 넘음(정지 감지) — round %d", round)
		}
		// 설정을 모두 앞으로 옮긴 입력과 성패가 같고(오류면 메시지까지), 성공이면 SVG 바이트가 같다
		rF, errF := Render(srcFront)
		if (err == nil) != (errF == nil) {
			t.Fatalf("round %d: 설정 위치만 다른 입력의 성패가 다름(%v / %v)\n%s\n%s", round, err, errF, src, srcFront)
		}
		if err == nil && rF.SVG != r.SVG {
			t.Fatalf("round %d: 설정을 모두 앞으로 옮긴 입력의 SVG 가 다름\n%s\n%s", round, src, srcFront)
		}
		if err != nil {
			var de, deF *Error
			if !errors.As(err, &de) {
				t.Fatalf("round %d: 오류가 *Error 가 아님: %v\n%s", round, err, src)
			}
			if !errors.As(errF, &deF) || de.Msg != deF.Msg {
				t.Fatalf("round %d: 설정 위치만 다른 입력의 오류 메시지가 다름(%v / %v)", round, err, errF)
			}
			failed++
			if isLong {
				longFailed++
			}
			continue
		}
		drew++
		if isLong {
			longDrew++
		}
		r2, _ := Render(src)
		if r2.SVG != r.SVG {
			t.Fatalf("round %d: 두 번 그린 결과가 다름\n%s", round, src)
		}
		c, _, err := Parse(src)
		if err != nil {
			t.Fatal(err)
		}
		svg, _, err := drawSVG(c)
		if err != nil || svg != r.SVG {
			t.Fatalf("round %d: drawSVG 가 Render 와 다름(%v)", round, err)
		}
		ts := c.ticks
		labs, gx, _ := axisInfo(svg)
		if len(gx) != len(labs) || len(labs) != len(ts) {
			t.Fatalf("round %d: 격자선 %d·라벨 %d·눈금 %d 가 다름\n%s", round, len(gx), len(labs), len(ts), src)
		}
		if len(ts) > MaxTicks {
			t.Fatalf("round %d: 눈금 %d 개 — 상한 %d 를 넘음", round, len(ts), MaxTicks)
		}
		rlo, rhi := timeRange(c.Tasks)
		if len(ts) == 0 {
			empty++
		} else {
			if ts[0].Before(rlo) || ts[len(ts)-1].After(rhi) {
				t.Fatalf("round %d: 눈금 %v..%v 가 범위 %v..%v 밖\n%s", round, ts[0], ts[len(ts)-1], rlo, rhi, src)
			}
			for k := 1; k < len(ts); k++ {
				if !ts[k].After(ts[k-1]) {
					t.Fatalf("round %d: 눈금이 엄격 증가가 아님 %v → %v", round, ts[k-1], ts[k])
				}
			}
			cands := ladder(true, true)
			if c.HasTick {
				cands = append([]rung{c.Tick}, cands...)
			}
			if _, ok := ticksOnSomeRung(ts, rlo, rhi, cands); !ok {
				t.Fatalf("round %d: 눈금 %v 가 어느 간격의 경계에도 맞지 않음(범위 %v..%v)\n%s", round, ts, rlo, rhi, src)
			}
		}
		checkLayout(t, "round "+strconv.Itoa(round), svg) // rect·text viewBox 안, 축 라벨끼리 안 겹침
	}
	t.Logf("그림 %d·오류 %d(긴 축 라벨 그림 %d·오류 %d), tickInterval 명시 %d, ±292년 밖 시작 %d, 눈금 0개 %d, 가장 느린 판 %v",
		drew, failed, longDrew, longFailed, explicit, farYears, empty, slowest)
	if drew < 150 || longDrew == 0 || longFailed == 0 || explicit < 50 || farYears < 100 {
		t.Errorf("표본이 모자람: 그림 %d·긴 라벨 그림 %d·오류 %d·명시 %d·먼 해 %d", drew, longDrew, longFailed, explicit, farYears)
	}
}

// 설정 지시 — 줄 전체가 지시 하나인 줄만 무시하고 그린다. 그 밖에 %%{ 가 든 줄(지시 앞뒤·사이의 글, acc 값·작업 이름·
// 제목·구역 이름·설정 값·주석 안, accDescr 블록 안)은 원문 + 그 줄 번호(er·sequence 와 같은 공통 규칙 — itda-hyve#409 ④-1 후속).
func TestDirectiveLines(t *testing.T) {
	const d = "%%{init: {}}%%"
	const one = "한 줄에 하나만"
	const b = "\nb :2026-01-02, 1d"
	for _, c := range []struct {
		src  string
		line int
		msg  string
	}{
		{"gantt\n" + d + " 작업 :2026-01-01, 1d " + d + b, 2, one},
		{"gantt\n" + d + " ??? " + d + b, 2, one},
		{"gantt\naccTitle: 앞 " + d + " 뒤" + b, 2, one},
		{"gantt\naccDescr: 앞 " + d + " 뒤" + b, 2, one},
		{"gantt\n작업 " + d + " :2026-01-01, 1d", 2, one},
		{"gantt\n작업 :2026-01-01, 1d " + d, 2, one},
		{"gantt\ntitle 제목 " + d + b, 2, one},
		{"gantt\nsection 구역 " + d + b, 2, one},
		{"gantt\ndateFormat YYYY-MM-DD " + d + b, 2, one},
		{"gantt\n%% 메모 " + d + b, 2, one},
		{"gantt\naccDescr {\n" + d + "\n}" + b, 3, "블록 안"},
		{"gantt\naccDescr {\n글 " + d + "\n}" + b, 3, one},
		{"gantt\n%%{init: {" + b, 2, "한 줄로 닫아야"},
	} {
		errAt(t, c.src, c.line, c.msg)
	}
	for _, src := range []string{d + "\ngantt" + b, "gantt\n  " + d + "  " + b, "gantt\nsection s\n" + d + b} {
		svg, iss := drawn(t, src)
		if len(iss) != 1 || !strings.Contains(iss[0].Msg, "설정 지시") || strings.Contains(svg, "%%{") {
			t.Errorf("%q: 경고 %+v", src, iss)
		}
	}
}

// 줄 머리 %% 주석 — 원본 cleanupComments 처럼 accDescr { … } 블록 안에서도 지운다(설명에 넣지 않는다). 블록 밖 주석은 그대로
// 건너뛰고, 뒤에 글이 없는 맨 %% 줄은 원문 + 그 줄 번호(er 와 같은 차례·규칙 — itda-hyve#409 ④-1 후속 리뷰 1회차 [2]).
func TestCommentLines(t *testing.T) {
	const head, tail = "gantt\n", "\nb :2026-01-02, 1d"
	for _, c := range []struct{ src, desc string }{
		{head + "accDescr {\n설명\n%% 메모\n}" + tail, "설명"},
		{head + "accDescr {\n  %% 메모 }\n설명\n}" + tail, "설명"}, // 주석 줄은 줄째 지운다 — 안의 } 로 닫지 않는다
		{head + "accDescr { 앞\n%%  \n뒤 }" + tail, "앞\n뒤"},    // %% 뒤 공백만 있어도 원본 정규식은 지운다(블록 줄바꿈은 그대로 — 정책 B)
	} {
		r, err := Render(c.src)
		if err != nil {
			t.Errorf("%q: %v", c.src, err)
			continue
		}
		if !strings.Contains(r.SVG, "<title>"+c.desc+"</title>") || strings.Contains(r.SVG, "메모") || strings.Contains(r.SVG, "%%") {
			t.Errorf("%q: 설명이 %q 가 아니거나 주석이 남음\n%s", c.src, c.desc, r.SVG)
		}
	}
	// 블록 밖 맨 %% 는 원본 lexer 의 주석 규칙(jison:34)이 건너뛴다 — 경고 없이 그린다(itda-hyve#409 ④-1 후속 수정 2).
	for _, src := range []string{head + "%% 밖 주석" + tail, head + "%%" + tail, head + "  %%\r" + tail} {
		if r, err := Render(src); err != nil || len(r.Issues) != 0 {
			t.Errorf("%q 블록 밖 주석: %v %+v", src, err, r.Issues)
		}
	}
	// 블록 안 맨 %% 는 원본이 설명 글로 넣는다 — 이 판은 그리지 않는다(보수적).
	for _, c := range []struct {
		src  string
		line int
	}{{head + "accDescr {\n설명\n%%\n}" + tail, 4}, {head + "accDescr { 앞\n  %%\r\n뒤 }" + tail, 3}} {
		_, err := Render(c.src)
		var de *Error
		if !errors.As(err, &de) || de.Line != c.line || !strings.Contains(de.Msg, "블록 안의 글 없는") {
			t.Errorf("%q: 오류 %v, 기대 %d줄 블록 안의 글 없는 %%%% 줄", c.src, err, c.line)
		}
	}
}
