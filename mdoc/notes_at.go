package mdoc

import (
	"slices"
	"strings"

	"github.com/yuin/goldmark/ast"
)

// Notes 는 발표자 노트 컨테이너 하나의 원문 자리다(줄은 1부터, frontmatter 를 포함한 원문 줄 번호 — TableAt 과 같다).
// InnerStart > InnerEnd 면 안쪽에 줄이 없다(여는 줄 바로 다음이 닫는 줄 — InnerStart 는 닫는 줄, InnerEnd 는 여는 줄이다).
type Notes struct {
	Open       int    `json:"open"`        // 여는 ::: 줄
	Close      int    `json:"close"`       // 닫는 ::: 줄
	InnerStart int    `json:"inner_start"` // 안쪽 첫 줄
	InnerEnd   int    `json:"inner_end"`   // 안쪽 끝 줄
	Inner      string `json:"inner"`       // 안쪽 원문(줄 끝 \n 으로 이어 붙이고 마지막 \n 은 뺀다). 인용·목록 안이면 그 표시까지 원문 그대로다
}

// NotesAt 은 line 줄이 든 발표자 노트 컨테이너를 찾는다(itda-hyve#763). 판정은 렌더가 class="notes" 를 내는 것과 같은 classes() 이고
// (transform.go notes), 렌더와 같은 파서·같은 깊이 상한을 쓴다(TableAt 꼴 — 상한을 넘으면 거짓). 노트 안 노트면 가장 안쪽을 고른다.
// 닫는 줄이 없는 노트는 바꿔 넣을 줄 범위가 정해지지 않으므로 거짓이다 — 그 안의 줄이면 바깥 노트로 올라가지도 않는다.
func NotesAt(src []byte, line int) (Notes, bool) {
	src = normalize(src)
	meta, body, _, _ := parseFrontmatter(src, nil)
	st := newState(body, meta, Options{})
	doc, over := st.parseGuarded(body)
	if over > 0 || line < 1 || line > len(st.lines) {
		return Notes{}, false
	}
	var found *Container
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		c, ok := n.(*Container)
		if !entering || !ok || !slices.Contains(c.classes(), "notes") {
			return ast.WalkContinue, nil
		}
		if st.containsLine(c, line) {
			found = c // 바깥부터 들어가므로 마지막에 찾은 것이 가장 안쪽이다
		}
		return ast.WalkContinue, nil
	})
	if found == nil || !found.closed || found.endLine == 0 {
		return Notes{}, false
	}
	open := st.lines.line(found.Pos())
	out := Notes{Open: open, Close: found.endLine, InnerStart: open + 1, InnerEnd: found.endLine - 1}
	if out.InnerStart <= out.InnerEnd {
		ls := make([]string, 0, out.InnerEnd-out.InnerStart+1)
		for i := out.InnerStart; i <= out.InnerEnd; i++ {
			ls = append(ls, string(st.lineBytes(i)))
		}
		out.Inner = strings.Join(ls, "\n")
	}
	return out, true
}

// containsLine 은 line 이 컨테이너의 여는 줄부터 끝까지에 드는지다. 닫히지 않은 컨테이너는 닫는 줄이 없어 끝을 다음 형제(없으면 부모의
// 다음 형제…)의 앞까지로 본다 — 그 뒤 줄이 바깥 노트로 올라가 잘못된 범위를 바꾸지 않게 한다.
func (st *state) containsLine(c *Container, line int) bool {
	open := st.lines.line(c.Pos())
	if open < 1 || line < open {
		return false
	}
	if c.closed && c.endLine > 0 {
		return line <= c.endLine
	}
	for n := ast.Node(c); n != nil; n = n.Parent() {
		if s := n.NextSibling(); s != nil {
			return line < st.blockLine(s)
		}
	}
	return true
}
