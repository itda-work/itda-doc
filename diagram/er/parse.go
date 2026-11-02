package er

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/itda-work/itda-doc/diagram"
)

// 원문을 한 번 훑는다 — 줄마다 아래 허용 꼴(양성 목록) 하나에 통째로 맞아야 그리고, 어느 꼴에도 맞지 않으면 그 자리에서
// *Error(줄 번호)로 멈춘다(원문 길이에 선형). 허용 꼴은 mermaid@12.2.0 erDiagram.jison 이 같은 뜻으로 읽는다고 이 판이 확신할
// 수 있는 좁은 꼴만 모은 것이다(#208 「허용 목록 문법」 — sequence ③ 과 같은 결정). 원본이 그리는 구석 꼴(낱말 기수
// `one or more`·따옴표 엔티티 이름·한 줄 여러 문장·한 줄 블록 …)은 원문 + 줄 번호 경고로 간다(의도된 축소).
//
//	한 줄에 문장 하나. 줄 머리 %% 는 주석(원본 전처리 cleanupComments 가 지운다 — 블록 안에서도)
//	ID        [\p{L}_][\p{L}\p{N}_-]* — 키워드로 시작하는 이름·BMP 밖 글자·끝 -·-- 를 뺀다(idOK)
//	머리 줄   erDiagram(대소문자 구분 — 원본 detector /^\s*erDiagram/)
//	엔티티    ID · ID[별칭] · 둘 + ` {`(줄 끝에서 블록 열기) · ID {} · 모두 + :::이름[,이름…](무시)
//	속성      타입[?] 이름[ 키[, 키…]][ "설명"] — 블록 안 한 줄에 하나, 블록 닫기는 } 한 줄(jison:34–44·279–307)
//	관계      ID 왼기호선오른기호 ID : 역할 — 왼 |o || }o }|, 선 -- .., 오른 o| || o{ |{ (jison:62–90·309–328)
//	방향      direction TB|BT|LR|RL(jison:19–22 — 이 낱말이 든 다른 줄은 받지 않는다)
//	접근성    accTitle: 값 · accDescr: 값(줄 끝까지) · accDescr { … }(jison:12–18)
//	무시      %%{…}%%(한 줄) · frontmatter 의 title 밖 키 · style · classDef · class · :::
//	라벨      별칭·역할·설명은 "…" 또는 ID 꼴 — #·;·%·"·`·\·<·>·&·*·~·$·제어 문자(C0·DEL) 없음, _ 는 글자 사이에만,
//	          -·+·= 나 「수.」로 시작하지 않음(원본은 라벨을 마크다운으로 그린다 — createText markdown=true), 200 자
//
// 원본 전처리(encodeEntities — 엔티티 `#…;` 를 내부 표식으로 지켜 준다)는 본문 줄마다 먼저 지나간다(공통 internal/diagram).

// Parse 는 mermaid erDiagram 원문을 읽는다. 그릴 수 없는 문법이면 *Error 다. issues 는 쓰지 않은 줄이다.
func Parse(src string) (*chart, []Issue, error) {
	p := &parser{c: &chart{byID: map[string]int{}}, said: map[string]bool{}}
	// 잘못된 UTF-8 은 읽기 전에 U+FFFD 로. XML 이 허용하지 않는 글자는 SVG 를 쓰는 경계(diagram.Esc)에서 거른다.
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
	c       *chart
	issues  []Issue
	line    int  // 지금 읽는 줄(1부터)
	header  bool // 머리 줄(erDiagram)을 지났다
	said    map[string]bool
	inAcc   bool // accDescr { … } 안
	accLine int
	acc     diagram.AccBlock
	raw     string     // 지금 줄의 원문(엔티티 전처리 뒤, 앞뒤 빈칸 그대로) — accDescr { 여는 줄의 값
	block   *openBlock // 열린 속성 블록(없으면 nil)
	nAttrs  int        // 속성 합계
}

// openBlock 은 열린 속성 블록이다. 원본은 블록이 닫힐 때(문장을 줄일 때) addEntity·addAttributes 를 부른다 — 블록 안에는
// 다른 문장이 올 수 없어 여는 자리에서 엔티티를 더해도 차례가 같다. 속성은 닫을 때 원문 차례로 붙인다(원본 attributes
// 재귀가 거꾸로 쌓고 addAttributes 가 뒤에서부터 넣어 결국 원문 차례).
type openBlock struct {
	ent   int
	attrs []attribute
	line  int
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
		// 줄 중간의 CR(원본 전처리가 줄바꿈으로 바꾼다)·FF·VT(원본 lexer 가 건너뛰지 않는다)는 받지 않는다.
		if strings.ContainsAny(raw, "\r\f\v") {
			return p.fail("줄 안에 CR·FF·VT 글자가 있음 — 이 판은 그 줄을 그리지 않는다")
		}
		// ASCII 밖 공백·서식 문자는 줄째 그리지 않는다(원본 JS \s 와 Go 의 공백 판정이 갈리는 갈래 — sequence ③ 과 같다).
		if r, ok := diagram.OddSpace(raw); ok {
			return p.fail("ASCII 밖 공백·서식 문자(U+%04X)가 있음 — 이 판은 그 줄을 그리지 않는다(일반 빈칸으로 바꿔 쓴다)", r)
		}
		t := strings.Trim(raw, " \t")
		if diagram.HasMarker(t) {
			return p.fail("본문 줄에 Mermaid 내부 표식 문자(ﬂ°·¶ß)가 있음 — 이 판은 그 줄을 그리지 않는다(엔티티 표기 #…; 는 그대로 쓸 수 있다)")
		}
		// 설정 지시 — 줄 전체가 지시 하나인 줄(블록 밖)만 무시하는 지시다. 그 밖에 %%{ 가 든 줄(지시 앞뒤·사이의 글, acc 값·
		// 라벨·주석 안, 블록 안)은 원본 removeDirectives 가 지시만 지우고 남은 글을 읽는 갈래라 그리지 않는다(리뷰 1회차).
		if msg := diagram.DirectiveProblem(t, p.inAcc || p.block != nil); msg != "" {
			return p.fail("%s", msg)
		}
		// 줄 머리 %% — 원본 전처리 cleanupComments(/^\s*%%(?!{)[^\n]+\n?/gm)가 어느 상태에서든 지운다. 뒤에 글자가 없는
		// 맨 %% 는 지우지 않아 원본 lexer 가 읽지 못 한다.
		if strings.HasPrefix(t, "%%") && !strings.HasPrefix(t, "%%{") {
			if strings.TrimLeft(raw, " \t") == "%%" {
				return p.fail("글 없는 %s 줄 — 원본은 주석으로 지우지 않아 읽지 못 한다(%s 뒤에 글을 둔다)", "%%", "%%")
			}
			continue
		}
		t = diagram.EncodeEntities(t)
		if p.inAcc { // 블록 글은 줄 끝 빈칸까지 값이다(diagram.AccBlock)
			if err := p.accBody(diagram.EncodeEntities(strings.TrimLeft(raw, " \t"))); err != nil {
				return err
			}
			continue
		}
		if p.block != nil {
			if err := p.blockLine(t); err != nil {
				return err
			}
			continue
		}
		if t == "" {
			continue
		}
		p.raw = diagram.EncodeEntities(raw)
		if err := p.statement(t); err != nil {
			return err
		}
	}
	if p.inAcc {
		return &Error{Line: p.accLine, Msg: "accDescr { 가 닫히지 않음"}
	}
	if p.block != nil {
		return &Error{Line: p.block.line, Msg: "속성 블록 { 가 } 로 닫히지 않음"}
	}
	if !p.header {
		p.line = 0
		return p.fail("mermaid 그림이 비어 있음")
	}
	if len(p.c.Entities) == 0 {
		p.line = 0
		return p.fail("그릴 엔티티가 없음")
	}
	return nil
}

// 허용 꼴. 지시자(accTitle·direction·style …)는 ASCII 만 소문자로 바꾼 줄(low)에 맞추고, 이름·라벨은 원래 줄 t 의 같은
// 자리를 잘라 온다(match). 관계 기호의 o 는 소문자만 받는다(원본은 대소문자를 가리지 않지만 이 판은 좁힌다).
const (
	idPat   = `[\p{L}_][\p{L}\p{N}_-]*`
	listPat = idPat + `(?:,` + idPat + `)*`
	entPat  = `(` + idPat + `)(?::::(` + listPat + `))?`
	// 속성 타입·이름 — 원본 ATTRIBUTE_WORD([\*A-Za-z_À-￿][A-Za-z0-9\-\_\[\]\(\)\.,À-￿\*]*)의 부분집합.
	// ASCII 밖은 글자(\p{L})만 받고 U+00C0–U+FFFF 범위는 wordOK 가 본다.
	wordPat = `[A-Za-z_\p{L}][A-Za-z0-9_\p{L}()\[\].,-]*`
	keyPat  = `(?:PK|FK|UK)`
)

var (
	reAccTitle = regexp.MustCompile(`^acctitle[ \t]*:[ \t]*(.*)$`)
	reAccDescr = regexp.MustCompile(`^accdescr[ \t]*:[ \t]*(.*)$`)
	reAccBlock = regexp.MustCompile(`^accdescr[ \t]*\{(.*)$`)
	reDir      = regexp.MustCompile(`^direction[ \t]+(tb|bt|lr|rl)$`)
	reRel      = regexp.MustCompile(`^` + entPat + `[ \t]+(\|o|\|\||\}o|\}\|)(--|\.\.)(o\||\|\||o\{|\|\{)[ \t]+` + entPat +
		`[ \t]*:[ \t]*(?:"([^"]*)"|(` + idPat + `))$`)
	reEnt  = regexp.MustCompile(`^(` + idPat + `)(?:\[(?:"([^"]*)"|(` + idPat + `))\])?(?::::(` + listPat + `))?(?:[ \t]*(\{)(?:[ \t]*(\}))?)?$`)
	reAttr = regexp.MustCompile(`^(` + wordPat + `)(\?)?[ \t]+(` + wordPat + `)(?:[ \t]+(` + keyPat + `(?:[ \t]*,[ \t]*` + keyPat + `)*))?(?:[ \t]+"([^"]*)")?$`)
	// style·classDef — 원본 style 상태(jison:48–54·91–92)의 토큰(STYLE_TEXT·: · , · #)만으로 된 줄. 이름은 ID 꼴.
	styleText = `[A-Za-z0-9_:#-]+(?:[ \t]+[A-Za-z0-9_:#-]+)*`
	reStyle   = regexp.MustCompile(`^(style|classdef)[ \t]+(` + listPat + `)[ \t]+(` + styleText + `(?:[ \t]*,[ \t]*` + styleText + `)*)$`)
	reClass   = regexp.MustCompile(`^class[ \t]+(` + listPat + `)[ \t]+(` + listPat + `)$`)
	reKeySep  = regexp.MustCompile(`[ \t]*,[ \t]*`)
)

// match 는 줄을 허용 꼴 하나에 맞춘다 — ASCII 만 소문자로 바꾼 줄 low(바이트 길이가 같다)에서 찾고, 값은 원래 줄 t 의
// 같은 자리를 잘라 온다(sequence 와 같은 규칙).
func match(re *regexp.Regexp, t, low string) []string {
	ix := re.FindStringSubmatchIndex(low)
	if ix == nil {
		return nil
	}
	m := make([]string, len(ix)/2)
	for i := range m {
		if ix[2*i] >= 0 {
			m[i] = t[ix[2*i]:ix[2*i+1]]
		}
	}
	return m
}

// statement 는 블록 밖의 줄 하나(앞뒤 공백을 지운 것)를 허용 꼴에 맞춰 읽는다.
func (p *parser) statement(t string) error {
	if strings.HasPrefix(t, "%%{") { // 설정 지시 — 줄 전체가 지시 하나인 것만 여기 온다(parse 의 DirectiveProblem)
		p.note("init", "설정 지시(%%{…}%%)는 쓰지 않음 — 색·글꼴은 테마가 정한다")
		return nil
	}
	low := diagram.ASCIILower(t)
	if !p.header {
		if t != "erDiagram" {
			return p.fail("mermaid 그림 종류를 알 수 없음 — 첫 줄이 erDiagram 이 아님(%s)", diagram.Quote(diagram.Clip(firstWord(t))))
		}
		p.header = true
		return nil
	}
	// 접근성 — 원본 lexer 의 첫 규칙들이라 다른 규칙보다 먼저 읽힌다. 값은 줄 끝까지(`;`·`#` 포함, jison:12–15).
	if m := match(reAccTitle, t, low); m != nil {
		return p.accValue(m[1], &p.c.AccTitle, &p.c.AccTitleLine, "accTitle")
	}
	if m := match(reAccDescr, t, low); m != nil {
		return p.accValue(m[1], &p.c.AccDescr, &p.c.AccDescrLine, "accDescr")
	}
	if m := match(reAccBlock, t, low); m != nil {
		p.acc.Reset() // 블록형 — 선언마다 버퍼를 새로 시작한다(마지막 선언이 이긴다)
		p.inAcc, p.accLine = true, p.line
		// { 뒤의 글은 줄 끝 빈칸까지 값이다 — 앞뒤를 지운 t 가 아니라 원래 줄에서 첫 { 뒤를 자른다(t 의 첫 { 와 같은 자리)
		_, rest, _ := strings.Cut(p.raw, "{")
		return p.accBody(rest)
	}
	// 방향 — 원본 규칙 .*direction\s+(TB|BT|RL|LR)[^\n]* 은 줄 어디서든(\s+ 는 줄바꿈도 넘는다) 줄 전체를 방향 문장으로
	// 먹는다. 정확히 `direction TB|BT|LR|RL` 한 줄만 받고, 이 낱말(대소문자 무시)이 든 다른 줄은 받지 않는다.
	if strings.Contains(low, "direction") {
		m := reDir.FindStringSubmatch(low)
		if m == nil {
			return p.fail("direction 낱말이 든 줄은 `direction TB`·`BT`·`LR`·`RL` 한 줄로만 쓴다 — 원본은 이 낱말이 든 줄을 통째로 방향 문장으로 읽는다(%s)", diagram.Clip(t))
		}
		p.c.Dir = map[string]Dir{"tb": TB, "bt": BT, "lr": LR, "rl": RL}[m[1]]
		return nil
	}
	if m := match(reStyle, t, low); m != nil {
		if err := p.idList(m[2]); err != nil {
			return err
		}
		kw := map[string]string{"style": "style", "classdef": "classDef"}[diagram.ASCIILower(m[1])]
		p.note(kw, fmt.Sprintf("%s 줄은 쓰지 않음 — 색·선은 테마가 정한다", kw))
		return nil
	}
	if strings.HasPrefix(low, "style") || strings.HasPrefix(low, "classdef") {
		return p.fail("style·classDef 줄을 확신할 수 없음 — `style 이름 fill:#f9f,stroke:#333` 꼴(이름은 글자·숫자·_·-, 값은 영문·숫자·-·_·:·#)만 받는다(%s)", diagram.Clip(t))
	}
	// 그 밖의 문장 — 맨 #·;·% 는 받지 않는다(원본 lexer 는 # 을 BRKT 토큰으로, ; 를 낱자로 읽어 문법 오류다. 따옴표 안의
	// 글은 라벨 규칙이 본다).
	if m := match(reClass, t, low); m != nil {
		if err := p.idList(m[1]); err != nil {
			return err
		}
		if err := p.idList(m[2]); err != nil {
			return err
		}
		p.note("class", "class·::: 지정은 쓰지 않음 — 색·선은 테마가 정한다")
		return nil
	}
	if m := reRel.FindStringSubmatch(t); m != nil {
		return p.relation(m)
	}
	if m := reEnt.FindStringSubmatch(t); m != nil {
		return p.entityStmt(m)
	}
	return p.unknown(t, low)
}

// unknown 은 허용 꼴 어디에도 맞지 않은 줄의 *Error 다 — 까닭을 짐작할 수 있는 것은 그 안내를 붙인다.
func (p *parser) unknown(t, low string) error {
	head := firstWord(low)
	switch {
	case head == "title" || strings.HasPrefix(head, "title:"):
		return p.fail("title 문장은 원본 ER 문법에 없다(원본은 엔티티 이름으로 읽는다) — 그림 제목은 맨 앞 --- 의 title(캡션)로 쓴다")
	case head == "subgraph" || head == "end":
		return p.fail("subgraph 묶음은 이 판이 그리지 않는다")
	case head == "}":
		return p.fail("짝 없는 } — 속성 블록은 `이름 {` 로 열고 } 한 줄로 닫는다")
	case strings.HasPrefix(t, `"`):
		return p.fail("따옴표 엔티티 이름은 이 판이 그리지 않는다 — 이름[\"보이는 이름\"] 꼴의 별칭으로 쓴다(%s)", diagram.Clip(t))
	case reWordCard.MatchString(" " + low + " "):
		return p.fail("낱말 기수·관계(one or more·zero or many·to·1+ …)는 이 판이 그리지 않는다 — 기호(|o || }o }| o| o{ |{)로 쓴다(%s)", diagram.Clip(t))
	case strings.Contains(t, ".-") || strings.Contains(t, "-."):
		return p.fail("관계 선은 -- (식별)·.. (비식별)만 그린다 — .- · -. 는 이 판이 그리지 않는다(%s)", diagram.Clip(t))
	case strings.Contains(t, "{") && !strings.HasSuffix(t, "{") && !strings.Contains(t, "--") && !strings.Contains(t, ".."):
		return p.fail("한 줄 블록(`이름 { 타입 이름 }`)은 이 판이 그리지 않는다 — 속성은 블록 안에 한 줄에 하나(%s)", diagram.Clip(t))
	case strings.ContainsAny(t, "#;%"):
		return p.fail("맨 #·;·%s 는 쓰지 못 함 — 원본 lexer 가 기호 토큰으로 읽어 문법 오류다(주석은 줄 머리 %s, 엔티티 표기는 #35; 꼴)", "%", "%%")
	case strings.Contains(t, "--") || strings.Contains(t, ".."):
		if !strings.Contains(t, ":") {
			return p.fail("관계에 콜론(:)과 역할이 없음 — `A ||--o{ B : 역할` 처럼 쓴다(원본 문법도 역할을 요구한다)")
		}
		return p.fail("관계 꼴이 다름 — `이름 왼기호선오른기호 이름 : 역할`(기호와 선은 붙이고 이름과는 빈칸, 왼기호 |o || }o }|, 오른기호 o| || o{ |{)(%s)", diagram.Clip(t))
	}
	return p.fail("알 수 없는 문장(%s) — 11장 지원 표의 꼴로 쓴다", diagram.Clip(t))
}

var reWordCard = regexp.MustCompile(` (?:one|zero|only|many|optionally|to)(?:[ (]|$)| [01]\+ |\bmany\([01]\)`)

// keywordPrefixes 는 원본 lexer 가 UNICODE_TEXT(이름 토큰, jison:93)보다 먼저 읽는 규칙 가운데 이름 꼴(idPat)로
// 시작할 수 있는 낱말이다(ASCII 대소문자 무시 — 원본 %options case-insensitive). 원본 lexer 는 규칙 차례의 첫 일치를
// 쓰므로 이런 낱말로 시작하는 이름은 다른 토큰으로 갈릴 수 있다 — 이 판은 낱말 경계(\b)를 따지지 않고 앞머리만으로
// 막는다(보수적). 낱말 사이에 빈칸이 드는 규칙(zero or one·only one·optionally to)은 이 판의 허용 꼴이 이름 뒤에 그런
// 낱말을 두지 않아 해당이 없다.
var keywordPrefixes = []string{
	"accdescr", "acctitle", // jison:14–16 accDescr\s*":" · accDescr\s*"{" · jison:12 accTitle\s*":"
	"direction",                         // jison:19–22(줄 규칙은 statement 가 먼저 본다)
	"erdiagram",                         // jison:27
	"style", "class", "subgraph", "end", // jison:48·54(classDef)·55·56·57
	"one", "many", "to", // jison:58–60·72(one …) · 68–70(many(0)·many(1)·many) · 87(to)
}

// idOK 는 이름(엔티티·별칭·역할·class 이름)이 허용 꼴 안에서 원본 lexer 가 UNICODE_TEXT 한 토큰으로 읽고 마크다운이
// 글자 그대로 그리는 이름인지 본다.
func (p *parser) idOK(id, what string) error {
	low := diagram.ASCIILower(id)
	for _, r := range id {
		if r > 0xffff {
			return p.fail("%s %s 에 BMP 밖 글자(U+%04X)가 있음 — 이 판은 그리지 않는다", what, diagram.Quote(diagram.Clip(id)), r)
		}
	}
	switch {
	case strings.HasSuffix(id, "-") || strings.Contains(id, "--"):
		return p.fail("%s %s 가 -로 끝나거나 --를 담음 — 관계 선과 헷갈리는 이름은 받지 않는다", what, diagram.Quote(diagram.Clip(id)))
	case !underscoreOK(id):
		return p.fail("%s %s 의 _ 가 글자 사이가 아님 — 원본은 이름을 마크다운으로 그려 기울임이 될 수 있다", what, diagram.Quote(diagram.Clip(id)))
	case low == "title" || low == "pk" || low == "fk" || low == "uk":
		return p.fail("%s %s 는 쓰지 못 함 — title 은 원본 ER 의 문장이 아니고 PK·FK·UK 는 키 낱말이다", what, diagram.Quote(id))
	case strings.HasPrefix(low, "u-"):
		return p.fail("%s %s 는 u- 로 시작해 원본 lexer 가 u 를 MD_PARENT 기호로 먼저 읽는다 — 다른 이름 + 별칭으로", what, diagram.Quote(diagram.Clip(id)))
	}
	for _, w := range keywordPrefixes {
		if strings.HasPrefix(low, w) {
			return p.fail("%s %s 는 키워드 %s 로 시작해 원본 lexer 가 다른 토큰으로 먼저 읽을 수 있다 — 다른 이름 + 별칭(이름[\"%s\"])으로", what, diagram.Quote(diagram.Clip(id)), w, diagram.Clip(id))
		}
	}
	return nil
}

// underscoreOK 는 _ 가 모두 글자·숫자 사이에 있는지 본다(마크다운 기울임 _…_ 이 될 수 없게).
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

func isAlnum(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsNumber(r) }

func (p *parser) idList(s string) error {
	ids := strings.Split(s, ",")
	if len(ids) > maxClassNameList {
		return p.fail("이름 목록이 %d 개를 넘음", maxClassNameList)
	}
	for _, id := range ids {
		if err := p.idOK(id, "이름"); err != nil {
			return err
		}
	}
	return nil
}

// labelOK 는 따옴표 라벨(별칭·역할·설명)의 글(따옴표 안, 엔티티 전처리 뒤)을 본다 — 원본 lexer 가 그 따옴표 토큰으로
// 읽고(ENTITY_NAME 은 %·\·CR·LF·VT·BS 를 받지 않는다 — jison:25) 마크다운이 글자 그대로 그리는 글만 받는다.
func (p *parser) labelOK(s, what string) error {
	for _, r := range s {
		switch {
		case r < 0x20 && r != '\t', r == 0x7f:
			return p.fail("%s에 제어 문자가 있음", what)
		case strings.ContainsRune("#;\"`%\\<>&*~$", r):
			return p.fail("%s에 %s 를 쓰지 못 함 — 원본은 라벨을 마크다운·HTML 로 그리거나 주석·기호로 읽는다(글자는 엔티티 #35; 꼴로)", what, diagram.Quote(string(r)))
		}
	}
	if !underscoreOK(s) {
		return p.fail("%s의 _ 가 글자 사이가 아님 — 원본은 마크다운으로 그려 기울임이 될 수 있다", what)
	}
	if reMDStart.MatchString(strings.TrimLeft(s, " \t")) {
		return p.fail("%s가 -·+·= 나 「수.」로 시작함 — 원본은 마크다운 목록·구분선으로 그린다", what)
	}
	return nil
}

var reMDStart = regexp.MustCompile(`^(?:[-+=]|\d{1,9}[.)](?:[ \t]|$))`)

// text 는 라벨 글을 정책 B 로 읽는다(공통 diagram.Label — 엔티티 표식은 받는 것만 한 번 글자로, 앞뒤 빈칸을 지운 뒤 안쪽 연속
// 빈칸·탭·제어 문자(C0·DEL)는 원문 + 경고). ER 라벨은 <br> 을 줄바꿈으로 받지 않는다(labelOK 가 < 를 막는다 — 옛 DecodeLabel 경로도 닿지
// 않았다).
func (p *parser) text(s string) (string, error) {
	v, msg := diagram.Label(s, "라벨", false)
	if msg != "" {
		return "", p.fail("%s", msg)
	}
	return v, nil
}

// accValue 는 accTitle:·accDescr: 의 값을 읽는다 — 원본처럼 앞뒤만 trim, 안쪽 빈칸·탭은 그대로(공통 diagram.AccValue — 빈 값,
// 맨 &·<·>, 탭·줄바꿈 밖의 제어 문자(unicode.Cc)는 원문).
func (p *parser) accValue(v string, dst *string, line *int, kw string) error {
	s, msg := diagram.AccValue(v, kw)
	if msg != "" {
		return p.fail("%s", msg)
	}
	*dst, *line = s, p.line
	return nil
}

// accBody 는 accDescr { … } 의 한 줄(여는 줄의 { 뒤 포함 — 앞 빈칸만 지운 것, 끝 빈칸은 그대로)을 모은다(공통 diagram.AccBlock —
// 닫는 } 는 줄 끝에만, 블록 전체 trim 뒤 원본 setAccDescription 의 \n\s+ → \n, 줄바꿈·안쪽 빈칸은 그대로).
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
		// 빈 블록은 앞 설명을 그대로 둔다 — 원본은 } 규칙이 상태만 닫아 값 토큰이 생기지 않는다(jison:16–18)
		return nil
	}
	s, msg := diagram.AccSet(v, "accDescr")
	if msg != "" {
		return p.fail("%s", msg)
	}
	p.c.AccDescr, p.c.AccDescrLine = s, p.line
	return nil
}

// eachText 는 모델에서 SVG·Result 로 나가는 사용자 글자를 하나씩 f 에 넘긴다(field 는 「형.필드」, what 은 경고 문구의 이름,
// line 은 그 값이 나온 줄). **출력 글자 필드 목록은 여기 한곳이다** — 모델에 글 필드를 더하면 여기에도 더한다(TestGateCoversAllText
// 가 모델의 string·[]string 필드와 견준다). 엔티티 ID 는 그리지 않지만(그리는 것은 Name) 함께 본다.
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
	for _, e := range c.Entities {
		for _, x := range []struct {
			field, v string
			line     int
		}{{"entity.ID", e.ID, e.Line}, {"entity.Name", e.Name, e.Line}, {"entity.Alias", e.Alias, e.AliasLine}} {
			if err := f(x.field, "엔티티 이름", x.v, x.line); err != nil {
				return err
			}
		}
		for _, a := range e.Attrs {
			for _, x := range []struct{ field, v string }{{"attribute.Type", a.Type}, {"attribute.Name", a.Name}, {"attribute.Comment", a.Comment}} {
				if err := f(x.field, "속성", x.v, a.Line); err != nil {
					return err
				}
			}
			for _, k := range a.Keys {
				if err := f("attribute.Keys", "속성 키", k, a.Line); err != nil {
					return err
				}
			}
		}
	}
	for _, r := range c.Relations {
		if err := f("relation.Role", "역할", r.Role, r.Line); err != nil {
			return err
		}
	}
	return nil
}

// addEntity 는 엔티티를 찾아 없으면 더한다(원본 addEntity — 처음 차례, 별칭은 비어 있을 때만 채운다).
func (p *parser) addEntity(id, alias string) (int, error) {
	if k, ok := p.c.byID[id]; ok {
		if e := p.c.Entities[k]; e.Alias == "" && alias != "" {
			e.Alias, e.AliasLine = alias, p.line
		}
		return k, nil
	}
	if len(p.c.Entities) >= MaxEntities {
		return 0, p.fail("엔티티가 %d 개를 넘음", MaxEntities)
	}
	name, err := p.text(id)
	if err != nil {
		return 0, err
	}
	p.c.byID[id] = len(p.c.Entities)
	p.c.Entities = append(p.c.Entities, &entity{ID: id, Name: name, Alias: alias, Line: p.line, AliasLine: p.line})
	return len(p.c.Entities) - 1, nil
}

// entityStmt 는 엔티티 선언(reEnt 의 짝)을 읽는다: 1 이름, 2 따옴표 별칭, 3 ID 별칭, 4 ::: 이름들, 5 {, 6 }.
func (p *parser) entityStmt(m []string) error {
	if err := p.idOK(m[1], "엔티티 이름"); err != nil {
		return err
	}
	alias := ""
	switch {
	case m[2] != "" || strings.Contains(m[0], `[""]`):
		if m[2] == "" {
			return p.fail("빈 별칭 [\"\"] 은 원본이 읽지 못 한다")
		}
		if err := p.labelOK(m[2], "별칭"); err != nil {
			return err
		}
		a, err := p.text(m[2])
		if err != nil {
			return err
		}
		if a == "" {
			return p.fail("별칭이 비어 있음")
		}
		alias = a
	case m[3] != "":
		if err := p.idOK(m[3], "별칭"); err != nil {
			return err
		}
		a, err := p.text(m[3])
		if err != nil {
			return err
		}
		alias = a
	}
	if m[4] != "" {
		if err := p.idList(m[4]); err != nil {
			return err
		}
	}
	k, err := p.addEntity(m[1], alias)
	if err != nil {
		return err
	}
	if m[4] != "" {
		p.note("class", "class·::: 지정은 쓰지 않음 — 색·선은 테마가 정한다")
	}
	if m[5] != "" && m[6] == "" {
		p.block = &openBlock{ent: k, line: p.line}
	}
	return nil
}

// relation 은 관계(reRel 의 짝)를 읽는다: 1 A, 2 A 의 :::, 3 왼기호, 4 선, 5 오른기호, 6 B, 7 B 의 :::, 8 따옴표 역할, 9 ID 역할.
func (p *parser) relation(m []string) error {
	if err := p.idOK(m[1], "엔티티 이름"); err != nil {
		return err
	}
	if err := p.idOK(m[6], "엔티티 이름"); err != nil {
		return err
	}
	for _, l := range []string{m[2], m[7]} {
		if l != "" {
			if err := p.idList(l); err != nil {
				return err
			}
		}
	}
	role := ""
	if m[9] != "" {
		if err := p.idOK(m[9], "역할"); err != nil {
			return err
		}
		role = m[9]
	} else {
		// 따옴표 역할 — 빈 "" 도 원본 WORD 토큰(jison:26)이라 라벨 없는 선이다
		if err := p.labelOK(m[8], "역할"); err != nil {
			return err
		}
		role = m[8]
	}
	r, err := p.text(role)
	if err != nil {
		return err
	}
	if len(p.c.Relations) >= MaxRelations {
		return p.fail("관계가 %d 개를 넘음", MaxRelations)
	}
	a, err := p.addEntity(m[1], "")
	if err != nil {
		return err
	}
	b, err := p.addEntity(m[6], "")
	if err != nil {
		return err
	}
	if m[2] != "" || m[7] != "" {
		p.note("class", "class·::: 지정은 쓰지 않음 — 색·선은 테마가 정한다")
	}
	p.c.Relations = append(p.c.Relations, relation{A: a, B: b, Left: symCard[m[3]], Right: symCard[m[5]],
		Identifying: m[4] == "--", Role: r, Line: p.line})
	return nil
}

var symCard = map[string]Card{
	"|o": ZeroOrOne, "o|": ZeroOrOne, "||": ExactlyOne, "}o": ZeroOrMore, "o{": ZeroOrMore, "}|": OneOrMore, "|{": OneOrMore,
}

// blockLine 은 속성 블록 안의 줄 하나를 읽는다 — 빈 줄, 속성 한 줄, 닫는 } 한 줄.
func (p *parser) blockLine(t string) error {
	if t == "" {
		return nil
	}
	if t == "}" {
		b := p.block
		p.block = nil
		e := p.c.Entities[b.ent]
		if len(e.Attrs)+len(b.attrs) > MaxAttrsPerEnt {
			return &Error{Line: b.line, Msg: fmt.Sprintf("엔티티 %s 의 속성이 %d 개를 넘음", diagram.Quote(diagram.Clip(e.ID)), MaxAttrsPerEnt)}
		}
		e.Attrs = append(e.Attrs, b.attrs...)
		return nil
	}
	m := reAttr.FindStringSubmatch(t)
	if m == nil {
		switch {
		case strings.ContainsAny(t, "~`*"):
			return p.fail("속성의 ~·`·* (제네릭 ~T~·백틱 이름·별표)는 이 판이 그리지 않는다(%s)", diagram.Clip(t))
		case strings.HasSuffix(t, "}"):
			return p.fail("블록 닫기 } 는 한 줄에 따로 둔다(%s)", diagram.Clip(t))
		}
		return p.fail("속성은 `타입 이름 [PK, FK] [\"설명\"]` 꼴로 한 줄에 하나(키는 대문자 PK·FK·UK)(%s)", diagram.Clip(t))
	}
	for _, w := range []string{m[1], m[3]} {
		if err := p.wordOK(w); err != nil {
			return err
		}
	}
	typ, err := p.text(m[1] + m[2])
	if err != nil {
		return err
	}
	name, err := p.text(m[3])
	if err != nil {
		return err
	}
	var keys []string
	if m[4] != "" {
		keys = reKeySep.Split(m[4], -1)
	}
	cmt := ""
	if m[5] != "" {
		if err := p.labelOK(m[5], "속성 설명"); err != nil {
			return err
		}
		if cmt, err = p.text(m[5]); err != nil {
			return err
		}
	}
	if p.nAttrs >= MaxAttrsTotal {
		return p.fail("속성 합계가 %d 개를 넘음", MaxAttrsTotal)
	}
	p.nAttrs++
	if len(p.block.attrs) >= MaxAttrsPerEnt {
		return p.fail("엔티티의 속성이 %d 개를 넘음", MaxAttrsPerEnt)
	}
	p.block.attrs = append(p.block.attrs, attribute{Type: typ, Name: name, Keys: keys, Comment: cmt, Line: p.line})
	return nil
}

// wordOK 는 속성 타입·이름이 원본 ATTRIBUTE_WORD 한 토큰이고(ASCII 밖 글자는 U+00C0–U+FFFF) 키 낱말 규칙
// \b(PK|FK|UK)\b(블록 상태에서 ATTRIBUTE_WORD 보다 먼저 — jison:35)에 먼저 걸리지 않으며 마크다운이 글자 그대로
// 그리는지 본다.
func (p *parser) wordOK(w string) error {
	for _, r := range w {
		if r > 0x7f && (r < 0xc0 || r > 0xffff) {
			return p.fail("속성 %s 에 받지 않는 글자(U+%04X)가 있음 — ASCII 밖은 U+00C0–U+FFFF 의 글자만", diagram.Quote(diagram.Clip(w)), r)
		}
	}
	low := diagram.ASCIILower(w)
	for _, k := range []string{"pk", "fk", "uk"} {
		if strings.HasPrefix(low, k) && (len(low) == 2 || !isWordByte(low[2])) {
			return p.fail("속성 %s 는 키 낱말 %s 로 시작해 원본 lexer 가 키로 먼저 읽는다 — 다른 이름으로", diagram.Quote(diagram.Clip(w)), strings.ToUpper(k))
		}
	}
	if !underscoreOK(w) {
		return p.fail("속성 %s 의 _ 가 글자 사이가 아님 — 원본은 마크다운으로 그려 기울임이 될 수 있다", diagram.Quote(diagram.Clip(w)))
	}
	if strings.Contains(w, "](") {
		return p.fail("속성 %s 의 ](…) 는 원본이 마크다운 링크로 그린다", diagram.Quote(diagram.Clip(w)))
	}
	return nil
}

func isWordByte(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}
