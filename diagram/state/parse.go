package state

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/itda-work/itda-doc/diagram"
)

// 원문을 한 번 훑는다 — 줄마다 아래 허용 꼴(양성 목록) 하나에 통째로 맞아야 그리고, 어느 꼴에도 맞지 않으면 그 자리에서
// *Error(줄 번호)로 멈춘다(원문 길이에 선형). 허용 꼴은 mermaid@12.2.0 stateDiagram.jison 이 같은 뜻으로 읽는다고 이 판이
// 확신할 수 있는 좁은 꼴만 모은 것이다(itda-hyve#208 「허용 목록 문법」 — er ④-1 과 같은 결정). 원본이 그리는 구석 꼴(메모·복합 상태를
// 끝점으로 쓰는 전이·중첩 복합 상태·동시 영역 --·[[fork]]·한 줄 여러 문장 …)은 원문 + 줄 번호 경고로 간다(의도된 축소).
//
//	한 줄에 문장 하나. 줄 머리 %% 는 주석(원본 전처리 cleanupComments 가 지운다 — 블록 안에서도), 글 없는 맨 %% 도 주석
//	          (원본 lexer \%\%(?!\{)[^\n]* — jison:84). 그 밖의 자리의 %%·# 는 받지 않는다(jison:83·processId)
//	ID        [\p{L}_][\p{L}\p{N}_]* — 키워드로 시작하는 이름·as·root·_start·_end 로 끝나는 이름·BMP 밖 글자를 뺀다(idOK)
//	머리 줄   stateDiagram · stateDiagram-v2(대소문자 구분 — 원본 detector /^\s*stateDiagram/)
//	상태      ID · ID : 설명 · state "설명" as ID · 모두 + ID 뒤 :::이름(무시) (jison:205–212·233–243·338)
//	특수 상태 state ID <<choice>> · <<fork>> · <<join>> (jison:116–118)
//	전이      끝 --> 끝 · 끝 --> 끝 : 라벨 — 끝은 ID(:::이름) 또는 [*] (jison:157·213–224)
//	복합 상태 state ID { · state "설명" as ID { … } 한 줄 — 한 단계, 안에는 상태·특수 상태·전이·무시 줄 (jison:136–137·228·244)
//	방향      direction TB|BT|LR|RL — 뿌리에서만, 이 낱말이 든 다른 줄은 받지 않는다 (jison:75–78·123–126)
//	접근성    accTitle: 값 · accDescr: 값(줄 끝까지) · accDescr { … } — 뿌리에서만 (jison:89–95)
//	무시      %%{…}%%(한 줄) · frontmatter 의 title 밖 키 · style · classDef · class · ::: · click · hide empty description
//	라벨      설명·전이 라벨·복합 상태 설명 — labelOK(마크다운·HTML·lexer 가 다르게 읽는 글자 없음), 안쪽 빈칸은 한 칸씩, 200 자
//
// 원본 전처리(encodeEntities — 엔티티 `#…;` 를 내부 표식으로 지켜 준다)는 본문 줄마다 먼저 지나간다(공통 diagram).
// 표식은 정책 B(entity.go restore)로만 한 번 되돌린다 — 공통 DecodeLabel 은 쓰지 않는다.

// Parse 는 mermaid stateDiagram 원문을 읽는다. 그릴 수 없는 문법이면 *Error 다. issues 는 쓰지 않은 줄이다.
func Parse(src string) (*chart, []Issue, error) {
	p := &parser{c: &chart{byID: map[string]int{}, compByID: map[string]int{}}, said: map[string]bool{}, comp: -1}
	// 잘못된 UTF-8 은 읽기 전에 U+FFFD 로. XML 이 허용하지 않는 글자는 SVG 를 쓰는 경계(diagram.Esc)에서 거른다.
	if err := p.parse(strings.ToValidUTF8(src, "�")); err != nil {
		return nil, nil, err
	}
	// 출력 관문 — 본문을 다 읽은 뒤, 내보낼 사용자 글자를 모두 한 번에 본다(본문 오류가 먼저다).
	if err := p.c.gate(); err != nil {
		return nil, nil, err
	}
	return p.c, p.issues, nil
}

type parser struct {
	c       *chart
	issues  []Issue
	line    int  // 지금 읽는 줄(1부터)
	header  bool // 머리 줄(stateDiagram)을 지났다
	said    map[string]bool
	inAcc   bool // accDescr { … } 안
	accLine int
	acc     diagram.AccBlock
	raw     string // 지금 줄의 원문(엔티티 전처리 뒤, 앞뒤 빈칸 그대로) — accDescr { 여는 줄의 값
	comp    int    // 열린 복합 상태(chart.Comps 의 차례), 없으면 -1
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
		// ASCII 밖 공백·서식 문자는 줄째 그리지 않는다(원본 JS \s 와 Go 의 공백 판정이 갈리는 갈래 — er 와 같다).
		if r, ok := diagram.OddSpace(raw); ok {
			return p.fail("ASCII 밖 공백·서식 문자(U+%04X)가 있음 — 이 판은 그 줄을 그리지 않는다(일반 빈칸으로 바꿔 쓴다)", r)
		}
		t := strings.Trim(raw, " \t")
		if diagram.HasMarker(t) {
			return p.fail("본문 줄에 Mermaid 내부 표식 문자(ﬂ°·¶ß)가 있음 — 이 판은 그 줄을 그리지 않는다(엔티티 표기 #…; 는 그대로 쓸 수 있다)")
		}
		// 설정 지시 — 줄 전체가 지시 하나인 줄(뿌리)만 무시하는 지시다. 그 밖에 %%{ 가 든 줄(지시 앞뒤·사이의 글, acc 값·
		// 라벨·주석 안, 복합 상태·accDescr 블록 안)은 원본 removeDirectives 가 지시만 지우고 남은 글을 읽는 갈래라 그리지 않는다.
		if msg := diagram.DirectiveProblem(t, p.inAcc || p.comp >= 0); msg != "" {
			return p.fail("%s", msg)
		}
		// 줄 머리 %% — 원본 전처리 cleanupComments(/^\s*%%(?!{)[^\n]+\n?/gm)가 어느 상태에서든 지운다. 뒤에 글이 없는 맨 %% 는
		// 지우지 않지만 원본 lexer 의 주석 규칙(\%\%(?!\{)[^\n]* — jison:84, INITIAL·struct)이 건너뛴다. accDescr { … } 안의
		// 맨 %% 는 원본이 설명 글로 넣는다(acc_descr_multiline [^\}]*) — 이 판은 그리지 않는다.
		if strings.HasPrefix(t, "%%") && !strings.HasPrefix(t, "%%{") {
			if p.inAcc && strings.TrimLeft(raw, " \t") == "%%" {
				return p.fail("accDescr { … } 안의 글 없는 %s 줄 — 원본은 설명 글로 읽는다(이 판은 그리지 않는다)", "%%")
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
	if p.comp >= 0 {
		cp := p.c.Comps[p.comp]
		return &Error{Line: cp.Line, Msg: fmt.Sprintf("복합 상태 %s 의 { 가 } 로 닫히지 않음", diagram.Quote(diagram.Clip(cp.ID)))}
	}
	if !p.header {
		p.line = 0
		return p.fail("mermaid 그림이 비어 있음")
	}
	if len(p.c.States) == 0 {
		p.line = 0
		return p.fail("그릴 상태가 없음")
	}
	return nil
}

// 허용 꼴. 지시자(state·as·direction·accTitle·style …)는 ASCII 만 소문자로 바꾼 줄(low)에 맞추고, 이름·라벨은 원래 줄 t 의
// 같은 자리를 잘라 온다(match — er·sequence 와 같은 규칙).
const (
	idPat = `[\p{L}_][\p{L}\p{N}_]*`
	// endPat 은 전이의 한 끝이다 — [*] 또는 ID(:::이름). 묶음: 1 [*]·ID 전체, 2 ID, 3 ::: 이름.
	endPat = `(\[\*\]|(` + idPat + `)(?::::(` + idPat + `))?)`
	// 쓰지 않는 줄의 이름 — style·class 는 원본 lexer 가 \w(ASCII) 로 읽는다(jison:103·107), classDef 도 \w+(jison:99).
	asciiID   = `[a-z0-9_]+`
	styleText = `[a-z0-9_:#.-]+(?:[ \t]+[a-z0-9_:#.-]+)*`
)

var (
	reAccTitle = regexp.MustCompile(`^acctitle[ \t]*:[ \t]*(.*)$`)
	reAccDescr = regexp.MustCompile(`^accdescr[ \t]*:[ \t]*(.*)$`)
	reAccBlock = regexp.MustCompile(`^accdescr[ \t]*\{(.*)$`)
	reDir      = regexp.MustCompile(`^direction[ \t]+(tb|bt|lr|rl)$`)
	reTrans    = regexp.MustCompile(`^` + endPat + `[ \t]*-->[ \t]*` + endPat + `(?:[ \t]*:(.*))?$`)
	reDecl     = regexp.MustCompile(`^(` + idPat + `)(?::::(` + idPat + `))?(?:[ \t]*:(.*))?$`)
	reSpecial  = regexp.MustCompile(`^state[ \t]+(` + idPat + `)[ \t]*<<(choice|fork|join)>>$`)
	reAsState  = regexp.MustCompile(`^state[ \t]+"([^"]*)"[ \t]+as[ \t]+(` + idPat + `)([ \t]*\{)?$`)
	reComp     = regexp.MustCompile(`^state[ \t]+(` + idPat + `)[ \t]*\{$`)
	reStyle    = regexp.MustCompile(`^style[ \t]+(` + asciiID + `(?:,` + asciiID + `)*)[ \t]+(` + styleText + `(?:[ \t]*,[ \t]*` + styleText + `)*)$`)
	reClassDef = regexp.MustCompile(`^classdef[ \t]+(` + asciiID + `)[ \t]+(` + styleText + `(?:[ \t]*,[ \t]*` + styleText + `)*)$`)
	reClass    = regexp.MustCompile(`^class[ \t]+(` + asciiID + `(?:,[ \t]*` + asciiID + `)*)[ \t]+(` + asciiID + `)$`)
	reClick    = regexp.MustCompile(`^click[ \t]+(` + idPat + `)[ \t]+(?:"[^"]*"[ \t]+"[^"]*"|href[ \t]+"[^"]*")$`)
)

// match 는 줄을 허용 꼴 하나에 맞춘다 — ASCII 만 소문자로 바꾼 줄 low(바이트 길이가 같다)에서 찾고, 값은 원래 줄 t 의
// 같은 자리를 잘라 온다.
func match(re *regexp.Regexp, t, low string) []string {
	m, _ := matchP(re, t, low)
	return m
}

// matchP 는 match 에 더해 묶음마다 맞았는지(빈 글로 맞은 것과 맞지 않은 것을 가른다)를 돌려준다.
func matchP(re *regexp.Regexp, t, low string) ([]string, []bool) {
	ix := re.FindStringSubmatchIndex(low)
	if ix == nil {
		return nil, nil
	}
	m := make([]string, len(ix)/2)
	ok := make([]bool, len(ix)/2)
	for i := range m {
		if ix[2*i] >= 0 {
			m[i], ok[i] = t[ix[2*i]:ix[2*i+1]], true
		}
	}
	return m, ok
}

// statement 는 블록 밖·복합 상태 안의 줄 하나(앞뒤 공백을 지운 것)를 허용 꼴에 맞춰 읽는다.
func (p *parser) statement(t string) error {
	if strings.HasPrefix(t, "%%{") { // 설정 지시 — 줄 전체가 지시 하나인 것만 여기 온다(parse 의 DirectiveProblem)
		p.note("init", "설정 지시(%%{…}%%)는 쓰지 않음 — 색·글꼴은 테마가 정한다")
		return nil
	}
	low := diagram.ASCIILower(t)
	if !p.header {
		if t != "stateDiagram" && t != "stateDiagram-v2" {
			return p.fail("mermaid 그림 종류를 알 수 없음 — 첫 줄이 stateDiagram·stateDiagram-v2 가 아님(%s)", diagram.Quote(diagram.Clip(firstWord(t))))
		}
		p.header = true
		return nil
	}
	in := p.comp >= 0
	if in && t == "}" {
		return p.closeComp()
	}
	// 방향 — 원본 규칙 .*direction\s+(TB|BT|RL|LR)[^\n]* 은 다른 규칙보다 먼저(click·href·따옴표·default 다음 — jison:69–78)
	// 줄 어디서든(\s+ 는 줄바꿈도 넘는다) 줄 전체를 방향 문장으로 먹는다. 정확히 `direction TB|BT|LR|RL` 한 줄만 받고, 이
	// 낱말(대소문자 무시)이 든 다른 줄은 받지 않는다. 복합 상태 안의 방향(jison:123–126)은 원본에서 그 묶음 안의 배치 방향만
	// 바꾼다(dataFetcher getDir) — 이 판의 배치는 묶음마다 방향을 따로 두지 않아 그리지 않는다.
	if strings.Contains(low, "direction") {
		m := reDir.FindStringSubmatch(low)
		if m == nil {
			return p.fail("direction 낱말이 든 줄은 `direction TB`·`BT`·`LR`·`RL` 한 줄로만 쓴다 — 원본은 이 낱말이 든 줄을 통째로 방향 문장으로 읽는다(%s)", diagram.Clip(t))
		}
		if in {
			return p.fail("복합 상태 안의 direction 은 원본에서 그 묶음 안의 방향만 바꾼다 — 이 판은 그리지 않는다(방향은 뿌리에 한 줄로)")
		}
		p.c.Dir = map[string]Dir{"tb": TB, "bt": BT, "lr": LR, "rl": RL}[m[1]] // 여럿이면 마지막이 이긴다(원본 getDir)
		return nil
	}
	// 줄 머리가 아닌 %% — 원본 processId 가 이름 중간의 %% 에서 잘라 뒤를 주석으로 읽는다(jison:49–64).
	if strings.Contains(t, "%%") {
		return p.fail("줄 중간의 %s 는 쓰지 못 함 — 주석은 줄 머리에 둔다(원본은 그 자리에서 이름을 자른다)", "%%")
	}
	// 접근성 — 원본 lexer 의 INITIAL 규칙(jison:89–95)이라 복합 상태 안에서는 읽지 못 한다. 값은 줄 끝까지(`;`·`#` 포함).
	if m := match(reAccTitle, t, low); m != nil {
		if in {
			return p.fail("복합 상태 안의 accTitle 은 원본이 읽지 못 한다")
		}
		return p.accValue(m[1], &p.c.AccTitle, &p.c.AccTitleLine, "accTitle")
	}
	if m := match(reAccDescr, t, low); m != nil {
		if in {
			return p.fail("복합 상태 안의 accDescr 는 원본이 읽지 못 한다")
		}
		return p.accValue(m[1], &p.c.AccDescr, &p.c.AccDescrLine, "accDescr")
	}
	if m := match(reAccBlock, t, low); m != nil {
		if in {
			return p.fail("복합 상태 안의 accDescr 는 원본이 읽지 못 한다")
		}
		p.acc.Reset() // 블록형 — 선언마다 버퍼를 새로 시작한다(마지막 선언이 이긴다)
		p.inAcc, p.accLine = true, p.line
		// { 뒤의 글은 줄 끝 빈칸까지 값이다 — 앞뒤를 지운 t 가 아니라 원래 줄에서 첫 { 뒤를 자른다(t 의 첫 { 와 같은 자리)
		_, rest, _ := strings.Cut(p.raw, "{")
		return p.accBody(rest)
	}
	// 쓰지 않는 줄 — style·classDef·class 는 뿌리·복합 상태 안 모두(jison:97–108 <INITIAL,struct>), click·hide empty
	// description 은 뿌리에서만(jison:69·155 — 상태 없는 규칙은 INITIAL 에서만 쓰인다).
	if m := reStyle.FindStringSubmatch(low); m != nil {
		if err := p.nameCount(m[1]); err != nil {
			return err
		}
		p.note("style", "style 줄은 쓰지 않음 — 색·선은 테마가 정한다")
		return nil
	}
	if m := reClassDef.FindStringSubmatch(low); m != nil {
		// classDef default … — 원본 lexer 는 CLASSDEF 상태에서 DEFAULT\s+(jison:98, 대소문자 무시)가 \w+\s+ 보다 먼저 이겨
		// DEFAULT_CLASSDEF_ID 를 내는데 문법(classDefStatement 291–298)에 그 토큰을 받는 생산식이 없다 — 원본 문법 오류.
		if m[1] == "default" {
			return p.fail("classDef default 는 원본 문법 오류다 — 원본 lexer 가 default 를 따로 읽는데 문법이 받지 않는다(다른 클래스 이름으로)")
		}
		p.note("classDef", "classDef 줄은 쓰지 않음 — 색·선은 테마가 정한다")
		return nil
	}
	if m := reClass.FindStringSubmatch(low); m != nil {
		if err := p.nameCount(m[1]); err != nil {
			return err
		}
		p.note("class", "class·::: 지정은 쓰지 않음 — 색·선은 테마가 정한다")
		return nil
	}
	if strings.HasPrefix(low, "style") || strings.HasPrefix(low, "classdef") || strings.HasPrefix(low, "class") {
		if word := firstWord(low); word == "style" || word == "classdef" || word == "class" {
			return p.fail("style·classDef·class 줄을 확신할 수 없음 — `style 이름 fill:#f9f,stroke:#333`·`classDef 이름 fill:#f00`·`class 이름 클래스` 꼴(이름은 영문·숫자·_, 값은 영문·숫자·-·_·:·#·.)만 받는다(%s)", diagram.Clip(t))
		}
	}
	if m := match(reClick, t, low); m != nil {
		if in {
			return p.fail("복합 상태 안의 click 은 원본이 읽지 못 한다")
		}
		if err := p.idOK(m[1], "이름"); err != nil {
			return err
		}
		p.note("click", "click 줄은 쓰지 않음 — 문서의 그림은 누를 수 없다")
		return nil
	}
	if low == "hide empty description" {
		if in {
			return p.fail("복합 상태 안의 hide empty description 은 원본이 읽지 못 한다")
		}
		p.note("hide", "hide empty description 줄은 쓰지 않음 — 원본 v3 렌더도 쓰지 않는다")
		return nil
	}
	// state … 문장(원본 STATE 상태 — jison:114–135)
	if m := match(reSpecial, t, low); m != nil {
		return p.special(m[1], map[string]Kind{"choice": Choice, "fork": Fork, "join": Join}[diagram.ASCIILower(m[2])])
	}
	if m := match(reAsState, t, low); m != nil {
		if m[3] != "" {
			return p.openComp(m[2], m[1], true)
		}
		return p.declare(m[2], "", m[1], true)
	}
	if m := match(reComp, t, low); m != nil {
		return p.openComp(m[1], "", false)
	}
	if m, ok := matchP(reTrans, t, low); m != nil {
		return p.transition(m, ok[7])
	}
	if m, ok := matchP(reDecl, t, low); m != nil {
		return p.declare(m[1], m[2], m[3], ok[3])
	}
	return p.unknown(t, low)
}

// unknown 은 허용 꼴 어디에도 맞지 않은 줄의 *Error 다 — 까닭을 짐작할 수 있는 것은 그 안내를 붙인다.
func (p *parser) unknown(t, low string) error {
	head := firstWord(low)
	switch {
	case head == "note" || head == "end" && strings.HasPrefix(low, "end note"):
		return p.fail("메모(note)는 이 판이 그리지 않는다")
	case head == "state" && (strings.Contains(t, "[[") || strings.Contains(t, "<<")):
		return p.fail("특수 상태는 `state 이름 <<choice>>`·`<<fork>>`·`<<join>>` 꼴만 그린다([[fork]] 꼴은 이 판이 그리지 않는다)(%s)", diagram.Clip(t))
	case head == "state" && strings.HasSuffix(t, "{"):
		return p.fail("복합 상태는 `state 이름 {`·`state \"설명\" as 이름 {` 꼴로 연다 — 이름은 한 낱말(%s)", diagram.Clip(t))
	case head == "state":
		return p.fail("state 문장은 `state \"설명\" as 이름`·`state 이름 <<choice>>`·`state 이름 {` 꼴만 그린다 — `state 이름` 한 줄은 원본이 상태를 만들지 않는다(%s)", diagram.Clip(t))
	case head == "scale":
		return p.fail("scale 은 이 판이 그리지 않는다")
	case t == "--":
		return p.fail("동시 영역(--)은 이 판이 그리지 않는다")
	case t == "}":
		return p.fail("짝 없는 } — 복합 상태는 `state 이름 {` 로 열고 } 한 줄로 닫는다")
	case strings.Contains(t, "{") || strings.Contains(t, "}"):
		return p.fail("{ … } 는 복합 상태(`state 이름 {` 한 줄 … } 한 줄)로만 쓴다(%s)", diagram.Clip(t))
	case strings.Contains(t, ";"):
		return p.fail("; 로 이은 여러 문장·맨 ; 는 쓰지 못 함 — 한 줄에 문장 하나(%s)", diagram.Clip(t))
	case strings.Contains(t, "#"):
		return p.fail("맨 # 는 쓰지 못 함 — 원본 lexer 는 # 부터 줄 끝을 주석으로 지운다(주석은 줄 머리 %s, 글자 #은 엔티티 #35;)", "%%")
	case strings.HasPrefix(t, `"`):
		return p.fail("따옴표 상태 이름은 이 판이 그리지 않는다 — `state \"보이는 이름\" as 이름` 으로 쓴다(%s)", diagram.Clip(t))
	case strings.Contains(t, "-->"):
		return p.fail("전이 꼴이 다름 — `이름 --> 이름`·`이름 --> 이름 : 라벨`(끝은 이름 또는 [*], 한 줄에 전이 하나)(%s)", diagram.Clip(t))
	case strings.Contains(t, "--") || strings.Contains(t, "->"):
		return p.fail("전이는 --> 만 그린다(%s)", diagram.Clip(t))
	}
	return p.fail("알 수 없는 문장(%s) — 12장 지원 표의 꼴로 쓴다", diagram.Clip(t))
}

// keywordPrefixes 는 원본 lexer 가 ID 규칙([^:\n\s\-\{]+ — jison:158)보다 먼저 읽는 규칙 가운데 이름 꼴(idPat)로 시작할 수
// 있는 낱말이다(ASCII 대소문자 무시 — 원본 %options case-insensitive). jison 은 낱말 글자로 끝나는 규칙에 \b 를 붙이고
// (lex-parser: click·href·default·hide empty description) 나머지는 \s+ 가 뒤따라야 맞는다(state·note·class·classDef·style·
// scale·stateDiagram) — 이 판은 낱말 경계를 따지지 않고 앞머리만으로 막는다(보수적. 안내서 12장 키워드 표).
var keywordPrefixes = []string{
	"click",     // jison:69 — click\b
	"href",      // jison:70 — href\b
	"default",   // jison:73 — default\b
	"direction", // jison:75–78·123–126 — 줄 어디서든(statement 가 먼저 본다)
	"scale",     // jison:85·110 — \s+
	"acctitle",  // jison:89
	"accdescr",  // jison:91·93
	"class",     // jison:97·102 — classDef\s+·class\s+
	"style",     // jison:106 — \s+
	"state",     // jison:114·152·153 — state\s+·stateDiagram\s+
	"note",      // jison:140 — \s+
	"hide",      // jison:155 — hide empty description\b
}

// idOK 는 이름(상태·복합 상태·::: 이름·click 이름)이 허용 꼴 안에서 원본 lexer 가 ID 한 토큰으로 읽고 원본 dataFetcher 가
// 그 이름 그대로 상태로 쓰며 마크다운이 글자 그대로 그리는 이름인지 본다.
func (p *parser) idOK(id, what string) error {
	low := diagram.ASCIILower(id)
	for _, r := range id {
		if r > 0xffff {
			return p.fail("%s %s 에 BMP 밖 글자(U+%04X)가 있음 — 이 판은 그리지 않는다", what, diagram.Quote(diagram.Clip(id)), r)
		}
	}
	switch {
	case len([]rune(id)) > MaxLabel: // 이름도 상자에 적히는 글이다 — 라벨과 같은 200자 상한(리뷰 1회차 참고 관찰)
		return p.fail("%s %s 가 %d 자를 넘음", what, diagram.Quote(diagram.Clip(id)), MaxLabel)
	case !underscoreOK(id):
		return p.fail("%s %s 의 _ 가 글자 사이가 아님 — 원본은 이름을 마크다운으로 그려 기울임이 될 수 있다", what, diagram.Quote(diagram.Clip(id)))
	case low == "as":
		return p.fail("%s %s 는 쓰지 못 함 — state … as 의 낱말이다", what, diagram.Quote(id))
	case id == "root":
		return p.fail("%s %s 는 쓰지 못 함 — 원본은 root 를 뿌리 문서의 id 로 써서 상태로 그리지 않는다", what, diagram.Quote(id))
	case strings.HasSuffix(id, "_start") || strings.HasSuffix(id, "_end"):
		return p.fail("%s %s 는 _start·_end 로 끝나 원본이 [*] 에 붙이는 id(root_start·묶음_end)와 겹칠 수 있다 — 다른 이름으로", what, diagram.Quote(diagram.Clip(id)))
	}
	for _, w := range keywordPrefixes {
		if strings.HasPrefix(low, w) {
			return p.fail("%s %s 는 키워드 %s 로 시작해 원본 lexer 가 다른 토큰으로 먼저 읽을 수 있다 — 다른 이름 + 설명(이름 : %s)으로", what, diagram.Quote(diagram.Clip(id)), w, diagram.Clip(id))
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

func (p *parser) nameCount(s string) error {
	if n := strings.Count(s, ",") + 1; n > maxClassNameList {
		return p.fail("이름 목록이 %d 개를 넘음", maxClassNameList)
	}
	return nil
}

// labelOK 는 라벨(설명·전이 라벨·복합 상태 설명 — 엔티티 전처리 뒤, 앞뒤 공백을 지운 것)을 본다. 원본은 라벨을 마크다운으로
// 그린다(labelType markdown — dataFetcher) — 마크다운·HTML 로 그려질 글자, lexer 가 주석·문장 구분·다른 토큰으로 읽는 글자
// (#·;·%·"·`·{·}·[·] — <<fork>>·[[fork]] 꼴 포함)를 받지 않는다. `:` 는 원본 설명 토큰(DESCR \s*":"(?:[^:\n;]|":"[^:\n;])+
// — jison:160)이 같은 값으로 읽는 꼴만 — 글 사이에 하나씩(:: 없음, 첫 글자·끝 글자가 아님).
func (p *parser) labelOK(s, what string) error {
	if s == "" {
		return p.fail("%s이 비어 있음", what)
	}
	for _, r := range s {
		switch {
		case r < 0x20 && r != '\t', r == 0x7f:
			return p.fail("%s에 제어 문자가 있음", what)
		case strings.ContainsRune("#;\"`%\\<>&*~$[]{}", r):
			return p.fail("%s에 %s 를 쓰지 못 함 — 원본은 라벨을 마크다운·HTML 로 그리거나 주석·기호로 읽는다(글자는 엔티티 #35; 꼴로)", what, diagram.Quote(string(r)))
		}
	}
	if strings.Contains(s, "::") || strings.HasPrefix(s, ":") || strings.HasSuffix(s, ":") {
		return p.fail("%s의 : 는 글 사이에 하나씩만 — 원본은 ::·끝 : 를 설명으로 읽지 못 하거나 다른 값으로 읽는다", what)
	}
	if !underscoreOK(s) {
		return p.fail("%s의 _ 가 글자 사이가 아님 — 원본은 마크다운으로 그려 기울임이 될 수 있다", what)
	}
	if reMDStart.MatchString(s) {
		return p.fail("%s이 -·+·= 나 「수.」로 시작함 — 원본은 마크다운 목록·구분선으로 그린다", what)
	}
	return nil
}

var reMDStart = regexp.MustCompile(`^(?:[-+=]|\d{1,9}[.)](?:[ \t]|$))`)

// entityFail 은 정책 B 가 받지 않는 엔티티(bad — `#…;`)의 *Error 다(문구는 공통 diagram.EntityProblem).
func (p *parser) entityFail(what, bad string) *Error {
	return p.fail("%s", diagram.EntityProblem(what, bad))
}

// label 은 보이는 라벨(설명·전이 라벨·복합 상태 설명) 하나를 보고 푼다 — 문법상 앞뒤 빈칸을 지운 뒤 labelOK, 안쪽 빈칸은
// 한 칸씩만(연속 빈칸·탭은 원본 화면의 배치를 확인하지 못해 받지 않는다 — 접근 방식 리뷰 5장), 엔티티 표식은 restore 로 한 번.
// 되돌린 글은 labelOK 를 다시 보지 않는다(금지 글자를 엔티티로 적는 통로다) — 글자 수 상한만 따로 본다.
func (p *parser) label(s, what string) (string, error) {
	s = strings.Trim(s, " \t")
	if err := p.labelOK(s, what); err != nil {
		return "", err
	}
	if strings.Contains(s, "\t") || strings.Contains(s, "  ") {
		return "", p.fail("%s 안에 연속 빈칸이나 탭이 있음 — 이 판은 글 사이 빈칸을 한 칸씩만 그린다", what)
	}
	v, bad := restore(s)
	if bad != "" {
		return "", p.entityFail(what, bad)
	}
	if err := p.xmlOK(v, what); err != nil {
		return "", err
	}
	if utf8.RuneCountInString(v) > MaxLabel {
		return "", p.fail("%s이 %d 자를 넘음", what, MaxLabel)
	}
	return v, nil
}

// xmlOK 는 받을 값(엔티티를 되돌린 것)이 최종 SVG 쓰기 경로(diagram.Esc → XmlChars)에서 그대로 남는지 본다 — XML 1.0 Char
// 밖의 글자(원문에 실제로 든 U+FFFE·U+FFFF 등)는 그 경로가 U+FFFD 로 바꾸므로, 받은 글자와 SVG 글자가 같도록 그 줄을 그리지
// 않는다(리뷰 3회차 [1]). 공통 XmlChars 는 바꾸지 않고 묻기만 한다.
func (p *parser) xmlOK(v, what string) error {
	if msg := xmlProblem(v, what); msg != "" {
		return p.fail("%s", msg)
	}
	return nil
}

// xmlProblem 은 v 가 XmlChars 를 지나며 바뀌면 그 까닭(줄 번호 없는 경고 문구)이다 — 개별 검사(xmlOK)와 출력 관문(gate)이
// 같은 문구를 쓴다(공통 diagram.XMLProblem).
func xmlProblem(v, what string) string { return diagram.XMLProblem(v, what) }

// eachText 는 모델에서 SVG·Result 로 나가는 사용자 글자를 하나씩 f 에 넘긴다(field 는 「형.필드」, what 은 경고 문구의
// 이름, line 은 그 값이 나온 줄). **출력 글자 필드 목록은 여기 한곳이다** — 모델에 글 필드를 더하면 여기에도 더한다
// (TestGateCoversAllText 가 모델의 내보낸 string·[]string 필드와 견준다). 새 그림 종류도 이 꼴의 관문을 둔다(설계 2장).
func (c *chart) eachText(f func(field, what, v string, line int) error) error {
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
	for _, s := range c.States {
		if err := f("node.ID", "상태 이름", s.ID, s.Line); err != nil {
			return err
		}
		for k, d := range s.Descr {
			if err := f("node.Descr", "상태 설명", d, s.DescrLines[k]); err != nil {
				return err
			}
		}
	}
	for _, t := range c.Trans {
		if err := f("transition.Label", "전이 라벨", t.Label, t.Line); err != nil {
			return err
		}
	}
	for _, cp := range c.Comps {
		if err := f("composite.ID", "복합 상태 이름", cp.ID, cp.Line); err != nil {
			return err
		}
		if err := f("composite.Title", "복합 상태 설명", cp.Title, cp.Line); err != nil {
			return err
		}
	}
	return nil
}

// gate 는 출력 관문이다 — 내보낼 사용자 글자가 모두 diagram.XmlChars(SVG 쓰기 경로 Esc 와 Result.Caption)를 지나도 그대로인지
// 보고, 바뀌는 값이 있으면 그 값이 나온 줄의 *Error 다(받은 글자 = 출력 글자 — 리뷰 3·4회차, itda-hyve#208 규칙으로 구조로 닫음).
func (c *chart) gate() error {
	return c.eachText(func(_, what, v string, line int) error {
		if msg := xmlProblem(v, what); msg != "" {
			return &Error{Line: line, Msg: msg}
		}
		return nil
	})
}

// accValue 는 accTitle:·accDescr: 의 값을 읽는다 — 원본은 값을 trim 한다(jison:268·269), 안쪽 빈칸·탭은 그대로다. 빈 값은 받지
// 않는다(공통 diagram.AccValue — 여섯 종류가 같다).
func (p *parser) accValue(v string, dst *string, line *int, kw string) error {
	s, msg := diagram.AccValue(v, kw)
	if msg != "" {
		return p.fail("%s", msg)
	}
	*dst, *line = s, p.line
	return nil
}

// accBody 는 accDescr { … } 의 한 줄(여는 줄의 { 뒤 포함 — 앞 빈칸만 지운 것, 끝 빈칸은 그대로)을 모은다(공통 diagram.AccBlock —
// 블록 전체 trim 뒤 원본 setAccDescription 의 \n\s+ → \n, 닫는 } 는 줄 끝에만, 빈 블록은 앞 설명을 그대로 둔다).
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
		return nil // 빈 블록은 앞 설명을 그대로 둔다(원본 \s* 가 { 뒤 빈칸·줄바꿈을 먹고 } 규칙은 상태만 닫는다 — jison:93·94)
	}
	return p.accSet(v, &p.c.AccDescr, &p.c.AccDescrLine, "accDescr")
}

// accSet 은 접근성 값(원본처럼 다듬은 것)의 엔티티 표식만 한 번 되돌려 dst 에 둔다(공통 diagram.AccSet — 맨 &·<·>·탭·줄바꿈 밖의
// 제어 문자는 받지 않는다).
func (p *parser) accSet(v string, dst *string, line *int, kw string) error {
	s, msg := diagram.AccSet(v, kw)
	if msg != "" {
		return p.fail("%s", msg)
	}
	*dst, *line = s, p.line
	return nil
}

// mention 은 상태 id 를 찾아 없으면 더한다 — 원본 dataFetcher 의 nodeDb(처음 나온 차례). 복합 상태 안에서 처음 나오거나
// 뿌리에서 먼저 나온 뒤 복합 상태 안에서 다시 나오면 그 묶음의 구성원이다(원본 insertOrUpdateNode 의 Object.assign 은
// 뿌리에서 다시 나와도 parentId 를 지우지 않는다). 두 복합 상태에 나오는 상태와 복합 상태 id 를 상태로 쓰는 것은 받지 않는다.
func (p *parser) mention(id string, kind Kind) (int, error) {
	if _, ok := p.c.compByID[id]; ok {
		return 0, p.fail("복합 상태 %s 를 전이 끝점·상태 선언으로 쓰지 못 함 — 이 판은 묶음을 끝점으로 그리지 않는다(안의 상태로 잇는다)", diagram.Quote(diagram.Clip(id)))
	}
	if k, ok := p.c.byID[id]; ok {
		st := p.c.States[k]
		if p.comp >= 0 && st.Cluster != p.comp {
			if st.Cluster >= 0 {
				return 0, p.fail("상태 %s 가 두 복합 상태(%s·%s)에 나옴 — 이 판은 그리지 않는다", diagram.Quote(diagram.Clip(id)),
					diagram.Quote(diagram.Clip(p.c.Comps[st.Cluster].ID)), diagram.Quote(diagram.Clip(p.c.Comps[p.comp].ID)))
			}
			st.Cluster = p.comp
			p.c.Comps[p.comp].n++
		}
		return k, nil
	}
	if len(p.c.States) >= MaxStates {
		return 0, p.fail("상태가 %d 개를 넘음", MaxStates)
	}
	p.c.byID[id] = len(p.c.States)
	p.c.States = append(p.c.States, &node{ID: id, Kind: kind, Cluster: p.comp, Line: p.line})
	if p.comp >= 0 {
		p.c.Comps[p.comp].n++
	}
	return len(p.c.States) - 1, nil
}

// end 는 전이의 한 끝을 읽는다(reTrans 의 끝 묶음 셋: [*]·ID 전체, ID, ::: 이름). first 는 왼끝 — [*] 는 왼끝이면 그 범위의
// 시작, 오른끝이면 끝 상태다(원본 docTranslator: 범위 id + _start·_end, 같은 범위에서는 한 상태).
func (p *parser) end(whole, id, cls string, first bool) (int, error) {
	if whole == "[*]" {
		scope := "root"
		if p.comp >= 0 {
			scope = p.c.Comps[p.comp].ID
		}
		if first {
			return p.mention(scope+"_start", Start)
		}
		return p.mention(scope+"_end", End)
	}
	if err := p.idOK(id, "상태 이름"); err != nil {
		return 0, err
	}
	if cls != "" {
		if err := p.idOK(cls, "::: 이름"); err != nil {
			return 0, err
		}
	}
	k, err := p.mention(id, Normal)
	if err != nil {
		return 0, err
	}
	if cls != "" {
		p.note("class", "class·::: 지정은 쓰지 않음 — 색·선은 테마가 정한다")
	}
	return k, nil
}

// transition 은 전이(reTrans 의 짝)를 읽는다: 1·2·3 왼끝, 4·5·6 오른끝, 7 라벨(콜론 뒤). hasLabel 은 콜론이 있었다는 뜻
// (빈 라벨 `A --> B :` 도 콜론이 있다 — 받지 않는다).
func (p *parser) transition(m []string, hasLabel bool) error {
	lbl := ""
	if hasLabel {
		s, err := p.label(m[7], "전이 라벨")
		if err != nil {
			return err
		}
		lbl = s
	}
	if len(p.c.Trans) >= MaxTransitions {
		return p.fail("전이가 %d 개를 넘음", MaxTransitions)
	}
	a, err := p.end(m[1], m[2], m[3], true)
	if err != nil {
		return err
	}
	b, err := p.end(m[4], m[5], m[6], false)
	if err != nil {
		return err
	}
	p.c.Trans = append(p.c.Trans, transition{From: a, To: b, Label: lbl, Line: p.line})
	return nil
}

// declare 는 상태 선언을 읽는다 — `ID`·`ID:::이름`·`ID : 설명`(descr 는 콜론 뒤, hasDescr 는 콜론이 있었다는 뜻)·
// `state "설명" as ID`. 설명은 원본 description 배열에 쌓인다(dataFetcher).
func (p *parser) declare(id, cls, descr string, hasDescr bool) error {
	if err := p.idOK(id, "상태 이름"); err != nil {
		return err
	}
	if cls != "" {
		if err := p.idOK(cls, "::: 이름"); err != nil {
			return err
		}
	}
	d := ""
	if hasDescr {
		s, err := p.label(descr, "상태 설명")
		if err != nil {
			return err
		}
		d = s
	}
	if k, ok := p.c.byID[id]; ok && hasDescr && p.c.States[k].Kind != Normal {
		return p.fail("선택·갈래·합류 상태 %s 에 설명을 붙이지 못 함 — 원본은 모양이 보통 상태로 바뀐다", diagram.Quote(diagram.Clip(id)))
	}
	k, err := p.mention(id, Normal)
	if err != nil {
		return err
	}
	if cls != "" {
		p.note("class", "class·::: 지정은 쓰지 않음 — 색·선은 테마가 정한다")
	}
	if hasDescr {
		st := p.c.States[k]
		if len(st.Descr) >= MaxDescrs {
			return p.fail("상태 %s 의 설명이 %d 줄을 넘음", diagram.Quote(diagram.Clip(id)), MaxDescrs)
		}
		st.Descr = append(st.Descr, d)
		st.DescrLines = append(st.DescrLines, p.line)
	}
	return nil
}

// special 은 특수 상태 선언(state ID <<choice>>·<<fork>>·<<join>>)이다. 원본 dataFetcher 는 상태를 처음 만들 때만 모양을
// 정한다 — 먼저 나온 상태를 뒤에서 특수 상태로 선언하면 모양이 바뀌지 않아 받지 않는다(처음 쓰기 전에 선언한다).
func (p *parser) special(id string, k Kind) error {
	if err := p.idOK(id, "상태 이름"); err != nil {
		return err
	}
	if _, ok := p.c.byID[id]; ok {
		return p.fail("특수 상태 %s 는 처음 쓰기 전에 선언한다 — 원본은 먼저 나온 상태의 모양을 바꾸지 않는다", diagram.Quote(diagram.Clip(id)))
	}
	_, err := p.mention(id, k)
	return err
}

// openComp 는 복합 상태를 연다(state ID { · state "설명" as ID {). 한 단계만 받는다.
func (p *parser) openComp(id, descr string, hasDescr bool) error {
	if err := p.idOK(id, "복합 상태 이름"); err != nil {
		return err
	}
	title := ""
	if hasDescr {
		if descr != strings.Trim(descr, " \t") {
			return p.fail("복합 상태 설명의 앞뒤에 빈칸을 두지 못 함 — 원본은 이 꼴의 설명을 다듬지 않는다")
		}
		s, err := p.label(descr, "복합 상태 설명")
		if err != nil {
			return err
		}
		title = s
	}
	_, twice := p.c.compByID[id]
	switch {
	case p.comp >= 0:
		return p.fail("복합 상태 안의 복합 상태(중첩)는 이 판이 그리지 않는다")
	case twice:
		return p.fail("복합 상태 %s 를 두 번 열었음 — 이 판은 그리지 않는다", diagram.Quote(diagram.Clip(id)))
	}
	if k, ok := p.c.byID[id]; ok {
		return &Error{Line: p.c.States[k].Line, Msg: fmt.Sprintf("복합 상태 %s 를 전이 끝점·상태 선언으로 쓰지 못 함 — 이 판은 묶음을 끝점으로 그리지 않는다(안의 상태로 잇는다)", diagram.Quote(diagram.Clip(id)))}
	}
	if len(p.c.Comps) >= MaxComposites {
		return p.fail("복합 상태가 %d 개를 넘음", MaxComposites)
	}
	if title == "" {
		title = id // 이름은 idPat 글자뿐이라 표식이 없다
	}
	p.c.compByID[id] = len(p.c.Comps)
	p.c.Comps = append(p.c.Comps, &composite{ID: id, Title: title, Line: p.line})
	p.comp = len(p.c.Comps) - 1
	return nil
}

func (p *parser) closeComp() error {
	cp := p.c.Comps[p.comp]
	p.comp = -1
	if cp.n == 0 {
		return &Error{Line: cp.Line, Msg: fmt.Sprintf("빈 복합 상태 %s — 이 판은 그리지 않는다(안에 상태를 하나 이상 둔다)", diagram.Quote(diagram.Clip(cp.ID)))}
	}
	return nil
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}
