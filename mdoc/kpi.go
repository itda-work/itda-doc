package mdoc

import (
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/util"
)

// KPI 타일(표준 어휘 kpi, itda-work/itda-hyve#362). :::kpi 안에 요구 구조가 있을 때만 KPI 마크업으로 그린다 — 구조 게이트:
//
//   - 바로 안의 첫 정의 목록: 용어 = 이름, 첫 설명 = 큰 값, 둘째 이후 설명 = 보조 글.
//   - 정의 목록이 없으면 바로 안의 첫 3열 표: 행마다 1열 = 이름, 2열 = 값, 3열 = 보조 글(머리 행은 자유 — 쓰지 않는다).
//   - 둘 다 있으면 정의 목록이 KPI 이고 표는 그대로 뒤에 남는다(WarnKPIAmbiguous).
//   - 둘 다 없으면 손대지 않는다 — 종전 그대로 <div class="kpi"> 다(회사 테마가 kpi 를 자기 어휘로 쓴 문서와 호환).
//     보드 문서(dashboard:)에서만 WarnKPIEmpty 로 알린다.
//
// 마크업: <div class="kpi kpi-tiles"><dl><div class="kpi-item"><dt>이름</dt><dd>값</dd><dd class="kpi-up">▲ 보조 글</dd></div>…</dl></div>.
// 보조 글이 ▲·+ 로 시작하면 kpi-up, ▼·−·- 로 시작하면 kpi-down 이다 — 기호는 글자로 남는다(색만으로 뜻을 전하지 않게).
// kpi-tiles 는 게이트를 지났다는 표식이다 — 화면(web 의 slides.js SINGLE·dashboard.js·base.css)은 .kpi 가 아니라 이것만 보고
// KPI 로 다룬다. 구조 없는 회사 어휘 .kpi 는 표식이 없어 종전 모양·나눔 그대로다.
// 원문은 정의 목록·표 그대로라 다른 뷰어에서도 읽힌다.

// KPIList 는 KPI 항목 묶음이다(<dl>). 자식은 KPIItem 이다.
type KPIList struct{ ast.BaseBlock }

// KPIItem 은 KPI 하나다(<div class="kpi-item">). 자식은 정의 목록의 dt·dd 다.
type KPIItem struct{ ast.BaseBlock }

var (
	KindKPIList = ast.NewNodeKind("KPIList")
	KindKPIItem = ast.NewNodeKind("KPIItem")
)

func (n *KPIList) Kind() ast.NodeKind         { return KindKPIList }
func (n *KPIList) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }
func (n *KPIItem) Kind() ast.NodeKind         { return KindKPIItem }
func (n *KPIItem) Dump(src []byte, level int) { ast.DumpHelper(n, src, level, nil, nil) }

// kpi 는 :::kpi 컨테이너 하나를 구조 게이트대로 바꾼다.
func (st *state) kpi(c *Container) {
	var dl *extast.DefinitionList
	var tbl *extast.Table
	for n := c.FirstChild(); n != nil; n = n.NextSibling() {
		switch n := n.(type) {
		case *extast.DefinitionList:
			if dl == nil {
				dl = n
			}
		case *extast.Table:
			if tbl == nil && len(n.Alignments) == 3 {
				tbl = n
			}
		}
	}
	line := st.lines.line(c.Pos())
	if dl != nil || tbl != nil {
		c.Attrs.Classes = append([]string{"kpi-tiles"}, c.Attrs.Classes...)
	}
	switch {
	case dl != nil:
		st.kpiFromList(dl)
		if tbl != nil {
			st.warn(line, WarnKPIAmbiguous, ":::kpi 안에 정의 목록과 3열 표가 함께 있어 정의 목록을 KPI 로 그리고 표는 그대로 둠")
		}
	case tbl != nil:
		st.kpiFromTable(tbl)
	case st.meta.Dashboard != nil:
		st.warn(line, WarnKPIEmpty, ":::kpi 안에 정의 목록이나 3열 표(이름 | 값 | 보조 글)가 없어 KPI 로 그리지 않음")
	}
}

// kpiFromList 는 정의 목록을 KPI 묶음으로 바꾼다. 용어가 새 항목을 연다(설명 없이 이어진 용어는 같은 항목).
func (st *state) kpiFromList(dl *extast.DefinitionList) {
	list := &KPIList{}
	list.SetPos(dl.Pos())
	var item *KPIItem
	descs := 0
	for n := dl.FirstChild(); n != nil; {
		next := n.NextSibling()
		switch n.(type) {
		case *extast.DefinitionTerm:
			if item == nil || descs > 0 {
				item = &KPIItem{}
				list.AppendChild(list, item)
				descs = 0
			}
		case *extast.DefinitionDescription:
			if item == nil { // 용어 없는 설명은 goldmark 이 만들지 않지만 — 혹시 오면 이름 없는 항목
				item = &KPIItem{}
				list.AppendChild(list, item)
			}
			if descs > 0 {
				st.kpiTrend(n)
			}
			descs++
		}
		item.AppendChild(item, n)
		n = next
	}
	dl.Parent().ReplaceChild(dl.Parent(), dl, list)
}

// kpiFromTable 은 3열 표를 KPI 묶음으로 바꾼다. 머리 행·캡션은 쓰지 않는다. 칸의 인라인(강조·링크 등)은 그대로 옮긴다.
func (st *state) kpiFromTable(t *extast.Table) {
	list := &KPIList{}
	list.SetPos(t.Pos())
	for r := t.FirstChild(); r != nil; r = r.NextSibling() {
		row, ok := r.(*extast.TableRow)
		if !ok {
			continue
		}
		cs := cells(row)
		item := &KPIItem{}
		list.AppendChild(list, item)
		dt := extast.NewDefinitionTerm()
		item.AppendChild(item, dt)
		for i, cell := range cs {
			var dst ast.Node = dt
			if i > 0 {
				if i == 2 && strings.TrimSpace(plainText(cell, st.src)) == "" {
					break // 빈 보조 글은 내지 않는다
				}
				dd := extast.NewDefinitionDescription()
				dd.IsTight = true
				item.AppendChild(item, dd)
				dst = dd
			}
			moveChildren(dst, cell)
			if i == 2 {
				st.kpiTrend(dst)
			}
		}
	}
	t.Parent().ReplaceChild(t.Parent(), t, list)
}

// kpiTrend 는 보조 글의 첫 글자로 증감 class 를 단다.
func (st *state) kpiTrend(dd ast.Node) {
	r, _ := utf8.DecodeRuneInString(strings.TrimSpace(plainText(dd, st.src)))
	switch r {
	case '▲', '+':
		dd.SetAttributeString("class", []byte("kpi-up"))
	case '▼', '−', '-':
		dd.SetAttributeString("class", []byte("kpi-down"))
	}
}

func moveChildren(dst, src ast.Node) {
	for c := src.FirstChild(); c != nil; {
		next := c.NextSibling()
		dst.AppendChild(dst, c)
		c = next
	}
}

func (r *htmlRenderer) kpiList(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<dl>\n")
	} else {
		_, _ = w.WriteString("</dl>\n")
	}
	return ast.WalkContinue, nil
}

func (r *htmlRenderer) kpiItem(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<div class="kpi-item">` + "\n")
	} else {
		_, _ = w.WriteString("</div>\n")
	}
	return ast.WalkContinue, nil
}
