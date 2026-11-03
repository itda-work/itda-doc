package timeline

import (
	"fmt"
	"strings"

	"github.com/itda-work/itda-doc/diagram"
)

// ---- 원본 엔티티 전처리(mermaid@12.2.0 utils.ts encodeEntities · Diagram.fromText 재현 — 리뷰 2회차 [1])
//
// 원본은 파싱 전에 원문 전체를 encodeEntities 로 지나친다: `style…:#…;`·`classDef…:#…;` 줄의 끝 `;` 를 자르고,
// 엔티티 `#\w+;`(JS \w = ASCII [A-Za-z0-9_] — Go regexp \w 와 같다)를 # 이 아닌 내부 표식으로 바꾼다(정수 `ﬂ°°N¶ß`,
// 이름 `ﬂ°N¶ß` — 원본 표식 문자를 그대로 쓴다). 그래서 timeline lexer 의 시기 토큰이 # 을 제외해도 엔티티의 # 은
// 주석으로 버려지지 않는다. 이 판도 같은 순서다: 전처리(본문 줄마다) → 시기의 남은 # 만 원문 + 경고 → 라벨을 만들 때
// 표식을 엔티티 정책 B 로 한 번 글자로 되돌린다(공통 diagram.Label·AccSet — 받지 않는 엔티티는 원문 + 경고) — 되돌린 글자가
// 문법 글자(: 등)여도 이미 문법을 지쳤으므로 다시 읽히지 않는다(원본과 같다). 재현 자체는 공통으로 올려 sequence(itda-hyve#409 ③) 와 함께 쓴다.

var encodeEntities = diagram.EncodeEntities

// 원문을 한 번 훑는다 — 모르는 문장·값은 그 자리에서 *Error 로 멈춘다(원문 길이에 선형).
// 줄 구분은 Mermaid timeline 의 jison 어휘 그대로다(mermaid@12.2.0 packages/mermaid/src/diagrams/timeline/parser/timeline.jison):
//
//	시기   콜론이 아닌 글자들(콜론·줄바꿈 밖 전부)
//	사건   “콜론 + 공백 하나” 로 시작해 다음 “콜론 + 공백” 또는 줄 끝까지(콜론도 뒤에 공백이 없으면 사건 글자다)
//	한 줄	시기 : 사건1 : 사건2 — 시기 하나와 사건 0개 이상. 사건 없이 시기만인 줄도 시기다
//	이어 쓰기	: 사건 — 바로 앞 줄까지의 마지막 시기에 사건을 더한다(구역이 바뀌었어도 마지막 시기 — 원본 addEvent 는
//	           currentTaskId-1, 마지막 시기가 없으면 원본은 그리다 죽는다 — 이 판은 원문 + 줄 번호 경고)
//
//	머리 줄      timeline · timeline LR(가로 — 방향 없는 것과 같다)
//	제목         title 글(그림 안 제목) — frontmatter title 은 캡션(공통 규칙)
//	구역         section 이름 — 이름에 콜론은 쓰지 못 한다(원본 문법). 구역 앞의 시기는 구역 없는 시기
//	줄바꿈       글 안의 <br>(<br/>·<br />) — 상자 안 줄바꿈. 다른 HTML 은 글자 그대로(이스케이프)
//	접근성       accTitle: 글 · accDescr: 글 · accDescr { … }
//	쓰지 않음    %%{init: …}%%(줄 전체가 지시 하나일 때만 — %%{ 가 든 그 밖의 줄은 미지원) · frontmatter 의 title 밖 키(config:·disableMulticolor·themeVariables 포함)
//	미지원       timeline TD(세로) · 그 밖의 방향 낱말 · title/section 이름 없음 · section 이름의 콜론 · 시기 없는 사건 ·
//	             콜론 뒤 공백 없음(시기:사건 — 원본도 읽지 못 한다) · 빈 사건(시기 :) · # 주석(원본 lexer 에는 있지만
//	             공통 규칙은 %% 만 — # 로 시작하는 줄은 알 수 없는 문장) · 상한 초과 · 알 수 없는 문장

// Parse 는 mermaid timeline 원문을 읽는다. 그릴 수 없는 문법이면 *Error 다. issues 는 쓰지 않은 줄이다.
func Parse(src string) (*tl, []Issue, error) {
	p := &parser{c: &tl{}, said: map[string]bool{}, last: -1}
	// 잘못된 UTF-8 은 읽기 전에 U+FFFD 로(파서가 바이트 단위로 자른 문자열을 다루지만 라벨은 룬 단위로 센다).
	// XML 이 허용하지 않는 글자는 SVG 를 쓰는 경계(diagram.Esc)에서 거른다 — gantt 와 같다.
	if err := p.parse(strings.ToValidUTF8(src, "�")); err != nil {
		return nil, nil, err
	}
	// 출력 관문 — 본문을 다 읽은 뒤, 내보낼 사용자 글자를 모두 한 번에 본다(본문 오류가 먼저다 — state 와 같은 꼴).
	if err := diagram.Gate(p.c.eachText); err != nil {
		return nil, nil, err
	}
	return p.c, p.issues, nil
}

type parser struct {
	c       *tl
	issues  []Issue
	line    int             // 지금 읽는 줄(1부터)
	header  bool            // 머리 줄을 지났다
	last    int             // 마지막 시기(chart.Periods 의 차례, 없으면 -1)
	nEvents int             // 사건 합계(상한)
	said    map[string]bool // 한 번만 알릴 Issue
	inAcc   bool            // accDescr { … } 안
	accLine int
	acc     diagram.AccBlock
	raw     string // 지금 줄의 원문(엔티티 전처리 뒤, 줄 끝 CR 만 지운 것) — accDescr { 여는 줄의 값
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
		// 줄 중간의 CR(원본 전처리가 줄바꿈으로 바꾼다)·FF·VT(원본 lexer 가 건너뛰지 않는다)는 받지 않는다(er·state 와 같다).
		if strings.ContainsAny(strings.TrimSuffix(lines[i], "\r"), "\r\f\v") {
			return p.fail("줄 안에 CR·FF·VT 글자가 있음 — 이 판은 그 줄을 그리지 않는다")
		}
		// ASCII 밖 공백·서식 문자는 줄째 그리지 않는다(원본 JS \s 와 Go 의 공백 판정이 갈리는 갈래 — er·sequence·state 와 같다,
		// itda-hyve#409 ④-2 후속 리뷰 1회차 [2]·[3]: 받으면 긴 라벨 줄바꿈(strings.Fields)이 NBSP 를 빈칸으로 바꾸고 accDescr 블록의 줄 머리
		// 정리가 원본 \n\s+ 와 갈린다).
		if r, ok := diagram.OddSpace(lines[i]); ok {
			return p.fail("ASCII 밖 공백·서식 문자(U+%04X)가 있음 — 이 판은 그 줄을 그리지 않는다(일반 빈칸으로 바꿔 쓴다)", r)
		}
		// 원문에 이미 들어 있는 Mermaid 내부 표식 문자(encodeEntities 의 ﬂ°·¶ß)는 그리지 않는다(2026-10-30 사용자
		// 결정 — 원본은 그리지만, 이 판의 복원은 이 판이 만든 표식만 되돌리므로 원문 표식과 구별한다. 리뷰 3회차 [2]).
		// 전처리가 만드는 표식과 겹치지 않게 검사는 원문 상태에서 한다.
		if strings.Contains(t, "ﬂ°") || strings.Contains(t, "¶ß") {
			return p.fail("본문 줄에 Mermaid 내부 표식 문자(ﬂ°·¶ß)가 있음 — 이 판은 그 줄을 그리지 않는다(엔티티 표기 #…; 는 그대로 쓸 수 있다)")
		}
		// 설정 지시 — 줄 전체가 지시 하나인 줄(accDescr { … } 밖)만 무시하는 지시다. 그 밖에 %%{ 가 든 줄(지시 앞뒤·사이의
		// 글, acc 값·사건 글·주석 안, accDescr 블록 안)은 원본 removeDirectives 가 지시만 지우고 남은 글을 읽는 갈래라
		// 그리지 않는다(er·sequence 와 같은 공통 규칙 — itda-hyve#409 ④-1 후속). 엔티티 전처리 앞이다(원본도 지시를 먼저 지운다).
		if msg := diagram.DirectiveProblem(t, p.inAcc); msg != "" {
			return p.fail("%s", msg)
		}
		// 줄 머리 %% 주석 — 원본 전처리 cleanupComments(/^\s*%%(?!{)[^\n]+\n?/gm)가 lexer 상태와 관계없이(accDescr { … } 블록 안도)
		// 먼저 지운다. 그래서 지시 검사 뒤, 엔티티 전처리·상태 분기 앞에 둔다(er 와 같은 차례 — itda-hyve#409 ④-1 후속 리뷰 1회차 [2]).
		// 뒤에 글이 없는 맨 %% 는 원본 전처리가 지우지 않는다. 블록 밖에서는 원본 lexer 의 주석 규칙(\%%(?!\{)[^\n]* —
		// jison:14)이 건너뛰므로 이 판도 경고 없이 건너뛴다. accDescr { … } 블록 안에서는 원본이 설명 글로 넣는다 — 이 판은
		// 그 줄을 그리지 않는다(보수적, itda-hyve#409 ④-1 후속 수정 2).
		// 여기 오는 %%{ 줄은 줄 전체가 지시 하나뿐이다(DirectiveProblem).
		if strings.HasPrefix(t, "%%") {
			if strings.HasPrefix(t, "%%{") {
				p.note("init", "설정 지시(%%{…}%%)는 쓰지 않음 — 색·글꼴은 테마가 정한다")
			} else if p.inAcc && strings.TrimLeft(strings.TrimSuffix(lines[i], "\r"), " \t") == "%%" {
				return p.fail("accDescr 블록 안의 글 없는 %s 줄 — 원본은 설명 글로 넣는 줄이라 이 판은 그리지 않는다(%s 뒤에 글을 둔다)", "%%", "%%")
			}
			continue
		}
		// 원본 전처리(Diagram.fromText → encodeEntities) — 엔티티를 표식으로 바꿔 둔다(라벨을 만들 때 되돌린다).
		// accDescr { … } 블록의 줄들도(상태 분기보다 먼저) 똑같이 지나간다(리뷰 3회차 [1]). frontmatter 는 공통
		// YAML 규칙이 그대로 읽도록 전처리하지 않는다.
		t = encodeEntities(t)
		p.raw = encodeEntities(strings.TrimSuffix(lines[i], "\r"))
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
	if len(p.c.Periods) == 0 {
		p.line = 0
		return p.fail("그릴 시기가 없음")
	}
	// 시기 없는 구역은 그리지 않는다(flowchart 의 빈 subgraph·gantt 의 빈 section 과 같은 경고)
	used := make([]bool, len(p.c.Sections))
	for _, d := range p.c.Periods {
		if d.Section >= 0 {
			used[d.Section] = true
		}
	}
	for i, s := range p.c.Sections {
		if !used[i] {
			p.issues = append(p.issues, Issue{Line: s.Line, Msg: fmt.Sprintf("빈 section %s 는 그리지 않음", diagram.Quote(s.Name))})
		}
	}
	return nil
}

// accOf 는 접근성 문장(accTitle·accDescr)을 원본처럼 대소문자 무시로 찾는다(공통 diagram.AccLine 은 정확 비교라
// 접두만 정규화해 넘긴다 — 공통 함수는 흐름도·간트와 공유하므로 여기서만 바꾼다).
func (p *parser) accOf(s, kw string) (byte, string, bool) {
	if len(s) < len(kw) || !strings.EqualFold(s[:len(kw)], kw) {
		return 0, "", false
	}
	return diagram.AccLine(kw+s[len(kw):], kw)
}

// accBody 는 accDescr { … } 의 한 줄(여는 줄의 { 뒤 포함 — 앞 빈칸만 지운 것, 끝 빈칸은 그대로)을 모은다(공통 diagram.AccBlock —
// 닫는 } 는 줄 끝에만, 블록 전체 trim 뒤 원본 setAccDescription 의 \n\s+ → \n, 줄바꿈·안쪽 빈칸은 그대로 — 정책 B). 빈 블록은
// 앞 설명을 그대로 둔다.
func (p *parser) accBody(t string) error {
	done, v, msg := p.acc.Add(t)
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
	return p.accSet(v, &p.c.AccDescr, &p.c.AccDescrLine, "accDescr")
}

// accSet 은 접근성 값(원본처럼 다듬은 것)의 엔티티 표식만 한 번 되돌려 dst 에 둔다(공통 diagram.AccSet).
func (p *parser) accSet(v string, dst *string, line *int, kw string) error {
	s, msg := diagram.AccSet(v, kw)
	if msg != "" {
		return p.fail("%s", msg)
	}
	*dst, *line = s, p.line
	return nil
}

func (p *parser) statement(s string, cur *int) error {
	if !p.header {
		return p.headerLine(s)
	}
	// 지시자(title·section·accTitle·accDescr)는 원본처럼 대소문자를 가리지 않는다(jison case-insensitive).
	w := firstWord(s)
	rest := strings.TrimSpace(s[len(w):])
	switch {
	case strings.EqualFold(w, "title"):
		if rest == "" {
			return p.fail("title 뒤에 제목이 없음")
		}
		name, err := p.text(rest)
		if err != nil {
			return err
		}
		one, msg := diagram.OneLine(name, "제목") // 제목은 한 줄(gantt 와 같게 — <br> 도 공백으로)
		if msg != "" {
			return p.fail("%s", msg)
		}
		p.c.Title, p.c.TitleLine = one, p.line
		return nil
	case strings.EqualFold(w, "section"):
		if rest == "" {
			return p.fail("section 이름이 없음")
		}
		if strings.ContainsRune(rest, ':') {
			// 원본 section 토큰은 콜론 앞에서 끝난다 — 뒤의 “:b” 는 콜론 뒤 공백이 없어 사건 토큰도 되지 못 한다(리뷰 [6]).
			// “a: b” 처럼 공백을 두면 원본은 구역과 이어 쓰기 사건으로 쪼갠다 — 이 판은 둘 다 그리지 않는다(안내서 9장).
			return p.fail("section 이름에 콜론은 쓰지 못 함(%s)", diagram.Quote(diagram.Clip(rest)))
		}
		name, err := p.text(rest)
		if err != nil {
			return err
		}
		if len(p.c.Sections) >= MaxSections {
			return p.fail("section 이 %d 개를 넘음", MaxSections)
		}
		*cur = len(p.c.Sections)
		p.c.Sections = append(p.c.Sections, &section{Name: name, First: len(p.c.Periods), Line: p.line})
		return nil
	}
	// 접근성 — 정책 B: 원본처럼 앞뒤만 trim, 안쪽 빈칸·탭은 그대로, 맨 &·<·>·제어 문자(Cc)는 원문(공통 diagram.AccValue)
	if k, v, ok := p.accOf(s, "accTitle"); ok && k == ':' {
		t, msg := diagram.AccValue(v, "accTitle")
		if msg != "" {
			return p.fail("%s", msg)
		}
		p.c.AccTitle, p.c.AccTitleLine = t, p.line
		return nil
	}
	if k, v, ok := p.accOf(s, "accDescr"); ok {
		if k == ':' {
			t, msg := diagram.AccValue(v, "accDescr")
			if msg != "" {
				return p.fail("%s", msg)
			}
			p.c.AccDescr, p.c.AccDescrLine = t, p.line // 선언이 여럿이면 마지막이 이긴다(원본도 setter 대입 — 콜론형과 같다)
			return nil
		}
		p.acc.Reset() // 선언마다 버퍼를 새로 시작한다(마지막 선언이 이긴다 — 리뷰 [4])
		p.inAcc, p.accLine = true, p.line
		// { 뒤의 글은 줄 끝 빈칸까지 값이다 — 앞뒤를 지운 줄이 아니라 원래 줄에서 첫 { 뒤를 자른다(같은 자리)
		_, rest, _ := strings.Cut(p.raw, "{")
		return p.accBody(rest)
	}
	return p.dataLine(s, *cur)
}

// headerLine 은 머리 줄(timeline · timeline LR)을 본다. 원본은 대소문자를 가리지 않는다(jison case-insensitive).
func (p *parser) headerLine(s string) error {
	f := strings.Fields(s)
	if len(f) == 0 || !strings.EqualFold(f[0], "timeline") {
		return p.fail("mermaid 그림 종류를 알 수 없음 — 첫 줄이 timeline 이 아님(%s)", diagram.Quote(diagram.Clip(firstWord(s))))
	}
	if len(f) > 2 {
		return p.fail("머리 줄의 낱말이 많음(%s) — 방향은 하나만", diagram.Quote(diagram.Clip(s)))
	}
	if len(f) == 2 {
		switch strings.ToUpper(f[1]) {
		case "LR": // 가로 — 방향 없는 것과 같다
		case "TD", "TB":
			return p.fail("세로 방향(timeline %s)은 아직 그리지 않음 — 가로(방향 없음 또는 LR)만 그린다", strings.ToUpper(f[1]))
		default:
			return p.fail("머리 줄의 방향 낱말 %s 을 모름 — LR(가로)만", diagram.Quote(diagram.Clip(f[1])))
		}
	}
	p.header = true
	return nil
}

// dataLine 은 시기·사건 줄을 읽는다 — 시기 : 사건1 : 사건2, 시기만, 이어 쓰기(: 사건).
// 사건 조각을 자르는 규칙은 jison event 토큰 그대로다(머리 주석).
func (p *parser) dataLine(s string, cur int) error {
	// 원본 lexer 의 시기(period) 토큰은 # 을 제외한다 — # 부터 줄 끝까지가 주석으로 버려진다. 이 판은 # 주석을
	// 지원하지 않으므로(공통 주석은 %% 만) 시기 자리에 # 이 나오면 그리지 않고 알린다(원본이 주석으로 숨긴 내용을
	// 그림에 더하지 않게). 사건(event) 토큰은 원본이 # 을 허용하므로 사건 안의 # 은 글자·엔티티 그대로다(리뷰 [2]).
	i := strings.IndexByte(s, ':')
	if i < 0 { // 시기만
		if strings.ContainsRune(s, '#') {
			return p.failHash()
		}
		name, err := p.text(s)
		if err != nil {
			return err
		}
		return p.addPeriod(name, nil, cur)
	}
	if strings.ContainsRune(s[:i], '#') {
		return p.failHash()
	}
	if i == 0 { // 이어 쓰기 — 마지막 시기에 사건을 더한다
		evs, err := p.events(s)
		if err != nil {
			return err
		}
		if p.last < 0 {
			return p.fail("사건(%s)이 시기 없이 먼저 왔음 — “시기 : 사건” 순서로 쓴다", diagram.Quote(diagram.Clip(firstEvent(evs))))
		}
		return p.addEvents(p.last, evs)
	}
	name, err := p.text(s[:i])
	if err != nil {
		return err
	}
	evs, err := p.events(s[i:])
	if err != nil {
		return err
	}
	return p.addPeriod(name, evs, cur)
}

func (p *parser) failHash() *Error {
	// %% 를 인자로 넘겨 실제 경고에는 주석 표기가 그대로 남게 한다(리뷰 2회차 [3] — 형식 문자열에 두면 % 로 줄어든다).
	return p.fail("시기의 # 부터는 Mermaid 가 주석으로 버리는 자리다 — 이 판은 그리지 않는다(주석은 %s, 사건 안의 # 은 글자)", "%%")
}

// events 는 “: 사건1 : 사건2” 꼴의 줄(첫 콜론부터)을 사건들로 자른다. 콜론 뒤에 공백이 있어야 구분 콜론이고,
// 공백 없는 콜론은 사건 글자다(10:30 처럼). 줄 끝의 콜론도 사건 글자다(jison “:”(?!\s) 와 같게).
func (p *parser) events(s string) ([]string, error) {
	var out []string
	i := 0
	for i < len(s) {
		if i+1 >= len(s) {
			return nil, p.fail("사건이 비었음")
		}
		if !isSpace(s[i+1]) {
			return nil, p.fail("콜론 뒤에 공백이 없음 — “시기 : 사건” 처럼 쓴다(%s)", diagram.Quote(diagram.Clip(s)))
		}
		j := i + 2
		start := j
		for j < len(s) {
			if s[j] == ':' && j+1 < len(s) && isSpace(s[j+1]) {
				break // 다음 구분 콜론
			}
			j++
		}
		text, err := p.text(s[start:j])
		if err != nil {
			return nil, err
		}
		if text == "" {
			return nil, p.fail("사건이 비었음")
		}
		out = append(out, text)
		i = j
	}
	return out, nil
}

func (p *parser) addPeriod(name string, evs []string, cur int) error {
	if name == "" {
		return p.fail("시기 이름이 없음")
	}
	if len(p.c.Periods) >= MaxPeriods {
		return p.fail("시기가 %d 개를 넘음", MaxPeriods)
	}
	d := &period{Name: name, Section: cur, Line: p.line}
	p.c.Periods = append(p.c.Periods, d)
	p.last = len(p.c.Periods) - 1
	return p.addEvents(len(p.c.Periods)-1, evs)
}

func (p *parser) addEvents(k int, evs []string) error {
	d := p.c.Periods[k]
	if len(d.Events)+len(evs) > MaxEventsPerPeriod {
		return p.fail("시기 하나의 사건이 %d 개를 넘음", MaxEventsPerPeriod)
	}
	if p.nEvents+len(evs) > MaxEvents {
		return p.fail("사건 합계가 %d 개를 넘음", MaxEvents)
	}
	d.Events = append(d.Events, evs...)
	for range evs {
		d.EventLines = append(d.EventLines, p.line)
	}
	p.nEvents += len(evs)
	return nil
}

// text 는 라벨 글을 정책 B 로 읽는다(공통 diagram.Label — <br> 은 줄바꿈, 엔티티 표식은 받는 것만 한 번 글자로, 앞뒤 빈칸을
// 지운 뒤 안쪽 연속 빈칸·탭·제어 문자(C0·DEL)는 원문 + 경고). 되돌린 글자가 문법 글자(: 등)여도 문법은 이미 지쳤다. 줄바꿈은 그대로
// 둔다(상자 안 줄바꿈 — 간트가 한 칸으로 뭉치는 것과 다른 점).
func (p *parser) text(s string) (string, error) {
	v, msg := diagram.Label(s, "라벨", true)
	if msg != "" {
		return "", p.fail("%s", msg)
	}
	return v, nil
}

// eachText 는 모델에서 SVG·Result 로 나가는 사용자 글자를 하나씩 f 에 넘긴다(field 는 「형.필드」, what 은 경고 문구의 이름,
// line 은 그 값이 나온 줄). **출력 글자 필드 목록은 여기 한곳이다** — 모델에 글 필드를 더하면 여기에도 더한다(TestGateCoversAllText
// 가 모델의 string·[]string 필드와 견준다).
func (c *tl) eachText(f diagram.TextFunc) error {
	for _, x := range []struct {
		field, what, v string
		line           int
	}{
		{"tl.FMCaption", "frontmatter title", c.FMCaption, c.FMLine},
		{"tl.Title", "제목", c.Title, c.TitleLine},
		{"tl.AccTitle", "accTitle", c.AccTitle, c.AccTitleLine},
		{"tl.AccDescr", "accDescr", c.AccDescr, c.AccDescrLine},
	} {
		if err := f(x.field, x.what, x.v, x.line); err != nil {
			return err
		}
	}
	for _, d := range c.Periods {
		if err := f("period.Name", "시기", d.Name, d.Line); err != nil {
			return err
		}
		for k, e := range d.Events {
			if err := f("period.Events", "사건", e, d.EventLines[k]); err != nil {
				return err
			}
		}
	}
	for _, sc := range c.Sections {
		if err := f("section.Name", "section 이름", sc.Name, sc.Line); err != nil {
			return err
		}
	}
	return nil
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' }

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}

func firstEvent(evs []string) string {
	if len(evs) > 0 {
		return evs[0]
	}
	return ""
}
