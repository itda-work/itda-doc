package classdiag

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/itda-work/itda-doc/diagram"
)

// 원문을 한 번 훑는다 — 줄마다 아래 허용 꼴(양성 목록) 하나에 통째로 맞아야 그리고, 어느 꼴에도 맞지 않으면 그 자리에서
// *Error(줄 번호)로 멈춘다(원문 길이에 선형). 허용 꼴은 mermaid@12.2.0 classDiagram.jison 이 같은 뜻으로 읽는다고 이 판이
// 확신할 수 있는 좁은 꼴만 모은 것이다(#208 「허용 목록 문법」). 원본이 그리는 구석 꼴(한 겹 밖의 이름 제네릭·namespace·note·
// 클래스 라벨 ["…"]·lollipop ()·한 줄 여러 문장 …)은 원문 + 줄 번호 경고로 간다(의도된 축소). 원본 lexer 는 대소문자를 가린다
// (%options case-insensitive 가 없다) — 지시자·키워드는 원문 그대로의 대소문자로 맞춘다.
//
//	한 줄에 문장 하나(; 없음). 줄 머리 %% 는 주석(원본 전처리 cleanupComments — 클래스 몸 안에서도), 뿌리의 글 없는 맨 %% 도
//	          주석(원본 lexer \%\%(?!\{)*[^\n]*(\r?\n?)+ — jison:28). 줄 중간의 %% 는 받지 않는다
//	이름      ASCII [A-Za-z0-9_] 와 UNICODE_TEXT 글자 — 토큰 머리(이름 머리·ASCII 밖 글자 뒤)가 키워드 + 낱말 경계면 뺀다(nameOK)
//	          뒤에 제네릭 한 겹 ~인자~ 를 붙일 수 있다(className — 인자는 이름 글자 낱말의 쉼표 목록, 클래스는 ~ 앞 이름으로 같다)
//	머리 줄   classDiagram · classDiagram-v2(원본 detector /^\s*classDiagram/ — 대소문자 구분)
//	클래스    class 이름 · class 이름 { … } · class 이름 {} · class 이름 { 멤버 } · class 이름 <<주석>> · class 이름:::css(무시)
//	주석      <<주석>> 이름(이미 있는 클래스) · 몸 안 <<주석>> — 주석 이름은 ASCII 한 낱말(원본 alphaNumToken ALPHA)
//	멤버      이름 : 멤버 · 몸 안의 줄 하나 — 가시성 + - # ~, 메서드 이름(인자) [반환형], 분류자 $ * (member),
//	          짝이 맞는 제네릭 ~…~ → <…>(genericText — 원본 parseGenericTypes 와 값이 같은 꼴만)
//	관계      이름 ["수"] [모양]--|..[모양] ["수"] 이름 [: 라벨] — 모양 <| |> * o < > (relRe)
//	방향      direction TB|BT|LR|RL — 뿌리에서만, 이 낱말이 든 다른 줄은 받지 않는다(jison:24–27)
//	접근성    accTitle: 값 · accDescr: 값(줄 끝까지) · accDescr { … } (jison:30–36)
//	무시      %%{…}%%(한 줄) · frontmatter 의 title 밖 키 · classDef · cssClass · style · ::: · click · link · callback
//
// 원본 전처리(encodeEntities — 엔티티 `#…;` 를 내부 표식으로 지켜 준다)는 본문 줄마다 먼저 지나간다(공통 internal/diagram).
// 표식은 정책 B(diagram.RestoreEntities — Label·AccSet 안)로만 한 번 되돌린다.

// Parse 는 mermaid classDiagram 원문을 읽는다. 그릴 수 없는 문법이면 *Error 다. issues 는 쓰지 않은 줄이다.
func Parse(src string) (*chart, []Issue, error) {
	p := &parser{c: &chart{byName: map[string]int{}}, said: map[string]bool{}, body: -1}
	// 잘못된 UTF-8 은 읽기 전에 U+FFFD 로. XML 이 허용하지 않는 글자는 출력 관문(gate)이 막는다.
	if err := p.parse(strings.ToValidUTF8(src, "�")); err != nil {
		return nil, nil, err
	}
	// 출력 관문 — 본문을 다 읽은 뒤, 내보낼 사용자 글자를 모두 한 번에 본다(본문 오류가 먼저다).
	if err := diagram.Gate(p.c.eachText); err != nil {
		return nil, nil, err
	}
	return p.c, p.issues, nil
}

type parser struct {
	c        *chart
	issues   []Issue
	line     int  // 지금 읽는 줄(1부터)
	header   bool // 머리 줄(classDiagram)을 지났다
	said     map[string]bool
	inAcc    bool // accDescr { … } 안
	accLine  int
	acc      diagram.AccBlock
	raw      string // 지금 줄의 원문(엔티티 전처리 뒤, 앞뒤 빈칸 그대로) — accDescr { 여는 줄의 값
	body     int    // 열린 클래스 몸(chart.Classes 의 차례), 없으면 -1
	bodyLine int
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
	for i := start; i < len(lines); i++ {
		p.line = i + 1
		raw := strings.TrimSuffix(lines[i], "\r") // 원본 전처리가 CRLF 를 LF 로 바꾼다
		// 줄 중간의 CR(원본 전처리가 줄바꿈으로 바꾼다)·FF·VT(원본 lexer 의 \s 가 줄바꿈 없이 건너뛴다)는 받지 않는다.
		if strings.ContainsAny(raw, "\r\f\v") {
			return p.fail("줄 안에 CR·FF·VT 글자가 있음 — 이 판은 그 줄을 그리지 않는다")
		}
		// ASCII 밖 공백·서식 문자는 줄째 그리지 않는다(원본 JS \s 와 Go 의 공백 판정이 갈리는 갈래 — er·state 와 같다).
		if r, ok := diagram.OddSpace(raw); ok {
			return p.fail("ASCII 밖 공백·서식 문자(U+%04X)가 있음 — 이 판은 그 줄을 그리지 않는다(일반 빈칸으로 바꿔 쓴다)", r)
		}
		t := strings.Trim(raw, " \t")
		if diagram.HasMarker(t) {
			return p.fail("본문 줄에 Mermaid 내부 표식 문자(ﬂ°·¶ß)가 있음 — 이 판은 그 줄을 그리지 않는다(엔티티 표기 #…; 는 그대로 쓸 수 있다)")
		}
		// 설정 지시 — 줄 전체가 지시 하나인 줄(뿌리)만 무시하는 지시다. 그 밖에 %%{ 가 든 줄(지시 앞뒤·사이의 글, acc 값·
		// 라벨·주석 안, 클래스 몸·accDescr 블록 안)은 원본 removeDirectives 가 지시만 지우고 남은 글을 읽는 갈래라 그리지 않는다.
		if msg := diagram.DirectiveProblem(t, p.inAcc || p.body >= 0); msg != "" {
			return p.fail("%s", msg)
		}
		// 줄 머리 %% — 원본 전처리 cleanupComments(/^\s*%%(?!{)[^\n]+\n?/gm)가 어느 상태에서든 지운다. 뒤에 글이 없는 맨 %% 는
		// 지우지 않는다 — 뿌리에서는 원본 lexer 의 주석 규칙(jison:28)이 건너뛰지만, accDescr { … } 안은 설명 글(jison:36
		// [^\}]*)이 되고 클래스 몸 안은 멤버(jison:87 [^{}\n]*)가 된다 — 이 판은 그 둘을 그리지 않는다.
		if strings.HasPrefix(t, "%%") && !strings.HasPrefix(t, "%%{") {
			if bare := strings.TrimLeft(raw, " \t") == "%%"; bare && p.inAcc {
				return p.fail("accDescr { … } 안의 글 없는 %s 줄 — 원본은 설명 글로 읽는다(이 판은 그리지 않는다)", "%%")
			} else if bare && p.body >= 0 {
				return p.fail("클래스 몸 안의 글 없는 %s 줄 — 원본은 멤버로 읽는다(이 판은 그리지 않는다)", "%%")
			}
			continue
		}
		if p.inAcc { // 블록 글은 줄 끝 빈칸까지 값이다(accBody)
			if err := p.accBody(diagram.EncodeEntities(strings.TrimLeft(raw, " \t"))); err != nil {
				return err
			}
			continue
		}
		if t == "" {
			continue
		}
		p.raw = diagram.EncodeEntities(raw)
		if err := p.statement(diagram.EncodeEntities(t)); err != nil {
			return err
		}
	}
	if p.inAcc {
		return &Error{Line: p.accLine, Msg: "accDescr { 가 닫히지 않음"}
	}
	if p.body >= 0 {
		return &Error{Line: p.bodyLine, Msg: fmt.Sprintf("클래스 %s 의 { 가 } 로 닫히지 않음", diagram.Quote(diagram.Clip(p.c.Classes[p.body].Name)))}
	}
	if !p.header {
		p.line = 0
		return p.fail("mermaid 그림이 비어 있음")
	}
	if len(p.c.Classes) == 0 {
		p.line = 0
		return p.fail("그릴 클래스가 없음")
	}
	return nil
}

// 허용 꼴. 원본 lexer 는 대소문자를 가린다 — 지시자는 원문 그대로 맞춘다. 이름은 넓게 잡은 뒤 nameOK 가 원본 토큰 규칙으로
// 다시 본다(정규식 안의 이름 꼴은 「이름이 될 수 있는 글자」의 겉 테두리일 뿐이다).
const (
	namePat = `[A-Za-z0-9_\p{L}]+`
	// gnamePat 은 이름 자리의 글이다 — 이름 꼴 뒤에 제네릭 ~…~ 하나가 붙을 수 있다(원본 className: alphaNumToken GENERICTYPE —
	// lexer 의 generic 상태 [^~]* 는 ~ 밖 모든 글자). 안쪽 글은 className 이 다시 본다(genericArg).
	gnamePat = namePat + `(?:~[^~]*~)?`
	endPat   = `<\||\|>|\*|o|<|>`
	wordPat  = `[A-Za-z0-9_]+` // 원본 ALPHA \w+ 한 토큰(주석 이름·classDef·cssClass·style 의 이름)
	// stylePat 은 쓰지 않는 classDef·style 의 값이다 — 속성:값 을 쉼표로 이은 것(속성은 영문, 값은 영문·숫자·#·%·.). 첫 속성만
	// 원본 ALPHA 토큰이고 나머지는 LABEL(":"[^:\n;]+) 안이다. -(MINUS)는 원본 styleComponent 에 없어 원본도 오류다.
	stylePat = `[A-Za-z]+:[A-Za-z0-9#%.]+(?:,[A-Za-z]+:[A-Za-z0-9#%.]+)*`
)

var (
	reDir      = regexp.MustCompile(`^direction[ \t]+(TB|BT|RL|LR)$`)
	reAccTitle = regexp.MustCompile(`^accTitle[ \t]*:[ \t]*(.*)$`)
	reAccDescr = regexp.MustCompile(`^accDescr[ \t]*:[ \t]*(.*)$`)
	reAccBlock = regexp.MustCompile(`^accDescr[ \t]*\{(.*)$`)
	reClassDef = regexp.MustCompile(`^classDef[ \t]+(` + wordPat + `(?:,` + wordPat + `)*)[ \t]+` + stylePat + `$`)
	reStyle    = regexp.MustCompile(`^style[ \t]+(` + wordPat + `)[ \t]+` + stylePat + `$`)
	reCSSClass = regexp.MustCompile(`^cssClass[ \t]+"(` + namePat + `(?:,` + namePat + `)*)"[ \t]*(` + wordPat + `)$`)
	// 따옴표 글은 한 글자 이상이다 — 원본 lexer 의 string 상태는 규칙 차례가 ["](닫기, 토큰 없음) 다음 [^"]*(STR)라(jison:60–62)
	// "" 는 STR 토큰을 내지 않는다. 필수 STR 자리가 "" 뿐인 `link A ""`·`callback A ""`·`click A href ""`(jison:392·394·402 — 앞 STR
	// 없이 끝나는 꼴이 문법에 없다)는 원본 문법 오류다. 툴팁 자리의 "" 는 토큰 없이 사라져 원본이 다른 유효한 꼴로 읽을 수 있다 —
	// 이 판은 툴팁 꼴을 받지 않으므로 그것도 원문이지만 원본 오류는 아니다(unknown 의 안내가 둘을 가른다).
	reClickHref = regexp.MustCompile(`^click[ \t]+(` + namePat + `)[ \t]+href[ \t]+"[^"]+"$`)
	reClickCall = regexp.MustCompile(`^click[ \t]+(` + namePat + `)[ \t]+call[ \t]+[A-Za-z_][A-Za-z0-9_]*\(\)$`)
	reLink      = regexp.MustCompile(`^(?:link|callback)[ \t]+(` + namePat + `)[ \t]+"[^"]+"$`)
	reClass     = regexp.MustCompile(`^class[ \t]+(` + gnamePat + `)(?:[ \t]*(\{)(?:[ \t]*(\}))?|[ \t]*<<(` + wordPat + `)>>|[ \t]*:::[ \t]*(` + wordPat + `))?$`)
	reClassLine = regexp.MustCompile(`^class[ \t]+(` + gnamePat + `)[ \t]*\{(.*)\}$`)
	reAnnot     = regexp.MustCompile(`^<<(` + wordPat + `)>>[ \t]*(` + gnamePat + `)$`)
	// reRel 은 관계다 — 묶음: 1 왼 이름, 2 왼 수량, 3 왼 모양, 4 선, 5 오른 모양, 6 오른 수량, 7 오른 이름, 8 라벨(콜론 뒤).
	reRel = regexp.MustCompile(`^(` + gnamePat + `)[ \t]*(?:"([^"]*)"[ \t]*)?(` + endPat + `)?(--|\.\.)(` + endPat + `)?[ \t]*(?:"([^"]*)"[ \t]*)?(` +
		gnamePat + `)(?:[ \t]*:(.*))?$`)
	reMember = regexp.MustCompile(`^(` + gnamePat + `)[ \t]*:(.*)$`)
)

// sub 는 FindStringSubmatchIndex 결과 ix 의 묶음 k 글(없으면 "", false)이다.
func sub(t string, ix []int, k int) (string, bool) {
	if ix[2*k] < 0 {
		return "", false
	}
	return t[ix[2*k]:ix[2*k+1]], true
}

// statement 는 클래스 몸 밖·안의 줄 하나(앞뒤 공백을 지우고 엔티티 전처리를 지난 것)를 허용 꼴에 맞춰 읽는다.
func (p *parser) statement(t string) error {
	if strings.HasPrefix(t, "%%{") { // 설정 지시 — 줄 전체가 지시 하나인 것만 여기 온다(parse 의 DirectiveProblem)
		p.note("init", "설정 지시(%%{…}%%)는 쓰지 않음 — 색·글꼴은 테마가 정한다")
		return nil
	}
	if !p.header {
		if t != "classDiagram" && t != "classDiagram-v2" {
			return p.fail("mermaid 그림 종류를 알 수 없음 — 첫 줄이 classDiagram·classDiagram-v2 가 아님(%s)", diagram.Quote(diagram.Clip(firstWord(t))))
		}
		p.header = true
		return nil
	}
	hasDir := strings.Contains(diagram.ASCIILower(t), "direction")
	if p.body >= 0 {
		if t == "}" {
			p.body = -1
			return nil
		}
		// 몸 안의 줄은 원본 lexer 가 멤버 한 줄(MEMBER [^{}\n]*)로 읽는다 — 방향 규칙도 쓰이지 않지만, 이 낱말이 든 줄은 뿌리와
		// 같게 받지 않는다(보수적).
		if hasDir {
			return p.fail("클래스 몸 안에 direction 낱말이 든 줄은 쓰지 못 함 — 원본은 멤버로 읽는다(방향은 뿌리에 한 줄로)")
		}
		return p.bodyLineMember(t, p.body)
	}
	// 방향 — 원본 규칙 .*direction\s+(TB|BT|RL|LR)[^\n]* 은 lexer 의 첫 규칙이라(jison:24–27) 줄 어디서든(\s+ 는 줄바꿈도
	// 넘는다) 줄 전체를 방향 문장으로 먹는다. 정확히 `direction TB|BT|LR|RL` 한 줄만 받고, 이 낱말(대소문자 무시)이 든 다른
	// 줄은 받지 않는다. 여럿이면 마지막이 이긴다(classDb setDirection).
	if hasDir {
		m := reDir.FindStringSubmatch(t)
		if m == nil {
			return p.fail("direction 낱말이 든 줄은 `direction TB`·`BT`·`LR`·`RL` 한 줄로만 쓴다 — 원본은 이 낱말이 든 줄을 통째로 방향 문장으로 읽는다(%s)", diagram.Clip(t))
		}
		p.c.Dir = map[string]Dir{"TB": TB, "BT": BT, "LR": LR, "RL": RL}[m[1]]
		return nil
	}
	// 줄 머리가 아닌 %% — 원본 lexer 의 주석 규칙(jison:28)이 그 자리부터 줄 끝과 뒤 줄바꿈까지 먹어 다음 줄이 이 줄에 붙는다.
	if strings.Contains(t, "%%") {
		return p.fail("줄 중간의 %s 는 쓰지 못 함 — 주석은 줄 머리에 둔다(원본은 그 자리부터 줄바꿈까지 지워 다음 줄을 이어 읽는다)", "%%")
	}
	// 접근성 — 값은 줄 끝까지(`;`·`#` 포함 — 원본 acc_title_value (?!\n|;|#)*[^\n]* 의 앞보기는 * 로 0 번 맞아 아무것도 막지 않는다).
	if m := reAccTitle.FindStringSubmatch(t); m != nil {
		return p.accValue(m[1], &p.c.AccTitle, &p.c.AccTitleLine, "accTitle")
	}
	if m := reAccDescr.FindStringSubmatch(t); m != nil {
		return p.accValue(m[1], &p.c.AccDescr, &p.c.AccDescrLine, "accDescr")
	}
	if reAccBlock.MatchString(t) {
		p.acc.Reset() // 블록형 — 선언마다 버퍼를 새로 시작한다(마지막 선언이 이긴다)
		p.inAcc, p.accLine = true, p.line
		// { 뒤의 글은 줄 끝 빈칸까지 값이다 — 앞뒤를 지운 t 가 아니라 원래 줄에서 첫 { 뒤를 자른다(t 의 첫 { 와 같은 자리)
		_, rest, _ := strings.Cut(p.raw, "{")
		return p.accBody(rest)
	}
	// ; — 원본 lexer 에 ; 규칙이 없다(문장 구분이 아니라 lexer 오류). 엔티티의 ; 는 전처리가 이미 표식으로 지켰다.
	if strings.Contains(t, ";") {
		return p.fail("; 는 쓰지 못 함 — 한 줄에 문장 하나(글자 ;은 엔티티 #59;)(%s)", diagram.Clip(t))
	}
	if ix := reClassDef.FindStringSubmatchIndex(t); ix != nil {
		if err := p.wordList(t, ix[2], ix[3]); err != nil {
			return err
		}
		if err := p.styleKey(t, ix[3]); err != nil {
			return err
		}
		p.note("classDef", "classDef 줄은 쓰지 않음 — 색·선은 테마가 정한다")
		return nil
	}
	if ix := reStyle.FindStringSubmatchIndex(t); ix != nil {
		if err := p.wordList(t, ix[2], ix[3]); err != nil {
			return err
		}
		if err := p.styleKey(t, ix[3]); err != nil {
			return err
		}
		p.note("style", "style 줄은 쓰지 않음 — 색·선은 테마가 정한다") // 원본 setCssStyle 은 있는 클래스에만 — 클래스를 만들지 않는다
		return nil
	}
	if ix := reCSSClass.FindStringSubmatchIndex(t); ix != nil {
		ids := t[ix[2]:ix[3]]
		if n := strings.Count(ids, ",") + 1; n > maxNameList {
			return p.fail("이름 목록이 %d 개를 넘음", maxNameList)
		}
		if err := p.wordList(t, ix[4], ix[5]); err != nil {
			return err
		}
		p.note("css", "cssClass·::: 지정은 쓰지 않음 — 색·선은 테마가 정한다") // 원본 setCssClass 는 있는 클래스에만
		return nil
	}
	for _, re := range []*regexp.Regexp{reClickHref, reClickCall, reLink} {
		if ix := re.FindStringSubmatchIndex(t); ix != nil {
			if err := p.nameOK(t, ix[2], ix[3], "클래스 이름"); err != nil {
				return err
			}
			p.note("click", "click·link·callback 줄은 쓰지 않음 — 문서의 그림은 누를 수 없다") // 원본 setLink·setClickEvent 는 있는 클래스에만
			return nil
		}
	}
	if strings.HasPrefix(t, "class") && len(t) > 5 && (t[5] == ' ' || t[5] == '\t') {
		return p.classStmt(t)
	}
	if ix := reAnnot.FindStringSubmatchIndex(t); ix != nil {
		if err := p.annotName(t, ix[2], ix[3]); err != nil {
			return err
		}
		name, _, err := p.className(t, ix[4], ix[5])
		if err != nil {
			return err
		}
		// 원본 addAnnotation 은 splitClassNameAndType 의 이름으로 classes.get(…)! 를 바로 쓴다 — 없는 클래스면 그리기가 멈추고,
		// 있으면 제네릭 인자는 쓰지 않는다(클래스를 만들지 않는다)
		k, ok := p.c.byName[name]
		if !ok {
			return p.fail("주석 %s 을 붙일 클래스 %s 가 아직 없음 — 클래스를 먼저 선언한다", diagram.Quote(t[ix[2]:ix[3]]), diagram.Quote(diagram.Clip(name)))
		}
		return p.annotate(k, t[ix[2]:ix[3]])
	}
	if ix := reRel.FindStringSubmatchIndex(t); ix != nil {
		return p.relation(t, ix)
	}
	if ix := reMember.FindStringSubmatchIndex(t); ix != nil {
		name, typ, err := p.className(t, ix[2], ix[3])
		if err != nil {
			return err
		}
		v := t[ix[4]:ix[5]]
		// 원본 LABEL ":"{1}[^:\n;]+ 은 첫 : 뒤에서 다음 : 앞까지다 — ::·::: 꼴이나 값 안의 : 는 다른 토큰이 되어 원본 문법 오류다.
		if strings.Contains(v, ":") {
			return p.fail("멤버 줄 `이름 : 멤버` 의 멤버에는 : 를 쓰지 못 함 — 원본은 : 에서 끊어 읽는다(:: · ::: 포함, 반환형은 `foo() int`)")
		}
		k, err := p.addClass(name, typ)
		if err != nil {
			return err
		}
		return p.member(k, strings.Trim(v, " \t"), false)
	}
	return p.unknown(t)
}

// reEmptyReqSTR 는 필수 STR 자리가 빈 "" 뿐인 링크·콜백·click href 줄이다(원본 문법 오류 — 위 reClickHref 주석).
var reEmptyReqSTR = regexp.MustCompile(`^(?:(?:link|callback)[ \t]+[^ \t"]+|click[ \t]+[^ \t"]+[ \t]+href)[ \t]+""$`)

// unquoted 는 t 에서 따옴표 구간("…")을 지운 글이다 — 안내 갈래를 고를 때 따옴표 안 글자(링크 주소의 ~·:::·. 따위)를 보지
// 않게 한다. 닫히지 않은 따옴표는 그 뒤를 모두 지운다.
func unquoted(t string) string {
	var b strings.Builder
	in := false
	for i := 0; i < len(t); i++ {
		if t[i] == '"' {
			in = !in
			continue
		}
		if !in {
			b.WriteByte(t[i])
		}
	}
	return b.String()
}

// hasEmptyQuote 는 t 의 따옴표 짝(여는 " 와 다음 ")에 안 글이 빈 것이 있는지다 — `"x""y"` 는 두 짝 모두 글이 있어 거짓이다.
func hasEmptyQuote(t string) bool {
	open := -1
	for i := 0; i < len(t); i++ {
		if t[i] != '"' {
			continue
		}
		if open < 0 {
			open = i
			continue
		}
		if i == open+1 {
			return true
		}
		open = -1
	}
	return false
}

// unknown 은 허용 꼴 어디에도 맞지 않은 줄의 *Error 다 — 까닭을 짐작할 수 있는 것은 그 안내를 붙인다. 안내가 원본 동작을
// 말하는 갈래는 그 갈래에 오는 입력 전부에 그 말이 맞는 것만 둔다(나머지는 「이 판이 받지 않는다」로만 말한다).
func (p *parser) unknown(t string) error {
	head := firstWord(t)
	u := unquoted(t)
	switch {
	case head == "namespace" || strings.HasPrefix(t, "namespace{"):
		return p.fail("namespace 는 이 판이 그리지 않는다")
	case head == "note" || strings.HasPrefix(t, "note "):
		return p.fail("메모(note)는 이 판이 그리지 않는다")
	case strings.Contains(u, "~"):
		// 클래스 이름 자리의 ~인자~ 는 className 이, 멤버 안의 ~ 는 member 가 먼저 본다 — 여기 오는 줄은 그 밖의 자리(click·
		// link·callback·cssClass·style·classDef 의 이름, 이름 뒤에 글이 더 붙은 꼴 따위)에 ~ 가 있다.
		return p.fail("이 자리의 제네릭(~…~)은 이 판이 그리지 않는다 — 클래스 이름 바로 뒤의 한 겹 `이름~T~` 와 멤버 안의 짝이 맞는 ~…~ 만 받는다(%s)", diagram.Clip(t))
	case strings.Contains(t, `["`) || strings.Contains(t, "[\""):
		return p.fail("클래스 라벨([\"…\"])은 이 판이 그리지 않는다(%s)", diagram.Clip(t))
	case strings.Contains(u, "()") && (strings.Contains(u, "--") || strings.Contains(u, "..")):
		return p.fail("lollipop 관계(())는 이 판이 그리지 않는다(%s)", diagram.Clip(t))
	case strings.Contains(u, "[*]"):
		return p.fail("[*] 는 이 판이 그리지 않는다")
	case strings.Contains(u, "`"):
		return p.fail("백틱 이름은 이 판이 그리지 않는다(%s)", diagram.Clip(t))
	case strings.HasPrefix(t, `"`):
		return p.fail("따옴표 이름은 이 판이 그리지 않는다(%s)", diagram.Clip(t))
	case head == "classDef" || head == "style" || head == "cssClass":
		return p.fail("classDef·style·cssClass 줄을 확신할 수 없음 — `classDef 이름 fill:#f96`·`style 이름 fill:#f96,stroke:#333`·`cssClass \"이름,이름\" css` 꼴(값은 영문 속성:영문·숫자·#·%%·.)만 받는다(%s)", diagram.Clip(t))
	case reEmptyReqSTR.MatchString(t):
		return p.fail("click·link·callback 줄의 빈 따옴표 \"\" 는 쓰지 못 함 — 원본 lexer 가 빈 따옴표에서 글 토큰(STR)을 내지 않아 주소·함수 자리가 비므로 원본도 문법 오류다(%s)", diagram.Clip(t))
	case (head == "click" || head == "link" || head == "callback") && hasEmptyQuote(t):
		return p.fail("빈 따옴표 \"\" 가 든 click·link·callback 줄은 이 판이 받지 않는다(지원 꼴 밖) — `click 이름 href \"주소\"`·`click 이름 call 함수()`·`link 이름 \"주소\"`·`callback 이름 \"함수\"` 꼴로 쓴다(%s)", diagram.Clip(t))
	case head == "click" || head == "link" || head == "callback":
		return p.fail("click·link·callback 줄은 `click 이름 href \"주소\"`·`click 이름 call 함수()`·`link 이름 \"주소\"`·`callback 이름 \"함수\"` 꼴만 받는다(%s)", diagram.Clip(t))
	case strings.Contains(u, ":::"):
		// 지시 줄(classDef·style·cssClass·click·link·callback)은 위 갈래가 먼저 받는다 — 여기 오는 줄은 class 로 시작하지 않는
		// 문장이다(class 문장은 classStmt). 원본 문법의 ::: 는 classStatement(CLASS className STYLE_SEPARATOR …) 안에만 있다.
		return p.fail("::: 지정은 `class 이름:::css` 꼴만 받는다 — 원본 문법에 `이름:::css` 문장·관계 안의 ::: 가 없다(%s)", diagram.Clip(t))
	case t == "}":
		return p.fail("짝 없는 } — 클래스 몸은 `class 이름 {` 로 열고 } 한 줄로 닫는다")
	case strings.Contains(u, "{") || strings.Contains(u, "}"):
		return p.fail("{ … } 는 클래스 몸(`class 이름 {` 한 줄 … } 한 줄, 또는 `class 이름 { 멤버 }`)으로만 쓴다(%s)", diagram.Clip(t))
	case strings.Contains(u, "--") || strings.Contains(u, ".."):
		return p.fail("관계 꼴이 다름 — `이름 관계 이름`·`이름 관계 이름 : 라벨`·`이름 \"수\" 관계 \"수\" 이름`(관계는 --·.. 양쪽에 모양 <|·|>·*·o·<·> 하나씩까지)(%s)", diagram.Clip(t))
	case strings.Contains(u, "."):
		return p.fail(". 이 든 이름은 이 판이 받지 않는다(%s)", diagram.Clip(t))
	}
	return p.fail("알 수 없는 문장(%s) — 13장 지원 표의 꼴로 쓴다", diagram.Clip(t))
}

// keywords 는 원본 lexer 가 이름 토큰(ALPHA \w+ — jison:140)보다 먼저 읽는 규칙 가운데 이름 글자로 시작하는 낱말이다
// (jison:30–117 의 차례, 대소문자 구분). lex-parser 는 낱말 글자로 끝나는 규칙 뒤에 \b 를 붙인다(lex.y:135 — "class" →
// class\b, \s*o → \s*o\b) — 그래서 이 낱말 뒤가 ASCII 낱말 글자(JS \b 의 \w — [A-Za-z0-9_])가 아니면 이 규칙이 이긴다.
// "call"[\s]+ 는 뒤에 공백이 와야 맞지만 이 판은 다른 낱말처럼 낱말 경계로 넓게 막는다(보수적). accTitle·accDescr 는
// \s*":" 가 뒤따라야 맞지만 같게 넓게 막는다.
var keywords = []string{
	"accTitle", "accDescr", // jison:30·32·34
	"classDiagram",                            // jison:41–42("classDiagram-v2" 포함)
	"call",                                    // jison:53
	"style", "classDef", "namespace", "class", // jison:63·64·66·77
	"cssClass", "callback", "link", "click", "note", "href", // jison:89–94·104("note for" 포함)
	"_self", "_blank", "_parent", "_top", // jison:114–117
	"o", // jison:124 — \s*o\b(AGGREGATION)
}

func isWordByte(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// nameOK 는 줄 t 의 t[s:e] 가 원본 lexer 에서 이름 토큰(ALPHA·UNICODE_TEXT)으로만 읽혀 classDb 가 그 글자 그대로 클래스 이름으로
// 쓰는 이름인지 본다(이어진 토큰은 className: alphaNumToken className 으로 빈칸 없이 이어진다 — 원본 값은 글자 그대로).
// UNICODE_TEXT 는 한 글자씩 토큰이라 ASCII 밖 글자 뒤마다 새 토큰이 시작된다 — 그 자리에서 키워드 규칙이 먼저 맞으면(가o·
// 가class) 원본은 이름을 끊는다. 이름 뒤 글자(t[e])가 키워드의 낱말 경계를 정한다.
func (p *parser) nameOK(t string, s, e int, what string) error {
	name := t[s:e]
	if name == "" {
		return p.fail("%s이 비어 있음", what)
	}
	for _, r := range name {
		switch {
		case r < 0x80 && isWordByte(byte(r)):
		case r > 0xffff:
			return p.fail("%s %s 에 BMP 밖 글자(U+%04X)가 있음 — 원본 lexer 가 읽지 못 한다", what, diagram.Quote(diagram.Clip(name)), r)
		case !inUnicodeText(r):
			return p.fail("%s %s 에 원본 lexer 가 이름으로 읽지 않는 글자(U+%04X)가 있음", what, diagram.Quote(diagram.Clip(name)), r)
		}
	}
	if utf8.RuneCountInString(name) > MaxLabel {
		return p.fail("%s %s 가 %d 자를 넘음", what, diagram.Quote(diagram.Clip(name)), MaxLabel)
	}
	if !underscoreOK(name) {
		return p.fail("%s %s 의 _ 가 글자 사이가 아님 — 원본은 이름을 마크다운으로 그려 기울임이 될 수 있다", what, diagram.Quote(diagram.Clip(name)))
	}
	if low := diagram.ASCIILower(name); strings.HasPrefix(low, "acctitle") || strings.HasPrefix(low, "accdescr") {
		return p.fail("%s %s 는 accTitle·accDescr 로 시작해 쓰지 못 함 — 접근성 문장과 헷갈린다", what, diagram.Quote(diagram.Clip(name)))
	}
	for i := s; i < e; i++ {
		if t[i] >= 0x80 || i > s && t[i-1] < 0x80 {
			continue // 토큰 머리는 이름 머리와 ASCII 밖 글자 바로 뒤의 ASCII 글자뿐이다(\w+ 는 ASCII 낱말을 통째로 먹는다)
		}
		for _, k := range keywords {
			if i+len(k) <= e && t[i:i+len(k)] == k && (i+len(k) == len(t) || !isWordByte(t[i+len(k)])) {
				return p.fail("%s %s 는 원본 lexer 가 키워드 %s 로 먼저 읽는다 — 다른 이름으로", what, diagram.Quote(diagram.Clip(name)), diagram.Quote(k))
			}
		}
	}
	return nil
}

// wordList 는 t[s:e] 의 쉼표 목록(ASCII 낱말 — 원본 ALPHA 토큰)마다 키워드 규칙이 먼저 맞지 않는지 본다.
func (p *parser) wordList(t string, s, e int) error {
	if n := strings.Count(t[s:e], ",") + 1; n > maxNameList {
		return p.fail("이름 목록이 %d 개를 넘음", maxNameList)
	}
	i := s
	for i < e {
		j := i
		for j < e && t[j] != ',' {
			j++
		}
		if err := p.nameOK(t, i, j, "이름"); err != nil {
			return err
		}
		i = j + 1
	}
	return nil
}

// styleKey 는 classDef·style 값의 첫 속성(원본 ALPHA 토큰 — 뒤는 LABEL 안이다)이 키워드 규칙에 먼저 맞지 않는지 본다. from 은
// 이름 목록 뒤 자리다.
func (p *parser) styleKey(t string, from int) error {
	i := from
	for i < len(t) && (t[i] == ' ' || t[i] == '\t') {
		i++
	}
	j := i
	for j < len(t) && t[j] != ':' {
		j++
	}
	return p.nameOK(t, i, j, "style 속성")
}

// annotName 은 주석 이름(원본 alphaNumToken — ASCII 한 낱말)을 본다.
func (p *parser) annotName(t string, s, e int) error {
	return p.nameOK(t, s, e, "주석 이름")
}

// underscoreOK 는 _ 가 모두 글자·숫자 사이에 있는지 본다(마크다운 기울임 _…_ 이 될 수 없게). ph 는 엔티티 자리표(글자로 센다).
func underscoreOK(s string) bool {
	r := []rune(s)
	for i, c := range r {
		if c != '_' {
			continue
		}
		if i == 0 || i == len(r)-1 || !isAlnum(r[i-1]) || !isAlnum(r[i+1]) {
			return false
		}
	}
	return true
}

func isAlnum(r rune) bool {
	return r == entPH || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsNumber(r)
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}

// addClass 는 클래스를 찾아 없으면 더한다(원본 addClass — 처음 나온 차례, 있으면 그대로). name 은 ~ 앞 이름, typ 은 제네릭
// 인자다 — 원본 addClass 는 이미 있는 이름이면 아무것도 하지 않으므로 인자는 처음 만든 자리의 것만 남는다.
func (p *parser) addClass(name, typ string) (int, error) {
	if k, ok := p.c.byName[name]; ok {
		return k, nil
	}
	if len(p.c.Classes) >= MaxClasses {
		return 0, p.fail("클래스가 %d 개를 넘음", MaxClasses)
	}
	p.c.byName[name] = len(p.c.Classes)
	p.c.Classes = append(p.c.Classes, &classNode{Name: name, Type: typ, Line: p.line})
	return len(p.c.Classes) - 1, nil
}

// annotate 는 클래스에 주석을 붙인다 — 원본은 주석을 쌓고 첫 주석만 그린다(textHelper annotations[0]). 이 판은 하나만 받는다.
func (p *parser) annotate(k int, a string) error {
	cl := p.c.Classes[k]
	if cl.AnnotLine != 0 {
		return p.fail("클래스 %s 에 주석을 두 번 붙였음 — 원본은 첫 주석만 그린다(이 판은 하나만 받는다)", diagram.Quote(diagram.Clip(cl.Name)))
	}
	cl.Annot, cl.AnnotLine = a, p.line
	return nil
}

// classStmt 는 class 로 시작하는 줄이다 — class 이름 · class 이름 { · class 이름 {} · class 이름 { 멤버 } · class 이름 <<주석>> ·
// class 이름:::css. 원본 lexer 는 class 뒤 class 상태에서 이름·{·<<·::: 를 읽는다(jison:77–81).
func (p *parser) classStmt(t string) error {
	if ix := reClass.FindStringSubmatchIndex(t); ix != nil {
		name, typ, err := p.className(t, ix[2], ix[3])
		if err != nil {
			return err
		}
		_, open := sub(t, ix, 2)
		_, shut := sub(t, ix, 3)
		annot, hasAnnot := sub(t, ix, 4)
		_, hasCSS := sub(t, ix, 5)
		if hasAnnot {
			if err := p.annotName(t, ix[8], ix[9]); err != nil {
				return err
			}
		}
		if hasCSS {
			if err := p.nameOK(t, ix[10], ix[11], "css 이름"); err != nil {
				return err
			}
		}
		k, err := p.addClass(name, typ)
		if err != nil {
			return err
		}
		switch {
		case hasAnnot:
			return p.annotate(k, annot)
		case hasCSS:
			p.note("css", "cssClass·::: 지정은 쓰지 않음 — 색·선은 테마가 정한다")
		case open && !shut:
			p.body, p.bodyLine = k, p.line
		}
		return nil
	}
	if ix := reClassLine.FindStringSubmatchIndex(t); ix != nil {
		// 한 줄 몸 class 이름 { 멤버 } — 원본은 { 뒤에서 } 앞까지를 멤버 하나(MEMBER [^{}\n]*)로 읽는다
		name, typ, err := p.className(t, ix[2], ix[3])
		if err != nil {
			return err
		}
		in := t[ix[4]:ix[5]]
		if strings.ContainsAny(in, "{}") {
			return p.fail("한 줄 몸 `class 이름 { 멤버 }` 안에 { } 를 쓰지 못 함")
		}
		k, err := p.addClass(name, typ)
		if err != nil {
			return err
		}
		return p.bodyLineMember(strings.Trim(in, " \t"), k)
	}
	return p.fail("class 문장은 `class 이름`·`class 이름 {`·`class 이름 <<주석>>`·`class 이름:::css` 꼴만 그린다 — 이름은 한 낱말 + 제네릭 한 겹 `~T~` 까지(라벨 [\"…\"] 은 그리지 않는다)(%s)", diagram.Clip(t))
}

// bodyLineMember 는 클래스 몸의 한 줄(앞뒤 공백을 지운 것)이다 — <<주석>> 이면 주석, 아니면 멤버 하나(원본 addMember).
func (p *parser) bodyLineMember(t string, k int) error {
	if t == "" {
		return nil // 원본 addMember 는 다듬어 빈 멤버를 버린다
	}
	if strings.ContainsAny(t, "{}") {
		return p.fail("클래스 몸 안의 { } 는 쓰지 못 함 — } 는 한 줄에 혼자 둔다(%s)", diagram.Clip(t))
	}
	if strings.Contains(t, ";") {
		return p.fail("클래스 몸 안의 ; 는 쓰지 못 함(글자 ;은 엔티티 #59;)(%s)", diagram.Clip(t))
	}
	if strings.HasPrefix(t, "<<") && strings.HasSuffix(t, ">>") && len(t) >= 4 {
		a := t[2 : len(t)-2]
		// 원본은 몸 안의 <<…>> 를 글 그대로 주석으로 쌓는다 — 이 판은 다른 두 꼴과 같은 ASCII 한 낱말만 받는다
		if !regexp.MustCompile(`^` + wordPat + `$`).MatchString(a) {
			return p.fail("주석 이름은 영문·숫자·_ 한 낱말만 받는다(%s)", diagram.Clip(t))
		}
		if err := p.annotName(t, 2, len(t)-2); err != nil {
			return err
		}
		return p.annotate(k, a)
	}
	return p.member(k, t, true)
}

// entPH 는 멤버 글 안의 엔티티 표식 하나를 대신하는 자리표다(사사로운 쓰임 글자 — 원문에 든 같은 글자는 member 가 먼저 막는다).
// 표식은 원본 멤버 해석(parseMember 의 정규식·indexOf(')'))에서 괄호·$·*·빈칸이 없는 덩어리라 한 글자로 바꿔도 해석이 같다.
const entPH = ''

var reEntMark = regexp.MustCompile(`ﬂ°°?\w+¶ß`)

// member 는 멤버 하나(원본 addMember 가 다듬은 글 — 엔티티 전처리 뒤)를 원본 ClassMember.parseMember 대로 읽어 클래스에 더한다.
// body 는 클래스 몸 안의 줄이라는 뜻이다(이름 : 멤버 꼴은 값에 : 가 없다). 받는 꼴(이 밖은 원문 + 경고):
//
//	속성    [가시성] 글 [분류자]                 — 가시성 + - # ~ 는 첫 글자, 분류자 $ * 는 끝 글자, 괄호 없음
//	메서드  [가시성] 이름(인자)[ 반환형][분류자] — ( ) 한 쌍, 분류자는 ) 바로 뒤 또는 끝 글자(하나만)
//	글자    글자·숫자·_(글자 사이)·빈칸(한 칸씩)·, . - = ? /, 몸 안이면 :, 엔티티 #…;, 제네릭 ~
//	제네릭  이름·인자·반환형 조각마다 짝이 맞는 ~…~ — 원본 parseGenericTypes 와 값이 같은 꼴만 <…> 로(genericText)
//
// 표시 글은 원본 classBox 가 그리는 member.text 의 글자다 — 가시성 + 이름(앞 빈칸 하나는 남는다 — "Preserve one space only")
// + 메서드면 (다듬은 인자) 와 반환형이 있으면 「 : 반환형」. 이름·인자·반환형에는 각각 parseGenericTypes 를 씌운다(원본
// parseMember 의 combinedText). 분류자 글자는 빠지고 정적은 밑줄·추상은 기울임으로 그린다.
func (p *parser) member(k int, s string, body bool) error {
	const what = "멤버"
	cl := p.c.Classes[k]
	if s == "" { // `이름 :` — 원본 LABEL(":"[^:\n;]+)은 : 뒤 글이 한 글자 이상이라 COLON 만 남아 문법 오류다
		return p.fail("%s이 비어 있음", what)
	}
	if len(cl.Attrs)+len(cl.Methods) >= MaxMembers {
		return p.fail("클래스 %s 의 멤버가 %d 개를 넘음", diagram.Quote(diagram.Clip(cl.Name)), MaxMembers)
	}
	if strings.ContainsRune(s, entPH) {
		return p.fail("%s에 U+E000 글자가 있음 — 이 판은 그 줄을 그리지 않는다", what)
	}
	if strings.Contains(s, "\t") || strings.Contains(s, "  ") {
		return p.fail("%s 안에 연속 빈칸이나 탭이 있음 — 이 판은 글 사이 빈칸을 한 칸씩만 그린다", what)
	}
	marks := reEntMark.FindAllString(s, -1)
	r := []rune(reEntMark.ReplaceAllString(s, string(entPH)))
	n := len(r)
	opens, closes, cls := 0, 0, 0
	for i, c := range r {
		switch {
		case c == entPH || unicode.IsLetter(c) || unicode.IsDigit(c) || unicode.IsNumber(c):
		case strings.ContainsRune(" ,.-=?/", c):
		case c == '_':
		case c == ':':
			if !body {
				return p.fail("%s에 : 를 쓰지 못 함", what)
			}
		case c == '(':
			opens++
		case c == ')':
			closes++
		case c == '~': // 첫 글자면 가시성, 그 밖은 제네릭 — 짝은 아래 genericText 가 본다
		case c == '+' || c == '#':
			if i != 0 {
				return p.fail("%s의 %s 는 첫 글자(가시성)로만 쓴다 — 마크다운이 이 글자를 서식으로 그릴 수 있다(글자는 엔티티 #35; 꼴로)", what, diagram.Quote(string(c)))
			}
		case c == '$' || c == '*':
			cls++
		default:
			return p.fail("%s에 %s 를 쓰지 못 함 — 원본은 멤버를 마크다운·HTML 로 그리거나 다른 토큰으로 읽는다(글자는 엔티티 #35; 꼴로)", what, diagram.Quote(string(c)))
		}
	}
	if !underscoreOK(string(r)) {
		return p.fail("%s의 _ 가 글자 사이가 아님 — 원본은 마크다운으로 그려 기울임이 될 수 있다", what)
	}
	var vis rune
	if strings.ContainsRune("+-#~", r[0]) {
		vis = r[0]
	}
	m := member{Line: p.line}
	var disp []rune
	if closes > 0 { // 원본 addMember: indexOf(')') > 0 이면 메서드(첫 글자 ) 는 위에서 막는다 — 아래 짝 검사)
		lp, rp := indexRune(r, '('), indexRune(r, ')')
		if opens != 1 || closes != 1 || lp < 0 || rp < lp {
			return p.fail("메서드는 `이름(인자)` 괄호 한 쌍만 쓴다(%s)", diagram.Clip(s))
		}
		start := 0
		if vis != 0 {
			start = 1
		}
		name := r[start:lp]
		if strings.Trim(string(name), " ") == "" {
			return p.fail("메서드 이름이 비어 있음(%s)", diagram.Clip(s))
		}
		params := strings.Trim(string(r[lp+1:rp]), " ")
		rest := r[rp+1:]
		// 원본 정규식 ([#+~-])?(.+)\((.*)\)([\s$*])?(.*)([$*])? 의 ) 뒤 — 받는 꼴은 ""·$·*·" 반환형"·" 반환형$|*" 뿐이다.
		ret, c := "", Plain
		switch {
		case len(rest) == 0:
		case len(rest) == 1 && (rest[0] == '$' || rest[0] == '*'):
			c = classifierOf(rest[0])
		case rest[0] == ' ' && len(rest) > 1:
			rt := rest[1:]
			if last := rt[len(rt)-1]; last == '$' || last == '*' {
				c, rt = classifierOf(last), rt[:len(rt)-1]
			}
			ret = string(rt)
			switch {
			case ret == "" || ret[0] == ' ' || ret[len(ret)-1] == ' ':
				return p.fail("메서드 반환형의 꼴이 다름 — `이름(인자) 반환형`(분류자 $·* 는 반환형 바로 뒤)(%s)", diagram.Clip(s))
			case ret[0] == ':':
				return p.fail("메서드 반환형 앞에 : 를 쓰지 못 함 — 원본은 「 : : 반환형」으로 그린다(`foo() int` 꼴로)(%s)", diagram.Clip(s))
			case strings.ContainsAny(ret, "$*"):
				return p.fail("분류자 $·* 는 하나만, ) 바로 뒤나 끝에 쓴다(%s)", diagram.Clip(s))
			}
		default:
			return p.fail("메서드의 ) 뒤는 빈칸 + 반환형이나 분류자 $·* 만 쓴다(%s)", diagram.Clip(s))
		}
		if strings.ContainsAny(string(r[:rp]), "$*") {
			return p.fail("분류자 $·* 는 ) 바로 뒤나 끝에만 쓴다(%s)", diagram.Clip(s))
		}
		m.Method, m.Cls = true, c
		gn, ok1 := genericText([]rune(keepOneSpace(string(name))))
		gp, ok2 := genericText([]rune(params))
		gr, ok3 := genericText([]rune(ret))
		if !ok1 || !ok2 || !ok3 {
			return p.fail("%s", genericMsg(s))
		}
		disp = append(disp, r[:start]...)
		disp = append(disp, gn...)
		disp = append(disp, '(')
		disp = append(disp, gp...)
		disp = append(disp, ')')
		if ret != "" {
			disp = append(disp, []rune(" : ")...)
			disp = append(disp, gr...)
		}
	} else {
		if opens > 0 {
			return p.fail("속성에 ( 를 쓰지 못 함 — 메서드는 `이름(인자)` 꼴(%s)", diagram.Clip(s))
		}
		start, end := 0, n
		if vis != 0 {
			start = 1
		}
		if last := r[n-1]; n > start && (last == '$' || last == '*') {
			m.Cls, end = classifierOf(last), n-1
		}
		if cls > 1 || cls == 1 && m.Cls == Plain {
			return p.fail("분류자 $·* 는 속성의 끝 글자 하나로만 쓴다(%s)", diagram.Clip(s))
		}
		id := string(r[start:max(start, end)])
		if strings.Trim(id, " ") == "" {
			return p.fail("속성 이름이 비어 있음(%s)", diagram.Clip(s))
		}
		gi, ok := genericText([]rune(keepOneSpace(id)))
		if !ok {
			return p.fail("%s", genericMsg(s))
		}
		disp = append(disp, r[:start]...)
		disp = append(disp, gi...)
	}
	if cls > 1 {
		return p.fail("분류자 $·* 는 하나만 쓴다(%s)", diagram.Clip(s))
	}
	// 가시성이 없으면 표시 글의 머리가 마크다운 서식이 될 수 있다(원본은 가시성 앞에만 \ 를 붙여 지킨다)
	if vis == 0 && (r[0] == '=' || reMDStart.MatchString(string(r))) {
		return p.fail("%s이 = 나 「수.」로 시작함 — 원본은 마크다운 목록·구분선으로 그린다", what)
	}
	m.Vis = vis
	// 자리표를 표식으로 되돌린 뒤 정책 B(diagram.Label — 엔티티 되돌림·제어 문자·XML·200 자)로 읽는다
	out := string(disp)
	for _, mk := range marks {
		out = strings.Replace(out, string(entPH), mk, 1)
	}
	v, msg := diagram.Label(out, what, false)
	if msg != "" {
		return p.fail("%s", msg)
	}
	m.Text = v
	if m.Method {
		cl.Methods = append(cl.Methods, m)
	} else {
		cl.Attrs = append(cl.Attrs, m)
	}
	return nil
}

// genericMsg 는 멤버의 제네릭 ~ 꼴을 받지 않을 때의 안내다.
func genericMsg(s string) string {
	return "멤버의 제네릭 ~…~ 꼴을 받지 않음 — 이름 글자 바로 뒤에서 ~ 로 열고, 닫는 ~ 뒤는 빈칸·쉼표·~·끝이며, 짝이 맞고 원본 " +
		"parseGenericTypes 가 같은 짝으로 읽는 꼴만 그린다(`List~int~`·`Map~K, V~`·`Array~Array~T~~`)(" + diagram.Clip(s) + ")"
}

func indexRune(r []rune, c rune) int {
	for i, x := range r {
		if x == c {
			return i
		}
	}
	return -1
}

func classifierOf(r rune) Classifier {
	if r == '$' {
		return Static
	}
	return Abstract
}

// keepOneSpace 는 원본 parseMember 의 「Preserve one space only」다 — id.startsWith(' ') ? ' ' + id.trim() : id.trim().
func keepOneSpace(id string) string {
	if strings.HasPrefix(id, " ") {
		return " " + strings.Trim(id, " ")
	}
	return strings.Trim(id, " ")
}

var reMDStart = regexp.MustCompile(`^\d{1,9}[.)](?: |$)`)

// labelOK 는 관계 라벨·양끝 수량 글(엔티티 전처리 뒤, 앞뒤 공백을 지운 것)을 본다. 관계 라벨은 원본이 마크다운으로 그린다
// (getData labelType markdown) — 마크다운·HTML 로 그려질 글자, lexer 가 다른 토큰으로 읽는 글자(#·;·"·`·%·\·<·>·&·*·~·$·[·]·{·}·:)를
// 받지 않는다. 수량 글(createLabel markdown: false)은 * 를 글자로 받는다.
func (p *parser) labelOK(s, what string, card bool) error {
	if s == "" {
		return p.fail("%s이 비어 있음", what)
	}
	bad := "#;\"`%\\<>&*~$[]{}:|"
	if card {
		bad = "#;\"`%\\<>&~$[]{}:|"
	}
	for _, r := range s {
		if strings.ContainsRune(bad, r) {
			return p.fail("%s에 %s 를 쓰지 못 함 — 원본은 라벨을 마크다운·HTML 로 그리거나 다른 토큰으로 읽는다(글자는 엔티티 #35; 꼴로)", what, diagram.Quote(string(r)))
		}
	}
	if !card {
		if !underscoreOK(s) {
			return p.fail("%s의 _ 가 글자 사이가 아님 — 원본은 마크다운으로 그려 기울임이 될 수 있다", what)
		}
		if strings.ContainsAny(s[:1], "-+=") || reMDStart.MatchString(s) {
			return p.fail("%s이 -·+·= 나 「수.」로 시작함 — 원본은 마크다운 목록·구분선으로 그린다", what)
		}
	}
	return nil
}

// label 은 관계 라벨·수량 글 하나를 보고 정책 B(diagram.Label — <br> 은 줄바꿈으로 읽지 않는다: < 를 받지 않는다)로 푼다.
func (p *parser) label(s, what string, card bool) (string, error) {
	s = strings.Trim(s, " \t")
	if err := p.labelOK(s, what, card); err != nil {
		return "", err
	}
	v, msg := diagram.Label(s, what, false)
	if msg != "" {
		return "", p.fail("%s", msg)
	}
	if card {
		if utf8.RuneCountInString(v) > MaxCard {
			return "", p.fail("%s이 %d 자를 넘음", what, MaxCard)
		}
		if v == "none" { // 원본 getData 는 relationTitle 이 'none' 이면 수량 글이 없다고 본다
			return "", p.fail("%s none 은 원본이 「수량 없음」으로 읽는다 — 다른 글로", what)
		}
	}
	return v, nil
}

var endOf = map[string]End{"<|": Extension, "|>": Extension, "*": Composition, "o": Aggregation, "<": Dependency, ">": Dependency}

// relation 은 관계(reRel 의 짝)를 읽는다. 원본 addRelation 은 왼 클래스·오른 클래스 차례로 addClass 한 뒤 양끝을
// splitClassNameAndType 의 이름으로 바꾼다 — `A~T~ --> B` 의 왼끝은 클래스 A 다.
func (p *parser) relation(t string, ix []int) error {
	n1, ty1, err := p.className(t, ix[2], ix[3])
	if err != nil {
		return err
	}
	n2, ty2, err := p.className(t, ix[14], ix[15])
	if err != nil {
		return err
	}
	l1, _ := sub(t, ix, 3)
	line, _ := sub(t, ix, 4)
	l2, _ := sub(t, ix, 5)
	// 오른 모양 o 바로 뒤가 낱말 글자면 원본은 o 를 모양(\s*o\b)이 아니라 이름 머리로 읽는다(--oB → -- 와 oB)
	if l2 == "o" && ix[11] < len(t) && isWordByte(t[ix[11]]) {
		return p.fail("관계 모양 o 바로 뒤에 이름을 붙여 쓰지 못 함 — 원본은 o 를 이름 머리로 읽는다(빈칸을 둔다)(%s)", diagram.Clip(t))
	}
	r := relation{Type1: endOf[l1], Type2: endOf[l2], Dotted: line == "..", Line: p.line}
	if v, ok := sub(t, ix, 2); ok {
		s, err := p.label(v, "왼쪽 수량 글", true)
		if err != nil {
			return err
		}
		r.Card1 = s
	}
	if v, ok := sub(t, ix, 6); ok {
		s, err := p.label(v, "오른쪽 수량 글", true)
		if err != nil {
			return err
		}
		r.Card2 = s
	}
	if v, ok := sub(t, ix, 8); ok {
		if strings.HasPrefix(v, ":") || strings.Contains(v, ":") {
			return p.fail("관계 라벨에 : 를 쓰지 못 함 — 원본 LABEL(\":\"[^:\\n;]+)은 : 에서 끊는다(::·::: 포함)")
		}
		s, err := p.label(v, "관계 라벨", false)
		if err != nil {
			return err
		}
		r.Label = s
	}
	if len(p.c.Rels) >= MaxRelations {
		return p.fail("관계가 %d 개를 넘음", MaxRelations)
	}
	a, err := p.addClass(n1, ty1)
	if err != nil {
		return err
	}
	b, err := p.addClass(n2, ty2)
	if err != nil {
		return err
	}
	if a == b && (r.Card1 != "" || r.Card2 != "") {
		return p.fail("자기 관계에 양끝 수량 글을 붙이지 못 함 — 이 판은 고리의 끝에 수량 글을 두지 않는다")
	}
	r.From, r.To = a, b
	p.c.Rels = append(p.c.Rels, r)
	return nil
}

// eachText 는 모델에서 SVG·Result 로 나가는 사용자 글자를 하나씩 f 에 넘긴다(field 는 「형.필드」, what 은 경고 문구의
// 이름, line 은 그 값이 나온 줄). **출력 글자 필드 목록은 여기 한곳이다** — 모델에 글 필드를 더하면 여기에도 더한다
// (TestGateCoversAllText 가 모델의 string·[]string 필드와 견준다).
func (c *chart) eachText(f diagram.TextFunc) error {
	for _, x := range []struct {
		field, what, v string
		line           int
	}{
		{"chart.FMCaption", "frontmatter title", c.FMCaption, c.FMLine},
		{"chart.AccTitle", "accTitle", c.AccTitle, c.AccTitleLine},
		{"chart.AccDescr", "accDescr", c.AccDescr, c.AccDescrLine},
	} {
		if err := f(x.field, x.what, x.v, x.line); err != nil {
			return err
		}
	}
	for _, cl := range c.Classes {
		if err := f("classNode.Name", "클래스 이름", cl.Name, cl.Line); err != nil {
			return err
		}
		if err := f("classNode.Type", "제네릭 인자", cl.Type, cl.Line); err != nil {
			return err
		}
		if err := f("classNode.Annot", "주석 이름", cl.Annot, cl.AnnotLine); err != nil {
			return err
		}
		for _, ms := range [][]member{cl.Attrs, cl.Methods} {
			for _, m := range ms {
				if err := f("member.Text", "멤버", m.Text, m.Line); err != nil {
					return err
				}
			}
		}
	}
	for _, r := range c.Rels {
		for _, x := range []struct{ field, what, v string }{
			{"relation.Label", "관계 라벨", r.Label}, {"relation.Card1", "왼쪽 수량 글", r.Card1}, {"relation.Card2", "오른쪽 수량 글", r.Card2},
		} {
			if err := f(x.field, x.what, x.v, r.Line); err != nil {
				return err
			}
		}
	}
	return nil
}

// accValue 는 accTitle:·accDescr: 의 값을 읽는다(공통 diagram.AccValue — 원본 $2.trim(), 여섯 종류와 같다).
func (p *parser) accValue(v string, dst *string, line *int, kw string) error {
	s, msg := diagram.AccValue(v, kw)
	if msg != "" {
		return p.fail("%s", msg)
	}
	*dst, *line = s, p.line
	return nil
}

// accBody 는 accDescr { … } 의 한 줄을 모은다(공통 diagram.AccBlock — 원본 acc_descr_multiline [^\}]* 와 $1.trim()·
// setAccDescription 의 \n\s+ → \n, 닫는 } 는 줄 끝에만, 빈 블록은 앞 설명을 그대로 둔다).
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
	s, msg := diagram.AccSet(v, "accDescr")
	if msg != "" {
		return p.fail("%s", msg)
	}
	p.c.AccDescr, p.c.AccDescrLine = s, p.line
	return nil
}
