package sequence

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/itda-work/itda-doc/diagram"
)

// 원문을 한 번 훑는다 — 줄마다 아래 허용 꼴(양성 목록) 하나에 통째로 맞아야 그리고, 어느 꼴에도 맞지 않으면 그 자리에서
// *Error(줄 번호)로 멈춘다(원문 길이에 선형). 허용 꼴은 mermaid@12.2.0 parser/sequenceDiagram.jison 이 같은 뜻으로 읽는다고
// 이 판이 확신할 수 있는 좁은 꼴만 모은 것이다(itda-hyve#208 두 번째 접근 방식 결정 — 2026-10-30 「허용 목록 문법」). 원본이
// 그리는 구석 꼴(공백·기호 든 참여자 이름, `;` 로 이은 여러 문장, 예약어 이름 …)은 원문 + 줄 번호 경고로 간다(의도된 축소).
//
//	한 줄에 문장 하나 — `;` 는 받지 않는다(원본 NEWLINE 토큰 jison:83). 줄 머리 %% 는 주석(jison:28)
//	ID        [\p{L}_][\p{L}\p{N}_]* — 예약어와 「예약어 + ASCII 낱말 글자 아닌 것」으로 시작하는 이름은 뺀다(idOK)
//	머리 줄   sequenceDiagram
//	참여자    (participant|actor) ID [as 라벨]
//	메시지    ID 화살표 [+|-] ID : 라벨 — 화살표 10종 -> --> ->> -->> <<->> <<-->> -x --x -) --)(jison:332–356·85–94)
//	메모      note (left of|right of|over) ID[, ID] : 라벨 — `,` 는 over 에만(jison:276–286)
//	활성      (activate|deactivate) ID
//	묶음      (loop|opt|alt|par|critical|break) [라벨] · (else|and|option) [라벨] · end · rect 색 · box [색] [이름]
//	번호      autonumber · autonumber N · autonumber N N · autonumber off(N 은 6 자리 이하 정수)
//	제목      title 라벨 · title: 라벨(jison:70–71)
//	접근성    accTitle: 값 · accDescr: 값(값은 줄 끝까지 — `;`·`#` 포함, jison:72–75) · accDescr { … }(jison:76–78)
//	무시      %%{…}%%(한 줄) · frontmatter 의 title 밖 키 · (links|link|properties|details) ID : … · rect·box 색
//	라벨      `#`·`;`·`%%` 없음(엔티티 #…; 는 전처리가 지킨 뒤 판정), wrap:/nowrap: 시작 없음, 200 자
//
// 원본 전처리(encodeEntities — 엔티티 `#…;` 를 내부 표식으로 지켜 준다)는 본문 줄마다 먼저 지나간다(timeline ② 와 같은
// 순서, 공통 diagram).

// Parse 는 mermaid sequenceDiagram 원문을 읽는다. 그릴 수 없는 문법이면 *Error 다. issues 는 쓰지 않은 줄이다.
func Parse(src string) (*chart, []Issue, error) {
	p := &parser{c: &chart{byID: map[string]int{}}, open: map[string]int{}, said: map[string]bool{}, inBox: -1}
	// 잘못된 UTF-8 은 읽기 전에 U+FFFD 로(파서가 바이트 단위로 자른 문자열을 다루지만 라벨은 룬 단위로 센다).
	// XML 이 허용하지 않는 글자는 SVG 를 쓰는 경계(diagram.Esc)에서 거른다 — timeline 과 같다.
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
	line    int             // 지금 읽는 줄(1부터)
	header  bool            // 머리 줄(sequenceDiagram)을 지났다
	open    map[string]int  // 참여자별 열린 활성 수(짝 없는 deactivate 검사 — 원본 activationCount 와 같은 셈)
	nEvents int             // 메시지·메모 합계(상한)
	nBlocks int             // 묶음 합계(상한)
	said    map[string]bool // 한 번만 알릴 Issue
	inAcc   bool            // accDescr { … } 안
	accLine int
	acc     diagram.AccBlock
	stack   []event // 열린 묶음(evBlockStart 사건 — end·else 짝 검사)
	inBox   int     // 열린 box(chart.Boxes 차례, 없으면 -1)
	raw     string  // 지금 줄의 원문(엔티티 전처리 뒤, 줄 끝 CR 만 지운 것) — accDescr { 여는 줄의 값
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

// space 는 원본 lexer 가 건너뛰는 줄 안 공백 중 이 판이 줄 앞뒤에서 지우는 것이다(ASCII 만 — 원본 \s 의 NBSP 등은
// 지우지 않아 그 줄은 허용 꼴에 맞지 않는다. U+0085 처럼 Go 와 JS 의 공백 판정이 갈리는 글자를 피한다).
const space = " \t\r\f\v"

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
		t := strings.Trim(lines[i], space)
		trail := strings.TrimLeft(lines[i], space)[len(t):] // 줄 끝 공백(autonumber NUM 의 (?=[ \n]+) 검사)
		// 원문에 이미 들어 있는 Mermaid 내부 표식 문자(encodeEntities 의 ﬂ°·¶ß)는 그리지 않는다(2026-10-30 사용자
		// 결정 — timeline ② 리뷰 3회차 [2] 와 같다). 검사는 전처리가 만드는 표식과 겹치지 않게 원문 상태에서 한다.
		// ASCII 밖 공백·서식 문자는 줄째 그리지 않는다 — 원본 JS \s 와 Go 의 공백 판정이 갈리는 갈래를 통째로 닫는다
		// (2026-10-30 사용자 결정 — 리뷰 6회차 [1]: accDescr {U+FEFF} 가 앞 설명을 덮어씀). 줄 끝 공백까지 본다.
		if r, ok := oddSpace(lines[i]); ok {
			return p.fail("ASCII 밖 공백·서식 문자(U+%04X)가 있음 — 이 판은 그 줄을 그리지 않는다(일반 빈칸으로 바꿔 쓴다)", r)
		}
		if diagram.HasMarker(t) {
			return p.fail("본문 줄에 Mermaid 내부 표식 문자(ﬂ°·¶ß)가 있음 — 이 판은 그 줄을 그리지 않는다(엔티티 표기 #…; 는 그대로 쓸 수 있다)")
		}
		// 설정 지시 — 줄 전체가 지시 하나인 줄(accDescr 블록 밖)만 무시하는 지시다. 그 밖에 %%{ 가 든 줄(지시 앞뒤·사이의
		// 글, acc 값·주석 안, accDescr 블록 안)은 원본 removeDirectives 가 지시만 지우고 남은 글을 읽는 갈래라 그리지
		// 않는다(공통 diagram.DirectiveProblem — er ④ 리뷰 1회차에서 같은 구멍을 찾았다).
		if msg := diagram.DirectiveProblem(t, p.inAcc); msg != "" {
			return p.fail("%s", msg)
		}
		// 줄 머리 %% 주석 — 원본 전처리 cleanupComments(/^\s*%%(?!{)[^\n]+\n?/gm)가 lexer 상태와 관계없이(accDescr { … } 블록 안도)
		// 먼저 지운다. 그래서 지시 검사 뒤, 엔티티 전처리·상태 분기 앞에 둔다(er 와 같은 차례 — itda-hyve#409 ④-1 후속 리뷰 1회차 [2]).
		// 뒤에 글이 없는 맨 %% 는 원본 전처리가 지우지 않는다. 블록 밖에서는 원본 lexer 의 주석 규칙(\%%(?!\{)[^\n]* —
		// jison:28)이 건너뛰므로 이 판도 경고 없이 건너뛴다. accDescr { … } 블록 안에서는 원본이 설명 글로 넣는다 — 이 판은
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
		// accDescr { … } 블록의 줄들도 똑같이 지나간다(timeline ② 리뷰 3회차 [1] 과 같다).
		t = diagram.EncodeEntities(t)
		if p.inAcc { // 블록 글은 줄 끝 빈칸까지 값이다(diagram.AccBlock)
			if err := p.accBody(diagram.EncodeEntities(strings.TrimLeft(strings.TrimSuffix(lines[i], "\r"), space))); err != nil {
				return err
			}
			continue
		}
		if t == "" {
			continue
		}
		p.raw = diagram.EncodeEntities(strings.TrimSuffix(lines[i], "\r"))
		if err := p.statement(t, trail); err != nil {
			return err
		}
	}
	if p.inAcc {
		return &Error{Line: p.accLine, Msg: "accDescr { 가 닫히지 않음"}
	}
	if p.inBox >= 0 {
		return p.fail("box 가 end 로 닫히지 않음")
	}
	if len(p.stack) > 0 {
		f := p.stack[0]
		return &Error{Line: f.line, Msg: fmt.Sprintf("%s 가 end 로 닫히지 않음", blockName[f.block])}
	}
	if !p.header {
		p.line = 0
		return p.fail("mermaid 그림이 비어 있음")
	}
	if len(p.c.Actors) == 0 {
		p.line = 0
		return p.fail("그릴 참여자가 없음")
	}
	// 참여자 없는 빈 box 는 그리지 않는다(timeline 의 빈 section 과 같은 경고)
	for _, b := range p.c.Boxes {
		if b.Count == 0 {
			p.issues = append(p.issues, Issue{Line: b.Line, Msg: fmt.Sprintf("빈 box %s 는 그리지 않음", diagram.Quote(b.Name))})
		}
	}
	return nil
}

// 허용 꼴. 지시자는 대소문자를 가리지 않는다 — 원본 %options case-insensitive 는 JS 정규식의 i 깃발(u 깃발 없음)이라
// ASCII 만 접는다(ſ U+017F·K U+212A 는 s·k 가 아니다 — 리뷰 4회차 [1]). Go 의 (?i) 는 유니코드로 접으므로 쓰지 않고,
// 정규식은 소문자 지시자로 적어 ASCII 만 소문자로 바꾼 줄(match 의 low)에 맞춘다. 지시자 뒤는 공백을 요구한다(원본
// jison-lex 가 낱말로 끝나는 규칙에 붙이는 \b 보다 좁다).
const idPat = `[\p{L}_][\p{L}\p{N}_]*`

var (
	reDecl     = regexp.MustCompile(`^(participant|actor)\s+(` + idPat + `)(?:\s+as\s+(.+))?$`)
	reMsg      = regexp.MustCompile(`^(` + idPat + `)\s*(->>|<<->>|-->>|<<-->>|->|-->|-x|--x|-\)|--\))\s*([+-]?)\s*(` + idPat + `)\s*:(.*)$`)
	reNote     = regexp.MustCompile(`^note\s+(left of|right of|over)\s+(` + idPat + `)(?:\s*,\s*(` + idPat + `))?\s*:(.*)$`)
	reActivate = regexp.MustCompile(`^(activate|deactivate)\s+(` + idPat + `)$`)
	reBlock    = regexp.MustCompile(`^(loop|opt|alt|par|critical|break)(?:\s+(.*))?$`)
	reSection  = regexp.MustCompile(`^(else|and|option)(?:\s+(.*))?$`)
	reEnd      = regexp.MustCompile(`^end$`)
	reRect     = regexp.MustCompile(`^rect\s+(.+)$`)
	reBox      = regexp.MustCompile(`^box(?:\s+(.*))?$`)
	reAuto     = regexp.MustCompile(`^autonumber(?:\s+(off)|\s+(\d{1,6})(?: +(\d{1,6}))?)?$`)
	reTitle    = regexp.MustCompile(`^title:?\s+(.+)$`)
	reLink     = regexp.MustCompile(`^(links|link|properties|details)[ \t]+(` + idPat + `)\s*:`)
	reAccTitle = regexp.MustCompile(`^acctitle\s*:\s*(.*)$`)
	reAccDescr = regexp.MustCompile(`^accdescr\s*:\s*(.*)$`)
	reAccBlock = regexp.MustCompile(`^accdescr\s*\{(.*)$`)
)

// oddSpace 는 줄에서 ASCII 밖 공백·서식 문자를 찾는다(공통 diagram.OddSpace — er 가 함께 쓴다).
func oddSpace(s string) (rune, bool) { return diagram.OddSpace(s) }

// match 는 줄을 허용 꼴 하나에 맞춘다 — ASCII 만 소문자로 바꾼 줄 low(바이트 길이가 같다)에서 찾고, 값은 원래 줄 t 의
// 같은 자리를 잘라 온다(이름·라벨의 대소문자는 그대로).
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

// statement 는 줄 하나(앞뒤 공백을 지운 것)를 허용 꼴에 맞춰 읽는다. trail 은 지운 줄 끝 공백이다.
func (p *parser) statement(t, trail string) error {
	low := asciiLower(t)
	if !p.header {
		if low != "sequencediagram" {
			return p.fail("mermaid 그림 종류를 알 수 없음 — 첫 줄이 sequenceDiagram 이 아님(%s)", diagram.Quote(diagram.Clip(firstWord(t))))
		}
		p.header = true
		return nil
	}
	// 접근성 — 값은 줄 끝까지(`;`·`#`·`%%` 포함, 원본 acc_title/acc_descr 상태의 [^\n]* — jison:72–75)
	acc := []struct {
		re   *regexp.Regexp
		dst  *string
		line *int
		kw   string
	}{{reAccTitle, &p.c.AccTitle, &p.c.AccTitleLine, "accTitle"}, {reAccDescr, &p.c.AccDescr, &p.c.AccDescrLine, "accDescr"}}
	for _, a := range acc {
		if m := match(a.re, t, low); m != nil {
			if p.inBox >= 0 {
				return p.fail("box 안에는 participant·actor 선언만 올 수 있다(원본도 같다)")
			}
			// 정책 B — 원본처럼 앞뒤만 trim, 안쪽 빈칸·탭은 그대로, 맨 &·<·>·제어 문자(Cc)는 원문(공통 diagram.AccValue)
			v, msg := diagram.AccValue(m[1], a.kw)
			if msg != "" {
				return p.fail("%s", msg)
			}
			*a.dst, *a.line = v, p.line
			return nil
		}
	}
	if m := match(reAccBlock, t, low); m != nil {
		if p.inBox >= 0 {
			return p.fail("box 안에는 participant·actor 선언만 올 수 있다(원본도 같다)")
		}
		p.acc.Reset() // 블록형 — 선언마다 버퍼를 새로 시작한다(마지막 선언이 이긴다)
		p.inAcc, p.accLine = true, p.line
		// { 뒤의 글은 줄 끝 빈칸까지 값이다 — 앞뒤를 지운 t 가 아니라 원래 줄에서 첫 { 뒤를 자른다(t 의 첫 { 와 같은 자리)
		_, rest, _ := strings.Cut(p.raw, "{")
		return p.accBody(rest)
	}
	// 그 밖의 문장 — 라벨에 `#`(엔티티 밖)·`;`·`%%` 가 있으면 원본은 그 자리에서 주석·다음 문장으로 넘어간다.
	if strings.Contains(t, "#") {
		return p.fail("줄의 # 부터는 Mermaid 가 주석으로 버리는 자리다 — 이 판은 그리지 않는다(주석은 %s, 엔티티 표기는 #35; 꼴)", "%%")
	}
	if strings.Contains(t, ";") {
		return p.fail("`;` 로 문장을 잇지 못 함 — 이 판은 한 줄에 문장 하나만 읽는다(원본은 `;` 에서 다음 문장으로 넘어간다)")
	}
	if strings.Contains(t, "%%") {
		return p.fail("줄 중간의 %s 는 쓰지 못 함 — 주석은 줄 머리에 둔다", "%%")
	}
	// box 안에는 참여자 선언과 그 box 를 닫는 end 만 온다(원본 문법 box_line = participant_statement).
	if p.inBox >= 0 {
		if m := match(reDecl, t, low); m != nil {
			return p.declare(m[1], m[2], m[3])
		}
		if reEnd.MatchString(low) {
			p.inBox = -1
			return nil
		}
		return p.fail("box 안에는 participant·actor 선언만 올 수 있다(원본도 같다): %s", diagram.Clip(t))
	}
	if m := match(reDecl, t, low); m != nil {
		return p.declare(m[1], m[2], m[3])
	}
	if m := match(reNote, t, low); m != nil {
		return p.noteStmt(m[1], m[2], m[3], m[4])
	}
	if m := match(reActivate, t, low); m != nil {
		return p.activate(m[2], asciiLower(m[1]) == "activate")
	}
	if m := match(reBlock, t, low); m != nil {
		return p.startBlock(asciiLower(m[1]), m[2])
	}
	if m := match(reSection, t, low); m != nil {
		return p.section(asciiLower(m[1]), m[2])
	}
	if reEnd.MatchString(low) {
		return p.endBlock()
	}
	if m := match(reRect, t, low); m != nil {
		return p.startRect(m[1])
	}
	if m := match(reBox, t, low); m != nil {
		return p.startBox(m[1])
	}
	if m := match(reAuto, t, low); m != nil {
		return p.autonumber(m, trail)
	}
	if m := match(reTitle, t, low); m != nil {
		name, err := p.label(m[1], "제목")
		if err != nil {
			return err
		}
		one, msg := diagram.OneLine(name, "제목") // 제목은 한 줄(timeline·gantt 와 같게 — <br> 도 공백으로)
		if msg != "" {
			return p.fail("%s", msg)
		}
		p.c.Title, p.c.TitleLine = one, p.line
		return nil
	}
	if m := match(reLink, t, low); m != nil {
		// 링크 메뉴는 내지 않는다. 원본은 이 문장의 참여자를 먼저 더한다(jison:290–316 의 [$2, …]) — 같은 차례로 더한다.
		if err := p.idOK(m[2]); err != nil {
			return err
		}
		if err := p.reference(m[2]); err != nil {
			return err
		}
		p.note("linkmenu", fmt.Sprintf("링크 메뉴(%s)는 내지 않는다 — 그림에서 링크를 걸지 않는다", asciiLower(m[1])))
		return nil
	}
	if m := match(reMsg, t, low); m != nil {
		return p.message(m[1], m[2], m[3], m[4], m[5])
	}
	return p.unknown(t)
}

// hints 는 허용 꼴에 맞지 않은 줄의 머리 낱말별 안내다(어느 꼴로 써야 하는지).
var hints = map[string]string{
	"participant": "참여자 선언은 `participant 이름` · `participant 이름 as 별명` 꼴(이름은 글자·숫자·_ — 공백·기호는 as 별명에)",
	"actor":       "참여자 선언은 `actor 이름` · `actor 이름 as 별명` 꼴(이름은 글자·숫자·_ — 공백·기호는 as 별명에)",
	"note":        "메모는 `Note left of 이름: 글` · `Note right of 이름: 글` · `Note over 이름[,이름]: 글` 꼴",
	"activate":    "활성은 `activate 이름` 꼴",
	"deactivate":  "활성은 `deactivate 이름` 꼴",
	"autonumber":  "번호는 `autonumber` · `autonumber 10` · `autonumber 10 5`(6 자리 이하 정수, 수 사이는 빈칸) · `autonumber off` 꼴",
	"title":       "title 뒤에 제목이 없거나 꼴이 다름 — `title 제목` · `title: 제목`(콜론 뒤 빈칸)",
	"end":         "end 뒤에 글이 있음 — 원본도 읽지 못 한다",
	"rect":        "rect 뒤에 색이 없음 — `rect rgb(1,2,3)` 꼴",
	"create":      "create 참여자는 이 판이 그리지 않는다",
	"destroy":     "destroy 참여자는 이 판이 그리지 않는다",
	"par_over":    "par_over 묶음은 이 판이 그리지 않는다",
	"link":        "링크 메뉴는 `link 이름: …` 꼴(이 판은 쓰지 않고 넘어간다)",
	"links":       "링크 메뉴는 `links 이름: …` 꼴(이 판은 쓰지 않고 넘어간다)",
	"properties":  "`properties 이름: …` 꼴(이 판은 쓰지 않고 넘어간다)",
	"details":     "`details 이름: …` 꼴(이 판은 쓰지 않고 넘어간다)",
}

// unknown 은 허용 꼴 어디에도 맞지 않은 줄의 *Error 다 — 까닭을 짐작할 수 있는 것은 그 안내를 붙인다.
func (p *parser) unknown(t string) error {
	head := asciiLower(firstWord(t))
	if strings.HasPrefix(head, "title:") { // title:제목(콜론 뒤 빈칸 없음)
		head = "title"
	}
	switch {
	case strings.Contains(t, "@{"):
		return p.fail("참여자 설정(@{ … })은 이 판이 그리지 않는다 — participant 이름·as 별명만 쓴다")
	case strings.Contains(t, "()"):
		return p.fail("가운데 연결(() 화살표는 이 판이 그리지 않는다")
	case hints[head] != "":
		return p.fail("%s(%s)", hints[head], diagram.Clip(t))
	case strings.ContainsAny(t, `/\`) && strings.Contains(t, "-"):
		return p.fail("반쪽 화살표(-|\\ · -|/ · /|- … 16종)는 이 판이 그리지 않는다 — 고전 10종(-> --> ->> -->> <<->> <<-->> -x --x -) --))만 그린다")
	case strings.Contains(t, "->") || strings.Contains(t, "-x") || strings.Contains(t, "-)"):
		left, _, colon := strings.Cut(t, ":")
		if !colon {
			return p.fail("메시지에 콜론(:)이 없음 — `A->>B: 글` 처럼 쓴다(%s)", diagram.Clip(t))
		}
		return p.fail("메시지의 참여자 이름 %s 를 읽지 못 함 — 이름은 글자·숫자·_ 만(공백·기호는 participant … as 별명에)", diagram.Clip(strings.TrimSpace(left)))
	}
	return p.fail("알 수 없는 문장(%s) — 10장 지원 표의 꼴로 쓴다", diagram.Clip(t))
}

// keywordsB 는 원본 lexer 가 낱말로 읽는 지시자 중 ACTOR 규칙(jison:84)보다 앞에 있고 낱말 끝 \b 가 붙는 것이다
// (jison-lex 는 낱말 글자로 끝나는 규칙에 \b 를 붙이고, 처음 맞는 규칙을 쓴다 — jison:39–80). JS \b 는 ASCII 낱말
// 글자만 보므로 `end가` 처럼 지시자 뒤에 한글 등이 붙은 이름은 지시자로 먼저 읽힌다.
var keywordsB = []string{
	"box", "participant", "actor", "create", "destroy", "loop", "rect", "opt", "alt", "else", "par", "par_over", "and",
	"critical", "option", "break", "end", "over", "note", "activate", "deactivate", "sequencediagram", "autonumber", "off",
}

// reservedIDs 는 참여자 이름으로 받지 않는 낱말이다(ASCII 대소문자 무시) — keywordsB 와 \b 없이 앞자리를 먹는 낱말
// (title·accTitle·accDescr — 뒤 공백·콜론에 따라 문장이 된다)·링크 메뉴 지시자·as.
var reservedIDs = append([]string{"title", "acctitle", "accdescr", "links", "link", "properties", "details", "as"}, keywordsB...)

// idOK 는 참여자 이름이 허용 꼴(idPat) 안에서 원본 lexer 가 ACTOR 로 읽는 이름인지 본다 — 예약어 그 자체, 또는
// \b 지시자 뒤에 ASCII 낱말 글자가 아닌 글자가 붙은 이름(`end가`)은 원문 + 경고. 선언(ID 상태)에서는 원본이 읽지만
// 같은 규칙으로 막는다(단순화 — 선언한 이름은 메시지·메모에서 다시 쓰이므로).
func (p *parser) idOK(id string) error {
	low := asciiLower(id)
	for _, w := range reservedIDs {
		if low == w {
			return p.fail("참여자 이름 %s 는 예약어라 쓰지 못 함 — 원본 lexer 가 지시자로 먼저 읽는다(다른 이름 + as 별명으로)", diagram.Quote(id))
		}
	}
	for _, w := range keywordsB {
		if len(low) > len(w) && strings.HasPrefix(low, w) && !isWordByte(low[len(w)]) {
			return p.fail("참여자 이름 %s 는 예약어 %s 로 시작해 원본 lexer 가 지시자로 먼저 읽는다 — 다른 이름 + as 별명으로", diagram.Quote(id), w)
		}
	}
	return nil
}

func asciiLower(s string) string { return diagram.ASCIILower(s) } // 공통 — ASCII 만 접는다

func isWordByte(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// accBody 는 accDescr { … } 의 한 줄(여는 줄의 { 뒤 포함 — 앞 빈칸만 지운 것, 끝 빈칸은 그대로)을 모은다(공통 diagram.AccBlock —
// 닫는 } 는 그 줄 끝에만: 뒤에 글이 있으면 원본은 } 뒤를 다음 문장으로 읽는다, jison:77 popState — 리뷰 3회차 [1]). 원본은
// 블록 글을 통째로 trim 하고 setAccDescription 이 \n\s+ 를 \n 으로 바꾼다 — 줄바꿈·안쪽 빈칸은 그대로다(정책 B).
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
		// 빈 블록(공백·빈 줄뿐)은 앞 설명을 그대로 둔다 — 원본은 여는 토큰 accDescr\s*"{"\s* 가 공백을 먹고 곧장
		// } 규칙이 상태만 닫아 acc_descr_multiline_value 가 생기지 않는다(jison:76–78·190 — 리뷰 5회차 [1])
		return nil
	}
	s, msg := diagram.AccSet(v, "accDescr")
	if msg != "" {
		return p.fail("%s", msg)
	}
	p.c.AccDescr, p.c.AccDescrLine = s, p.line
	return nil
}

// declare 는 participant·actor 선언을 읽는다(별명 없는 다시 적기는 아무 것도 바꾸지 않고, 별명 있는 다시 적기는 이름·
// 모양을 바꾼다 — 원본 addActor). alias 는 as 뒤 라벨(없으면 "").
func (p *parser) declare(kw, id, alias string) error {
	if err := p.idOK(id); err != nil {
		return err
	}
	isActor := asciiLower(kw) == "actor"
	// 표시 이름은 별명이 없어도 늘 라벨 경로를 지난다(200자 상한). 참조용 ID 는 원문 그대로.
	name, err := p.text(id)
	if err != nil {
		return err
	}
	if alias != "" {
		if name, err = p.label(alias, "참여자 별명"); err != nil {
			return err
		}
	}
	if k, ok := p.c.byID[id]; ok {
		a := p.c.Actors[k]
		// box 충돌 검사·소속 갱신은 별명 없는 다시 적기의 조기 반환보다 앞선다(원본 addActor 의 순서 — 리뷰 1회차 [1]).
		if a.Box >= 0 && p.inBox >= 0 && a.Box != p.inBox {
			return p.fail("참여자 %s 는 이미 다른 box 에 있다 — 원본도 같은 참여자를 두 box 에 두지 못 한다", diagram.Quote(a.Name))
		}
		if a.Box < 0 && p.inBox >= 0 { // box 안에서 다시 정의되면(별명 없어도) 그 box 에 든다
			a.Box = p.inBox
			p.c.Boxes[p.inBox].Count++
		}
		if alias == "" {
			return nil // 별명 없는 다시 적기 — 이름·모양은 바꾸지 않는다(원본 early return)
		}
		a.Name, a.Actor, a.NameLine = name, isActor, p.line
		return nil
	}
	if len(p.c.Actors) >= MaxActors {
		return p.fail("참여자가 %d 개를 넘음", MaxActors)
	}
	p.c.byID[id] = len(p.c.Actors)
	p.c.Actors = append(p.c.Actors, &participant{ID: id, Name: name, Actor: isActor, Box: p.inBox, Line: p.line, NameLine: p.line})
	if p.inBox >= 0 {
		p.c.Boxes[p.inBox].Count++
	}
	return nil
}

// startBox 는 box [색] [이름] … end 를 연다. 색 값은 쓰지 않고 경고한다(띠는 테마 토큰) — 색 경계는 stripBoxColor.
func (p *parser) startBox(total string) error {
	if len(p.c.Boxes) >= MaxBoxes {
		return p.fail("box 가 %d 개를 넘음", MaxBoxes)
	}
	name, hadColor, err := p.stripBoxColor(total)
	if err != nil {
		return err
	}
	t, err := p.label(name, "box 이름")
	if err != nil {
		return err
	}
	if hadColor {
		p.note("boxcolor", "box 의 색은 쓰지 않음 — 색은 테마가 정한다")
	}
	p.inBox = len(p.c.Boxes)
	p.c.Boxes = append(p.c.Boxes, &box{Name: t, Line: p.line, First: len(p.c.Actors)})
	return nil
}

var blockKinds = map[string]int{"loop": bLoop, "opt": bOpt, "alt": bAlt, "par": bPar, "critical": bCritical, "break": bBreak, "rect": bRect}

// startBlock 은 loop·opt·alt·par·critical·break·rect … end 를 연다(머리 글은 비어도 된다 — 원본과 같다).
func (p *parser) startBlock(kw, cond string) error {
	if len(p.stack) >= MaxDepth {
		return p.fail("묶음 깊이가 %d 을 넘음", MaxDepth)
	}
	if p.nBlocks >= MaxBlocks {
		return p.fail("묶음이 %d 개를 넘음", MaxBlocks)
	}
	p.nBlocks++
	title := ""
	if kw != "rect" {
		var err error
		if title, err = p.label(cond, "묶음 조건 글"); err != nil {
			return err
		}
	}
	ev := event{kind: evBlockStart, line: p.line, block: blockKinds[kw], text: title}
	p.c.Events = append(p.c.Events, ev)
	p.stack = append(p.stack, ev)
	return nil
}

// startRect 는 rect 색 … end 를 연다 — 색은 이 판이 색으로 확신할 수 있는 꼴(isColor)만 받고, 쓰지 않는다(경고).
func (p *parser) startRect(color string) error {
	if !isColor(color) {
		return p.fail("rect 의 색 %s 를 확신할 수 없음 — 색 이름 또는 rgb/rgba/hsl/hsla(…) 꼴만 쓴다(색은 테마가 정한다)", diagram.Clip(color))
	}
	if err := p.startBlock("rect", ""); err != nil {
		return err
	}
	p.note("rectcolor", "rect 의 색은 쓰지 않음 — 색은 테마가 정한다")
	return nil
}

// section 은 else·and·option 구획을 연다 — 열린 묶음의 안쪽이 그 종류여야 한다(원본 문법: else 는 alt 안에, and 는
// par 안에, option 은 critical 안에 — 다른 묶음 안이면 원본도 읽지 못 한다).
func (p *parser) section(kw, cond string) error {
	want := map[string]int{"else": bAlt, "and": bPar, "option": bCritical}[kw]
	if len(p.stack) == 0 {
		return p.fail("%s 가 %s 묶음 밖에 있음", kw, blockName[want])
	}
	top := p.stack[len(p.stack)-1]
	if top.block != want {
		return p.fail("%s 는 %s 안에 쓴다(지금 열려 있는 것은 %s)", kw, blockName[want], blockName[top.block])
	}
	title, err := p.label(cond, "묶음 조건 글")
	if err != nil {
		return err
	}
	p.c.Events = append(p.c.Events, event{kind: evBlockElse, line: p.line, block: want, section: true, text: title})
	return nil
}

func (p *parser) endBlock() error {
	if len(p.stack) == 0 {
		return p.fail("end 짝이 없음 — loop·alt·opt·par·critical·break·rect·box 를 닫는 end 만 쓴다")
	}
	p.stack = p.stack[:len(p.stack)-1]
	p.c.Events = append(p.c.Events, event{kind: evBlockEnd, line: p.line})
	return nil
}

// activate 는 activate·deactivate 문장을 읽는다. 참여자는 이미 정의되어 있어야 한다(원본은 정의되지 않은 참여자를
// 활성화하다 렌더에 실패한다). 짝 없는 deactivate 는 원본 DB 도 오류다.
func (p *parser) activate(id string, on bool) error {
	if err := p.idOK(id); err != nil {
		return err
	}
	if _, ok := p.c.byID[id]; !ok {
		return p.fail("참여자 %s 가 없음 — participant 문장이나 메시지로 먼저 나와야 한다", diagram.Quote(diagram.Clip(id)))
	}
	kind := evActiveStart
	if on {
		p.open[id]++
	} else {
		if p.open[id] < 1 {
			return p.fail("활성되지 않은 참여자 %s 를 비활성함 — 원본도 오류로 돌린다", diagram.Quote(diagram.Clip(id)))
		}
		p.open[id]--
		kind = evActiveEnd
	}
	p.c.Events = append(p.c.Events, event{kind: kind, line: p.line, from: id})
	return nil
}

// cssColors 는 CSS 의 color 값으로 쓰이는 키워드의 닫힌 목록이다(원본 parseBoxData 가 window.CSS.supports('color', …)
// 으로 가려 내는 것 — box 줄의 앞 낱말이 색이면 색 값으로 읽고, 아니면 이름의 일부로 남긴다. `#hex` 는 원본도 쓰지
// 못 한다 — # 주석 때문). 「목록에 없으면 색이 아니다」는 이 목록이 닫혀 있다는 데 기댄다(리뷰 4회차 [2]):
//   - 명색 148개 + transparent — W3C CSS Color Module Level 4 §6.1 Named Colors·§6.3 transparent
//   - currentcolor — 같은 문서 §6.4
//   - 시스템 색 19개(AccentColor … VisitedText)와 폐기 예정 시스템 색 23개(ActiveBorder … WindowText) — 같은 문서
//     §6.2 System Colors·부록 A Deprecated CSS System Colors(폐기 예정이어도 브라우저는 받는다)
//   - 모든 속성 공통 키워드 inherit·initial·unset·revert·revert-layer — CSS Cascade Level 5 §7.3(revert-layer 는
//     원본 \w* 가 revert 까지만 잘라 box 머리에서는 원문 + 경고가 된다; rect 색으로는 받는다)
//
// 브라우저 고유의 비표준 키워드(-webkit-…·옛 이름)는 목록에 없다 — 미검증.
// 키는 ASCII 소문자이고 찾을 때도 asciiLower 로 접는다(CSS 키워드는 ASCII 대소문자만 무시한다).
var cssColors = map[string]bool{}

func init() {
	for _, c := range strings.Fields(`aliceblue antiquewhite aqua aquamarine azure beige bisque black blanchedalmond blue
blueviolet brown burlywood cadetblue chartreuse chocolate coral cornflowerblue cornsilk crimson cyan darkblue darkcyan
darkgoldenrod darkgray darkgreen darkgrey darkkhaki darkmagenta darkolivegreen darkorange darkorchid darkred darksalmon
darkseagreen darkslateblue darkslategray darkslategrey darkturquoise darkviolet deeppink deepskyblue dimgray dimgrey
dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro ghostwhite gold goldenrod gray green greenyellow grey
honeydew hotpink indianred indigo ivory khaki lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral lightcyan
lightgoldenrodyellow lightgray lightgreen lightgrey lightpink lightsalmon lightseagreen lightskyblue lightslategray
lightslategrey lightsteelblue lightyellow lime limegreen linen magenta maroon mediumaquamarine mediumblue mediumorchid
mediumpurple mediumseagreen mediumslateblue mediumspringgreen mediumturquoise mediumvioletred midnightblue mintcream
mistyrose moccasin navajowhite navy oldlace olive olivedrab orange orangered orchid palegoldenrod palegreen paleturquoise
palevioletred papayawhip peachpuff peru pink plum powderblue purple rebeccapurple red rosybrown royalblue saddlebrown salmon
sandybrown seagreen seashell sienna silver skyblue slateblue slategray slategrey snow springgreen steelblue tan teal thistle
tomato turquoise violet wheat white whitesmoke yellow yellowgreen
transparent currentcolor inherit initial unset revert revert-layer
AccentColor AccentColorText ActiveText ButtonBorder ButtonFace ButtonText Canvas CanvasText Field FieldText GrayText Highlight
HighlightText LinkText Mark MarkText SelectedItem SelectedItemText VisitedText
ActiveBorder ActiveCaption AppWorkspace Background ButtonHighlight ButtonShadow CaptionText InactiveBorder InactiveCaption
InactiveCaptionText InfoBackground InfoText Menu MenuText Scrollbar ThreeDDarkShadow ThreeDFace ThreeDHighlight
ThreeDLightShadow ThreeDShadow Window WindowFrame WindowText`) { // transparent·currentcolor·전체 키워드도 CSS.supports 참 — 리뷰 1회차 [6]; 시스템 색 — 4회차 [2]
		cssColors[asciiLower(c)] = true
	}
}

// stripBoxColor 는 box 줄의 색 경계를 원본 정규식 ^((?:rgba?|hsla?)\s*\(.*\)|\w*)(.*)$ (sequenceDb.ts:400)
// 대로 나눈 뒤, 이 판이 확신할 수 있는 경우만 받는다(itda-hyve#208 결정 — 리뷰 2회차 [2]):
//
//	① 후보가 빈 문자열(한글 이름 등 \w 밖으로 시작) → 이름 전체
//	② 후보가 색(isColor) + 나머지가 공백이나 줄 끝으로 시작 → 색을 떼고(경고) 나머지가 이름
//	③ 후보가 함수 꼴도 색 이름도 아닌 일반 낱말(\w+) + 나머지 공백/끝 → 원본처럼(색 판정 실패) 이름 전체
//	그 밖의 「색일 수도 있는」 경우 — 함수 꼴인데 인자가 틀림(rgb(1)), 낱말 뒤가 공백 아닌 구두점(red-team) — 은 원문 + 경고.
func (p *parser) stripBoxColor(s string) (name string, hadColor bool, err error) {
	m := reBoxHead.FindStringSubmatch(s)
	cand, rest := m[1], m[2]
	nameStartsSpace := rest == "" || rest[0] == ' ' || rest[0] == '\t'
	switch {
	case cand == "": // ①
		return s, false, nil
	case isColor(cand) && nameStartsSpace: // ②
		return strings.TrimSpace(rest), true, nil
	case rePlainWord.MatchString(cand) && !cssColors[asciiLower(cand)] && nameStartsSpace: // ③
		return s, false, nil
	}
	return "", false, p.fail("box 머리 %s 의 색 경계를 확신할 수 없음 — 색 이름 또는 rgb/rgba/hsl/hsla(…) 꼴 뒤에 빈칸을 두고 이름을 쓴다(색은 테마가 정한다)", diagram.Clip(s))
}

var (
	reBoxHead   = regexp.MustCompile(`(?s)^((?:rgba?|hsla?)\s*\(.*\)|\w*)(.*)$`) // 원본 sequenceDb.ts:400
	rePlainWord = regexp.MustCompile(`^\w+$`)
	// 색 함수 — CSS 옛 쉼표 꼴만: rgb 는 세 값이 모두 정수이거나 모두 퍼센트, hsl 은 색상 정수 + 퍼센트 둘, 넷째
	// 알파는 수(소수 가능) 또는 퍼센트. rgb·rgba·hsl·hsla 모두 알파를 생략·포함할 수 있다(CSS Color 4 의 별칭).
	reRGB = regexp.MustCompile(`^rgba?\(\s*(?:-?\d+\s*,\s*-?\d+\s*,\s*-?\d+|-?\d+%\s*,\s*-?\d+%\s*,\s*-?\d+%)(?:\s*,\s*` + alphaPat + `)?\s*\)$`)
	reHSL = regexp.MustCompile(`^hsla?\(\s*-?\d+\s*,\s*-?\d+%\s*,\s*-?\d+%(?:\s*,\s*` + alphaPat + `)?\s*\)$`)
)

const alphaPat = `-?(?:\d+(?:\.\d+)?|\.\d+)%?`

// isColor 는 색 값이 이 판이 CSS 색으로 확신할 수 있는 꼴인지 본다 — CSS 색 이름(대소문자 무시) 또는 reRGB·reHSL.
func isColor(s string) bool {
	return cssColors[asciiLower(s)] || reRGB.MatchString(s) || reHSL.MatchString(s)
}

// autonumber 는 autonumber · autonumber N · autonumber N N · autonumber off 를 읽는다(m 은 reAuto 의 짝). 수를 적지
// 않은 `autonumber` 는 세던 수에 이어 번호를 켜기만 한다(원본 sequenceIndex = start || sequenceIndex 와 같다 — 0 도
// 「없음」으로 세던 값을 쓴다). 원본 NUM 토큰(jison:30)은 수 뒤에 빈칸이나 줄바꿈을 요구한다 — 줄 끝 공백이 탭 등이면
// 원본은 수로 읽지 않으므로 받지 않는다.
func (p *parser) autonumber(m []string, trail string) error {
	ev := event{kind: evAutonumber, line: p.line}
	switch {
	case m[1] != "":
		ev.numOff = true
	case m[2] != "":
		if trail != "" && trail[0] != ' ' {
			return p.fail("autonumber 의 수 뒤에는 빈칸이나 줄 끝만 온다(원본 NUM 토큰) — 탭 등을 두지 않는다")
		}
		ev.numStart, ev.haveStart = atof(m[2]), true
		ev.numStep, ev.haveStep = 1, true // `autonumber 10` 의 단계는 1(원본 문법이 정한 값)
		if m[3] != "" {
			ev.numStep = atof(m[3])
		}
	}
	p.c.Events = append(p.c.Events, ev)
	return nil
}

func atof(s string) float64 {
	n, _ := strconv.ParseFloat(s, 64) // reAuto 가 6 자리 이하 정수만 넘긴다
	return n
}

// noteStmt 는 Note left of A: 글 · Note right of A: 글 · Note over A: 글 · Note over A,B: 글 을 읽는다.
func (p *parser) noteStmt(where, id1, id2, text string) error {
	placement := map[string]byte{"left of": leftOf, "right of": rightOf, "over": over}[asciiLower(where)]
	if placement != over && id2 != "" {
		return p.fail("left of·right of 메모는 참여자 하나만 둔다(원본도 같다)")
	}
	ids := []string{id1}
	if id2 != "" {
		ids = append(ids, id2)
	}
	for _, id := range ids {
		if err := p.idOK(id); err != nil {
			return err
		}
	}
	msg, err := p.label(text, "메모 글")
	if err != nil {
		return err
	}
	if p.nEvents >= MaxEvents {
		return p.fail("메시지·메모 합계가 %d 개를 넘음", MaxEvents)
	}
	p.nEvents++
	for _, id := range ids {
		if err := p.reference(id); err != nil {
			return err
		}
	}
	p.c.Events = append(p.c.Events, event{kind: evNote, line: p.line, from: id1, over2: id2, placement: placement, text: msg})
	return nil
}

// arrowKinds 는 고전 화살표 10종이다(reMsg 의 화살표 짝 — 대소문자 무시로 -X 도 가위표, 원본 lexer 와 같다).
var arrowKinds = map[string]arrowKind{
	"->>": {Head: 'f'}, "<<->>": {Head: 'f', Both: true}, "-->>": {Dotted: true, Head: 'f'},
	"<<-->>": {Dotted: true, Head: 'f', Both: true}, "->": {Head: 'n'}, "-->": {Dotted: true, Head: 'n'},
	"-x": {Head: 'x'}, "--x": {Dotted: true, Head: 'x'}, "-)": {Head: 'o'}, "--)": {Dotted: true, Head: 'o'},
}

// message 는 보낸이 화살표 [+|-] 받는이 : 글 을 읽는다(원본 signal 규칙 jison:332–356 — + 는 받는이 활성, - 는 보낸이 종료).
func (p *parser) message(from, arrow, mark, to, text string) error {
	if err := p.idOK(from); err != nil {
		return err
	}
	if err := p.idOK(to); err != nil {
		return err
	}
	if mark == "-" && (to[0] == 'x' || to[0] == 'X') { // 원본 lexer 는 `-x` 를 가위표 토큰(jison:91)으로 `-` 보다 먼저 읽는다
		return p.fail("메시지 끝 - 바로 뒤의 x 로 시작하는 이름(%s)은 원본이 가위표 화살표로 읽는다 — 이름을 바꾸거나 별명을 쓴다", diagram.Quote(to))
	}
	msg, err := p.label(text, "메시지 글")
	if err != nil {
		return err
	}
	if p.nEvents >= MaxEvents {
		return p.fail("메시지·메모 합계가 %d 개를 넘음", MaxEvents)
	}
	p.nEvents++
	if err := p.reference(from); err != nil {
		return err
	}
	if err := p.reference(to); err != nil {
		return err
	}
	ev := event{kind: evMessage, line: p.line, from: from, to: to, ar: arrowKinds[asciiLower(arrow)], text: msg}
	switch mark {
	case "+": // 받는이 활성(원본 activeStart on $4)
		ev.activate = 1
		p.c.Events = append(p.c.Events, ev)
		p.open[to]++
		p.c.Events = append(p.c.Events, event{kind: evActiveStart, line: p.line, from: to})
	case "-": // 보낸이 종료(원본 activeEnd on $1)
		ev.activate = -1
		if p.open[from] < 1 {
			return p.fail("활성되지 않은 참여자 %s 를 비활성함 — 원본도 오류로 돌린다", diagram.Quote(diagram.Clip(from)))
		}
		p.c.Events = append(p.c.Events, ev)
		p.open[from]--
		p.c.Events = append(p.c.Events, event{kind: evActiveEnd, line: p.line, from: from})
	default:
		p.c.Events = append(p.c.Events, ev)
	}
	return nil
}

// reference 는 참여자를 찾아 없으면 그 자리에 만든다(선언 없이 메시지·메모에 처음 나온 이름 — 원본 addParticipant
// 사건과 같은 순서, box 에는 들어가지 않는다).
func (p *parser) reference(id string) error {
	if _, ok := p.c.byID[id]; ok {
		return nil
	}
	if len(p.c.Actors) >= MaxActors {
		return p.fail("참여자가 %d 개를 넘음", MaxActors)
	}
	name, err := p.text(id)
	if err != nil {
		return err
	}
	p.c.byID[id] = len(p.c.Actors)
	p.c.Actors = append(p.c.Actors, &participant{ID: id, Name: name, Box: -1, Line: p.line, NameLine: p.line})
	return nil
}

// label 은 라벨(별명·메시지·메모·조건·구획·box 이름·제목)을 읽는다 — wrap:/nowrap: 앞말(콜론 붙은 꼴 포함 — 원본
// extractWrap 의 ^:?(?:no)?wrap:)으로 시작하면 그리지 않는다(itda-hyve#208 결정 — 원본과 같은 뜻으로 읽을 수 없다).
// `#`·`;`·`%%` 는 statement 가 줄 단위로 먼저 막았다.
func (p *parser) label(s, where string) (string, error) {
	if reWrap.MatchString(asciiLower(strings.TrimSpace(s))) {
		return "", p.fail("%s의 wrap:/nowrap: 앞말은 이 판이 그리지 않는다 — 원본과 같은 뜻으로 읽을 수 없다(줄바꿈은 글 폭에 따른다)", where)
	}
	return p.text(s)
}

var reWrap = regexp.MustCompile(`^:?(?:no)?wrap:`) // ASCII 소문자로 바꾼 라벨에 맞춘다

// text 는 라벨 글을 정책 B 로 읽는다(공통 diagram.Label — <br> 은 줄바꿈, 엔티티 표식은 받는 것만 한 번 글자로, 앞뒤 빈칸을
// 지운 뒤 안쪽 연속 빈칸·탭·제어 문자(C0·DEL)는 원문 + 경고). 줄바꿈은 그대로 둔다(상자 안 줄바꿈).
func (p *parser) text(s string) (string, error) {
	v, msg := diagram.Label(s, "라벨", true)
	if msg != "" {
		return "", p.fail("%s", msg)
	}
	return v, nil
}

// eachText 는 모델에서 SVG·Result 로 나가는 사용자 글자를 하나씩 f 에 넘긴다(field 는 「형.필드」, what 은 경고 문구의 이름,
// line 은 그 값이 나온 줄). **출력 글자 필드 목록은 여기 한곳이다** — 모델에 글 필드를 더하면 여기에도 더한다(TestGateCoversAllText
// 가 모델의 string·[]string 필드와 견준다). 참여자 ID·메시지 끝 이름은 그리지 않지만(그리는 것은 Name) 함께 본다.
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
	for _, a := range c.Actors {
		if err := f("participant.ID", "참여자 이름", a.ID, a.Line); err != nil {
			return err
		}
		if err := f("participant.Name", "참여자 이름", a.Name, a.NameLine); err != nil {
			return err
		}
	}
	for _, b := range c.Boxes {
		if err := f("box.Name", "box 이름", b.Name, b.Line); err != nil {
			return err
		}
	}
	for _, e := range c.Events {
		for _, x := range []struct{ field, v string }{{"event.from", e.from}, {"event.to", e.to}, {"event.over2", e.over2}, {"event.text", e.text}} {
			if err := f(x.field, "라벨", x.v, e.line); err != nil {
				return err
			}
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
