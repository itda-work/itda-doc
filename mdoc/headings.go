package mdoc

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
)

// 제목 id·속성·번호(1.3)와 목차(1.8).

// TOCItem 은 목차의 제목 하나다.
type TOCItem struct {
	Level    int    // 1~6 (h1~h6)
	ID       string // 본문 제목의 id. 링크는 "#"+ID
	Number   string // "1.2". 번호를 쓰지 않거나 {-} 인 제목은 ""
	Text     string // 글자만(꾸밈 없음)
	Children []TOCItem
}

type headingInfo struct {
	id       string
	classes  []string
	data     [][2]string
	number   string
	text     string
	unnumber bool
	unlisted bool
}

// 제목 끝의 { … } 속성. 앞에 공백이 있어야 한다(Pandoc).
var headingAttrRe = regexp.MustCompile(`(?:^|[ \t])\{([^{}]*)\}[ \t]*$`)

func (st *state) processHeadings(hs []*ast.Heading) {
	used := map[string]bool{}
	infos := make([]*headingInfo, len(hs))

	// 1) 속성을 떼고 글자를 모은다. 직접 적은 id 를 먼저 등록해 자동 id 가 피해 가게 한다.
	for i, h := range hs {
		info := &headingInfo{}
		infos[i] = info
		st.headings[h] = info
		line := st.lines.line(h.Pos())
		if a, ok := st.stripHeadingAttrs(h, line); ok {
			info.classes = st.dropHeadingNotes(a.Classes, line)
			info.data = a.Data
			info.unnumber = a.Unnumbered
			info.unlisted = a.has("unlisted")
			if a.ID != "" {
				if used[a.ID] {
					st.warn(line, WarnDuplicateID, "제목 id #"+a.ID+" 가 겹침")
				}
				info.id = a.ID
				used[a.ID] = true
			}
		}
		info.text = plainText(h, st.src)
	}

	// 2) 나머지는 GitHub 규칙으로 id 를 만든다. 겹치면 -1, -2.
	// next 는 base 마다 다음에 볼 번호다 — 그보다 작은 번호는 이미 쓰였고 쓰인 id 는 계속 쓰인 채라 다시 보지 않는다
	// (같은 제목 n 개가 n² 번 찾지 않게).
	next := map[string]int{}
	for _, info := range infos {
		if info.id != "" {
			continue
		}
		base := slug(info.text)
		if base == "" {
			base = "section"
		}
		id := base
		if used[id] {
			n := max(next[base], 1)
			for id = base + "-" + strconv.Itoa(n); used[id]; id = base + "-" + strconv.Itoa(n) {
				n++
			}
			next[base] = n + 1
		}
		info.id = id
		used[id] = true
	}

	// 3) 번호와 목차는 문서에서 가장 높은 제목 단계를 1로 센다.
	// h1 없이 ## 부터 쓴 문서가 "0.1" 이 되지 않게 하려는 것이다(Pandoc 은 0.1 을 낸다).
	top := 7
	for _, h := range hs {
		top = min(top, h.Level)
	}
	if st.meta.NumberSections {
		var counters [6]int
		for i, h := range hs {
			if infos[i].unnumber {
				continue
			}
			d := h.Level - top
			counters[d]++
			for j := d + 1; j < len(counters); j++ {
				counters[j] = 0
			}
			parts := make([]string, d+1)
			for j := range parts {
				parts[j] = strconv.Itoa(counters[j])
			}
			infos[i].number = strings.Join(parts, ".")
		}
	}

	// 4) 목차 트리. toc-depth 도 같은 기준으로 센다.
	var flat []TOCItem
	for i, h := range hs {
		if infos[i].unlisted || h.Level-top >= st.meta.TOCDepth {
			continue
		}
		flat = append(flat, TOCItem{Level: h.Level, ID: infos[i].id, Number: infos[i].number, Text: infos[i].text})
	}
	st.toc = buildTree(flat)
}

// buildTree 는 단계 목록을 트리로 묶는다. 단계를 건너뛴 제목(h1 다음 h3)은 바로 위 제목의 자식이 된다.
func buildTree(flat []TOCItem) []TOCItem {
	var walk func(i, level int) ([]TOCItem, int)
	walk = func(i, level int) ([]TOCItem, int) {
		var out []TOCItem
		for i < len(flat) && flat[i].Level >= level {
			it := flat[i]
			i++
			it.Children, i = walk(i, it.Level+1)
			out = append(out, it)
		}
		return out, i
	}
	var out []TOCItem
	for i := 0; i < len(flat); {
		var part []TOCItem
		part, i = walk(i, flat[i].Level)
		out = append(out, part...)
	}
	return out
}

// dropHeadingNotes 는 제목 class 에서 notes 를 경고와 함께 버린다(#344). 발표자 노트는 :::notes 컨테이너만이라(transform.go
// notes) 제목이 화면(slides.js 의 .notes 선택자)에서 노트로 분류되지 않게, DropNotes 와 상관없이 늘 뺀다.
func (st *state) dropHeadingNotes(classes []string, line int) []string {
	out := classes[:0:0]
	for _, c := range classes {
		if c == "notes" {
			st.warn(line, WarnBadAttribute, "제목의 .notes 는 쓰지 않습니다 — 발표자 노트는 :::notes 로 씁니다")
			continue
		}
		out = append(out, c)
	}
	return out
}

// stripHeadingAttrs 는 제목 끝의 {…} 를 읽고 인라인에서 떼어 낸다.
// 속성 자리가 글자 노드로만 되어 있을 때만 뗀다(속성 안에 꾸밈이 있으면 속성이 아니다).
func (st *state) stripHeadingAttrs(h *ast.Heading, line int) (attrs, bool) {
	if h.Lines().Len() == 0 {
		return attrs{}, false
	}
	seg := h.Lines().At(h.Lines().Len() - 1)
	raw := seg.Value(st.src)
	loc := headingAttrRe.FindSubmatchIndex(raw)
	if loc == nil {
		return attrs{}, false
	}
	a, err := parseAttrs(string(raw[loc[2]:loc[3]]))
	if err != nil {
		st.warn(line, WarnBadAttribute, "제목 속성을 읽지 못해 글자로 둠: "+err.Error())
		return attrs{}, false
	}
	cut := seg.Start + loc[2] - 1 // '{' 의 위치
	if !trimInlineFrom(h, cut, st.src) {
		return attrs{}, false
	}
	return a, true
}

// trimInlineFrom 은 parent 의 인라인 중 cut 이후를 지우고 남은 끝 공백을 뗀다.
// cut 뒤에 글자 아닌 노드가 걸리면 아무것도 바꾸지 않고 거짓을 돌려준다.
func trimInlineFrom(parent ast.Node, cut int, src []byte) bool {
	for c := parent.LastChild(); c != nil; c = c.PreviousSibling() {
		t, ok := c.(*ast.Text)
		if !ok {
			return false
		}
		if t.Segment.Start < cut {
			break
		}
	}
	for c := parent.LastChild(); c != nil; {
		prev := c.PreviousSibling()
		t := c.(*ast.Text)
		if t.Segment.Start >= cut {
			parent.RemoveChild(parent, c)
			c = prev
			continue
		}
		if t.Segment.Stop > cut {
			t.Segment = t.Segment.WithStop(cut)
		}
		t.Segment = t.Segment.TrimRightSpace(src)
		break
	}
	return true
}

// slug 는 GitHub(github-slugger)과 같은 규칙의 id 다: 소문자, 글자·숫자·_·- 와 공백만 남기고 공백은 - 로.
// 공백을 몰아 쓰지 않는 것까지 같다("a - b" → "a---b").
func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// plainText 는 인라인의 글자만 모은다. 코드 글자는 넣고, raw HTML 과 이미지 대체 글은 뺀다(GitHub 의 textContent 와 같게).
func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch c := c.(type) {
		case *ast.Text:
			v := c.Segment.Value(src)
			if !c.IsRaw() {
				v = util.UnescapePunctuations(v)
				v = util.ResolveNumericReferences(v)
				v = util.ResolveEntityNames(v)
			}
			b.Write(v)
			if c.SoftLineBreak() || c.HardLineBreak() {
				b.WriteByte(' ')
			}
		case *ast.String:
			b.Write(c.Value)
		case *ast.AutoLink:
			b.Write(c.Label(src))
			return ast.WalkSkipChildren, nil
		case *ast.RawHTML:
			return ast.WalkSkipChildren, nil
		case *MathInline:
			b.WriteString(c.Tex)
		case *ast.Image:
			if c != n {
				return ast.WalkSkipChildren, nil
			}
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(b.String())
}
