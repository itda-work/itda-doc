package mdoc

import (
	"strings"

	"github.com/yuin/goldmark/ast"
)

// 보드 타일 잇기(itda-work/itda-hyve#1038 3단계 D) — 타일을 여는 제목의 속성 to=도착 id[,도착 id…].
// 값은 다른 key=value 처럼 제목의 data-to 로 나가고(render.go heading), 선을 그리는 일은 화면(web/dashboard.js links)이 한다.
// 여기서는 보드 문서에서만(boardPlace 가 부른다 — 같은 타일 판정) 도착을 하나씩 검사해 틀린 도착만 떼고 WarnDashboardLink 경고 한 줄을 낸다.
// 맞는 도착은 적은 차례대로 남는다. 다 떼이면 to 가 HTML 에 없다. 타일 제목이 아닌 곳의 to 는 boardPlace 의 Walk 가 뗀다.

// boardLinks 는 타일을 여는 제목들(tiles — 읽는 차례)의 to 를 검사한다. 도착이 될 수 있는 것은 tiles 의 id 뿐이다 —
// 구역 제목 줄·머리 제목·타일 안 낮은 제목·상자 안 제목·보드에 넣지 않는 제목의 id 는 「타일을 여는 제목이 아님」 으로 버린다.
func (st *state) boardLinks(tiles []*ast.Heading) {
	tileID := map[string]bool{}
	for _, h := range tiles {
		if info := st.headings[h]; info != nil && info.id != "" {
			tileID[info.id] = true
		}
	}
	anyID := map[string]bool{} // 문서의 모든 제목 id(processHeadings 가 정한 것 — 직접 적은 것·저절로 붙은 것)
	for _, info := range st.headings {
		if info != nil && info.id != "" {
			anyID[info.id] = true
		}
	}
	for _, h := range tiles {
		v, ok := st.boardKey(h, "to")
		if !ok {
			continue
		}
		self := st.headings[h].id
		var keep, why []string
		seen := map[string]bool{}
		for _, raw := range strings.Split(v, ",") {
			id := strings.TrimSpace(raw) // 따옴표로 감싼 값(to="a, b")의 항목 앞뒤 공백
			switch {
			case id == "":
				why = append(why, "빈 항목")
			case seen[id]:
				why = append(why, "#"+id+"(두 번 적음)")
			case id == self:
				why = append(why, "#"+id+"(자기 자신)")
			case tileID[id]:
				keep = append(keep, id)
			case anyID[id]:
				why = append(why, "#"+id+"(타일을 여는 제목이 아님)")
			default:
				why = append(why, "#"+id+"(그 id 의 제목이 없음)")
			}
			if id != "" {
				seen[id] = true
			}
		}
		if len(why) == 0 {
			continue
		}
		msg := "타일 잇기 to=" + v + " 에서 버린 도착 — " + strings.Join(why, " · ")
		if len(keep) == 0 {
			st.dropKey(h, "to", WarnDashboardLink, msg)
			continue
		}
		st.setKey(h, "to", strings.Join(keep, ","))
		st.warn(st.lines.line(h.Pos()), WarnDashboardLink, msg)
	}
}

// setKey 는 제목 h 의 key 속성 값을 바꾼다(자리는 그대로 — data-* 의 차례가 바뀌지 않는다).
func (st *state) setKey(h *ast.Heading, key, val string) {
	info := st.headings[h]
	out := make([][2]string, len(info.data))
	for i, kv := range info.data {
		if kv[0] == key {
			kv[1] = val
		}
		out[i] = kv
	}
	info.data = out
}
