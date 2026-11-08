package mdoc

import (
	"slices"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/util"
)

// 남은 할 일 모음 :::todos 와 todo-left 경고(itda-doc#74 — 경고는 [!TODO] 알림만 센다).
//
// 모으는 대상은 알림 > [!TODO] 와 체크하지 않은 할 일 목록 항목(- [ ])이다 — 체크한 - [x] 는 뺀다. 발표자 노트(:::notes) 안의 것은
// 목차(headings.go inNotes)처럼 DropNotes 와 상관없이 뺀다. 각주 정의 안의 것도 뺀다(walkTodos). 모음 자리(TodosNav) 자신은 대상이 아니다.
// 항목은 문서 차례이고, 항목마다 첫 줄 글(인라인 서식은 글자로)·속한 제목 경로(번호 매김이 켜져 있으면 번호 포함)·그 자리 id 를 가진다.

// todoItem 은 모음의 할 일 하나다.
type todoItem struct {
	node  ast.Node // *Alert 또는 *ast.ListItem
	alert bool     // [!TODO] 알림이면 참, - [ ] 항목이면 거짓
	line  int      // 원문 줄
	text  string   // 첫 줄 글
	path  []string // 속한 제목 경로(바깥부터) — 「2. 예산」·「2.1 인건비」
	id    string   // 그 자리 id(todo-k). 문서에 모음 자리가 없으면 ""(id 를 달지 않는다)
}

// todosContainers 는 :::todos 컨테이너를 정리한다 — notes 보다 먼저 부르므로 DropNotes 와 상관없이 경고와 판정이 같다.
// class notes 가 붙은 것(:::todos{.notes})은 건드리지 않는다 — 노트 뜻이 먼저라 transform.go notes 가 이름과 상관없이 노트로 정리하고
// DropNotes 면 안을 뗀다(mdoc.go Options.DropNotes). 여기서 속성을 먼저 지우면 노트가 본문 모음으로 바뀌어 비공개 글이 나간다.
// 몸통(자식 블록)이 있으면 모음으로 바꾸지 않고 보통 컨테이너(<div class="todos">)로 두고 경고한다(kpi.go kpi-empty 와 같은 꼴 —
// 적은 글을 버리지 않는다). 속성(class·id·key=value)은 노트(transform.go notes)처럼 경고와 함께 버린다.
// 모음이 될 컨테이너를 돌려준다(노트 안에 있어 DropNotes 가 뗄 것도 함께 — 그래서 할 일 id 를 다는지가 노트 공개와 상관없이 같다).
func (st *state) todosContainers(doc ast.Node) []*Container {
	var out []*Container
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		c, ok := n.(*Container)
		if !ok || !entering || c.Name != "todos" || slices.Contains(c.classes(), "notes") {
			return ast.WalkContinue, nil
		}
		line := st.lines.line(c.Pos())
		if len(c.Attrs.Classes) > 0 || c.Attrs.ID != "" || len(c.Attrs.Data) > 0 || c.Attrs.Unnumbered || c.Width != "" {
			st.warn(line, WarnBadAttribute, ":::todos 에는 class·id·속성을 붙이지 않습니다 — 버림")
			c.Attrs, c.Width = attrs{}, ""
		}
		if c.HasChildren() {
			st.warn(line, WarnTodosBody, ":::todos 는 빈 컨테이너로 씁니다 — 안에 글이 있어 할 일 모음 대신 보통 상자로 그림")
			return ast.WalkContinue, nil
		}
		out = append(out, c)
		return ast.WalkSkipChildren, nil
	})
	return out
}

// placeTodos 는 모음이 될 컨테이너 가운데 아직 문서에 붙어 있는 것(DropNotes 가 떼지 않은 것)을 모음 자리로 바꾼다.
// 발표(frontmatter slides)·보드(dashboard) 문서에는 모음을 넣지 않는다(spec/slides.md·dashboard.md) — 자리를 만들지 않고 컨테이너를
// 떼며, 그래서 빈 상자·인용·목록 항목만 남으면 그것도 뗀다(dropEmptied — 맨 위가 아닌 자리에서도 빈 장·타일이 생기지 않게).
// 할 일에 todo-N id 를 다는지를 돌려준다 — 모음이 될 컨테이너가 있으면(노트 안이라 뗀 것도 세어 DropNotes 와 상관없이 같다) 참, 발표·보드면 거짓.
func (st *state) placeTodos(cs []*Container) bool {
	drop := st.meta.Slides != nil || st.meta.Dashboard != nil
	for _, c := range cs {
		if !attached(c) {
			continue
		}
		if drop {
			dropEmptied(c)
			continue
		}
		nav := &TodosNav{}
		nav.SetPos(c.Pos())
		c.Parent().ReplaceChild(c.Parent(), c, nav)
	}
	return !drop && len(cs) > 0
}

// dropEmptied 는 n 을 떼고, 그 때문에 빈 조상(노트가 아닌 컨테이너·인용·목록·목록 항목)을 위로 올라가며 함께 뗀다.
// 노트 컨테이너는 떼지 않는다 — 노트 자리는 DropNotes 와 상관없이 맨 위 블록 구조가 같아야 한다(notes_invariant_test.go).
func dropEmptied(n ast.Node) {
	for {
		p := n.Parent()
		p.RemoveChild(p, n)
		if p.HasChildren() || !emptiable(p) {
			return
		}
		n = p
	}
}

func emptiable(n ast.Node) bool {
	switch n := n.(type) {
	case *Container:
		return !slices.Contains(n.classes(), "notes")
	case *ast.Blockquote, *ast.List, *ast.ListItem:
		return true
	}
	return false
}

// attached 는 n 이 문서(Document)까지 이어져 있는지다 — DropNotes 가 뗀 노트 안 블록은 조상 사슬이 문서 앞에서 끊긴다.
func attached(n ast.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if _, ok := p.(*ast.Document); ok {
			return true
		}
	}
	return false
}

// walkTodos 는 모음 대상이 되는 할 일 노드를 문서 차례로 fn 에 넘긴다. 제목도 함께 넘긴다(경로를 쌓으려고 — 할 일이 아니면 todo 는 거짓).
// 건너뛰는 곳: 발표자 노트(:::notes — 목차처럼 DropNotes 와 상관없이), 모음 자리(TodosNav), 각주 정의 목록(FootnoteList).
// 각주 안 할 일은 본문·노트 각주 구분 없이 모으지 않는다 — goldmark 은 각주 정의를 어디에 적었든 문서 끝 목록으로 옮기므로 노트 조상
// 검사로는 노트 전용 각주를 가를 수 없고, DropNotes 면 노트 전용 각주가 떨어져(transform.go pruneFootnotes) 모음이 달라진다.
// 알림 상자 자체는 각주 안에서도 그대로 그린다.
func walkTodos(doc ast.Node, fn func(n ast.Node, todo, alert bool)) {
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *Container:
			if slices.Contains(n.classes(), "notes") {
				return ast.WalkSkipChildren, nil
			}
		case *TodosNav, *extast.FootnoteList:
			return ast.WalkSkipChildren, nil
		case *ast.Heading:
			fn(n, false, false)
		case *Alert:
			if n.AlertType == "todo" {
				fn(n, true, true)
			}
		case *ast.ListItem:
			if box := taskBox(n); box != nil && !box.IsChecked {
				fn(n, true, false)
			}
		}
		return ast.WalkContinue, nil
	})
}

// collectTodos 는 남은 할 일을 문서 차례로 모은다. 알림 판정(transform.go alert) 뒤, 제목 id 배정(processHeadings) 앞에 부른다.
// withIDs 면(문서에 모음 자리가 있으면) k 번째 할 일에 todo-k 를 단다 — 할 일 id 가 이긴다: 노트 밖 할 일의 차례로만 정하고, 겹치는
// 다른 id(제목·컨테이너, 본문·노트, 자동·직접 적은 것)는 비킨다(제목은 processHeadings, 컨테이너는 finishTodos). 그래서 공개 할 일 id 는
// DropNotes 와 상관없이 같다. 비킨 id 를 가리키는 문서 안 링크(#todo-1)는 원문 그대로 둔다 — 할 일로 간다.
func (st *state) collectTodos(doc ast.Node, withIDs bool) {
	walkTodos(doc, func(n ast.Node, todo, alert bool) {
		if todo {
			st.todos = append(st.todos, todoItem{node: n, alert: alert, line: st.blockLine(n), text: firstLineOf(n.FirstChild(), st.src)})
		}
	})
	if !withIDs {
		return
	}
	st.todoIDs = map[string]bool{}
	for i := range st.todos {
		id := "todo-" + strconv.Itoa(i+1)
		st.todoIDs[id] = true
		st.todos[i].id = id
		if li, ok := st.todos[i].node.(*ast.ListItem); ok {
			li.SetAttributeString("id", []byte(id))
		}
	}
}

// finishTodos 는 제목 번호·id 를 정한 뒤(processHeadings) 할 일마다 속한 제목 경로를 채우고, 할 일 id 와 겹치는 컨테이너 id 를 비키게 하고,
// 남은 [!TODO] 알림이 있으면 todo-left 경고 하나를 첫 알림 줄에 낸다(:::todos 유무와 상관없이 — - [ ] 항목은 세지 않는다).
func (st *state) finishTodos(doc ast.Node) {
	var stack []todoCrumb
	i := 0
	walkTodos(doc, func(n ast.Node, todo, _ bool) {
		if todo {
			for _, c := range stack {
				st.todos[i].path = append(st.todos[i].path, c.label)
			}
			i++
			return
		}
		h := n.(*ast.Heading)
		info := st.headings[h]
		if info == nil {
			return
		}
		for len(stack) > 0 && stack[len(stack)-1].level >= h.Level {
			stack = stack[:len(stack)-1]
		}
		label := info.text
		if info.number != "" {
			label = info.number + " " + label
		}
		stack = append(stack, todoCrumb{h.Level, label})
	})
	st.yieldContainerIDs(doc)
	// 경고는 [!TODO] 알림만 센다 — - [ ] 는 모음에는 들지만 일부러 쓴 체크리스트 문서가 경고 0 으로 남게 세지 않는다
	n, first := 0, 0
	for _, it := range st.todos {
		if it.alert {
			if n == 0 {
				first = it.line
			}
			n++
		}
	}
	if n > 0 {
		st.warn(first, WarnTodoLeft, "남은 할 일 알림 "+strconv.Itoa(n)+"개 — 끝낸 일은 [!TODO] 알림을 지웁니다")
	}
}

// yieldContainerIDs 는 할 일 id 와 겹치는 컨테이너 id(본문·노트 모두 — 컨테이너 id 는 늘 직접 적은 것)를 「id-1」·「id-2」… 가운데
// 어디에도 안 쓰인 것으로 바꾸고 duplicate-id 경고를 낸다(문서 차례). 제목 쪽은 processHeadings 가 같은 규칙으로 비킨다.
func (st *state) yieldContainerIDs(doc ast.Node) {
	if len(st.todoIDs) == 0 {
		return
	}
	all := map[string]bool{}
	for id := range st.todoIDs {
		all[id] = true
	}
	for _, info := range st.headings {
		all[info.id] = true
	}
	var hits []*Container
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if c, ok := n.(*Container); ok && entering && c.Attrs.ID != "" {
			if st.todoIDs[c.Attrs.ID] {
				hits = append(hits, c)
			} else {
				all[c.Attrs.ID] = true
			}
		}
		return ast.WalkContinue, nil
	})
	for _, c := range hits {
		old := c.Attrs.ID
		c.Attrs.ID = freeID(old, all)
		st.warn(st.lines.line(c.Pos()), WarnDuplicateID, "컨테이너 id #"+old+" 가 할 일 id 와 겹침 — #"+c.Attrs.ID+" 로 바꿈")
	}
}

// freeID 는 「id-1」·「id-2」… 가운데 used 에 없는 첫 것을 used 에 넣고 돌려준다.
func freeID(id string, used map[string]bool) string {
	for k := 1; ; k++ {
		if c := id + "-" + strconv.Itoa(k); !used[c] {
			used[c] = true
			return c
		}
	}
}

// todoCrumb 는 제목 경로의 한 칸이다(제목 단계와 「번호 글」).
type todoCrumb struct {
	level int
	label string
}

// taskBox 는 목록 항목의 할 일 상자(- [ ]·- [x])다. 할 일 항목이 아니면 nil.
func taskBox(li *ast.ListItem) *extast.TaskCheckBox {
	fc := li.FirstChild()
	if fc == nil {
		return nil
	}
	box, _ := fc.FirstChild().(*extast.TaskCheckBox)
	return box
}

// firstLineOf 는 블록 b(문단·글 블록)의 첫 줄 글이다 — 글자만(plainText 와 같은 규칙), 첫 줄바꿈에서 멈춘다. 문단이 아니면 "".
func firstLineOf(b ast.Node, src []byte) string {
	switch b.(type) {
	case *ast.Paragraph, *ast.TextBlock:
	default:
		return ""
	}
	return inlineText(b, src, true)
}

// todoID 는 그 노드의 할 일 id 다(없으면 "") — render.go alert 가 쓴다.
func (st *state) todoID(n ast.Node) string {
	for _, it := range st.todos {
		if it.node == n {
			return it.id
		}
	}
	return ""
}

// renderTodos 는 모음 자리를 그린다:
// <nav class="todos"><ul><li class="todos-alert|todos-task"><a href="#todo-1"><span class="todos-path">2. 예산 › 2.1 인건비</span>
// <span class="todos-text">…</span></a></li></ul></nav>. 경로가 없으면(첫 제목 앞) 경로 칸을 넣지 않는다.
// 남은 할 일이 없으면 <p class="todos-empty">남은 할 일 없음</p> 한 줄이다(lang 이 한국어가 아니면 영어).
func (r *htmlRenderer) todosNav(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	var b strings.Builder
	b.WriteString(`<nav class="todos"`)
	if v, ok := node.AttributeString("data-line"); ok {
		b.WriteString(` data-line="`)
		b.Write(util.EscapeHTML(v.([]byte)))
		b.WriteByte('"')
	}
	b.WriteString(">\n")
	if len(r.st.todos) == 0 {
		empty := "No remaining todos"
		if r.korean() {
			empty = "남은 할 일 없음"
		}
		b.WriteString(`<p class="todos-empty">` + empty + "</p>\n")
	} else {
		b.WriteString("<ul>\n")
		for _, it := range r.st.todos {
			kind := "todos-task"
			if it.alert {
				kind = "todos-alert"
			}
			b.WriteString(`<li class="` + kind + `"><a href="#`)
			b.Write(util.EscapeHTML([]byte(it.id)))
			b.WriteString(`">`)
			if len(it.path) > 0 {
				b.WriteString(`<span class="todos-path">`)
				b.Write(util.EscapeHTML([]byte(strings.Join(it.path, " › "))))
				b.WriteString("</span> ")
			}
			text := it.text
			if text == "" {
				text = r.alertTitle("todo")
			}
			b.WriteString(`<span class="todos-text">`)
			b.Write(util.EscapeHTML([]byte(text)))
			b.WriteString("</span></a></li>\n")
		}
		b.WriteString("</ul>\n")
	}
	b.WriteString("</nav>\n")
	_, _ = w.WriteString(b.String())
	return ast.WalkSkipChildren, nil
}
