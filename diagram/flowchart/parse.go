package flowchart

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/itda-work/itda-doc/diagram"
)

// 읽는 문법(Mermaid flowchart 의 부분집합). 원문을 한 번 훑는다 — 닫는 표지를 찾다 실패하면 그 자리에서 *Error 라
// 같은 글자를 두 번 읽지 않는다(원문 길이에 선형).
//
//	머리 줄      flowchart|graph [TD|TB|BT|LR|RL]   (graph 만 적으면 TB)
//	문장 구분    줄바꿈 또는 ;(따옴표·괄호·|…| 밖)
//	주석         %% 로 시작하는 줄. 줄 전체가 %%{…}%% 지시 하나인 줄은 쓰지 않음(Issue), 그 밖에 %%{ 가 든 줄은 Error
//	frontmatter  첫 줄 --- … --- 은 YAML. 글인 title 은 캡션, 나머지 키는 쓰지 않음(Issue). 깨진 YAML·글 아닌 title 은 Error
//	노드         id, id[글] (글) ([글]) [[글]] [(글)] ((글)) {글} {{글}} [/글/] [\글\] [/글\] [\글/]
//	             글은 "따옴표" 로 감쌀 수 있다. <br> 은 줄바꿈, #quot; #35; 같은 Mermaid 엔티티는 정책 B 로 푼다(text)
//	화살표       --> --- -.-> -.- ==> === ~~~, 길게(---> ----), 양쪽 <--> <-.-> <==>,
//	             글: -->|글|  -- 글 -->  -. 글 .->  == 글 ==>
//	잇기         A --> B --> C,  A & B --> C & D
//	subgraph     subgraph id [제목] … end, subgraph 제목 … end (안에 또 subgraph 는 미지원)
//	접근성       accTitle: 글, accDescr: 글, accDescr { … }
//	쓰지 않음    style classDef class linkStyle click direction :::클래스 (Issue — 그림은 그린다)
//	미지원(Error) 다른 그림 종류, subgraph 중첩·subgraph 로 잇기, (((글))) >글] @{…} "`마크다운`", --o --x 화살표

// 다른 mermaid 그림 종류(머리 낱말). 이 판은 그리지 않는다 — 원문 그대로 두고 경고에 이름을 적는다.
// gantt 는 diagram/gantt(itda-hyve#409 ①), timeline 은 diagram/timeline(itda-hyve#409 ②), sequenceDiagram 은 diagram/sequence
// (itda-hyve#409 ③), erDiagram 은 diagram/er(itda-hyve#409 ④), stateDiagram 은 diagram/state(itda-hyve#409 ④-2), classDiagram 은 diagram/classdiag
// (itda-hyve#409 ④-3)가 그린다(mdoc 가 종류로 갈라 보낸다)라 여기 없다.
var otherKinds = map[string]bool{
	"journey": true, "pie": true, "quadrantChart": true, "requirementDiagram": true,
	"gitGraph": true, "C4Context": true, "C4Container": true, "C4Component": true, "C4Dynamic": true, "C4Deployment": true,
	"mindmap": true, "sankey": true, "sankey-beta": true, "xychart": true, "xychart-beta": true,
	"block": true, "block-beta": true, "packet": true, "packet-beta": true, "kanban": true, "architecture-beta": true,
	"radar-beta": true, "treemap-beta": true, "zenuml": true, "flowchart-elk": true,
}

type parser struct {
	g        *Graph
	issues   []Issue
	line     int // 지금 읽는 줄(1부터)
	cur      int // 열린 subgraph(없으면 -1)
	header   bool
	inAcc    bool // accDescr { … } 안
	accLine  int
	acc      diagram.AccBlock
	raw      string // 지금 줄의 원문(줄 끝 CR 만 지운 것) — accDescr { 여는 줄의 값
	labTok   int    // 지금 줄에서 라벨 글(text)이 지킨 엔티티 토큰 수 — 줄 전체의 토큰 수와 견준다(entityLine)
	clusters map[string]int
	said     map[string]bool // 한 번만 알릴 Issue
}

// Parse 는 mermaid 원문을 읽는다. 그릴 수 없는 문법이면 *Error 다. issues 는 쓰지 않은 줄이다.
func Parse(src string) (*Graph, []Issue, error) {
	p := &parser{g: &Graph{byID: map[string]int{}}, cur: -1, clusters: map[string]int{}, said: map[string]bool{}}
	// 잘못된 UTF-8 은 읽기 전에 U+FFFD 로(파서가 룬 단위로 읽는다). XML 이 허용하지 않는 글자는 여기서가 아니라
	// SVG 를 쓰는 경계(svg.go 의 esc)에서 거른다 — 엔티티·YAML 이 디코딩한 뒤에 생기는 글자도 있어서다(리뷰 R2)
	if err := p.parse(strings.ToValidUTF8(src, "\uFFFD")); err != nil {
		return nil, nil, err
	}
	// 출력 관문 — 본문을 다 읽은 뒤, 내보낼 사용자 글자를 모두 한 번에 본다(본문 오류가 먼저다 — state 와 같은 꼴).
	if err := diagram.Gate(p.g.eachText); err != nil {
		return nil, nil, err
	}
	return p.g, p.issues, nil
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
		if err := p.frontmatter(fm); err != nil {
			return err
		}
	}
	for i := start; i < len(lines); i++ {
		p.line = i + 1
		t := strings.TrimSpace(lines[i])
		p.raw = strings.TrimSuffix(lines[i], "\r")
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
		// 원문에 이미 들어 있는 Mermaid 내부 표식 문자(ﬂ°·¶ß)는 그리지 않는다(2026-10-30 사용자 결정 — 다른 종류와 같다). 흐름도는
		// 줄 단위 엔티티 전처리를 하지 않고 라벨 글에서만 표식을 만든다(text) — 라벨의 표식이 모두 이 판이 만든 것이게 한다.
		if diagram.HasMarker(t) {
			return p.fail("본문 줄에 Mermaid 내부 표식 문자(ﬂ°·¶ß)가 있음 — 이 판은 그 줄을 그리지 않는다(엔티티 표기 #…; 는 그대로 쓸 수 있다)")
		}
		// 설정 지시 — 줄 전체가 지시 하나인 줄(accDescr { … } 밖)만 무시하는 지시다. 그 밖에 %%{ 가 든 줄(지시 앞뒤·사이의
		// 글, acc 값·노드 글·간선 글·주석 안, accDescr 블록 안)은 원본 removeDirectives 가 지시만 지우고 남은 글을 읽는
		// 갈래라 그리지 않는다(er·sequence 와 같은 공통 규칙 — itda-hyve#409 ④-1 후속).
		if msg := diagram.DirectiveProblem(t, p.inAcc); msg != "" {
			return p.fail("%s", msg)
		}
		// 줄 머리 %% 주석 — 원본 전처리 cleanupComments(/^\s*%%(?!{)[^\n]+\n?/gm)가 lexer 상태와 관계없이(accDescr { … } 블록 안도)
		// 먼저 지운다. 그래서 지시 검사 뒤, 엔티티 전처리·상태 분기 앞에 둔다(er 와 같은 차례 — itda-hyve#409 ④-1 후속 리뷰 1회차 [2]).
		// 뒤에 글이 없는 맨 %% 는 원본 전처리가 지우지 않는다 — 이 판은 그 줄을 그리지 않는다(er 와 같게, 보수적).
		// 여기 오는 %%{ 줄은 줄 전체가 지시 하나뿐이다(DirectiveProblem).
		if strings.HasPrefix(t, "%%") {
			if strings.HasPrefix(t, "%%{") {
				p.note("init", "설정 지시(%%{…}%%)는 쓰지 않음 — 색·글꼴은 테마가 정한다")
			} else if strings.TrimLeft(strings.TrimSuffix(lines[i], "\r"), " \t") == "%%" {
				return p.fail("글 없는 %s 줄 — 원본 전처리가 주석으로 지우지 않는 줄이라 이 판은 그리지 않는다(%s 뒤에 글을 둔다)", "%%", "%%")
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
		stmts := splitStatements(t)
		accLine := false
		if _, _, ok := diagram.AccLine(t, "accTitle"); ok {
			stmts, accLine = []string{t}, true // 접근성 글은 줄 끝까지(Mermaid 와 같다) — 글 안의 ; 와 엔티티 #35; 로 나누지 않는다
		} else if _, _, ok := diagram.AccLine(t, "accDescr"); ok {
			stmts, accLine = []string{t}, true // accDescr { 도 줄 머리면 그 줄 하나 — { 뒤 글은 원래 줄에서 자른다(statement)
		}
		ignoredOnly := !accLine && p.header && ignoredLine(stmts)
		if !accLine && !ignoredOnly && diagram.StyleCut(t) {
			return p.fail("style…:…#…; 꼴이 든 줄 — 원본 전처리는 줄 끝 ; 를 자르고 엔티티를 지켜 이 판(라벨 글만 전처리)과 뜻이 갈린다(style·classDef 는 그 줄 하나로 따로 쓴다)")
		}
		p.labTok = 0
		for _, st := range stmts {
			if st = strings.TrimSpace(st); st == "" {
				continue
			}
			if err := p.statement(st); err != nil {
				return err
			}
			if p.inAcc {
				break // accDescr { 뒤의 글은 다음 줄부터
			}
		}
		// 라벨 글만 전처리해도 줄 전체 전처리와 같으려면 줄의 엔티티 토큰이 모두 라벨 안에 통째로 있어야 한다(diagram.EntityTokens —
		// 라벨 밖·라벨 경계에 걸친 #…; 는 원본이 표식으로 지켜 문법·값의 뜻이 갈린다. 리뷰 1회차 [1]). 접근성 줄은 값이 줄 끝까지라
		// 같고, 쓰지 않는 줄(style·classDef …)만 있는 줄은 그리지 않으므로 보지 않는다.
		if !accLine && !ignoredOnly && diagram.EntityTokens(t) != p.labTok {
			return p.fail("엔티티 #…; 가 라벨 글 밖에 있거나 라벨 경계에 걸쳐 있음 — 원본은 줄 전체를 전처리해 이 판과 뜻이 갈린다(엔티티는 라벨 글 안에만 쓴다)")
		}
	}
	if p.inAcc {
		return &Error{Line: p.accLine, Msg: "accDescr { 가 닫히지 않음"}
	}
	if !p.header {
		p.line = 0
		return p.fail("mermaid 그림이 비어 있음")
	}
	if p.cur >= 0 {
		c := p.g.Clusters[p.cur]
		return &Error{Line: c.Line, Msg: fmt.Sprintf("subgraph %s 가 end 로 닫히지 않음", quote(c.ID))}
	}
	for _, n := range p.g.Nodes {
		if _, ok := p.clusters[n.ID]; ok {
			return &Error{Line: n.Line, Msg: fmt.Sprintf("subgraph %s 를 노드로 쓰는 것(subgraph 로 잇는 화살표)은 아직 그리지 않음", quote(n.ID))}
		}
	}
	if len(p.g.Nodes) == 0 {
		return p.fail("그릴 노드가 없음")
	}
	used := make([]bool, len(p.g.Clusters))
	for _, n := range p.g.Nodes {
		if n.Cluster >= 0 {
			used[n.Cluster] = true
		}
	}
	for i, c := range p.g.Clusters {
		if !used[i] {
			p.issues = append(p.issues, Issue{Line: c.Line, Msg: fmt.Sprintf("빈 subgraph %s 는 그리지 않음", quote(c.Title))})
		}
	}
	return nil
}

// frontmatter 는 --- 사이의 YAML 읽기(공통 diagram)를 흐름도 규칙으로 바꾼다: 글인 title 은 캡션,
// 나머지 키는 쓰지 않는다(키마다 경고). YAML 오류·글 아닌 title 은 그리지 않는다(*Error — 원문 그대로).
// diagram.ParseFrontmatter 의 줄은 frontmatter 소스 기준(1부터)이라 원문 줄(+1)로 바꾼다 — 블록은 원문 둘째 줄부터.
func (p *parser) frontmatter(src string) error {
	fm, err := diagram.ParseFrontmatter(src)
	if err != nil {
		de := err.(*diagram.Error)
		return &Error{Line: de.Line + 1, Msg: de.Msg}
	}
	for _, k := range fm.Others {
		p.line = k.Line + 1
		p.note("fm:"+k.Key, fmt.Sprintf("frontmatter 의 설정(%s)은 쓰지 않음 — 색·글꼴은 테마가 정한다", diagram.Clip(k.Key)))
	}
	if fm.HasTitle {
		p.line = fm.TitleLine + 1
		v, msg := diagram.Caption(fm.Raw) // 정책 B — 원본은 frontmatter 를 엔티티 전처리 전에 떼어 낸다(글자 그대로)
		if msg != "" {
			return p.fail("%s", msg)
		}
		p.g.Title, p.g.TitleLine = v, p.line
	}
	return nil
}

// splitStatements 는 한 줄을 ; 로 나눈다(따옴표·괄호·|…| 안의 ; 는 글자).
func splitStatements(s string) []string {
	var out []string
	depth, inQ, inPipe, from := 0, false, false, 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"':
			inQ = !inQ
		case inQ:
		case c == '[' || c == '(' || c == '{':
			depth++
		case c == ']' || c == ')' || c == '}':
			if depth > 0 {
				depth--
			}
		case c == '|' && depth == 0:
			inPipe = !inPipe
		case c == ';' && depth == 0 && !inPipe:
			out = append(out, s[from:i])
			from = i + 1
		}
	}
	return append(out, s[from:])
}

// ignoredLine 은 줄의 문장이 모두 쓰지 않는 문장(style·classDef·class·linkStyle·click·direction)인지다. 원본 전처리가 그런 줄의
// 끝 ; 를 잘라도(StyleCut) 그 문장들은 그리지 않으므로 그림의 뜻이 바뀌지 않는다 — ; 로 다른 문장과 이은 줄은 아니다(그 문장은
// 그린다).
func ignoredLine(stmts []string) bool {
	n := 0
	for _, st := range stmts {
		if st = strings.TrimSpace(st); st == "" {
			continue
		}
		switch firstWord(st) {
		case "style", "classDef", "class", "linkStyle", "click", "direction":
			n++
		default:
			return false
		}
	}
	return n > 0
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}

func (p *parser) statement(s string) error {
	if !p.header {
		return p.headerLine(s)
	}
	w := firstWord(s)
	rest := strings.TrimSpace(s[len(w):])
	switch w {
	case "subgraph":
		return p.subgraph(rest)
	case "end":
		if rest != "" {
			return p.fail("end 뒤에 글이 있음(%s)", quote(clip(rest)))
		}
		if p.cur < 0 {
			return p.fail("닫을 subgraph 가 없는 end")
		}
		p.cur = -1
		return nil
	case "direction":
		p.note("direction", "subgraph 의 direction 은 쓰지 않음 — 그림 전체의 방향으로 그린다")
		return nil
	case "style", "classDef", "class", "linkStyle":
		p.note(w, fmt.Sprintf("%s 줄은 쓰지 않음 — 색·선은 테마가 정한다", w))
		return nil
	case "click":
		p.note("click", "click 줄(링크·스크립트)은 쓰지 않음")
		return nil
	}
	// 접근성 — 정책 B: 원본처럼 앞뒤만 trim, 안쪽 빈칸·탭은 그대로, 맨 &·<·>·제어 문자(Cc)는 원문(공통 diagram.AccValue). 원본은
	// 원문 전체를 엔티티 전처리하므로 값의 `#…;` 를 표식으로 지켜 둔 뒤 정책대로 되돌린다(diagram.EncodeEntities).
	if k, v, ok := diagram.AccLine(s, "accTitle"); ok && k == ':' {
		t, msg := diagram.AccValue(diagram.EncodeEntities(v), "accTitle")
		if msg != "" {
			return p.fail("%s", msg)
		}
		p.g.AccTitle, p.g.AccTitleLine = t, p.line
		return nil
	}
	if k, v, ok := diagram.AccLine(s, "accDescr"); ok {
		if k == ':' {
			t, msg := diagram.AccValue(diagram.EncodeEntities(v), "accDescr")
			if msg != "" {
				return p.fail("%s", msg)
			}
			p.g.AccDescr, p.g.AccDescrLine = t, p.line
			return nil
		}
		// accDescr { … } — 같은 줄에서 닫힐 수 있다. 줄 머리면 { 뒤 글은 원래 줄에서 줄 끝 빈칸까지 자른다(; 로 나눈 문장
		// 가운데 하나면 그 문장의 { 뒤 글).
		p.acc.Reset()
		p.inAcc, p.accLine = true, p.line
		rest := v
		if _, _, head := diagram.AccLine(strings.TrimSpace(p.raw), "accDescr"); head {
			_, rest, _ = strings.Cut(p.raw, "{")
		}
		return p.accBody(rest)
	}
	return p.chain(s)
}

func (p *parser) headerLine(s string) error {
	f := strings.Fields(s)
	switch f[0] {
	case "graph", "flowchart":
	default:
		if otherKinds[f[0]] {
			return p.fail("mermaid 의 %s 그림은 아직 그리지 않음(flowchart·graph 만)", f[0])
		}
		return p.fail("mermaid 그림 종류를 알 수 없음 — 첫 줄이 flowchart·graph 가 아님(%s)", quote(clip(f[0])))
	}
	p.header = true
	p.g.Dir = TB
	if len(f) > 2 {
		return p.fail("머리 줄 뒤에 글이 있음 — 줄을 바꾸거나 ; 로 나눈다")
	}
	if len(f) == 2 {
		switch f[1] {
		case "TD", "TB":
			p.g.Dir = TB
		case "BT":
			p.g.Dir = BT
		case "LR":
			p.g.Dir = LR
		case "RL":
			p.g.Dir = RL
		default:
			return p.fail("방향 %s 를 모름(TD·TB·BT·LR·RL)", quote(clip(f[1])))
		}
	}
	return nil
}

func (p *parser) subgraph(rest string) error {
	if p.cur >= 0 {
		return p.fail("subgraph 안의 subgraph 는 아직 그리지 않음")
	}
	if len(p.g.Clusters) >= MaxSubgraphs {
		return p.fail("subgraph 가 %d 개를 넘음", MaxSubgraphs)
	}
	var id, title string
	switch {
	case rest == "":
		id = fmt.Sprintf("subgraph-%d", len(p.g.Clusters)+1)
	case strings.HasSuffix(rest, "]") && strings.Contains(rest, "["):
		i := strings.Index(rest, "[")
		id, title = strings.TrimSpace(rest[:i]), strings.TrimSpace(rest[i+1:len(rest)-1])
		title = unquote(title)
		if id == "" {
			id = title
		}
	default:
		id = unquote(rest)
		title = id
	}
	if strings.HasPrefix(title, "`") {
		return p.fail("마크다운 글자(\"`…`\")는 아직 그리지 않음")
	}
	t, err := p.text(title)
	if err != nil {
		return err
	}
	if _, dup := p.clusters[id]; dup {
		return p.fail("subgraph id %s 가 겹침", quote(clip(id)))
	}
	if _, isNode := p.g.byID[id]; isNode {
		return p.fail("subgraph %s 를 노드로 쓰는 것(subgraph 로 잇는 화살표)은 아직 그리지 않음", quote(clip(id)))
	}
	p.clusters[id] = len(p.g.Clusters)
	p.cur = len(p.g.Clusters)
	one, msg := diagram.OneLine(t, "라벨") // subgraph 제목은 한 줄(<br> 은 빈칸 하나)
	if msg != "" {
		return p.fail("%s", msg)
	}
	p.g.Clusters = append(p.g.Clusters, &Cluster{ID: id, Title: one, Line: p.line})
	return nil
}

// ---- 노드·화살표

func (p *parser) chain(s string) error {
	pos := 0
	prev, err := p.group(s, &pos)
	if err != nil {
		return err
	}
	for {
		pos = skipSpace(s, pos)
		if pos >= len(s) {
			return nil
		}
		lk, err := p.link(s, &pos)
		if err != nil {
			return err
		}
		pos = skipSpace(s, pos)
		next, err := p.group(s, &pos)
		if err != nil {
			return err
		}
		for _, a := range prev {
			for _, b := range next {
				if len(p.g.Edges) >= MaxEdges {
					return p.fail("화살표가 %d 개를 넘음", MaxEdges)
				}
				e := lk
				e.From, e.To, e.Line = a, b, p.line
				p.g.Edges = append(p.g.Edges, &e)
			}
		}
		prev = next
	}
}

func skipSpace(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

// group 은 노드 하나 또는 & 로 묶은 노드들이다.
func (p *parser) group(s string, pos *int) ([]int, error) {
	var out []int
	for {
		*pos = skipSpace(s, *pos)
		n, err := p.nodeRef(s, pos)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
		j := skipSpace(s, *pos)
		if j < len(s) && s[j] == '&' {
			*pos = j + 1
			continue
		}
		return out, nil
	}
}

func isIDRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// readID 는 노드 이름이다: 글자·숫자·_, 그리고 글자 사이의 - . (화살표의 시작이 아닌 것).
func readID(s string, i int) int {
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if isIDRune(r) {
			i += n
			continue
		}
		if (r == '-' || r == '.') && i+1 < len(s) {
			if r2, _ := utf8.DecodeRuneInString(s[i+1:]); isIDRune(r2) {
				i++
				continue
			}
		}
		break
	}
	return i
}

type shapeSyn struct {
	open, close string
	shape       Shape
	bad         string // 미지원이면 까닭
}

// 긴 여는 표지부터. [/ 와 [\ 는 닫는 표지로 모양이 갈린다(slant).
var shapes = []shapeSyn{
	{"(((", ")))", 0, "겹원 노드((((글))))는 아직 그리지 않음"},
	{"((", "))", ShapeCircle, ""},
	{"([", "])", ShapeStadium, ""},
	{"[(", ")]", ShapeCylinder, ""},
	{"[[", "]]", ShapeSubroutine, ""},
	{"{{", "}}", ShapeHexagon, ""},
	{"[/", "", 0, ""},
	{"[\\", "", 0, ""},
	{"(", ")", ShapeRound, ""},
	{"[", "]", ShapeRect, ""},
	{"{", "}", ShapeDiamond, ""},
	{">", "]", 0, "깃발 노드(>글])는 아직 그리지 않음"},
}

func (p *parser) nodeRef(s string, pos *int) (int, error) {
	start := *pos
	end := readID(s, start)
	if end == start {
		return 0, p.fail("노드 이름이 와야 할 자리(%s)", quote(clip(s[start:])))
	}
	id := s[start:end]
	if id == "end" || id == "subgraph" {
		return 0, p.fail("%s 는 노드 이름으로 쓸 수 없음", id)
	}
	*pos = end
	label, shape, has := "", ShapeRect, false
	if strings.HasPrefix(s[end:], "@{") {
		return 0, p.fail("노드 모양 문법 @{…}는 아직 그리지 않음")
	}
	for _, sy := range shapes {
		if !strings.HasPrefix(s[end:], sy.open) {
			continue
		}
		if sy.bad != "" {
			return 0, p.fail("%s", sy.bad)
		}
		var err error
		label, shape, err = p.shapeText(s, pos, sy)
		if err != nil {
			return 0, err
		}
		has = true
		break
	}
	if strings.HasPrefix(s[*pos:], ":::") {
		j := readID(s, *pos+3)
		*pos = j
		p.note(":::", "노드 클래스(:::…)는 쓰지 않음 — 색·선은 테마가 정한다")
	}
	return p.node(id, label, shape, has)
}

// shapeText 는 여는 표지 뒤의 글과 닫는 표지를 읽는다.
func (p *parser) shapeText(s string, pos *int, sy shapeSyn) (string, Shape, error) {
	i := *pos + len(sy.open)
	closes := []string{sy.close}
	if sy.close == "" { // [/ … /]·\]  [\ … \]·/]
		closes = []string{"/]", "\\]"}
	}
	var raw string
	if strings.HasPrefix(s[i:], "\"`") {
		return "", 0, p.fail("마크다운 글자(\"`…`\")는 아직 그리지 않음")
	}
	if i < len(s) && s[i] == '"' {
		q := strings.IndexByte(s[i+1:], '"')
		if q < 0 {
			return "", 0, p.fail("따옴표가 닫히지 않음")
		}
		raw = s[i+1 : i+1+q]
		i = skipSpace(s, i+1+q+1)
		if !hasAny(s[i:], closes) {
			return "", 0, p.fail("라벨 뒤에 %s 가 와야 함", strings.Join(closes, " 또는 "))
		}
	} else {
		at := -1
		for _, c := range closes {
			if k := strings.Index(s[i:], c); k >= 0 && (at < 0 || k < at) {
				at = k
			}
		}
		if at < 0 {
			return "", 0, p.fail("%s 로 연 라벨이 닫히지 않음", sy.open)
		}
		raw = s[i : i+at]
		i += at
	}
	shape := sy.shape
	cl := closes[0]
	for _, c := range closes {
		if strings.HasPrefix(s[i:], c) {
			cl = c
		}
	}
	if sy.close == "" {
		switch {
		case sy.open == "[/" && cl == "/]":
			shape = ShapeLean
		case sy.open == "[\\" && cl == "\\]":
			shape = ShapeLeanAlt
		case sy.open == "[/":
			shape = ShapeTrap
		default:
			shape = ShapeTrapAlt
		}
	}
	*pos = i + len(cl)
	t, err := p.text(strings.TrimSpace(raw))
	return t, shape, err
}

func hasAny(s string, pre []string) bool {
	for _, x := range pre {
		if strings.HasPrefix(s, x) {
			return true
		}
	}
	return false
}

// node 는 id 의 노드를 찾거나 만든다. 모양·글을 적었으면 덮어쓴다(Mermaid 처럼 나중 정의가 이긴다).
func (p *parser) node(id, label string, shape Shape, has bool) (int, error) {
	i, ok := p.g.byID[id]
	if !ok {
		if len(p.g.Nodes) >= MaxNodes {
			return 0, p.fail("노드가 %d 개를 넘음", MaxNodes)
		}
		if utf8.RuneCountInString(id) > MaxLabel {
			return 0, p.fail("노드 이름이 %d 자를 넘음", MaxLabel)
		}
		i = len(p.g.Nodes)
		p.g.byID[id] = i
		p.g.Nodes = append(p.g.Nodes, &Node{ID: id, Label: id, Cluster: -1, Line: p.line, LabelLine: p.line})
	}
	n := p.g.Nodes[i]
	if has {
		n.Label, n.Shape, n.LabelLine = label, shape, p.line
	}
	if p.cur >= 0 && n.Cluster < 0 {
		n.Cluster = p.cur // 처음 들어간 subgraph 가 이긴다
	}
	return i, nil
}

// link 는 화살표 하나와 그 글(-->|글|, -- 글 -->)을 읽는다.
func (p *parser) link(s string, pos *int) (Edge, error) {
	i := *pos
	e := Edge{Length: 1}
	if i < len(s) && s[i] == '<' {
		e.Both = true
		i++
	}
	bad := func() (Edge, error) {
		return Edge{}, p.fail("화살표가 와야 할 자리(%s)", quote(clip(s[*pos:])))
	}
	if i >= len(s) {
		return bad()
	}
	run := func(c byte, j int) int {
		for j < len(s) && s[j] == c {
			j++
		}
		return j
	}
	switch c := s[i]; c {
	case '-', '=':
		e.Stroke = StrokeNormal
		if c == '=' {
			e.Stroke = StrokeThick
		}
		j := run(c, i)
		n := j - i
		if c == '-' && n == 1 && j < len(s) && s[j] == '.' { // -.-> -..-> -.- / -. 글 .->
			e.Stroke = StrokeDotted
			k := run('.', j)
			dots := k - j
			if k < len(s) && s[k] == '-' {
				i = k + 1
				if i < len(s) && s[i] == '>' {
					e.Arrow = true
					i++
				}
				e.Length = dots
			} else if dots == 1 && k < len(s) && (s[k] == ' ' || s[k] == '\t') {
				end := strings.Index(s[k:], ".-")
				if end < 0 {
					return Edge{}, p.fail("화살표 글(-. 글 .->)이 닫히지 않음")
				}
				e.Label = strings.TrimSpace(s[k : k+end])
				m := run('.', k+end)
				i = run('-', m)
				if i < len(s) && s[i] == '>' {
					e.Arrow = true
					i++
				}
				e.Length = m - (k + end)
			} else {
				return bad()
			}
			break
		}
		if n < 2 {
			return bad()
		}
		switch {
		case j < len(s) && s[j] == '>':
			e.Arrow, e.Length, i = true, n-1, j+1
		case j < len(s) && (s[j] == 'o' || s[j] == 'x'):
			return Edge{}, p.fail("끝이 o·x 인 화살표(--o·--x)는 아직 그리지 않음")
		case n >= 3:
			e.Length, i = n-2, j
		case j < len(s) && (s[j] == ' ' || s[j] == '\t'): // -- 글 -->  == 글 ==>
			two := string([]byte{c, c})
			end := strings.Index(s[j:], two)
			if end < 0 {
				return Edge{}, p.fail("화살표 글(%s 글 %s>)이 닫히지 않음", two, two)
			}
			e.Label = strings.TrimSpace(s[j : j+end])
			k := run(c, j+end)
			m := k - (j + end)
			switch {
			case k < len(s) && s[k] == '>':
				e.Arrow, e.Length, i = true, m-1, k+1
			case k < len(s) && (s[k] == 'o' || s[k] == 'x'):
				return Edge{}, p.fail("끝이 o·x 인 화살표(--o·--x)는 아직 그리지 않음")
			case m >= 3:
				e.Length, i = m-2, k
			default:
				return Edge{}, p.fail("화살표 글(%s 글 %s>)이 닫히지 않음", two, two)
			}
		default:
			return bad()
		}
	case '~':
		j := run('~', i)
		if j-i < 3 {
			return bad()
		}
		e.Stroke, e.Length, i = StrokeInvisible, j-i-2, j
	default:
		return bad()
	}
	if e.Both && !e.Arrow {
		return bad()
	}
	if e.Length < 1 {
		e.Length = 1
	}
	if e.Length > MaxLength {
		e.Length = MaxLength
	}
	j := skipSpace(s, i)
	if j < len(s) && s[j] == '|' {
		end := strings.IndexByte(s[j+1:], '|')
		if end < 0 {
			return Edge{}, p.fail("화살표 글 |…| 이 닫히지 않음")
		}
		if e.Label != "" {
			return Edge{}, p.fail("화살표 글이 둘(-- 글 --> 와 |글|)")
		}
		e.Label = strings.TrimSpace(s[j+1 : j+1+end])
		i = j + 1 + end + 1
	}
	if strings.HasPrefix(e.Label, "\"`") {
		return Edge{}, p.fail("마크다운 글자(\"`…`\")는 아직 그리지 않음")
	}
	t, err := p.text(unquote(e.Label))
	if err != nil {
		return Edge{}, err
	}
	e.Label = t
	*pos = i
	return e, nil
}

// ---- 글

// text 는 라벨 글을 정책 B 로 읽는다. 흐름도는 줄 단위 엔티티 전처리를 하지 않으므로(줄 루프의 HasMarker 뒤) 라벨 글에 원본
// 전처리(diagram.EncodeEntities)를 먼저 씌운다 — 원본 flow.jison 의 라벨 토큰(text 상태 [^\[\]\(\)\{\}\|\"]+·따옴표 STR·간선 글
// EDGE_TEXT)이 #·; 를 글자로 받는다. 라벨 값이 줄 전체를 전처리한 것과 같은 것은 줄 루프의 두 검사가 지킬 때뿐이다 — 줄에
// style…:…#…; 꼴(diagram.StyleCut — 원본은 줄의 마지막 ; 를 자른다)이 없고, 줄의 엔티티 토큰이 모두 라벨 안에 있다(labTok —
// 리뷰 1회차 [1] 반례 `A["style x:#35;"];`·`A[style] -->|x:#35;| B`). 라벨 밖의 ; 는 문장 구분으로 읽던 그대로다. 그 뒤는
// 공통 diagram.Label 이다: <br> 은 줄바꿈, 받는 엔티티만 한 번 글자로, 앞뒤 빈칸을 지운 뒤 안쪽 연속 빈칸·탭·제어 문자(C0·DEL)는 원문.
// 다른 HTML 은 글자 그대로다(실행하지 않는다).
func (p *parser) text(s string) (string, error) {
	p.labTok += diagram.EntityTokens(s)
	v, msg := diagram.Label(diagram.EncodeEntities(s), "라벨", true)
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
	p.g.AccDescr, p.g.AccDescrLine = d, p.line
	return nil
}

// eachText 는 모델에서 SVG·Result 로 나가는 사용자 글자를 하나씩 f 에 넘긴다(field 는 「형.필드」, what 은 경고 문구의 이름,
// line 은 그 값이 나온 줄). **출력 글자 필드 목록은 여기 한곳이다** — 모델에 글 필드를 더하면 여기에도 더한다(TestGateCoversAllText
// 가 모델의 string·[]string 필드와 견준다). 노드·subgraph ID 는 그리지 않지만(그리는 것은 Label·Title) 함께 본다.
func (g *Graph) eachText(f diagram.TextFunc) error {
	for _, x := range []struct {
		field, what, v string
		line           int
	}{
		{"Graph.Title", "frontmatter title", g.Title, g.TitleLine},
		{"Graph.AccTitle", "accTitle", g.AccTitle, g.AccTitleLine},
		{"Graph.AccDescr", "accDescr", g.AccDescr, g.AccDescrLine},
	} {
		if err := f(x.field, x.what, x.v, x.line); err != nil {
			return err
		}
	}
	for _, n := range g.Nodes {
		if err := f("Node.ID", "노드 이름", n.ID, n.Line); err != nil {
			return err
		}
		if err := f("Node.Label", "라벨", n.Label, n.LabelLine); err != nil {
			return err
		}
	}
	for _, e := range g.Edges {
		if err := f("Edge.Label", "라벨", e.Label, e.Line); err != nil {
			return err
		}
	}
	for _, c := range g.Clusters {
		if err := f("Cluster.ID", "subgraph 이름", c.ID, c.Line); err != nil {
			return err
		}
		if err := f("Cluster.Title", "라벨", c.Title, c.Line); err != nil {
			return err
		}
	}
	return nil
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// clip·quote 는 경고에 넣는 원문 조각·이름 묶음이다(공통 diagram).
func clip(s string) string  { return diagram.Clip(s) }
func quote(s string) string { return diagram.Quote(s) }
