package mdoc

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// NoteSlot 은 슬라이드 한 장에 새 발표자 노트(:::notes 컨테이너)를 넣을 자리다(itda-hyve#781). 줄 번호는 TableAt 과 같다
// (1부터, frontmatter 포함). Line 줄의 머리에 넣는다 — 문서 끝이면 줄 수 + 1 이다(끝이 줄바꿈이면 그 뒤 빈 마지막 줄이 Line 이다).
// PadBefore 는 Line-1 줄이 있고 빈 줄이 아니라서 넣는 글 앞에 빈 줄을 하나 더해야 하는지다(::: 는 문단 중간에서 열리지 않는다 —
// container.go Open). PadAfter 는 Line 줄이 있고 빈 줄이 아니라서 넣는 글 뒤에 빈 줄을 더해야 하는지다.
type NoteSlot struct {
	Line      int  `json:"line"`
	PadBefore bool `json:"pad_before"`
	PadAfter  bool `json:"pad_after"`
}

// NoteSlotAt 은 원문 line 줄(그 장에 그려진 마지막 블록의 줄)이 든 최상위 블록(문서의 직계 자식 — 목록·인용·::: 안의 줄이면 그것을
// 감싼 최상위 블록) 뒤에 노트를 넣을 자리를 정한다. 블록 끝은 다음 최상위 블록 시작 앞에서 빈 줄을 뺀 마지막 줄이다
// (InsertSpotAt 의 span 과 같은 규칙 — 코드 펜스의 닫는 줄·표 캡션·컨테이너의 닫는 :::·HTML 블록의 끝 줄·목록의 마지막 항목이
// 저절로 든다). 그 다음 줄이 빈 줄이면(문서 끝의 빈 마지막 줄은 빼고) 그 빈 줄 다음에 넣는다 — 앞뒤 블록과 빈 줄 하나로 띄운다.
// 닫히지 않은 코드 펜스·::: 컨테이너·$$ 수식은 문서 끝까지이므로 문서 끝이다(그 안에 들어가 노트가 되지 않는다 — 호출자가 넣은
// 글을 NotesAt 으로 다시 읽어 거부한다). line 이 0 이면 frontmatter 다음 줄(그 줄이 빈 줄이면 그 다음, frontmatter 가 없으면 1) — 표지(제목 슬라이드)의 노트다
// (itda-hyve#179 「frontmatter 바로 뒤 노트는 제목 슬라이드」).
// 어느 블록에도 들지 않는 줄(블록 사이 빈 줄·frontmatter 안·범위 밖)·중첩 상한을 넘은 문서는 거짓이다. 렌더와 같은 파서로 읽는다.
func NoteSlotAt(src []byte, line int) (slot NoteSlot, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			slot, ok = NoteSlot{}, false // 읽기 전용 판정이 예기치 못하게 멈추면 넣지 않는다(바인딩이 부른다)
		}
	}()
	src = normalize(src)
	meta, body, _, _ := parseFrontmatter(src, nil)
	st := newState(body, meta, Options{})
	doc, over := st.parseGuarded(body)
	n := len(st.lines)
	if over > 0 || line < 0 || line > n {
		return NoteSlot{}, false
	}
	orig := bytes.Split(src, []byte("\n")) // 빈 줄 판정은 원문으로 — frontmatter 의 닫는 --- 도 빈 줄이 아니다
	blank := func(l int) bool { return l < 1 || l > len(orig) || len(bytes.TrimSpace(orig[l-1])) == 0 }
	// docEnd 는 문서 끝에 넣는 줄이다 — 끝이 줄바꿈이면 그 뒤 빈 마지막 줄(위치가 글 끝), 아니면 줄 수 + 1
	docEnd := n + 1
	if len(src) > 0 && src[len(src)-1] == '\n' {
		docEnd = n
	}
	pad := func(at int) (NoteSlot, bool) {
		return NoteSlot{Line: at, PadBefore: at > 1 && !blank(at-1), PadAfter: at <= n && at != docEnd && !blank(at)}, true
	}
	if line == 0 {
		if !meta.Present {
			return pad(1)
		}
		_, end, fm := splitFrontmatter(src)
		if !fm || end <= 0 {
			return pad(1)
		}
		at := bytes.Count(src[:end-1], []byte("\n")) + 2
		if at < docEnd && at <= n && blank(at) {
			at++ // frontmatter 뒤 빈 줄 다음 — 블록 뒤와 같다
		}
		return pad(at)
	}
	var tops []ast.Node
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		if _, ok := c.(*extast.FootnoteList); ok {
			continue // 각주 정의는 문서 끝 목록으로 옮겨져 원래 자리가 없다 — 그 줄은 앞 블록의 구간에 든다
		}
		if st.blockLine(c) > 0 {
			tops = append(tops, c)
		}
	}
	for i := 0; i < len(tops); i++ {
		t := tops[i]
		start := st.blockLine(t)
		if line < start {
			break
		}
		next := i + 1
		capEnd := 0
		if tb, ok := t.(*extast.Table); ok {
			capEnd = st.readTable(tb).CaptionEnd // 캡션 문단은 다음 형제다 — 표와 한 블록으로 본다
			for next < len(tops) && st.blockLine(tops[next]) <= capEnd {
				next++
			}
		}
		end := n
		if next < len(tops) {
			end = st.blockLine(tops[next]) - 1
		}
		for end > start && blank(end) {
			end--
		}
		end = max(end, capEnd)
		if line > end {
			i = next - 1
			continue
		}
		if next >= len(tops) && unclosedTail(st, t) {
			return pad(docEnd)
		}
		at := end + 1
		if at < docEnd && at <= n && blank(at) {
			at++ // 블록 뒤 빈 줄 다음 — 빈 줄을 더 넣지 않는다
		}
		return pad(at)
	}
	return NoteSlot{}, false
}

// unclosedTail 은 마지막 최상위 블록 t 가 닫는 줄 없이 문서 끝까지 이어지는 코드 펜스·::: 컨테이너·$$ 수식(또는 그것으로 끝나는
// 목록·인용)인지다 — 그 뒤 어디에 넣어도 그 블록 안이다.
func unclosedTail(st *state, t ast.Node) bool {
	for n := t; n != nil; n = n.LastChild() {
		switch n := n.(type) {
		case *ast.FencedCodeBlock:
			return st.fenceCloses[n] == 0
		case *Container:
			if !n.closed || n.endLine == 0 {
				return true
			}
			return false // 닫힌 컨테이너 안은 그 닫는 줄에서 끝난다
		case *MathBlock:
			return !n.closed
		}
	}
	return false
}
