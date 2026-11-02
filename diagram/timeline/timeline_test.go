package timeline

import (
	"errors"
	"fmt"
	"html"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/diagram"
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
	// 머리 줄 — timeline · timeline LR(가로, 대소문자 무시 — 원본 jison case-insensitive)
	for _, h := range []string{"timeline", "timeline LR", "timeline lr", "Timeline"} {
		svg, issues := drawn(t, h+"\n2026 : 사건")
		if len(issues) != 0 {
			t.Errorf("머리 %q: 경고 %+v", h, issues)
		}
		if !strings.Contains(svg, ">2026<") {
			t.Errorf("머리 %q: 시기가 없음\n%s", h, svg)
		}
	}

	// title — 그림 안 제목(대소문자 무시). 제목의 <br>·줄바꿈은 한 줄로 뭉친다(gantt 와 같게). frontmatter title 은 캡션(공통)
	svg, issues := drawn(t, "timeline\ntitle 회사 연혁\n2026 : 사건")
	if len(issues) != 0 || !strings.Contains(svg, `class="tl-title"`) || !strings.Contains(svg, ">회사 연혁<") {
		t.Errorf("title 소문자: 경고 %+v·제목 없음\n%s", issues, svg)
	}
	for _, h := range []string{"TITLE 회사 연혁", "Title 회사 연혁"} {
		svg, issues = drawn(t, "timeline\n"+h+"\n2026 : 사건")
		if len(issues) != 0 || !strings.Contains(svg, `class="tl-title"`) || !strings.Contains(svg, ">회사 연혁<") {
			t.Errorf("title %q: 경고 %+v·제목 없음\n%s", h, issues, svg)
		}
	}
	svg, _ = drawn(t, "timeline\ntitle 회사<br>10년\n2026 : 사건")
	if !strings.Contains(svg, ">회사 10년<") { // <br> 은 공백으로 뭉친다
		t.Errorf("title 의 <br> 이 공백으로 뭉치지 않음\n%s", svg)
	}
	if r, err := Render("---\ntitle: 캡션\n---\ntimeline\n2026 : 사건"); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(r.SVG, `aria-label="캡션"`) {
		t.Errorf("frontmatter title 이 캡션이 아님\n%s", r.SVG)
	}

	// section — 구역 없는 앞 시기, 구역 머리 상자(시기 폭만큼 걸침). 대소문자 무시(리뷰 [3])
	svg, _ = drawn(t, "timeline\n서두 : 첫째\nsection 창업기\n2016 : 설립\n2018 : 투자\nsection 성장기\n2020 : 매출")
	for _, s := range []string{`class="tl-sec tl-c2"`, ">창업기<", ">성장기<", ">서두<", ">설립<", ">투자<", ">매출<"} {
		if !strings.Contains(svg, s) {
			t.Errorf("section: %s 없음\n%s", s, svg)
		}
	}
	// 구역 머리 폭 = 그 구역 시기 폭(시기 2개 × 160 + 간격 20 = 340)
	if !strings.Contains(svg, `width="340"`) {
		t.Errorf("구역 머리 폭이 시기 폭과 다름\n%s", svg)
	}
	svg, _ = drawn(t, "timeline\nSECTION 성장기\n2016 : 설립\nSection 도약기\n2026 : 매출")
	if !strings.Contains(svg, ">성장기<") || !strings.Contains(svg, ">도약기<") || strings.Count(svg, `class="tl-sec `) != 2 {
		t.Errorf("SECTION 대소문자가 구역으로 읽히지 않음\n%s", svg)
	}

	// 시기만 있는 줄 · 한 줄에 사건 여럿(콜론+공백 구분) · 다음 줄 이어 쓰기
	svg, _ = drawn(t, "timeline\n2016\n2018 : 투자 : 매각\n: 상장")
	if !strings.Contains(svg, ">2016<") || !strings.Contains(svg, ">투자<") || !strings.Contains(svg, ">매각<") || !strings.Contains(svg, ">상장<") {
		t.Errorf("시기·사건이 없음\n%s", svg)
	}
	if n := strings.Count(svg, `class="tl-evn `); n != 3 {
		t.Errorf("사건 상자 %d 개, 기대 3\n%s", n, svg)
	}

	// 이어 쓰기는 구역이 바뀌어도 마지막 시기에 붙는다(원본 addEvent — currentTaskId-1)
	svg, _ = drawn(t, "timeline\nsection A\n2016 : 설립\nsection B\n2018 : 투자\n: 확장")
	if !strings.Contains(svg, ">확장<") || strings.Count(svg, `class="tl-evn `) != 3 {
		t.Errorf("이어 쓰기가 마지막 시기에 붙지 않음\n%s", svg)
	}

	// 사건 안의 공백 없는 콜론·줄 끝 콜론은 사건 글자다(원본 jison “:”(?!\s))
	svg, _ = drawn(t, "timeline\n1월 : 09:30 회의")
	if !strings.Contains(svg, ">09:30 회의<") {
		t.Errorf("공백 없는 콜론이 사건에서 잘림\n%s", svg)
	}
	svg, _ = drawn(t, "timeline\n1월 : 회의 :")
	if !strings.Contains(svg, ">회어 :") && !strings.Contains(svg, ">회의 :<") {
		t.Errorf("줄 끝 콜론 처리가 다름\n%s", svg)
	}

	// <br> 줄바꿈(세 꼴) — 상자 안에서 줄이 나뉜다
	for _, br := range []string{"<br>", "<br/>", "<br />"} {
		svg, _ = drawn(t, "timeline\n1월 : 첫째 줄"+br+"둘째 줄")
		if !strings.Contains(svg, ">첫째 줄<") || !strings.Contains(svg, ">둘째 줄<") {
			t.Errorf("%s 줄바꿈이 안 풀림\n%s", br, svg)
		}
	}

	// Mermaid 엔티티
	svg, _ = drawn(t, "timeline\n1월 : 비율 #35;")
	if !strings.Contains(svg, ">비율 #<") {
		t.Errorf("엔티티가 안 풀림\n%s", svg)
	}

	// 접근성 — accTitle(aria-label)·accDescr(<title>). 키워드도 대소문자 무시(리뷰 [3]), 선언이 여럿이면 마지막이
	// 이긴다(콜론형·블록형 같게 — 리뷰 [4]), 블록형도 200자 상한을 지난다.
	svg, _ = drawn(t, "timeline\naccTitle: 연혁\naccDescr: 10년 연혁\n2026 : 사건")
	if !strings.Contains(svg, `aria-label="연혁"`) || !strings.Contains(svg, "<title>10년 연혁</title>") {
		t.Errorf("접근성 글이 없음\n%s", svg)
	}
	svg, _ = drawn(t, "timeline\nACCTITLE: 대문자\nACCDESCR: 대문자 설명\n2026 : 사건")
	if !strings.Contains(svg, `aria-label="대문자"`) || !strings.Contains(svg, "<title>대문자 설명</title>") {
		t.Errorf("접근성 키워드 대소문자가 안 읽힘\n%s", svg)
	}
	svg, _ = drawn(t, "timeline\naccDescr {\n두 줄\n설명\n}\n2026 : 사건")
	if !strings.Contains(svg, "<title>두 줄\n설명</title>") { // 블록 줄바꿈은 그대로(정책 B — 원본 setAccDescription)
		t.Errorf("accDescr 블록이 안 읽힘\n%s", svg)
	}
	svg, _ = drawn(t, "timeline\naccDescr { first }\naccDescr { second }\n2026 : 사건")
	if !strings.Contains(svg, "<title>second</title>") {
		t.Errorf("accDescr 블록 반복이 마지막 선언으로 덮지 않음\n%s", svg)
	}
	svg, _ = drawn(t, "timeline\naccDescr: first\naccDescr: second\n2026 : 사건")
	if !strings.Contains(svg, "<title>second</title>") {
		t.Errorf("accDescr 콜론형 반복이 마지막 선언으로 덮지 않음\n%s", svg)
	}
	long := strings.Repeat("가", MaxLabel)
	svg, _ = drawn(t, "timeline\naccDescr { "+long+" }\n2026 : 사건") // 200자는 그린다
	if !strings.Contains(svg, "<title>"+long+"</title>") {
		t.Errorf("accDescr 블록 200자가 안 읽힘\n%s", svg)
	}
	errAt(t, "timeline\naccDescr { "+long+"가 }\n2026 : 사건", 2, "accDescr 값이 200 자")   // 한 줄 블록은 그 줄
	errAt(t, "timeline\naccDescr {\n"+long+"가\n}\n2026 : 사건", 4, "accDescr 값이 200 자") // 여러 줄 블록은 닫는 줄

	// 주석 · 빈 줄
	drawn(t, "timeline\n%% 메모\n\n2026 : 사건")

	// 쓰지 않는 줄(경고 한 번) — %%{init}%%·frontmatter 설정
	svg, issues = drawn(t, "%%{init: {\"theme\":\"dark\"}}%%\ntimeline\n2026 : 사건")
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, "설정 지시") {
		t.Errorf("%%{init}%% 경고가 다름: %+v", issues)
	}
	if !strings.Contains(svg, ">2026<") {
		t.Errorf("%%{init}%% 가 있는데 그림이 없음\n%s", svg)
	}
	_, issues = drawn(t, "---\nconfig:\n  disableMulticolor: true\n---\ntimeline\n2026 : 사건")
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, "config") {
		t.Errorf("frontmatter 설정 경고가 다름: %+v", issues)
	}

	// 오류 — 줄 번호와 함께
	errAt(t, "timeline\n2026 :사건", 2, "콜론 뒤에 공백")
	errAt(t, "timeline\n2026:", 2, "사건이 비었음")
	errAt(t, "timeline\n2026 : ", 2, "사건이 비었음")
	errAt(t, "timeline\n2026 :  : 사건", 2, "사건이 비었음")
	errAt(t, "timeline\n: 사건", 2, "시기 없이 먼저")
	errAt(t, "timeline\nsection", 2, "section 이름이 없음")
	errAt(t, "timeline\nsection a : b\n2026 : 사건", 2, "콜론은 쓰지 못 함")
	errAt(t, "timeline\ntitle", 2, "제목이 없음")
	errAt(t, "timeline TD\n2026 : 사건", 1, "세로 방향")
	errAt(t, "timeline TB\n2026 : 사건", 1, "세로 방향")
	errAt(t, "timeline RL\n2026 : 사건", 1, "방향 낱말")
	errAt(t, "timeline LR TD\n2026 : 사건", 1, "낱말이 많음")
	errAt(t, "timeline\n# 메모\n2026 : 사건", 2, "시기의 # 부터는") // 원본 lexer 의 # 주석 — 이 판은 %% 만(리뷰 [2])
	errAt(t, "timeline\n2026 # 메모\n2027 : 사건", 2, "시기의 # 부터는")
	errAt(t, "timeline\n2026 # 메모 : 사건", 2, "시기의 # 부터는")
	// 사건 안의 # 은 원본이 허용하므로 글자·엔티티로 그린다(시기와 문맥을 나눈다 — 리뷰 [2])
	svg, _ = drawn(t, "timeline\n2026 : 메모 #1")
	if !strings.Contains(svg, ">메모 #1<") {
		t.Errorf("사건 안의 # 가 글자로 안 남음\n%s", svg)
	}
	// 시기의 엔티티(#…;)는 원본 전처리(encodeEntities)가 지켜 주므로 그림이 된다(리뷰 2회차 [1] 회귀)
	for _, label := range []string{"단계 #35;1", "A #amp; B", "시기#lt;script#gt;"} {
		svg, issues = drawn(t, "timeline\n"+label+" : 사건")
		if len(issues) != 0 {
			t.Errorf("시기 엔티티 %q: 경고 %+v", label, issues)
		}
	}
	svg, _ = drawn(t, "timeline\n단계 #35;1 : 사건")
	if !strings.Contains(svg, ">단계 #1<") { // #35; → # — 공통 엔티티 디코딩
		t.Errorf("시기 엔티티가 글자로 안 풀림\n%s", svg)
	}
	svg, _ = drawn(t, "timeline\n시기#lt;script#gt; : 사건")
	if !strings.Contains(svg, "&lt;script&gt;") || strings.Contains(svg, "<script") {
		t.Errorf("시기 엔티티의 이스케이프가 다름\n%s", svg)
	}
	// 엔티티가 문법 글자(:)로 풀려도 문법으로 다시 읽히지 않는다(#58; = :)
	svg, issues = drawn(t, "timeline\n2026 : 사건\n시기 #58; 뒤 : 사건")
	if len(issues) != 0 || !strings.Contains(svg, ">시기 : 뒤<") {
		t.Errorf("엔티티가 문법 글자로 풀려 잘못 읽힘: %+v\n%s", issues, svg)
	}
	// 경고 전문 — 주석 표기 %% 가 경고에 그대로 남는다(리뷰 2회차 [3])
	_, err := Render("timeline\n2026 # 메모 : 사건")
	de, ok := err.(*diagram.Error)
	if !ok {
		t.Fatalf("# 경고가 *Error 가 아님: %v", err)
	}
	if want := "2번째 줄: 시기의 # 부터는 Mermaid 가 주석으로 버리는 자리다 — 이 판은 그리지 않는다(주석은 %%, 사건 안의 # 은 글자)"; de.Error() != want {
		t.Errorf("# 경고 전문이 다름:\n got %q\nwant %q", de.Error(), want)
	}
	errAt(t, "timeline\n2026 :사건: 사건", 2, "콜론 뒤에 공백")
	// 다른 종류의 지시자(dateFormat·click …)는 이 판·원본 모두 시기 이름으로 읽는다(timeline 문법은 title·section 만 안다)
	svg, _ = drawn(t, "timeline\ndateFormat YYYY\n2026 : 사건")
	if !strings.Contains(svg, ">dateFormat YYYY<") {
		t.Errorf("콜론 없는 줄이 원본처럼 시기로 읽히지 않음\n%s", svg)
	}
	errAt(t, "timeline\naccTitle:\n2026 : 사건", 2, "글이 없음")
	errAt(t, "timeline\naccDescr {\n풀리지 않음\n2026 : 사건", 2, "닫히지 않음")
	errAt(t, "graph TD\nA --> B", 1, "timeline 이 아님")
	errAt(t, "timeline", 0, "그릴 시기가 없음")
	errAt(t, "timeline\nsection 빈 구역", 0, "그릴 시기가 없음") // 시기 없는 section 만 있는 그림
	// frontmatter 오류 — YAML 을 읽지 못하면 frontmatter 시작 줄을 가리킨다
	errAt(t, "---\ntitle: [가\n---\ntimeline\n2026 : 사건", 1, "YAML")

	// 시기 없는 구역은 경고하고 나머지를 그린다(gantt 의 빈 section 과 같다)
	svg, issues = drawn(t, "timeline\nsection 빈 구역\nsection A\n2026 : 사건")
	if len(issues) != 1 || !strings.Contains(issues[0].Msg, "빈 section") {
		t.Errorf("빈 section 경고가 다름: %+v", issues)
	}
	if !strings.Contains(svg, ">2026<") || strings.Contains(svg, ">빈 구역<") {
		t.Errorf("빈 section 을 그리거나 시기를 안 그림\n%s", svg)
	}

	// 색 순환 — 구역이 없으면 시기마다(tl-c1..8), 9번째 시기는 다시 1번
	svg, _ = drawn(t, "timeline\n1 : a\n2 : b\n3 : c\n4 : d\n5 : e\n6 : f\n7 : g\n8 : h\n9 : i")
	for i := 1; i <= 8; i++ {
		if !strings.Contains(svg, fmt.Sprintf("tl-per tl-c%d", i)) {
			t.Errorf("시기 색 순환 tl-c%d 없음\n%s", i, svg)
		}
	}
}

func TestLimits(t *testing.T) {
	// 시기 상한
	var b strings.Builder
	b.WriteString("timeline\n")
	for i := 0; i <= MaxPeriods; i++ { // 61 개
		fmt.Fprintf(&b, "%d : 사건\n", 2000+i)
	}
	errAt(t, b.String(), MaxPeriods+2, "시기가 60 개를 넘음")
	b.Reset()
	b.WriteString("timeline\n")
	for i := 0; i < MaxPeriods; i++ { // 60 개는 그린다
		fmt.Fprintf(&b, "%d : 사건\n", 2000+i)
	}
	drawn(t, b.String())

	// 사건 합계 상한 — 시기 15 × 사건 20 = 300 은 그리고, 하나 더 넘으면 원문
	b.Reset()
	b.WriteString("timeline\n")
	for i := 0; i < MaxEvents/MaxEventsPerPeriod; i++ {
		fmt.Fprintf(&b, "%d", 2000+i)
		for k := 0; k < MaxEventsPerPeriod; k++ {
			b.WriteString(" : 사건")
		}
		b.WriteString("\n")
	}
	drawn(t, b.String()) // 300 개는 그린다
	errAt(t, b.String()+"2026 : 사건\n", MaxEvents/MaxEventsPerPeriod+2, "사건 합계가 300")

	// 시기 하나의 사건 상한
	b.Reset()
	b.WriteString("timeline\n2026")
	for i := 0; i < MaxEventsPerPeriod; i++ {
		b.WriteString(" : 사건")
	}
	drawn(t, b.String()) // 20 개는 그린다
	errAt(t, b.String()+" : 하나 더", 2, "시기 하나의 사건이 20")

	// 구역 상한
	b.Reset()
	b.WriteString("timeline\n")
	for i := 0; i < MaxSections; i++ {
		fmt.Fprintf(&b, "section 구역%d\n%d : 사건\n", i, 2000+i)
	}
	drawn(t, strings.TrimSuffix(b.String(), "\n")) // 20 개는 그린다
	errAt(t, b.String()+"section 넘침\n2027 : 사건\n", MaxSections*2+2, "section 이 20 개")

	// 라벨 상한
	long := strings.Repeat("가", MaxLabel)
	drawn(t, "timeline\n"+long+" : 사건") // 200 자는 그린다
	errAt(t, "timeline\n"+long+"가 : 사건", 2, "라벨이 200 자")
	errAt(t, "timeline\n2026 : "+long+"가", 2, "라벨이 200 자")

	// 원문 상한(공통)
	errAt(t, "timeline\n2026 : "+strings.Repeat("사건 ", 5000), 0, "16KB")
}

// TestDeterminism 은 같은 입력을 두 번 그려 바이트가 같은지 본다(map 순회·환경 의존 없음).
func TestDeterminism(t *testing.T) {
	for i, src := range []string{
		"timeline\n2026 : 사건",
		"timeline\ntitle 제목\nsection A\n2016 : 설립 : 이전\n2018 : 투자\nsection B\n2020 : 매출",
		"timeline\n" + strings.Repeat("매우 긴 시기 이름입니다 ", 10) + " : " + strings.Repeat("아주 긴 사건 이름이라서 상자 안에서 줄이 바뀐다 ", 6),
		"timeline\n1<br>2 : a<br>b",
	} {
		a, err := Render(src)
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		b, _ := Render(src)
		if a.SVG != b.SVG {
			t.Errorf("%d: 두 번 렌더가 다름\n%s\n%s", i, a.SVG, b.SVG)
		}
	}
}

// ---- SVG 구조 도우미(gantt_test.go 와 같은 방식 — 패키지가 다르니 여기에 둔다)

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

// checkLayout 은 SVG 하나의 배치 성질을 본다 — 요소·글자가 viewBox 안, 시기 상자 한 줄(같은 y·높이·폭, 등거리), 사건 상자가
// 제 열 안에서 순서대로(인접 겹침 없음), 구역 머리가 그 구역 시기 폭과 같고 서로 겹치지 않음, 같은 종류 글자 겹침 없음,
// 점선·시간축 선이 모든 상자보다 먼저 그려지고(반투명 상자 뒤로 점선이 비치지 않게) 사건 상자마다 종이색 바탕 rect 가
// 같은 기하로 바로 앞에 있다(수정 1 — 점선이 사건 글자를 가로지르던 것을 구조로 막는다).
func checkLayout(t *testing.T, name, svg string) {
	t.Helper()
	m := regexp.MustCompile(`viewBox="0 0 ([\d.]+) ([\d.]+)"`).FindStringSubmatch(svg)
	if m == nil {
		t.Fatalf("%s: viewBox 없음", name)
	}
	vw, _ := strconv.ParseFloat(m[1], 64)
	vh, _ := strconv.ParseFloat(m[2], 64)
	f := func(e el, k string) float64 {
		v, err := strconv.ParseFloat(e.attrs[k], 64)
		if err != nil {
			t.Fatalf("%s: <%s> 의 %s=%q 이 숫자가 아님", name, e.tag, k, e.attrs[k])
		}
		return v
	}
	els := splitEls(svg)
	var per, evn, sec []el
	lastLine, firstRect := -1, len(els)
	for k, e := range els {
		switch e.tag {
		case "svg", "title", "text", "tspan":
		case "rect":
			if k < firstRect {
				firstRect = k
			}
			x, y := f(e, "x"), f(e, "y")
			w, h := f(e, "width"), f(e, "height")
			if x < -0.05 || y < -0.05 || w < 0 || h < 0 || x+w > vw+0.05 || y+h > vh+0.05 {
				t.Fatalf("%s: <rect class=%q> 가 viewBox 밖 x=%v y=%v w=%v h=%v\n%s", name, e.attrs["class"], x, y, w, h, svg)
			}
			switch {
			case strings.Contains(e.attrs["class"], "tl-per"):
				per = append(per, e)
			case strings.Contains(e.attrs["class"], "tl-evn-bg"):
				// 사건 상자의 바탕 — 아래(사건 상자 검사)에서 바로 뒤 요소와 기하를 견준다
			case strings.Contains(e.attrs["class"], "tl-evn"):
				evn = append(evn, e)
			case strings.Contains(e.attrs["class"], "tl-sec"):
				sec = append(sec, e)
			}
		case "path":
			nums := regexp.MustCompile(`-?[\d.]+`).FindAllString(strings.ReplaceAll(strings.ReplaceAll(e.attrs["d"], "M", " "), "L", " "), -1)
			for j, ns := range nums {
				v, _ := strconv.ParseFloat(ns, 64)
				lim := vw
				if j%2 == 1 {
					lim = vh
				}
				if v < -0.05 || v > lim+0.05 {
					t.Fatalf("%s: path 좌표 %v 밖\n%s", name, v, svg)
				}
			}
		case "line":
			lastLine = k
			for _, kk := range []string{"x1", "x2", "y1", "y2"} {
				v, lim := f(e, kk), vw
				if kk[0] == 'y' {
					lim = vh
				}
				if v < -0.05 || v > lim+0.05 {
					t.Fatalf("%s: line %s=%v 밖\n%s", name, kk, v, svg)
				}
			}
		default:
			t.Fatalf("%s: 모르는 요소 <%s>", name, e.tag)
		}
	}
	// 점선·시간축 선(tl-drop·tl-axis)은 모든 상자(rect)보다 문서 순서에서 앞이다
	if lastLine > firstRect {
		t.Fatalf("%s: 선(줄 %d)이 상자(줄 %d)보다 먼저 그려지지 않음\n%s", name, lastLine, firstRect, svg)
	}
	// 사건 상자마다 종이색 바탕 rect(tl-evn-bg)가 같은 기하(모서리 rx 포함)로 바로 앞에 있다
	for k, e := range els {
		if e.tag != "rect" || !strings.Contains(e.attrs["class"], "tl-evn ") {
			continue
		}
		if k == 0 {
			t.Fatalf("%s: 사건 상자가 첫 요소임\n%s", name, svg)
		}
		bg := els[k-1]
		if bg.tag != "rect" || bg.attrs["class"] != "tl-evn-bg" {
			t.Fatalf("%s: 사건 상자 앞에 바탕 rect 가 없음(%s)\n%s", name, bg.attrs["class"], svg)
		}
		for _, a := range []string{"x", "y", "width", "height", "rx"} {
			if bg.attrs[a] != e.attrs[a] {
				t.Fatalf("%s: 바탕 rect 의 %s(%s)가 사건 상자(%s)와 다름\n%s", name, a, bg.attrs[a], e.attrs[a], svg)
			}
		}
	}
	if len(per) == 0 {
		t.Fatalf("%s: 시기 상자가 없음", name)
	}
	// 시기 상자는 한 줄 — 같은 y·높이·폭, x 는 등거리
	y0, h0 := f(per[0], "y"), f(per[0], "height")
	for i, e := range per {
		if f(e, "y") != y0 || f(e, "height") != h0 {
			t.Fatalf("%s: 시기 상자 %d 의 y·높이가 다름\n%s", name, i, svg)
		}
		if f(e, "width") != f(per[0], "width") {
			t.Fatalf("%s: 시기 상자 %d 의 폭이 다름\n%s", name, i, svg)
		}
		if i > 0 && f(e, "x")-f(per[i-1], "x") != f(per[1], "x")-f(per[0], "x") {
			t.Fatalf("%s: 시기 상자 %d 의 간격이 다름\n%s", name, i, svg)
		}
	}
	// 사건 상자는 제 열(시기 상자와 같은 x) 안에서 순서대로, 다음 상자가 앞 상자의 아래(높이 포함)에 있다(리뷰 [5] —
	// 간격을 잘못 잡아 겹치는 변이도 잡는다)
	byX := map[float64][][2]float64{}
	for _, e := range evn {
		byX[f(e, "x")] = append(byX[f(e, "x")], [2]float64{f(e, "y"), f(e, "height")})
	}
	for x, ys := range byX {
		found := false
		for _, e := range per {
			if f(e, "x") == x {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: 사건 상자(x=%v)가 시기 열 밖\n%s", name, x, svg)
		}
		for i := 1; i < len(ys); i++ {
			if ys[i][0] < ys[i-1][0]+ys[i-1][1]-0.05 {
				t.Fatalf("%s: 사건 상자 겹침(x=%v, %d번째 y=%v < 앞 상자 바닥 %v)\n%s", name, x, i, ys[i][0], ys[i-1][0]+ys[i-1][1], svg)
			}
		}
	}
	// 구역 머리 상자는 그 구역 시기들의 폭만큼 걸치고 서로 겹치지 않음(모두 같은 y·높이)
	for i, e := range sec {
		if f(e, "y") != f(sec[0], "y") || f(e, "height") != f(sec[0], "height") {
			t.Fatalf("%s: 구역 머리 %d 의 y·높이가 다름\n%s", name, i, svg)
		}
		sx, sw := f(e, "x"), f(e, "width")
		for j, e2 := range sec {
			if j == i {
				continue
			}
			x2 := f(e2, "x")
			if sx < x2+f(e2, "width")-0.05 && x2 < sx+sw-0.05 {
				t.Fatalf("%s: 구역 머리 겹침\n%s", name, svg)
			}
		}
		// 폭 = 그 구역 시기 상자 범위(첫 시기 x … 마지막 시기 x+폭)
		first, last := -1.0, -1.0
		for _, p := range per {
			px := f(p, "x")
			if px >= sx-0.05 && px+f(p, "width") <= sx+sw+0.05 {
				if first < 0 {
					first = px
				}
				last = px + f(p, "width")
			}
		}
		if first < 0 || sw != last-first && (sw < last-first-0.05 || sw > last-first+0.05) {
			t.Fatalf("%s: 구역 머리 폭(%v)이 시기 폭(%v..%v)과 다름\n%s", name, sw, first, last, svg)
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
	// 같은 종류 글자끼리 겹침 없음(시기·사건·구역 머리 글자 — 제목은 한 줄뿐이라 제외)
	for i := range boxes {
		for j := i + 1; j < len(boxes); j++ {
			a, b := boxes[i], boxes[j]
			ka, kb := strings.Fields(a.cls)[0], strings.Fields(b.cls)[0]
			if ka != kb || ka == "tl-title" {
				continue
			}
			if a.overlaps(b) {
				t.Fatalf("%s: 글자 %q 와 %q(%s)가 겹침\n%s", name, a.s, b.s, ka, svg)
			}
		}
	}
}

// TestLayoutProperties 는 무작위·극단 그림 수십 판으로 배치 성질을 본다 — 긴 라벨(한글 200자·영문 낱말), 사건 0/20 개,
// 구역 0/20 개, <br> 연속, 시기 60 개. 시드를 고정해 무작위도 결정적으로 만든다.
func TestLayoutProperties(t *testing.T) {
	rng := rand.New(rand.NewSource(2027))
	longName := func(r rune) string { return strings.Repeat(string(r), 1+rng.Intn(MaxLabel)) }
	ran, longSeen := 0, 0
	for round := 0; round < 80; round++ {
		var b strings.Builder
		b.WriteString("timeline\n")
		switch rng.Intn(4) { // 제목 변형(리뷰 [1]) — 없음·짧음·<br>·긴 것(200자 까지: 폭이 제목까지 넓는다)
		case 0:
			b.WriteString("title 제목 " + strconv.Itoa(round) + "\n")
		case 1:
			b.WriteString("title 첫째 줄<br>둘째 줄 " + strconv.Itoa(round) + "\n")
		case 2:
			b.WriteString("title " + strings.Repeat("가", 1+rng.Intn(MaxLabel)) + "\n")
		}
		nPeriod := 1 + rng.Intn(8)
		nSec := 0
		for i := 0; i < nPeriod; i++ {
			if i > 0 && nSec < MaxSections-2 && rng.Intn(3) == 0 {
				name := "구역 " + strconv.Itoa(i)
				if rng.Intn(6) == 0 {
					name = longName('가')
				}
				b.WriteString("section " + name + "\n")
				nSec++
			}
			name := "시기" + strconv.Itoa(i)
			switch rng.Intn(10) {
			case 0:
				name = longName('나')
			case 1:
				name = "긴 이름 " + strings.Repeat("word ", rng.Intn(20)) + strconv.Itoa(i)
			case 2:
				name = "첫째<br>둘째" + strconv.Itoa(i)
			case 3:
				name = "가<br><br>나" + strconv.Itoa(i) // <br> 연속
			}
			nEv := rng.Intn(MaxEventsPerPeriod + 1)
			switch rng.Intn(12) {
			case 0:
				nEv = MaxEventsPerPeriod // 극단: 사건 20개
			case 1:
				nEv = 0 // 극단: 사건 없음
			}
			b.WriteString(name)
			for k := 0; k < nEv; k++ {
				ev := "사건" + strconv.Itoa(k)
				switch rng.Intn(12) {
				case 0:
					ev = longName('다')
				case 1:
					ev = "10:" + strconv.Itoa(k%6) + "0 회의" // 공백 없는 콜론
				case 2:
					ev = "첫째 줄<br>둘째 줄 " + strconv.Itoa(k)
				}
				b.WriteString(" : " + ev)
			}
			b.WriteString("\n")
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
		if strings.Count(r.SVG, "<tspan") > nPeriod {
			longSeen++
		}
	}
	if ran < 60 || longSeen < 20 {
		t.Errorf("실행 %d 회·여러 줄 라벨 섞인 회차 %d — 표본이 모자람", ran, longSeen)
	}

	// 극단 — 상한 경계: 시기 60 · 구역 20(시기 3개씩) · 사건 20개 · 사건 0개 · 라벨 200자
	var b strings.Builder
	b.WriteString("timeline\n")
	for i := 0; i < MaxPeriods; i++ {
		fmt.Fprintf(&b, "%d : 사건\n", 2000+i)
	}
	svg, _ := drawn(t, strings.TrimSuffix(b.String(), "\n"))
	checkLayout(t, "시기 60", svg)

	b.Reset()
	b.WriteString("timeline\n")
	for i := 0; i < MaxSections; i++ {
		fmt.Fprintf(&b, "section 구역%d\n%d : 사건\n", i, 2000+i)
		fmt.Fprintf(&b, "%d : 사건\n", 2100+i)
	}
	svg, _ = drawn(t, strings.TrimSuffix(b.String(), "\n"))
	checkLayout(t, "구역 20", svg)

	b.Reset()
	b.WriteString("timeline\n2026")
	for i := 0; i < MaxEventsPerPeriod; i++ {
		b.WriteString(" : 사건")
	}
	svg, _ = drawn(t, b.String())
	checkLayout(t, "사건 20", svg)

	checkLayout(t, "사건 0", mustSVG(t, "timeline\n2026\n2027\n2028"))
	long := strings.Repeat("가", MaxLabel)
	checkLayout(t, "라벨 200자", mustSVG(t, "timeline\nsection "+long+"\n"+long+" : "+long+"\n2026 : 사건"))
}

func mustSVG(t *testing.T, src string) string {
	t.Helper()
	svg, _ := drawn(t, src)
	return svg
}

// 클래스 충돌(간트 수정 1회차와 같은 검사를 timeline 에) — 타임라인 SVG 의 클래스는 모두 tl- 접두사를 단다. app.css 의
// 일반 이름 규칙이 SVG 기하 속성에 걸리는 충돌을 막는다. .tl- 선택자는 base.css 의 .diagram-svg 아래에만 있어야 한다.
func TestClassCollisions(t *testing.T) {
	svg, _ := drawn(t, "timeline\ntitle 제목\nsection 구역\n2016 : 설립 : 이전\n2018 : 투자\n2020 : 매출")
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
		if w != "diagram-svg" && !strings.HasPrefix(w, "tl-") {
			t.Errorf("타임라인 클래스 %q 에 tl- 접두사가 없음", w)
		}
	}

	files, err := filepath.Glob("../../web/*.css")
	if err != nil || len(files) == 0 {
		t.Fatalf("dist css 를 못 찾음: %v", err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s 읽기 실패: %v", f, err)
		}
		for i, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, ".tl-") && !strings.Contains(line, ".diagram-svg") {
				t.Errorf("%s:%d — .tl- 선택자가 .diagram-svg 문맥 밖", f, i+1)
			}
		}
	}
}

// TestReviewR1 은 리뷰 1회차 발견의 재현 시험이다(원문 scratchpad result-timeline-r1.md — 리뷰어의 overlay 시험
// TestReviewLongTitle·AccLimit·RepeatedAcc·KeywordCase·HashPeriod 를 저장소 시험로 옮겼다).
func TestReviewR1(t *testing.T) {
	t.Run("긴 제목", func(t *testing.T) { // [1] 제목 폭이 viewBox 에 반영되지 않던 것 — 이제 폭을 제목까지 넓힌다(gantt 과 같게)
		checkLayout(t, "제목 30자", mustSVG(t, "timeline\ntitle "+strings.Repeat("가", 30)+"\n2026 : 사건"))
		checkLayout(t, "제목 200자", mustSVG(t, "timeline\ntitle "+strings.Repeat("가", MaxLabel)+"\n2026 : 사건"))
		checkLayout(t, "제목 <br>", mustSVG(t, "timeline\ntitle 첫째<br>둘째\n2026 : 사건"))
	})
	t.Run("accDescr 상한", func(t *testing.T) { // [4] 블록형이 200자 상한을 지나지 않던 것
		if _, err := Render("timeline\naccDescr { " + strings.Repeat("가", MaxLabel+1) + " }\n2026 : 사건"); err == nil {
			t.Fatal("201자 accDescr 블록이 통과함")
		}
	})
	t.Run("accDescr 반복", func(t *testing.T) { // [4] 이전 선언이 남던 것 — 마지막 선언이 이긴다(콜론형과 같게)
		c, _, err := Parse("timeline\naccDescr { first }\naccDescr { second }\n2026 : 사건")
		if err != nil {
			t.Fatal(err)
		}
		if c.AccDescr != "second" {
			t.Fatalf("AccDescr=%q, 기대 second", c.AccDescr)
		}
	})
	t.Run("키워드 대소문자", func(t *testing.T) { // [3] TITLE·SECTION 이 시기로 더해지던 것
		c, _, err := Parse("timeline\nTITLE 연혁\nSECTION 성장기\n2026 : 사건")
		if err != nil {
			t.Fatal(err)
		}
		if c.Title != "연혁" || len(c.Sections) != 1 || len(c.Periods) != 1 {
			t.Fatalf("title=%q sections=%d periods=%d", c.Title, len(c.Sections), len(c.Periods))
		}
	})
	t.Run("시기의 #", func(t *testing.T) { // [2] 원본이 주석으로 버리는 자리를 데이터로 그리던 것
		if _, _, err := Parse("timeline\n2026 # 메모 : 사건"); err == nil {
			t.Fatal("시기의 # 주석이 데이터로 그려짐")
		}
	})
}

// TestReviewR2 는 리뷰 2회차 발견의 재현 시험이다(원문 scratchpad result-timeline-r2.md — 리뷰어의 overlay 시험
// TestReviewR2Entities·UnsectionedPalette·EventContexts 를 저장소 시험로 옮겼다).
func TestReviewR2(t *testing.T) {
	t.Run("시기 엔티티", func(t *testing.T) { // [1] 정상 엔티티까지 주석으로 거부하던 것
		for _, label := range []string{"단계 #35;1", "A #amp; B", "시기#lt;script#gt;"} {
			if _, err := Render("timeline\n" + label + " : 사건"); err != nil {
				t.Errorf("유효한 Mermaid 엔티티 시기 %q 를 주석으로 거부: %v", label, err)
			}
		}
	})
	t.Run("구역 없는 앞 시기 색", func(t *testing.T) { // [2] 앞 시기가 시기 인덱스로 색을 미리 쓰던 것
		svg, _ := drawn(t, "timeline\n2010 : a\n2011 : b\nsection 성장기\n2012 : c")
		var got []string
		for _, e := range splitEls(svg) {
			if e.tag == "rect" && strings.HasPrefix(e.attrs["class"], "tl-per ") {
				got = append(got, e.attrs["class"])
			}
		}
		want := "tl-per tl-c1,tl-per tl-c1,tl-per tl-c2"
		if strings.Join(got, ",") != want {
			t.Fatalf("구역 없는 앞 시기 색: got %v, want %v", got, want)
		}
	})
	t.Run("사건 문맥", func(t *testing.T) { // 사건의 # ·엔티티·URL 조각과 구역을 넘는 이어 쓰기(1회차 확인 보강)
		for _, ev := range []string{"메모 #1", "비율 #35;", "https://example.com/a#part"} {
			if _, err := Render("timeline\n2026 : " + ev); err != nil {
				t.Errorf("사건 %q 를 거부: %v", ev, err)
			}
		}
		c, _, err := Parse("timeline\n2026 : 첫째\nsection B\n: 둘째")
		if err != nil || len(c.Periods) != 1 || len(c.Periods[0].Events) != 2 {
			t.Fatalf("구역을 넘은 이어 쓰기: %+v %v", c, err)
		}
	})
}

// TestHashBoundaryProperties 는 시기 `#` 경계를 구조로 닫는 속성 시험이다(리뷰 2회차 — 1·2회차 같은 경계 결함을 잡는다).
// 보장은 **일반 엔티티 보호와 남은 시기 `#` 판별**이다(리뷰 3회차 [3]으로 좁혀 적는다) — style/classDef `;` 자르기·
// 원문 표식 문자·CR/U+2028/U+2029 같은 특수 입력은 고정 사례(TestReviewR3)가 보고 이 시험은 다루지 않는다.
// 무작위 시기·사건 문자열(#·;·ASCII 낱말 글자·한글·공백·&·<·>)마다, 시험 안에 독립으로 쓴 판정기(원본 규칙:
// style `;` 자르기 → classDef `;` 자르기(순차 두 치환, 원본과 같은 순서) → 엔티티 #\w+; 제거 → # 가 남으면 거부)와
// Render 의 성패가 같은지 본다. 그려지는 판은 날 <·& 가 SVG 에 없는지(이스케이프 경계)도 본다.
func TestHashBoundaryProperties(t *testing.T) {
	entTok := regexp.MustCompile(`#[A-Za-z0-9_]+;`)
	reject := func(line string) bool { // 원본 전처리+lexer 규칙의 독립 구현(encodeEntities 와 같은 패키지 변수를 쓰지 않는다)
		s := regexp.MustCompile(`style.*:\S*#.*;`).ReplaceAllStringFunc(line, func(s string) string { return s[:len(s)-1] })
		s = regexp.MustCompile(`classDef.*:\S*#.*;`).ReplaceAllStringFunc(s, func(s string) string { return s[:len(s)-1] })
		return strings.Contains(entTok.ReplaceAllString(s, ""), "#")
	}
	// 정책 B 의 손 판정 — 앞뒤 빈칸을 지운 글 안에 연속 빈칸이 있거나 받지 않는 엔티티(#copy;·#160;·#32;)가 있으면 원문이다.
	// 이 시험이 쓰는 엔티티 가운데 받는 것은 #35;·#amp;·#lt;·#58; 뿐이다(구현의 표를 부르지 않는다).
	policyBad := func(s string) bool {
		s = strings.Trim(s, " ")
		for _, e := range entTok.FindAllString(s, -1) {
			switch e {
			case "#35;", "#amp;", "#lt;", "#58;":
			default:
				return true
			}
		}
		return strings.Contains(s, "  ")
	}
	knownTag := map[string]bool{"svg": true, "rect": true, "line": true, "path": true, "text": true, "tspan": true, "title": true}
	rng := rand.New(rand.NewSource(2028))
	letters := []rune("#;abzAZ09_가다 사건&<>") // 시기는 구조상 콜론을 못 가진다(시기 : 사건 구분) — 콜론은 넣지 않는다
	ran, rejected, drawn := 0, 0, 0
	for round := 0; round < 400; round++ {
		mk := func(n int) string {
			var b strings.Builder
			for i := 0; i < n; i++ {
				if rng.Intn(6) == 0 { // 엔티티 꼴을 확실히 섞는다(뮤턴트가 이 경계를 못 지나는지 보게) — 뒤 셋은 정책 B 가 받지 않는다
					b.WriteString([]string{"#35;", "#amp;", "#lt;", "#58;", "#copy;", "#160;", "#32;"}[rng.Intn(7)])
					continue
				}
				b.WriteRune(letters[rng.Intn(len(letters))])
			}
			return b.String()
		}
		period := mk(1 + rng.Intn(12))
		src := "timeline\n" + period
		bad := policyBad(period)
		if rng.Intn(2) == 0 {
			ev := mk(1 + rng.Intn(12))
			src += " : " + ev
			bad = bad || policyBad(ev)
		}
		want := reject(period)
		r, err := Render(src)
		ran++
		// 이 시험의 대상은 # 경계다 — # 아닌 문법 오류(콜론 꼴 등)는 다른 시험들이 본다
		got := err != nil && strings.Contains(err.Error(), "시기의 #")
		if got != want {
			t.Fatalf("round %d: 시기 %q 의 # 판정이 판정기와 다름 — Render 오류 %v, 판정기 거부 %v\n%s", round, period, err, want, src)
		}
		if !want && bad && err == nil {
			t.Fatalf("round %d: 정책 B 가 받지 않는 글(연속 빈칸·엔티티)을 그림\n%s", round, src)
		}
		if !want && !bad && err != nil && (strings.Contains(err.Error(), "연속 빈칸") || strings.Contains(err.Error(), "엔티티")) {
			t.Fatalf("round %d: 정책 B 가 받는 글을 원문으로 돌림 — %v\n%s", round, err, src)
		}
		if err != nil {
			rejected++
			continue
		}
		drawn++
		for _, m := range regexp.MustCompile(`<([a-zA-Z/]+)`).FindAllStringSubmatch(r.SVG, -1) {
			tag := strings.TrimPrefix(m[1], "/") // 닫는 태그의 / 는 뗀다
			if !knownTag[tag] {
				t.Fatalf("round %d: 날 < 가 SVG 에 있음(%q)\n%s", round, m[1], r.SVG)
			}
		}
		for i := 0; i < len(r.SVG); i++ { // & 는 이스케이프 꼴만
			if r.SVG[i] == '&' && !strings.HasPrefix(r.SVG[i:], "&amp;") && !strings.HasPrefix(r.SVG[i:], "&lt;") &&
				!strings.HasPrefix(r.SVG[i:], "&gt;") && !strings.HasPrefix(r.SVG[i:], "&quot;") && !strings.HasPrefix(r.SVG[i:], "&#39;") {
				t.Fatalf("round %d: 날 & 가 SVG 에 있음(%.20s)\n%s", round, r.SVG[i:], r.SVG)
			}
		}
	}
	if ran < 400 || rejected < 50 || drawn < 100 {
		t.Errorf("표본 부족 — 실행 %d·거부 %d·그림 %d", ran, rejected, drawn)
	}
}

// TestReviewR3 는 리뷰 3회차 발견의 재현 시험이다(원문 scratchpad result-timeline-r3.md — 리뷰어의 overlay 시험
// TestReviewR3Markers·TestReviewR3AccPreprocess 를 저장소 시험로 옮겼다).
func TestReviewR3(t *testing.T) {
	t.Run("원문의 내부 표식", func(t *testing.T) { // [2] — 2026-10-30 결정: 원문 표식 문자(ﬂ°·¶ß)가 있으면 원문 + 경고(원본은 그린다)
		for _, in := range []string{"A ¶ß B", "A ﬂ°zzz B", "A ﬂ°35¶ß B"} {
			errAt(t, "timeline\n"+in+" : 사건", 2, "내부 표식 문자")
			errAt(t, "timeline\n"+in, 2, "내부 표식 문자") // 시기만 있는 줄도 같다
		}
	})
	t.Run("accDescr 전처리", func(t *testing.T) { // [1] 여는 줄·블록의 뒤따르는 줄에 같은 내용이면 같은 설명이 나온다
		a, _, e := Parse("timeline\naccDescr { style:x#35; }\n2026 : 사건")
		if e != nil {
			t.Fatal(e)
		}
		b, _, e := Parse("timeline\naccDescr {\nstyle:x#35;\n}\n2026 : 사건")
		if e != nil {
			t.Fatal(e)
		}
		if a.AccDescr != b.AccDescr {
			t.Errorf("같은 내용이 다른 설명: 인라인=%q 블록=%q", a.AccDescr, b.AccDescr)
		}
		if a.AccDescr != "style:x#35" { // 원본 전처리가 style …#…; 의 끝 ; 를 자른다
			t.Errorf("accDescr 의 style 세미콜론 자르기: %q", a.AccDescr)
		}
	})
	t.Run("style·classDef 세미콜론", func(t *testing.T) { // [3] ; 자르기 치환의 고정 사례 — 사건·제목·접근성 설명 안에서
		c, _, e := Parse("timeline\ntitle 제목 style:x#35;\n2026 : style:x#35;\n: classDef:y#ff0000;")
		if e != nil {
			t.Fatal(e)
		}
		if c.Title != "제목 style:x#35" {
			t.Errorf("title 의 style 세미콜론 자르기: %q", c.Title)
		}
		if len(c.Periods) != 1 || len(c.Periods[0].Events) != 2 ||
			c.Periods[0].Events[0] != "style:x#35" || c.Periods[0].Events[1] != "classDef:y#ff0000" {
			t.Errorf("사건의 세미콜론 자르기: %+v", c.Periods)
		}
		d, _, e := Parse("timeline\naccDescr: classDef:z#00ff00;\n2026 : 사건")
		if e != nil || d.AccDescr != "classDef:z#00ff00" {
			t.Errorf("accDescr(콜론형)의 classDef 세미콜론 자르기: %q %v", d.AccDescr, e)
		}
	})
}

// 설정 지시 — 줄 전체가 지시 하나인 줄만 무시하고 그린다. 그 밖에 %%{ 가 든 줄(지시 앞뒤·사이의 글, acc 값·시기·사건 글·
// 제목·구역 이름·주석 안, accDescr 블록 안)은 원문 + 그 줄 번호(er·sequence 와 같은 공통 규칙 — #409 ④-1 후속).
func TestDirectiveLines(t *testing.T) {
	const d = "%%{init: {}}%%"
	const one = "한 줄에 하나만"
	for _, c := range []struct {
		src  string
		line int
		msg  string
	}{
		{"timeline\n" + d + " 2020 : 사건 " + d + "\n2021 : y", 2, one},
		{"timeline\n" + d + " ??? " + d + "\n2021 : y", 2, one},
		{"timeline\naccTitle: 앞 " + d + " 뒤\n2021 : y", 2, one},
		{"timeline\naccDescr: 앞 " + d + " 뒤\n2021 : y", 2, one},
		{"timeline\n2021 : 글 " + d, 2, one},
		{"timeline\n2021 " + d + " : 글", 2, one},
		{"timeline\n2021 : y\n : 이어 " + d, 3, one},
		{"timeline\ntitle 제목 " + d + "\n2021 : y", 2, one},
		{"timeline\nsection 구역 " + d + "\n2021 : y", 2, one},
		{"timeline\n%% 메모 " + d + "\n2021 : y", 2, one},
		{"timeline\naccDescr {\n" + d + "\n}\n2021 : y", 3, "블록 안"},
		{"timeline\naccDescr {\n글 " + d + "\n}\n2021 : y", 3, one},
		{"timeline\n%%{init: {\n2021 : y", 2, "한 줄로 닫아야"},
	} {
		errAt(t, c.src, c.line, c.msg)
	}
	for _, src := range []string{d + "\ntimeline\n2021 : y", "timeline\n  " + d + "  \n2021 : y", "timeline\nsection s\n" + d + "\n2021 : y"} {
		svg, iss := drawn(t, src)
		if len(iss) != 1 || !strings.Contains(iss[0].Msg, "설정 지시") || strings.Contains(svg, "%%{") {
			t.Errorf("%q: 경고 %+v", src, iss)
		}
	}
}

// 줄 머리 %% 주석 — 원본 cleanupComments 처럼 accDescr { … } 블록 안에서도 지운다(설명에 넣지 않는다). 블록 밖 주석은 그대로
// 건너뛰고, 뒤에 글이 없는 맨 %% 줄은 원문 + 그 줄 번호(er 와 같은 차례·규칙 — #409 ④-1 후속 리뷰 1회차 [2]).
func TestCommentLines(t *testing.T) {
	const head, tail = "timeline\n", "\n2021 : y"
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
	// 블록 밖 맨 %% 는 원본 lexer 의 주석 규칙(jison:14)이 건너뛴다 — 경고 없이 그린다(#409 ④-1 후속 수정 2).
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
