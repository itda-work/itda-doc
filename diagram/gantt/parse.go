package gantt

import (
	"fmt"
	"strings"
	"time"

	"github.com/itda-work/itda-doc/diagram"
)

// 원문을 두 단계로 읽는다 — 모르는 문장·값은 그 자리에서 *Error 로 멈춘다(원문 길이에 선형).
// 1단계에서 줄을 차례로 훑어 설정 줄을 확정하고 작업 줄은 구조(이름·태그·id·구역)만 읽는다. 2단계에서 최종 설정으로
// 제외 낱말을 판정하고(resolveExcludes) 작업의 날짜를 차례로 계산한다(computeTasks) — 설정 줄이 작업 뒤에 와도
// 그림 전체에 적용된다(Mermaid ganttDb 도 addTask 는 원문으로 두고 getTasks() 의 compileTasks 로 계산한다).
// 오류의 줄 번호는 그 줄(작업 줄·설정 줄) 그대로다.
//
//	머리 줄      gantt
//	설정 줄      dateFormat … · axisFormat … · tickInterval … · excludes … · inclusiveEndDates
//	제목         title 글(그림 안 제목) — frontmatter title 은 캡션(공통 규칙)
//	구역         section 이름
//	작업         이름 : [태그…,] [id,] 시작, 끝 — 태그(active·done·crit·milestone)를 뺀 나머지 개수로 읽는다(Mermaid 와 같게):
//	             1개 = 끝(시작은 바로 앞 작업의 끝), 2개 = 시작·끝, 3개 = id·시작·끝
//	시작         날짜(dateFormat) · after id [id…](가장 늦은 끝)
//	끝           날짜 · 기간(Ny·NM·Nw·Nd·Nh·Nm·Ns·Nms, 정수) · until id [id…](가장 이른 시작)
//	접근성       accTitle: 글 · accDescr: 글 · accDescr { … }
//	쓰지 않음    todayMarker off(경고 없이)·todayMarker … · topAxis · weekday · click · %%{init: …}%%(줄 전체가 지시 하나일 때만) · frontmatter 의 title 밖 키
//	미지원       %%{ 가 든 그 밖의 줄 · vert 태그 · includes · weekend friday|saturday · dateFormat 과 맞지 않는 날짜 · 모르는 id(after·until) ·
//	             같은 id 두 번 · 끝이 시작보다 앞 · 날짜 범위 100년 초과 · 상한 초과 · 알 수 없는 문장 ·
//	             excludes 의 모르는 낱말(weekends·요일 이름·dateFormat 꼴이 아님)·쉼표만 있는 값 ·
//	             같은 설정 줄이 둘 이상 · 축 라벨이 사다리 마지막 칸까지 겹침(그릴 때 — svg.go chooseTicks)

// Parse 는 mermaid gantt 원문을 읽는다. 그릴 수 없는 문법이면 *Error 다. issues 는 쓰지 않은 줄이다.
func Parse(src string) (*chart, []Issue, error) {
	p := &parser{c: &chart{}, byID: map[string]int{}, said: map[string]bool{},
		df: mustDateFormat(), af: defaultAxisFmt(), ex: &excludes{dates: map[string]bool{}, none: true}}
	// 잘못된 UTF-8 은 읽기 전에 U+FFFD 로(파서가 룬 단위로 읽는다). XML 이 허용하지 않는 글자는 SVG 를 쓰는
	// 경계(diagram.Esc)에서 거른다 — flowchart 와 같다.
	if err := p.parse(strings.ToValidUTF8(src, "�")); err != nil {
		return nil, nil, err
	}
	// 출력 관문 — 본문을 다 읽은 뒤, 내보낼 사용자 글자를 모두 한 번에 본다(본문 오류가 먼저다 — state 와 같은 꼴).
	if err := diagram.Gate(p.c.eachText); err != nil {
		return nil, nil, err
	}
	return p.c, p.issues, nil
}

// mustDateFormat·defaultAxisFmt 은 기본 꼴(YYYY-MM-DD·%Y-%m-%d)이다 — 상수처럼 쓴다.
func mustDateFormat() *dateFmt {
	d, err := parseDateFormat("YYYY-MM-DD")
	if err != nil {
		panic(err)
	}
	return d
}

func defaultAxisFmt() *axisFmt {
	a, err := parseAxisFormat("%Y-%m-%d")
	if err != nil {
		panic(err)
	}
	return a
}

type parser struct {
	c          *chart
	issues     []Issue
	line       int // 지금 읽는 줄(1부터)
	header     bool
	df         *dateFmt
	af         *axisFmt
	sawAnyDate bool
	sawAnyAxis bool
	headLine   int // gantt 머리 줄
	axisLine   int // axisFormat 줄(없으면 0) — 축 라벨이 겹쳐 그리지 못할 때 알리는 줄
	tick       rung
	hasTick    bool
	tickLine   int
	exDateLine int // 첫 날짜 제외 값의 줄(시각 꼴 검사용)
	ex         *excludes
	exLines    []exLine // excludes 줄의 낱말들 — 2단계에서 최종 dateFormat 으로 판정한다
	inclEnd    bool
	byID       map[string]int
	pending    []pendTask // 작업 줄의 구조만 읽은 것 — 날짜는 2단계(computeTasks)에서 계산한다
	lastEnd    time.Time  // 값 하나(끝) 작업의 시작 = 바로 앞 작업의 끝(2단계에서 갱신)
	hasLast    bool
	said       map[string]bool // 한 번만 알릴 Issue
	inAcc      bool            // accDescr { … } 안
	accLine    int
	acc        diagram.AccBlock
	raw        string // 지금 줄의 원문(줄 끝 CR 만 지운 것) — accDescr { 여는 줄의 값
}

// exLine 은 excludes 줄 하나의 낱말들이다(1단계에서 모은다).
type exLine struct {
	toks []string
	line int
}

// pendTask 은 작업 줄 하나다 — 구조(이름·태그·id·구역)는 1단계에서 읽고 날짜는 2단계에서 계산한다.
type pendTask struct {
	t    *task
	meta []string // 태그·id 를 뺀 시작·끝 토큰(1~2개)
	idx  int      // chart.Tasks 안의 차례 — after·until 은 자기보다 앞 작업의 id 만 참조한다
}

func (p *parser) fail(format string, a ...any) *Error {
	return &Error{Line: p.line, Msg: fmt.Sprintf(format, a...)}
}

func (p *parser) note(key, msg string) {
	if p.said[key] {
		return
	}
	p.said[key] = true
	p.issues = append(p.issues, Issue{Line: p.line, Msg: msg})
}

func (p *parser) parse(src string) error {
	lines := strings.Split(src, "\n")
	fm, start, hasFM, closed := diagram.SplitFrontmatter(lines)
	if hasFM {
		p.line = 1
		if !closed {
			return p.fail("frontmatter(---)가 닫히지 않음")
		}
		f, err := diagram.ParseFrontmatter(fm)
		if err != nil {
			de := err.(*diagram.Error)
			return &Error{Line: de.Line + 1, Msg: de.Msg}
		}
		for _, k := range f.Others {
			p.line = k.Line + 1
			p.note("fm:"+k.Key, fmt.Sprintf("frontmatter 의 설정(%s)은 쓰지 않음 — 색·글꼴은 테마가 정한다", diagram.Clip(k.Key)))
		}
		if f.HasTitle {
			p.line = f.TitleLine + 1
			v, msg := diagram.Caption(f.Raw) // 정책 B — 원본은 frontmatter 를 엔티티 전처리 전에 떼어 낸다(글자 그대로)
			if msg != "" {
				return p.fail("%s", msg)
			}
			p.c.FMCaption, p.c.FMLine = v, p.line
		}
	}
	cur := -1
	for i := start; i < len(lines); i++ {
		p.line = i + 1
		t := strings.TrimSpace(lines[i])
		p.raw = strings.TrimSuffix(lines[i], "\r")
		// 줄 중간의 CR(원본 전처리가 줄바꿈으로 바꾼다)·FF·VT(원본 lexer 가 건너뛰지 않는다)는 받지 않는다(er·state 와 같다).
		if strings.ContainsAny(strings.TrimSuffix(lines[i], "\r"), "\r\f\v") {
			return p.fail("줄 안에 CR·FF·VT 글자가 있음 — 이 판은 그 줄을 그리지 않는다")
		}
		// ASCII 밖 공백·서식 문자는 줄째 그리지 않는다(원본 JS \s 와 Go 의 공백 판정이 갈리는 갈래 — er·sequence·state 와 같다,
		// #409 ④-2 후속 리뷰 1회차 [2]·[3]: 받으면 긴 라벨 줄바꿈(strings.Fields)이 NBSP 를 빈칸으로 바꾸고 accDescr 블록의 줄 머리
		// 정리가 원본 \n\s+ 와 갈린다).
		if r, ok := diagram.OddSpace(lines[i]); ok {
			return p.fail("ASCII 밖 공백·서식 문자(U+%04X)가 있음 — 이 판은 그 줄을 그리지 않는다(일반 빈칸으로 바꿔 쓴다)", r)
		}
		// 원문에 이미 들어 있는 Mermaid 내부 표식 문자(ﬂ°·¶ß)는 그리지 않는다(2026-10-30 사용자 결정 — 다른 종류와 같다). 간트는
		// 줄 단위 엔티티 전처리를 하지 않고 라벨 글에서만 표식을 만든다(text·accText) — 라벨의 표식이 모두 이 판이 만든 것이게 한다.
		if diagram.HasMarker(t) {
			return p.fail("본문 줄에 Mermaid 내부 표식 문자(ﬂ°·¶ß)가 있음 — 이 판은 그 줄을 그리지 않는다(엔티티 표기 #…; 는 그대로 쓸 수 있다)")
		}
		// 설정 지시 — 줄 전체가 지시 하나인 줄(accDescr { … } 밖)만 무시하는 지시다. 그 밖에 %%{ 가 든 줄(지시 앞뒤·사이의
		// 글, acc 값·작업 이름·주석 안, accDescr 블록 안)은 원본 removeDirectives 가 지시만 지우고 남은 글을 읽는 갈래라
		// 그리지 않는다(er·sequence 와 같은 공통 규칙 — #409 ④-1 후속).
		if msg := diagram.DirectiveProblem(t, p.inAcc); msg != "" {
			return p.fail("%s", msg)
		}
		// 줄 머리 %% 주석 — 원본 전처리 cleanupComments(/^\s*%%(?!{)[^\n]+\n?/gm)가 lexer 상태와 관계없이(accDescr { … } 블록 안도)
		// 먼저 지운다. 그래서 지시 검사 뒤, 엔티티 전처리·상태 분기 앞에 둔다(er 와 같은 차례 — #409 ④-1 후속 리뷰 1회차 [2]).
		// 뒤에 글이 없는 맨 %% 는 원본 전처리가 지우지 않는다. 블록 밖에서는 원본 lexer 의 주석 규칙(\%%[^\n]* — jison:34)이
		// 건너뛰므로 이 판도 경고 없이 건너뛴다. accDescr { … } 블록 안에서는 원본이 설명 글로 넣는다 — 이 판은 그 줄을
		// 그리지 않는다(보수적, #409 ④-1 후속 수정 2).
		// 여기 오는 %%{ 줄은 줄 전체가 지시 하나뿐이다(DirectiveProblem).
		if strings.HasPrefix(t, "%%") {
			if strings.HasPrefix(t, "%%{") {
				p.note("init", "설정 지시(%%{…}%%)는 쓰지 않음 — 색·글꼴은 테마가 정한다")
			} else if p.inAcc && strings.TrimLeft(strings.TrimSuffix(lines[i], "\r"), " \t") == "%%" {
				return p.fail("accDescr 블록 안의 글 없는 %s 줄 — 원본은 설명 글로 넣는 줄이라 이 판은 그리지 않는다(%s 뒤에 글을 둔다)", "%%", "%%")
			}
			continue
		}
		if p.inAcc { // 블록 글은 줄 끝 빈칸까지 값이다(diagram.AccBlock)
			if err := p.accBody(strings.TrimLeft(p.raw, " \t")); err != nil {
				return err
			}
			continue
		}
		if t == "" {
			continue
		}
		if err := p.statement(t, &cur); err != nil {
			return err
		}
	}
	if p.inAcc {
		return &Error{Line: p.accLine, Msg: "accDescr { 가 닫히지 않음"}
	}
	if !p.header {
		p.line = 0
		return p.fail("mermaid 그림이 비어 있음")
	}
	p.c.DF, p.c.AF = p.df, p.af
	p.c.Tick, p.c.HasTick, p.c.TickLine = p.tick, p.hasTick, p.tickLine
	p.c.AxisLine = p.axisLine
	if p.c.AxisLine == 0 {
		p.c.AxisLine = p.headLine
	}
	// 2단계 — 최종 설정으로 제외 낱말을 판정하고 작업 날짜를 계산한다(머리 주석).
	if err := p.resolveExcludes(); err != nil {
		return err
	}
	p.c.Excl = p.ex
	// 시각이 있는 꼴(HH·mm)의 날짜 제외는 그리지 않는다 — Mermaid 는 그 꼴로 포맷한 문자열이 같은 「시각」만 제외해(작업 시작
	// 시각에 따라 걸리거나 안 걸린다) 띠(하루)와 끝 미루기가 어긋난다. weekends·요일 이름은 시각 꼴에서도 그린다.
	if p.exDateLine > 0 && (p.df.has("HH") || p.df.has("mm")) {
		p.line = p.exDateLine
		return p.fail("시각이 있는 dateFormat(%s)에서 날짜 제외(excludes 날짜)는 아직 그리지 않음 — weekends·요일 이름만", p.df.fmt)
	}
	if len(p.c.Tasks) == 0 {
		return p.fail("그릴 작업이 없음")
	}
	if err := p.computeTasks(); err != nil {
		return err
	}
	lo, hi := timeRange(p.c.Tasks)
	if hi.Sub(lo) > MaxYears*8766*time.Hour { // 1년 8766시간(율리우스 년 365.25일) × 상한
		return p.fail("날짜 범위가 %d년을 넘음", MaxYears)
	}
	// 작업 없는 구역은 그리지 않는다(flowchart 의 빈 subgraph 와 같은 경고)
	used := make([]bool, len(p.c.Sections))
	for _, t := range p.c.Tasks {
		if t.Section >= 0 {
			used[t.Section] = true
		}
	}
	for i, s := range p.c.Sections {
		if !used[i] {
			p.issues = append(p.issues, Issue{Line: s.Line, Msg: fmt.Sprintf("빈 section %s 는 그리지 않음", diagram.Quote(s.Name))})
		}
	}
	return nil
}

// timeRange 는 작업 전체의 시각 범위다.
func timeRange(ts []*task) (time.Time, time.Time) {
	lo, hi := ts[0].Start, ts[0].Start
	for _, t := range ts {
		if t.Start.Before(lo) {
			lo = t.Start
		}
		if t.End.After(hi) {
			hi = t.End
		}
		if t.Start.After(hi) {
			hi = t.Start
		}
		if t.End.Before(lo) {
			lo = t.End
		}
	}
	return lo, hi
}

func (p *parser) statement(s string, cur *int) error {
	if !p.header {
		if s != "gantt" {
			return p.fail("mermaid 그림 종류를 알 수 없음 — 첫 줄이 gantt 가 아님(%s)", diagram.Quote(diagram.Clip(firstWord(s))))
		}
		p.header, p.headLine = true, p.line
		return nil
	}
	w := firstWord(s)
	rest := strings.TrimSpace(s[len(w):])
	switch w {
	case "dateFormat":
		if p.sawAnyDate {
			return p.fail("dateFormat 이 둘 이상 있음")
		}
		d, err := parseDateFormat(rest)
		if err != nil {
			return p.fail("%s", err.Error())
		}
		p.df, p.sawAnyDate = d, true
		return nil
	case "axisFormat":
		if p.sawAnyAxis {
			return p.fail("axisFormat 이 둘 이상 있음")
		}
		// 원본은 axisFormat 값을 [^#\n;]+ 로 읽고(맨 # · ; 에서 멈춘다) 줄 단위 전처리의 #…; 표식을 축 글에서 되돌린다 — 이 판은
		// 그 갈래를 구현하지 않으므로 # 나 ; 가 든 값은 그 줄의 원문이다(2026-10-31 사용자 결정 「그 줄은 원문」).
		if strings.ContainsAny(rest, "#;") {
			return p.fail("axisFormat 에 # 나 ; 가 있음 — 원본은 그 글자에서 값을 끊거나 엔티티로 풀어 이 판과 뜻이 갈린다(그 글자를 빼고 쓴다)")
		}
		a, err := parseAxisFormat(rest)
		if err != nil {
			return p.fail("%s", err.Error())
		}
		p.af, p.sawAnyAxis, p.axisLine = a, true, p.line
		return nil
	case "tickInterval":
		if p.hasTick {
			return p.fail("tickInterval 이 둘 이상 있음")
		}
		r, err := parseTickInterval(rest)
		if err != nil {
			return p.fail("%s", err.Error())
		}
		p.tick, p.hasTick, p.tickLine = r, true, p.line
		return nil
	case "excludes":
		return p.addExcludes(rest)
	case "includes":
		return p.fail("includes 는 아직 그리지 않음 — 제외(excludes)만 그린다")
	case "weekend":
		return p.fail("weekend %s 는 아직 그리지 않음 — 주말은 토·일로 그린다", diagram.Quote(diagram.Clip(rest)))
	case "todayMarker":
		if rest == "off" {
			return nil // 오늘 선은 어떤 값이든 그리지 않는다 — off 는 알릴 것도 없다
		}
		p.note("todayMarker", "todayMarker 는 쓰지 않음 — 오늘 선을 그리지 않는다(기록 문서의 그림은 날짜에 의존하지 않는다)")
		return nil
	case "topAxis":
		p.note("topAxis", "topAxis 는 쓰지 않음 — 눈금은 아래에 그린다")
		return nil
	case "weekday":
		p.note("weekday", "weekday 는 쓰지 않음 — 주 눈금은 일요일에 시작한다")
		return nil
	case "click":
		p.note("click", "click 줄(링크·스크립트)은 쓰지 않음")
		return nil
	case "title":
		name, err := p.text(rest)
		if err != nil {
			return err
		}
		p.c.Title, p.c.TitleLine = name, p.line
		return nil
	case "section":
		name, err := p.text(rest)
		if err != nil {
			return err
		}
		if name == "" {
			return p.fail("section 이름이 없음")
		}
		if len(p.c.Sections) >= MaxSections {
			return p.fail("section 이 %d 개를 넘음", MaxSections)
		}
		*cur = len(p.c.Sections)
		p.c.Sections = append(p.c.Sections, &section{Name: name, First: len(p.c.Tasks), Line: p.line})
		return nil
	case "inclusiveEndDates":
		if rest != "" {
			return p.fail("inclusiveEndDates 뒤에 글이 있음(%s)", diagram.Quote(diagram.Clip(rest)))
		}
		p.inclEnd = true
		return nil
	}
	// 접근성 — 정책 B: 원본처럼 앞뒤만 trim, 안쪽 빈칸·탭은 그대로, 맨 &·<·>·제어 문자(Cc)는 원문(공통 diagram.AccValue). 원본은
	// 원문 전체를 엔티티 전처리하므로 값의 `#…;` 를 표식으로 지켜 둔 뒤 정책대로 되돌린다(diagram.EncodeEntities).
	if k, v, ok := diagram.AccLine(s, "accTitle"); ok && k == ':' {
		t, msg := diagram.AccValue(diagram.EncodeEntities(v), "accTitle")
		if msg != "" {
			return p.fail("%s", msg)
		}
		p.c.AccTitle, p.c.AccTitleLine = t, p.line
		return nil
	}
	if k, v, ok := diagram.AccLine(s, "accDescr"); ok {
		if k == ':' {
			t, msg := diagram.AccValue(diagram.EncodeEntities(v), "accDescr")
			if msg != "" {
				return p.fail("%s", msg)
			}
			p.c.AccDescr, p.c.AccDescrLine = t, p.line
			return nil
		}
		p.acc.Reset() // 선언마다 새 값(마지막 선언이 이긴다)
		p.inAcc, p.accLine = true, p.line
		// { 뒤의 글은 줄 끝 빈칸까지 값이다 — 앞뒤를 지운 줄이 아니라 원래 줄에서 첫 { 뒤를 자른다(같은 자리)
		_, rest, _ := strings.Cut(p.raw, "{")
		return p.accBody(rest)
	}
	return p.taskLine(s, *cur)
}

// addExcludes 는 excludes 줄 하나의 낱말을 모은다(여러 줄을 합친다 — Mermaid 와 같다). 낱말의 판정
// (weekends·요일 이름·dateFormat 꼴 날짜)은 2단계의 resolveExcludes 에서 최종 dateFormat 으로 한다 —
// dateFormat 줄이 이 줄 뒤에 와도 같은 꼴로 읽는다.
func (p *parser) addExcludes(rest string) error {
	if rest == "" {
		return p.fail("excludes 값이 없음")
	}
	toks := strings.FieldsFunc(rest, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
	if len(toks) == 0 {
		return p.fail("excludes 값이 없음")
	}
	p.exLines = append(p.exLines, exLine{toks: toks, line: p.line})
	return nil
}

// resolveExcludes 는 모은 excludes 낱말을 최종 dateFormat 으로 판정한다(2단계). weekends·요일 이름이 아니고
// dateFormat 으로 읽히지도 않는 낱말은 그 줄에서 *Error 다 — 모르는 값을 조용히 넣지 않는다(Mermaid 는 날짜 문자열
// 집합에 넣어 문자열 비교로만 판정해 아무 효과가 없다).
func (p *parser) resolveExcludes() error {
	timeish := p.df.has("HH") || p.df.has("mm") // 시각이 있는 꼴 — 날짜 부분만 적은 값은 아래 검사(시각 꼴의 날짜 제외)로 보낸다
	for _, ln := range p.exLines {
		p.line = ln.line
		for _, tok := range ln.toks {
			p.ex.none = false // 값 하나라도 있으면 제외가 켜진다 — 요일 분기의 continue 앞에서(요일만 적은 줄도 무효가 되지 않게)
			switch {
			case strings.EqualFold(tok, "weekends"):
				p.ex.weekends = true
			default:
				if d, ok := weekdayNames[strings.ToLower(tok)]; ok {
					p.ex.weekdays[d] = true
					continue
				}
				if _, err := p.df.parse(tok); err != nil {
					// 시각이 있는 꼴에서 날짜 부분(시·분을 뺀 꼴)만 적은 값은 그릴 때의 시각 꼴 검사가 그 줄에서
					// 오류를 낸다(「시각이 있는 dateFormat …」 — parse 마지막). 그 밖의 낱말은 이 자리에서 오류다.
					if !timeish {
						return p.fail("excludes 의 %s 은 weekends·요일 이름·dateFormat 꼴 날짜가 아님", diagram.Quote(diagram.Clip(tok)))
					}
					if _, e2 := p.df.dateOnly().parse(tok); e2 != nil {
						return p.fail("excludes 의 %s 은 weekends·요일 이름·dateFormat 꼴 날짜가 아님", diagram.Quote(diagram.Clip(tok)))
					}
				}
				p.ex.dates[tok] = true // 날짜 문자열(dateFormat 꼴) — 판정은 포맷 문자열 비교다
				if p.exDateLine == 0 {
					p.exDateLine = ln.line
				}
			}
		}
	}
	return nil
}

// computeTasks 는 작업의 날짜를 최종 설정(dateFormat·excludes·inclusiveEndDates)으로 차례로 계산한다(2단계 —
// 설정 줄이 작업 뒤에 와도 그림 전체에 적용된다). 오류의 줄은 그 작업 줄, 값 하나(끝) 작업의 시작은 바로 앞 작업의
// 계산된 끝이다.
func (p *parser) computeTasks() error {
	p.lastEnd, p.hasLast = time.Time{}, false
	for _, pt := range p.pending {
		p.line = pt.t.Line
		start := time.Time{}
		if len(pt.meta) == 2 {
			var err error
			start, err = p.startVal(pt.meta[0], pt.idx)
			if err != nil {
				return err
			}
		} else if !p.hasLast {
			return p.fail("첫 작업의 시작을 알 수 없음 — 값 하나(끝)는 바로 앞 작업의 끝에서 시작한다")
		} else {
			start = p.lastEnd
		}
		end, err := p.endVal(pt.meta[len(pt.meta)-1], start, pt.idx)
		if err != nil {
			return err
		}
		if end.Before(start) {
			return p.fail("끝이 시작보다 앞(%s)", diagram.Quote(diagram.Clip(pt.t.Name)))
		}
		pt.t.Start, pt.t.End = start, p.pushEnd(start, end)
		p.lastEnd, p.hasLast = pt.t.End, true
	}
	return nil
}

// taskLine 은 작업 줄(이름 : 메타…)을 읽는다.
func (p *parser) taskLine(s string, cur int) error {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return p.fail("알 수 없는 문장(%s)", diagram.Quote(diagram.Clip(s)))
	}
	// 간트는 라벨 글에만 엔티티 전처리를 씌운다(text). 작업 줄의 라벨(이름)은 줄의 앞부분이라, 원본의 줄 전체 전처리와 같으려면
	// style…:…#…; 꼴(원본은 줄의 마지막 ; 를 자른다 — 작업 줄에서는 콜론 뒤 값 쪽)이 없고 줄의 엔티티 토큰이 모두 이름 안에 있어야
	// 한다(#409 ④-2 후속 리뷰 1회차 [1] — 흐름도와 같은 검사). title·section·접근성 줄은 라벨이 줄 끝까지라 두 전처리가 같다.
	if diagram.StyleCut(s) {
		return p.fail("style…:…#…; 꼴이 든 작업 줄 — 원본 전처리는 줄 끝 ; 를 자르고 엔티티를 지켜 이 판(라벨 글만 전처리)과 뜻이 갈린다")
	}
	if diagram.EntityTokens(s) != diagram.EntityTokens(s[:i]) {
		return p.fail("엔티티 #…; 가 작업 이름 밖(콜론 뒤)에 있음 — 원본은 줄 전체를 전처리해 이 판과 뜻이 갈린다")
	}
	name, err := p.text(strings.TrimSpace(s[:i]))
	if err != nil {
		return err
	}
	if name == "" {
		return p.fail("작업 이름이 없음")
	}
	var meta []string
	for _, tok := range strings.Split(s[i+1:], ",") {
		if tok = strings.TrimSpace(tok); tok != "" {
			meta = append(meta, tok)
		}
	}
	t := &task{Name: name, Section: cur, Line: p.line}
	// 태그는 앞에서부터 뗀다(Mermaid ganttDb.getTaskTags 와 같게 — 중간의 태그는 값으로 읽는다)
	j := 0
tags:
	for ; j < len(meta); j++ {
		switch meta[j] {
		case "done":
			t.Done = true
		case "active":
			t.Active = true
		case "crit":
			t.Crit = true
		case "milestone":
			t.Milestone = true
		case "vert":
			return p.fail("vert 태그(세로 표식)는 아직 그리지 않음")
		default:
			break tags
		}
	}
	meta = meta[j:]
	if len(meta) == 0 {
		return p.fail("작업의 시작·끝이 없음(%s)", diagram.Quote(diagram.Clip(name)))
	}
	if len(meta) > 3 {
		return p.fail("작업의 값이 너무 많음(태그를 빼고 1~3개)%s", diagram.Quote(diagram.Clip(name)))
	}
	if len(meta) == 3 {
		if _, dup := p.byID[meta[0]]; dup {
			return p.fail("작업 id %s 가 겹침", diagram.Quote(diagram.Clip(meta[0])))
		}
		t.ID = meta[0]
		meta = meta[1:]
	}
	if len(p.c.Tasks) >= MaxTasks {
		return p.fail("작업이 %d 개를 넘음", MaxTasks)
	}
	k := len(p.c.Tasks)
	if t.ID != "" {
		p.byID[t.ID] = k
	}
	p.c.Tasks = append(p.c.Tasks, t)
	// 날짜(시작·끝)는 2단계(computeTasks)에서 최종 설정으로 계산한다 — 설정 줄이 이 줄 뒤에 와도 그림 전체에 적용된다.
	p.pending = append(p.pending, pendTask{t: t, meta: meta, idx: k})
	return nil
}

// startVal 은 시작 값을 읽는다: 날짜(dateFormat) 또는 after id […](가장 늦은 끝). self 는 지금 작업의 차례로,
// 참조하는 id 가 자기와 같거나 뒤의 작업이면 모르는 id 다(앞에 정의된 id 만 참조한다).
func (p *parser) startVal(tok string, self int) (time.Time, error) {
	if rest, ok := strings.CutPrefix(tok, "after"); ok {
		ids := strings.Fields(rest)
		if len(ids) == 0 {
			return time.Time{}, p.fail("after 뒤에 작업 id 가 없음")
		}
		// 최댓값은 첫 참조 작업의 끝으로 시작한다(until 과 같게) — 영값 time.Time{} 은 0001-01-01 이라 0000년 끝을 밀어낸다
		best := time.Time{}
		first := true
		for _, id := range ids {
			k, ok := p.byID[id]
			if !ok || k >= self {
				return time.Time{}, p.fail("after 의 작업 id %s 를 모름(앞에 정의된 id)", diagram.Quote(diagram.Clip(id)))
			}
			if e := p.c.Tasks[k].End; first || e.After(best) {
				best, first = e, false
			}
		}
		return best, nil
	}
	v, err := p.df.parse(tok)
	if err != nil {
		return time.Time{}, p.fail("%s", err.Error())
	}
	return v, nil
}

// endVal 은 끝 값을 읽는다: 기간·until id […](가장 이른 시작)·날짜(inclusiveEndDates 면 최소 단위만큼 더한다).
// self 는 startVal 과 같다.
func (p *parser) endVal(tok string, start time.Time, self int) (time.Time, error) {
	if rest, ok := strings.CutPrefix(tok, "until"); ok {
		ids := strings.Fields(rest)
		if len(ids) == 0 {
			return time.Time{}, p.fail("until 뒤에 작업 id 가 없음")
		}
		best := time.Time{}
		first := true
		for _, id := range ids {
			k, ok := p.byID[id]
			if !ok || k >= self {
				return time.Time{}, p.fail("until 의 작업 id %s 를 모름(앞에 정의된 id)", diagram.Quote(diagram.Clip(id)))
			}
			s := p.c.Tasks[k].Start
			if first || s.Before(best) {
				best, first = s, false
			}
		}
		return best, nil
	}
	if d, ok, err := parseDur(tok); err != nil {
		return time.Time{}, p.fail("%s", err.Error())
	} else if ok {
		return d.add(start), nil
	}
	v, err := p.df.parse(tok)
	if err != nil {
		return time.Time{}, p.fail("%s", err.Error())
	}
	if p.inclEnd {
		v = p.df.inclusiveStep(v)
	}
	return v, nil
}

// pushEnd 은 제외 날짜만큼 끝을 하루씩 민다(Mermaid ganttDb.checkTaskDates 와 같은 알고리즘): 시작+1일부터 끝까지
// 범위 안의 제외일 하나마다 끝에 하루를 더한다 — 막대에 구멍이 생기지 않고 오른쪽으로 늘어난다. 제외가 없으면 그대로.
func (p *parser) pushEnd(start, end time.Time) time.Time {
	if p.ex.none {
		return end
	}
	t := start.AddDate(0, 0, 1)
	for n := 0; !t.After(end); n++ {
		if n > 10000 { // Mermaid 와 같은 루프 상한
			break
		}
		if p.ex.check(t, p.df) {
			end = end.AddDate(0, 0, 1)
		}
		t = t.AddDate(0, 0, 1)
	}
	return end
}

// text 는 라벨 글(제목·구역·작업 이름)을 정책 B 로 읽는다. 간트는 줄 단위 엔티티 전처리를 하지 않으므로(줄 루프의 HasMarker 뒤)
// 라벨 글에 원본 전처리(diagram.EncodeEntities)를 먼저 씌운다 — 원본 gantt.jison 의 라벨 토큰(title·section 은 [^\n]+, 작업 이름
// taskTxt 는 [^:\n]+)이 #·; 를 글자로 받으므로 줄 전체를 전처리한 것과 라벨 값이 같다(그 밖의 자리 — 날짜·설정 — 는 읽던 그대로).
// 그 뒤는 공통 diagram.Label(<br> 은 줄바꿈, 받는 엔티티만 한 번 글자로, 안쪽 연속 빈칸·탭·제어 문자(C0·DEL)는 원문)과 OneLine(간트 라벨은
// 한 줄 — 줄바꿈은 빈칸 하나, 빈 줄은 원문)이다.
func (p *parser) text(s string) (string, error) {
	v, msg := diagram.Label(diagram.EncodeEntities(s), "라벨", true)
	if msg == "" {
		v, msg = diagram.OneLine(v, "라벨")
	}
	if msg != "" {
		return "", p.fail("%s", msg)
	}
	return v, nil
}

// accBody 는 accDescr { … } 의 한 줄(여는 줄의 { 뒤 포함 — 앞 빈칸만 지운 것, 끝 빈칸은 그대로)을 모은다(공통 diagram.AccBlock —
// 닫는 } 는 줄 끝에만, 블록 전체 trim 뒤 원본 setAccDescription 의 \n\s+ → \n, 줄바꿈·안쪽 빈칸은 그대로 — 정책 B). 빈 블록은
// 앞 설명을 그대로 둔다.
func (p *parser) accBody(t string) error {
	done, v, msg := p.acc.Add(diagram.EncodeEntities(t))
	if msg != "" {
		return p.fail("%s", msg)
	}
	if !done {
		return nil
	}
	p.inAcc = false
	if v == "" {
		return nil
	}
	d, msg := diagram.AccSet(v, "accDescr")
	if msg != "" {
		return p.fail("%s", msg)
	}
	p.c.AccDescr, p.c.AccDescrLine = d, p.line
	return nil
}

// eachText 는 모델에서 SVG·Result 로 나가는 사용자 글자를 하나씩 f 에 넘긴다(field 는 「형.필드」, what 은 경고 문구의 이름,
// line 은 그 값이 나온 줄). **출력 글자 필드 목록은 여기 한곳이다** — 모델에 글 필드를 더하면 여기에도 더한다(TestGateCoversAllText
// 가 모델의 string·[]string 필드와 견준다). 작업 id 는 그리지 않지만 함께 보고, axisFormat 의 글자 조각은 눈금 라벨에 그대로 들어간다.
func (c *chart) eachText(f diagram.TextFunc) error {
	for _, x := range []struct {
		field, what, v string
		line           int
	}{
		{"chart.FMCaption", "frontmatter title", c.FMCaption, c.FMLine},
		{"chart.Title", "제목", c.Title, c.TitleLine},
		{"chart.AccTitle", "accTitle", c.AccTitle, c.AccTitleLine},
		{"chart.AccDescr", "accDescr", c.AccDescr, c.AccDescrLine},
	} {
		if err := f(x.field, x.what, x.v, x.line); err != nil {
			return err
		}
	}
	if c.AF != nil {
		for _, ap := range c.AF.parts {
			if err := f("axisPart.s", "axisFormat", ap.s, c.AxisLine); err != nil {
				return err
			}
		}
	}
	for _, t := range c.Tasks {
		if err := f("task.Name", "작업 이름", t.Name, t.Line); err != nil {
			return err
		}
		if err := f("task.ID", "작업 id", t.ID, t.Line); err != nil {
			return err
		}
	}
	for _, sc := range c.Sections {
		if err := f("section.Name", "section 이름", sc.Name, sc.Line); err != nil {
			return err
		}
	}
	return nil
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}
