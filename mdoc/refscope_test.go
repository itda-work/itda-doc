package mdoc

import (
	"strings"
	"testing"
)

// 노트 안 참조 정의는 그 노트 안 참조에만 쓴다(itda-doc#89, refscope.go). 노트를 싣는 렌더와 빼는 렌더 모두에서 본다.

// refRender 는 src 를 노트를 싣고(drop=false)·빼고(drop=true) 그린다.
func refRender(t *testing.T, src string, drop bool) Result {
	t.Helper()
	r, err := Render([]byte(src), Options{DropNotes: drop})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func refWarns(r Result) int {
	n := 0
	for _, w := range r.Warnings {
		if w.Code == WarnNoteRefScope {
			n++
		}
	}
	return n
}

func TestNoteRefScopeBody(t *testing.T) {
	// 이슈 재현(그림 참조의 제목 → 본문 캡션)과 링크 참조(주소·title)
	cases := map[string]struct{ src, private, literal string }{
		"그림 참조": {"---\nslides: true\n---\n\n## 공개\n\n![공개 그림][pic]\n\n:::notes\n[pic]: missing.png \"PRIVATE-NOTE-REF-CAPTION-896\"\n:::\n",
			"PRIVATE-NOTE-REF-CAPTION-896", "![공개 그림][pic]"},
		"그림 참조(줄임꼴)": {"## 공개\n\n![pic]\n\n:::notes\n[pic]: private-pic.png \"PRIVATE-SHORT-CAPTION\"\n:::\n", "PRIVATE-SHORT", "![pic]"},
		"링크 참조": {"## 공개\n\n[공개 글][lnk] 과 [lnk]\n\n:::notes\n메모\n\n[lnk]: https://private.example/secret \"PRIVATE-LINK-TITLE\"\n:::\n",
			"private.example", "[공개 글][lnk]"},
	}
	for name, c := range cases {
		for _, drop := range []bool{false, true} {
			r := refRender(t, c.src, drop)
			if strings.Contains(r.HTML, c.private) || strings.Contains(r.HTML, "PRIVATE-") {
				t.Errorf("%s(drop=%v): 노트 안 정의가 본문에 쓰였다\n%s", name, drop, r.HTML)
			}
			if !strings.Contains(r.HTML, htmlEsc(c.literal)) {
				t.Errorf("%s(drop=%v): 정의 없는 참조처럼 글자로 남지 않았다(%q)\n%s", name, drop, c.literal, r.HTML)
			}
			if refWarns(r) == 0 {
				t.Errorf("%s(drop=%v): %s 경고가 없다 %+v", name, drop, WarnNoteRefScope, r.Warnings)
			}
		}
	}
}

func TestNoteRefScopeInside(t *testing.T) {
	// 노트 안 참조는 그 노트 안 정의를 쓴다(노트를 싣는 렌더) — 경고 없음
	src := "## 공개\n\n본문\n\n:::notes\n![노트 그림][np] 와 [노트 링크][nl]\n\n[np]: note.png \"NOTE-CAPTION\"\n[nl]: https://note.example/ \"NOTE-LINK\"\n:::\n"
	r := refRender(t, src, false)
	if !strings.Contains(r.HTML, `src="note.png"`) || !strings.Contains(r.HTML, "NOTE-CAPTION") || !strings.Contains(r.HTML, `href="https://note.example/"`) || refWarns(r) != 0 {
		t.Errorf("노트 안 참조가 노트 안 정의를 쓰지 못했다 %+v\n%s", r.Warnings, r.HTML)
	}
	if r2 := refRender(t, src, true); strings.Contains(r2.HTML, "NOTE") || strings.Contains(r2.HTML, "note.") {
		t.Errorf("노트를 뺀 렌더에 노트 글:\n%s", r2.HTML)
	}
	// 안쪽 노트(노트 안 인용 속 노트)의 참조도 바깥 노트의 정의를 본다
	nested := "## 공개\n\n:::notes\n> :::notes\n> [안쪽][o]\n> :::\n\n[o]: https://outer.example/\n:::\n"
	if r := refRender(t, nested, false); !strings.Contains(r.HTML, `href="https://outer.example/"`) || refWarns(r) != 0 {
		t.Errorf("안쪽 노트가 바깥 노트 정의를 못 봤다 %+v\n%s", r.Warnings, r.HTML)
	}
}

func TestNoteRefScopeDocDefs(t *testing.T) {
	// 노트 밖 정의는 노트 안 참조에서도 쓴다. 같은 이름이면 보이는 것 가운데 먼저 적은 것 — 본문은 본문 정의, 노트는 먼저 적은 노트 정의
	src := "## 공개\n\n[본문][d] [같은][same]\n\n:::notes\n[노트][d] [같은][same]\n\n[same]: https://note-same.example/\n:::\n\n[d]: https://doc.example/ \"DOC\"\n[same]: https://doc-same.example/\n"
	r := refRender(t, src, false)
	if strings.Count(r.HTML, `href="https://doc.example/"`) != 2 {
		t.Errorf("노트 밖 정의를 노트 안·밖 참조가 모두 써야 한다\n%s", r.HTML)
	}
	body, notes, _ := strings.Cut(r.HTML, `<div class="notes"`)
	if !strings.Contains(body, "doc-same.example") || strings.Contains(body, "note-same") || !strings.Contains(notes, "note-same.example") || refWarns(r) != 0 {
		t.Errorf("같은 이름: 본문은 본문 정의, 노트는 먼저 적은 노트 정의 %+v\n%s", r.Warnings, r.HTML)
	}
	// 다른 노트의 정의는 보지 않는다
	other := "## 공개\n\n:::notes\n[a]: https://note-a.example/\n:::\n\n:::notes\n[다른 노트][a]\n:::\n"
	if r := refRender(t, other, false); strings.Contains(r.HTML, "note-a.example") || refWarns(r) != 1 {
		t.Errorf("다른 노트의 정의를 썼다 %+v\n%s", r.Warnings, r.HTML)
	}
	// 정의가 아예 없는 참조는 지금처럼 글자, 이 경고는 없다
	if r := refRender(t, "## 공개\n\n[없음][none]\n", false); refWarns(r) != 0 {
		t.Errorf("정의 없는 참조에 노트 경고: %+v", r.Warnings)
	}
}
