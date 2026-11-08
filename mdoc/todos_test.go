package mdoc

import (
	"regexp"
	"strings"
	"testing"
)

// 알림 [!TODO] 와 할 일 모음 :::todos, todo-left 경고(itda-doc#74 — todos.go).

func codesOf(r Result, code string) []Warning {
	var out []Warning
	for _, w := range r.Warnings {
		if w.Code == code {
			out = append(out, w)
		}
	}
	return out
}

func TestTodoAlert(t *testing.T) {
	r := render(t, "> [!TODO]\n> 예산 확정")
	contains(t, r.HTML, "<div class=\"markdown-alert markdown-alert-todo\" data-line=\"1\">\n<p class=\"markdown-alert-title\">할 일</p>\n<p>예산 확정</p>\n</div>")
	notContains(t, r.HTML, "[!", "<blockquote", ` id="todo-`) // 모음 자리가 없으면 id 를 달지 않는다
	contains(t, render(t, "> [!todo]\n> a").HTML, "markdown-alert-todo")
	contains(t, render(t, "> [!TODO] 글\n> a").HTML, "<blockquote") // 표시가 혼자가 아니면 인용
	contains(t, render(t, "---\nlang: en\n---\n\n> [!TODO]\n> a").HTML, `<p class="markdown-alert-title">Todo</p>`)
}

func TestTodosCollect(t *testing.T) {
	src := "---\nnumber-sections: true\n---\n" + // 1~3
		"# 개요\n\n" + // 4
		":::todos\n:::\n\n" + // 6
		"> [!TODO]\n> 단가 **확정**하기\n> 둘째 줄\n\n" + // 9
		"# 예산\n\n## 인건비 {#todo-1}\n\n" + // 13, 15
		"- [ ] 견적 `받기`\n- [x] 끝난 일\n- 보통 항목\n  - [ ] 안쪽 할 일\n\n" + // 17~20
		":::notes\n- [ ] 노트 할 일\n\n> [!TODO]\n> 노트 알림\n:::\n" // 22~
	for _, drop := range []bool{false, true} {
		r := render(t, src, Options{DropNotes: drop})
		nav := regexp.MustCompile(`(?s)<nav class="todos" data-line="6">.*?</nav>`).FindString(r.HTML)
		if nav == "" {
			t.Fatalf("DropNotes=%v: 모음이 없음\n%s", drop, r.HTML)
		}
		want := `<ul>
<li class="todos-alert"><a href="#todo-1"><span class="todos-path">1 개요</span> <span class="todos-text">단가 확정하기</span></a></li>
<li class="todos-task"><a href="#todo-2"><span class="todos-path">2 예산 › 2.1 인건비</span> <span class="todos-text">견적 받기</span></a></li>
<li class="todos-task"><a href="#todo-3"><span class="todos-path">2 예산 › 2.1 인건비</span> <span class="todos-text">안쪽 할 일</span></a></li>
</ul>`
		if !strings.Contains(nav, want) {
			t.Errorf("DropNotes=%v: 모음\n%s\n기대\n%s", drop, nav, want)
		}
		notContains(t, nav, "끝난 일", "노트", "보통 항목")
		// 링크마다 앵커가 실재한다(할 일 id 가 이긴다 — 제목에 직접 적은 #todo-1 은 todo-1-1 로 비키고 duplicate-id)
		for _, m := range regexp.MustCompile(`href="#(todo-\d+)"`).FindAllStringSubmatch(nav, -1) {
			if n := strings.Count(r.HTML, ` id="`+m[1]+`"`); n != 1 {
				t.Errorf("DropNotes=%v: id %s 가 %d 개", drop, m[1], n)
			}
		}
		contains(t, r.HTML, `<div class="markdown-alert markdown-alert-todo" id="todo-1" data-line="9">`, `<li id="todo-2"><input disabled="" type="checkbox"> 견적 <code>받기</code></li>`, `<h2 id="todo-1-1"`)
		if d := codesOf(r, WarnDuplicateID); len(d) != 1 || d[0].Line != 15 {
			t.Errorf("DropNotes=%v: duplicate-id %v", drop, d)
		}
		w := codesOf(r, WarnTodoLeft)
		if len(w) != 1 || w[0].Line != 9 || !strings.Contains(w[0].Message, "남은 할 일 알림 1개") { // - [ ] 둘은 모음에만, 노트 안 알림은 빠진다
			t.Errorf("DropNotes=%v: todo-left %v", drop, w)
		}
	}
}

func TestTodosEmptyAndPlacement(t *testing.T) {
	r := render(t, ":::todos\n:::\n\n- [x] 끝\n")
	contains(t, r.HTML, "<nav class=\"todos\" data-line=\"1\">\n<p class=\"todos-empty\">남은 할 일 없음</p>\n</nav>")
	if len(r.Warnings) != 0 {
		t.Errorf("경고 %v", r.Warnings)
	}
	contains(t, render(t, "---\nlang: en\n---\n\n:::todos\n:::\n").HTML, `<p class="todos-empty">No remaining todos</p>`)

	// 첫 제목 앞의 할 일은 경로 칸이 없다. 모음은 제 앞뒤 모든 할 일을 문서 차례로 모은다
	r = render(t, "- [ ] 앞\n\n:::todos\n:::\n\n# 뒤 제목\n\n- [ ] 뒤\n")
	contains(t, r.HTML, `<a href="#todo-1"><span class="todos-text">앞</span></a>`, `<span class="todos-path">뒤 제목</span> <span class="todos-text">뒤</span>`)

	// - [ ] 만 있는 체크리스트 문서는 경고 0, 모음 자리가 없으니 id 도 없다
	r = render(t, "- [ ] 하나\n- [ ] 둘\n")
	if len(r.Warnings) != 0 {
		t.Errorf("체크리스트 문서에 경고 %v", r.Warnings)
	}
	notContains(t, r.HTML, "<nav", ` id="todo-`)
	// 모음 자리가 없어도 [!TODO] 알림은 경고한다(알림 개수만, 첫 알림 줄)
	r = render(t, "- [ ] 항목\n\n> [!TODO]\n> 하나\n\n> [!TODO]\n> 둘\n")
	if w := codesOf(r, WarnTodoLeft); len(w) != 1 || w[0].Line != 3 || !strings.Contains(w[0].Message, "알림 2개") {
		t.Errorf("todo-left %v", w)
	}

	// 빈 알림은 제목 글로
	contains(t, render(t, ":::todos\n:::\n\n> [!TODO]\n").HTML, `<span class="todos-text">할 일</span>`)
}

func TestTodosContainerMisuse(t *testing.T) {
	r := render(t, ":::todos\n- [ ] 안\n:::\n")
	contains(t, r.HTML, `<div class="todos"`)
	notContains(t, r.HTML, "<nav")
	if len(codesOf(r, WarnTodosBody)) != 1 {
		t.Errorf("todos-body 경고가 없음 %v", r.Warnings)
	}
	r = render(t, ":::todos{.x #y k=v}\n:::\n")
	contains(t, r.HTML, `<nav class="todos" data-line="1">`)
	notContains(t, r.HTML, `class="x"`, `id="y"`, "data-k")
	if len(codesOf(r, WarnBadAttribute)) != 1 {
		t.Errorf("bad-attribute 경고가 없음 %v", r.Warnings)
	}
	// 노트 안 몸통 있는 :::todos 도 노트를 빼든 내든 경고가 같다
	for _, drop := range []bool{false, true} {
		r = render(t, ":::notes\n:::todos\n글\n:::\n:::\n", Options{DropNotes: drop})
		if len(codesOf(r, WarnTodosBody)) != 1 {
			t.Errorf("DropNotes=%v: %v", drop, r.Warnings)
		}
	}
}

// 모음 자리는 목차처럼 슬라이드 단계 판정에서 내용이 아니다(mdoc.go slideLevel).
func TestTodosSlideLevel(t *testing.T) {
	if r := render(t, "# 큰 제목\n\n:::todos\n:::\n\n## 작은 제목\n\n본문\n"); r.SlideLevel != 2 {
		t.Errorf("SlideLevel %d", r.SlideLevel)
	}
}

// :::todos{.notes} 는 노트다(리뷰 1회차 P1) — 노트 판정이 먼저라 이름과 상관없이 노트로 정리되고 DropNotes 면 안이 빠진다.
// 노트 안 [!TODO] 는 모음·todo-left 에 들지 않는다.
func TestTodosNotesClass(t *testing.T) {
	for _, src := range []string{
		":::todos{.notes}\n> [!TODO]\n> 비공개 메모\n:::\n\n:::todos\n:::\n",
		":::todos\n:::\n\n::::todos{.notes}\n:::box\n- [ ] 비공개 메모\n:::\n::::\n",
	} {
		for _, drop := range []bool{false, true} {
			r := render(t, src, Options{DropNotes: drop})
			nav := regexp.MustCompile(`(?s)<nav class="todos".*?</nav>`).FindString(r.HTML)
			if nav == "" || !strings.Contains(nav, "남은 할 일 없음") {
				t.Errorf("DropNotes=%v %q: 모음이 비지 않음\n%s", drop, src, r.HTML)
			}
			notContains(t, r.HTML, `<div class="todos"`, ` id="todo-`)
			if w := codesOf(r, WarnTodoLeft); len(w) != 0 {
				t.Errorf("DropNotes=%v %q: todo-left %v", drop, src, w)
			}
			if drop {
				notContains(t, r.HTML, "비공개 메모")
			} else {
				contains(t, r.HTML, `<div class="notes"`, "비공개 메모")
			}
		}
	}
	// 빈 :::todos{.notes} 도 모음이 아니라 노트다
	r := render(t, ":::todos{.notes}\n:::\n")
	notContains(t, r.HTML, "<nav")
	contains(t, r.HTML, `<div class="notes"`)
}

// 발표·보드 문서에는 모음을 넣지 않는다(리뷰 1회차 P2) — 맨 위가 아닌 자리(상자·인용·목록·컨테이너 안)도 빼고, 빼서 빈 상자만 남으면
// 그것도 빼서 빈 장·타일이 생기지 않는다. 가로줄 앞뒤도 같다. 노트 자리는 남는다.
func TestTodosSlidesBoard(t *testing.T) {
	body := ":::todos\n:::\n\n" + // 맨 위
		"---\n\n:::box\n:::todos\n:::\n:::\n\n---\n\n" + // 가로줄 사이 상자 안 — 상자째 빠진다
		"> :::todos\n> :::\n\n" + // 인용 안
		"- 남는 항목\n-   :::todos\n    :::\n\n" + // 목록 항목 안 — 그 항목만 빠진다
		"::::columns\n:::column\n:::todos\n:::\n:::\n::::\n\n" + // 컨테이너 둘 안 — 둘 다 빠진다
		":::box\n남는 글\n\n:::todos\n:::\n:::\n\n" + // 다른 글이 있는 상자는 남는다
		":::notes\n:::todos\n:::\n:::\n\n" + // 노트 자리는 남는다
		"> [!TODO]\n> 할 일\n"
	for _, fm := range []string{"slides: true", "dashboard: true"} {
		for _, drop := range []bool{false, true} {
			r := render(t, "---\n"+fm+"\n---\n\n"+body, Options{DropNotes: drop})
			notContains(t, r.HTML, `class="todos"`, "<blockquote", `class="columns"`, `class="column"`, ` id="todo-`, "<div class=\"box\" data-line=\"10\"")
			contains(t, r.HTML, "남는 항목", "남는 글", `<div class="notes"`, "markdown-alert-todo")
			if strings.Count(r.HTML, `class="box"`) != 1 || strings.Count(r.HTML, "<li") != 1 || strings.Count(r.HTML, "<hr") != 2 {
				t.Errorf("%s DropNotes=%v: 상자·항목·가로줄 수\n%s", fm, drop, r.HTML)
			}
			if w := codesOf(r, WarnTodoLeft); len(w) != 1 {
				t.Errorf("%s DropNotes=%v: todo-left %v", fm, drop, w)
			}
		}
	}
	// 보통 문서에서는 상자 안 모음도 그린다
	contains(t, render(t, ":::box\n:::todos\n:::\n:::\n").HTML, `<div class="box"`, `<nav class="todos"`)
}

// 할 일 id 가 이긴다(리뷰 1·2회차 P2) — 노트 안 제목의 자동 id 가 todo-N 이어도 공개 할 일의 앵커는 DropNotes 와 상관없이 같고,
// 노트 안 id 가 비켜 중복이 없다. 직접 적은 노트 안 id 를 바꾸면 duplicate-id 경고.
func TestTodosNoteIDs(t *testing.T) {
	src := ":::todos\n:::\n\n> [!TODO]\n> 공개 작업\n\n- [ ] 공개 항목\n\n" +
		":::notes\n# todo-1\n\n## 둘째 {#todo-2}\n\n:::box{#todo-2}\n글\n:::\n:::\n\n# todo-1-1\n"
	for _, drop := range []bool{false, true} {
		r := render(t, src, Options{DropNotes: drop})
		contains(t, r.HTML, `<a href="#todo-1">`, `<a href="#todo-2">`, `markdown-alert-todo" id="todo-1"`, `<li id="todo-2">`, `<h1 id="todo-1-1"`)
		ids := map[string]int{}
		for _, m := range regexp.MustCompile(` id="([^"]+)"`).FindAllStringSubmatch(r.HTML, -1) {
			ids[m[1]]++
		}
		for id, n := range ids {
			if n != 1 {
				t.Errorf("DropNotes=%v: id %s 가 %d 개\n%s", drop, id, n, r.HTML)
			}
		}
		dup := codesOf(r, WarnDuplicateID)
		if drop {
			if len(dup) != 0 {
				t.Errorf("DropNotes=true: duplicate-id %v", dup)
			}
			continue
		}
		// 노트 안: 자동 id todo-1 → todo-1-2(todo-1-1 은 본문 제목이 씀), 직접 적은 todo-2 → todo-2-1(경고), 컨테이너 todo-2 → todo-2-2(경고)
		contains(t, r.HTML, `<h1 id="todo-1-2"`, `<h2 id="todo-2-1"`, `<div class="box" id="todo-2-2"`)
		if len(dup) != 2 {
			t.Errorf("DropNotes=false: duplicate-id %v", dup)
		}
	}
}

// todoIDView 는 공개 할 일 id(문서 차례)·모음 링크·todo-left 경고 수다 — DropNotes 참/거짓 비교용.
func todoIDView(t *testing.T, r Result) (ids, hrefs, left []string) {
	t.Helper()
	for _, m := range regexp.MustCompile(`(?:markdown-alert-todo"|<li) id="(todo-\d+)"`).FindAllStringSubmatch(r.HTML, -1) {
		ids = append(ids, m[1])
	}
	nav := regexp.MustCompile(`(?s)<nav class="todos".*?</nav>`).FindString(r.HTML)
	for _, m := range regexp.MustCompile(`href="#([^"]+)"`).FindAllStringSubmatch(nav, -1) {
		hrefs = append(hrefs, m[1])
	}
	for _, w := range codesOf(r, WarnTodoLeft) {
		left = append(left, w.Message) // 경고 수와 글(개수)을 함께 묶는다
	}
	return ids, hrefs, left
}

// uniqueIDs 는 HTML 의 모든 id 가 하나씩인지 본다.
func uniqueIDs(t *testing.T, label string, html string) {
	t.Helper()
	ids := map[string]int{}
	for _, m := range regexp.MustCompile(` id="([^"]+)"`).FindAllStringSubmatch(html, -1) {
		ids[m[1]]++
	}
	for id, n := range ids {
		if n != 1 {
			t.Errorf("%s: id %s 가 %d 개\n%s", label, id, n, html)
		}
	}
}

// 리뷰 2회차 재현 두 입력 — 공개 할 일 id·모음 링크·todo-left 가 DropNotes 와 상관없이 같다.
// [1] 노트의 직접 적은 id 가 본문 제목을 밀어내도 할 일 id 는 그대로다(할 일 id 가 이긴다).
// [2] 노트에서만 가리키는 각주 안 [!TODO] 는 모으지 않는다(각주 안 할 일은 모으지 않는다).
func TestTodosReview2(t *testing.T) {
	cases := []struct {
		src            string
		ids, hrefs     []string
		leftAlerts     string
		heading, notes string // 본문 제목 id(DropNotes 둘 다)·노트 제목 id(거짓일 때)
	}{
		{":::todos\n:::\n\n# todo-1\n\n> [!TODO]\n> 공개 작업\n\n:::notes\n# 메모 {#todo-1}\n:::\n",
			[]string{"todo-1"}, []string{"todo-1"}, "알림 1개", `<h1 id="todo-1-1"`, `<h1 id="todo-1-2"`},
		{":::todos\n:::\n\n> [!TODO]\n> 공개 작업\n\n:::notes\n메모[^n]\n\n[^n]: 각주\n\n    > [!TODO]\n    > 노트 각주 작업\n\n    - [ ] 노트 각주 항목\n:::\n",
			[]string{"todo-1"}, []string{"todo-1"}, "알림 1개", "", ""},
	}
	for i, c := range cases {
		var first [3]string
		for _, drop := range []bool{false, true} {
			r := render(t, c.src, Options{DropNotes: drop})
			ids, hrefs, left := todoIDView(t, r)
			if strings.Join(ids, ",") != strings.Join(c.ids, ",") || strings.Join(hrefs, ",") != strings.Join(c.hrefs, ",") {
				t.Errorf("[%d] DropNotes=%v: id %v 링크 %v", i+1, drop, ids, hrefs)
			}
			if w := codesOf(r, WarnTodoLeft); len(w) != 1 || !strings.Contains(w[0].Message, c.leftAlerts) {
				t.Errorf("[%d] DropNotes=%v: todo-left %v", i+1, drop, w)
			}
			notContains(t, regexp.MustCompile(`(?s)<nav class="todos".*?</nav>`).FindString(r.HTML), "각주")
			uniqueIDs(t, "review2", r.HTML)
			if c.heading != "" {
				contains(t, r.HTML, c.heading)
			}
			if !drop && c.notes != "" {
				contains(t, r.HTML, c.notes)
			}
			v := [3]string{strings.Join(ids, ","), strings.Join(hrefs, ","), strings.Join(left, "|")}
			if drop && v != first {
				t.Errorf("[%d] DropNotes 참/거짓 결과가 다름: %v / %v", i+1, first, v)
			}
			first = v
		}
	}
	// 각주 안 알림 상자 자체는 그대로 그린다
	contains(t, render(t, "글[^n]\n\n[^n]: 각주\n\n    > [!TODO]\n    > 각주 작업\n").HTML, "markdown-alert-todo", "각주 작업")
}

// 할 일 id 가 이긴다 — 본문 제목 자동 id·직접 적은 본문 {#todo-1}·본문 컨테이너 id 모두 비킨다. 직접 적은 것만 duplicate-id.
func TestTodosBodyIDsYield(t *testing.T) {
	// 본문 제목 # todo-1 은 조용히 todo-1-1 로
	for _, drop := range []bool{false, true} {
		r := render(t, ":::todos\n:::\n\n# todo-1\n\n> [!TODO]\n> 일\n\n[가기](#todo-1)\n", Options{DropNotes: drop})
		contains(t, r.HTML, `<h1 id="todo-1-1"`, `markdown-alert-todo" id="todo-1"`, `<a href="#todo-1">가기</a>`) // 링크는 원문 그대로 — 할 일로 간다
		if d := codesOf(r, WarnDuplicateID); len(d) != 0 {
			t.Errorf("자동 id 에 duplicate-id %v", d)
		}
		uniqueIDs(t, "auto", r.HTML)
	}
	// 직접 적은 본문 {#todo-1}·컨테이너 {#todo-2} 는 비키고 경고
	r := render(t, ":::todos\n:::\n\n# 제목 {#todo-1}\n\n:::box{#todo-2}\n글\n:::\n\n> [!TODO]\n> 일\n\n- [ ] 항목\n")
	contains(t, r.HTML, `<h1 id="todo-1-1"`, `<div class="box" id="todo-2-1"`, `markdown-alert-todo" id="todo-1"`, `<li id="todo-2">`)
	if d := codesOf(r, WarnDuplicateID); len(d) != 2 || d[0].Line != 4 || d[1].Line != 6 {
		t.Errorf("duplicate-id %v", d)
	}
	uniqueIDs(t, "explicit", r.HTML)
	// 모음 자리가 없으면 할 일 id 가 없으니 아무것도 비키지 않는다
	contains(t, render(t, "# 제목 {#todo-1}\n\n> [!TODO]\n> 일\n").HTML, `<h1 id="todo-1"`)
}

// 리뷰 3회차 재현 — 할 일 id 에 밀린 제목 id(직접 적은 것·자동)는 컨테이너가 이미 쓰는 id(todo-1-1)도 피한다.
// DropNotes 참/거짓 모두 id 가 하나씩이고, 목차 링크가 상자가 아니라 제목에 닿는다.
func TestTodosHeadingYieldAvoidsContainers(t *testing.T) {
	cases := []struct{ src, heading string }{
		{"---\ntoc: true\n---\n\n[TOC]\n\n:::todos\n:::\n\n:::box{#todo-1-1}\n상자\n:::\n\n# 제목 {#todo-1}\n\n> [!TODO]\n> 작업\n", "todo-1-2"},
		{"---\ntoc: true\n---\n\n[TOC]\n\n:::todos\n:::\n\n:::box{#todo-1-1}\n상자\n:::\n\n# todo 1\n\n> [!TODO]\n> 작업\n", "todo-1-2"},
	}
	for i, c := range cases {
		for _, drop := range []bool{false, true} {
			r := render(t, c.src, Options{DropNotes: drop})
			contains(t, r.HTML, `<div class="box" id="todo-1-1"`, `<h1 id="`+c.heading+`"`, `markdown-alert-todo" id="todo-1"`)
			uniqueIDs(t, "review3", r.HTML)
			contains(t, regexp.MustCompile(`(?s)<nav class="toc".*?</nav>`).FindString(r.HTML), `href="#`+c.heading+`"`)
			if len(r.TOC) != 1 || r.TOC[0].ID != c.heading {
				t.Errorf("[%d] DropNotes=%v: 목차 %+v", i+1, drop, r.TOC)
			}
		}
	}
}
