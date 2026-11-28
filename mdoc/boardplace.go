package mdoc

import (
	"regexp"
	"slices"
	"strconv"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// 보드 타일의 자리·크기(itda-work/itda-hyve#1038) — 타일을 여는 제목의 속성 at=열,행·span=너비x높이.
// 속성은 다른 key=value 처럼 제목의 data-at·data-span 으로 나가고(render.go heading), 놓는 일은 화면(web/dashboard.js pack)이 한다.
// 여기서는 보드 문서(frontmatter dashboard:)에서만 값을 검사해 틀린 것을 WarnDashboardPlace 경고와 함께 뗀다 — 뗀 속성은 HTML 에 없어
// 화면은 그 타일을 적지 않은 것처럼(자동 배치·내용 크기) 놓는다. 보드 문서가 아니면 검사하지 않는다(쪽·슬라이드에서는 보이지 않는 data-* 일 뿐이다).

// boardMaxRow 는 구역 안 행 번호(at 의 행 + 높이 − 1)의 상한이다 — web/dashboard.js MAX_ROW 와 같다(격자가 끝없이 늘지 않게).
const boardMaxRow = 99

var (
	boardAtRe   = regexp.MustCompile(`^\s*(\d{1,3})\s*,\s*(\d{1,3})\s*$`)
	boardSpanRe = regexp.MustCompile(`^\s*(\d{1,3})x(\d{1,3})\s*$`)
)

// boardCell 은 고정 타일이 확실히 차지하는 칸이다(0부터). 내용 너비는 화면에서야 정해져(dashboard.js tileWidth) span 이 없으면 1×1 로 본다 —
// 그래서 여기서 찾는 겹침은 화면이 찾는 겹침의 일부다(나머지는 화면이 자동 배치로 돌린다, pack 의 moved).
type boardCell struct{ c, r, w, h int }

func (a boardCell) overlaps(b boardCell) bool {
	return a.c < b.c+b.w && b.c < a.c+a.w && a.r < b.r+b.h && b.r < a.r+a.h
}

// boardPlace 는 보드 문서의 제목 at·span 을 검사한다. 타일 판정은 dashboard.js groups 와 같은 조건·같은 분기 차례다(맨 위 블록만):
//  1. 건너뛴다 — 목차(TOCNav = nav.toc)·할 일 모음(TodosNav = nav.todos)·멈춤(Pause = div.pause)·각주(FootnoteList = div.footnotes),
//     그리고 렌더된 class 에 notes·pause·footnotes 가 있는 제목·컨테이너(topClasses — 종류를 가리지 않는다, groups 의 .notes·.pause·.footnotes)
//  2. 구역 경계 — 가로줄(hr)·쪽 나눔(PageBreak), class page-break 가 있는 제목·컨테이너. 1 이 먼저라 {.pause .page-break} 는 건너뛰는 것이다
//  3. 타일 단계보다 높은 제목은 구역 제목 줄(경계), 4. 타일 단계 제목은 타일, 5. 그 밖은 앞 타일의 몸통(여기서는 볼 것 없음)
//
// 표지(section.cover)·제목 블록(header.title-block)·목차 쪽(section.toc-page)은 page 가 흐름에 넣는 것이라 mdoc 의 맨 위 블록에 없다.
// 타일 단계는 dashboard.level, 없으면 슬라이드 단계(slideLevel)다 — 화면이 받는 값(page.BoardView.Level)과 같다.
func (st *state) boardPlace(doc ast.Node) {
	dm := st.meta.Dashboard
	if dm == nil {
		return
	}
	cols := dm.Cols
	if cols == 0 {
		cols = 4
	}
	lvl := dm.Level
	if lvl == 0 {
		lvl = slideLevel(doc)
	}
	tiles := map[*ast.Heading]bool{}
	var order []*ast.Heading          // 타일을 여는 제목(읽는 차례) — to 검사(boardLinks)가 쓴다
	hidden := map[*ast.Heading]bool{} // 보드에 넣지 않는 맨 위 제목(1 로 건너뛴 것)
	var seg []boardCell               // 이 구역에서 자리를 지킨 고정 타일
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		cls := st.topClasses(c)
		_, isFootnotes := c.(*extast.FootnoteList)
		_, isPause := c.(*Pause)
		h, isHeading := c.(*ast.Heading)
		switch {
		case isTOC(c), isTodos(c), isPause, isFootnotes, slices.Contains(cls, "notes"), slices.Contains(cls, "pause"), slices.Contains(cls, "footnotes"):
			if isHeading {
				hidden[h] = true
			}
		case isBreak(c), slices.Contains(cls, "page-break"):
			seg = nil
		case isHeading && lvl > 0 && h.Level < lvl:
			seg = nil
		case isHeading && h.Level == lvl:
			tiles[h] = true
			order = append(order, h)
			if cell, ok := st.tilePlace(h, cols); ok {
				if slices.ContainsFunc(seg, cell.overlaps) {
					st.dropBoardKey(h, "at", "타일 자리 at="+v0(st, h)+" 가 앞의 타일과 겹쳐 자동으로 놓음")
				} else {
					seg = append(seg, cell)
				}
			}
		}
	}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && entering && !tiles[h] {
			for _, k := range []string{"at", "span", "to"} {
				code := WarnDashboardPlace
				if k == "to" {
					code = WarnDashboardLink // 타일 잇기(boardlink.go)
				}
				if _, has := st.boardKey(h, k); has {
					if hidden[h] {
						st.dropKey(h, k, code, "제목의 "+k+" — 보드에 넣지 않는 제목(.notes·.pause·.footnotes)이어서 버림")
					} else {
						st.dropKey(h, k, code, "제목의 "+k+" 은 보드 타일을 여는 제목(맨 위 "+strconv.Itoa(lvl)+"단계)에만 써서 버림")
					}
				}
			}
		}
		return ast.WalkContinue, nil
	})
	st.boardLinks(order)
}

// tilePlace 는 타일 제목 h 의 at·span 을 읽어 틀린 것을 경고와 함께 떼고, at 이 남으면 고정 칸을 돌려준다.
func (st *state) tilePlace(h *ast.Heading, cols int) (boardCell, bool) {
	w, ht, hasSpan := 1, 1, false
	if v, ok := st.boardKey(h, "span"); ok {
		m := boardSpanRe.FindStringSubmatch(v)
		switch {
		case m == nil:
			st.dropBoardKey(h, "span", "타일 크기 span="+v+" 를 읽지 못해 내용 크기로 놓음 — 너비x높이(소문자 x, 예: span=3x2)")
		default:
			sw, _ := strconv.Atoi(m[1])
			sh, _ := strconv.Atoi(m[2])
			switch {
			case sw < 1 || sh < 1:
				st.dropBoardKey(h, "span", "타일 크기 span="+v+" — 너비·높이는 1 이상이어서 내용 크기로 놓음")
			case sw > cols:
				st.dropBoardKey(h, "span", "타일 크기 span="+v+" — 너비가 격자 열 수("+strconv.Itoa(cols)+")보다 커서 내용 크기로 놓음")
			case sh > boardMaxRow:
				st.dropBoardKey(h, "span", "타일 크기 span="+v+" — 높이는 "+strconv.Itoa(boardMaxRow)+" 까지여서 내용 크기로 놓음")
			default:
				w, ht, hasSpan = sw, sh, true
			}
		}
	}
	v, ok := st.boardKey(h, "at")
	if !ok {
		return boardCell{}, false
	}
	m := boardAtRe.FindStringSubmatch(v)
	if m == nil {
		st.dropBoardKey(h, "at", "타일 자리 at="+v+" 를 읽지 못해 자동으로 놓음 — 열,행(1부터, 예: at=3,1)")
		return boardCell{}, false
	}
	c, _ := strconv.Atoi(m[1])
	r, _ := strconv.Atoi(m[2])
	switch {
	case c < 1 || r < 1:
		st.dropBoardKey(h, "at", "타일 자리 at="+v+" — 열·행은 1부터여서 자동으로 놓음")
	case c > cols || (hasSpan && c+w-1 > cols):
		st.dropBoardKey(h, "at", "타일 자리 at="+v+" — 격자("+strconv.Itoa(cols)+"열) 밖이어서 자동으로 놓음")
	case r+ht-1 > boardMaxRow:
		st.dropBoardKey(h, "at", "타일 자리 at="+v+" — 구역 안 행은 "+strconv.Itoa(boardMaxRow)+" 까지여서 자동으로 놓음")
	default:
		return boardCell{c: c - 1, r: r - 1, w: w, h: ht}, true
	}
	return boardCell{}, false
}

// v0 은 제목 h 의 at 값이다(경고 문구용).
func v0(st *state, h *ast.Heading) string {
	v, _ := st.boardKey(h, "at")
	return v
}

// boardKey 는 제목 h 에 적힌 key=value 값이다(processHeadings 가 뗀 속성 — 키는 소문자).
func (st *state) boardKey(h *ast.Heading, key string) (string, bool) {
	info := st.headings[h]
	if info == nil {
		return "", false
	}
	return attrs{Data: info.data}.get(key)
}

// dropBoardKey 는 제목 h 의 key 속성을 dashboard-place 경고와 함께 뗀다 — 화면은 그 속성이 없는 것처럼 놓는다.
func (st *state) dropBoardKey(h *ast.Heading, key, msg string) {
	st.dropKey(h, key, WarnDashboardPlace, msg)
}

// dropKey 는 제목 h 의 key 속성을 code 경고와 함께 뗀다.
func (st *state) dropKey(h *ast.Heading, key, code, msg string) {
	info := st.headings[h]
	info.data = attrs{Data: info.data}.without(key)
	st.warn(st.lines.line(h.Pos()), code, msg)
}
