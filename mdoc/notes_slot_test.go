package mdoc

import (
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// itda-hyve#781 — 노트 없는 장에 새 :::notes 를 넣을 자리.
func TestNoteSlotAt(t *testing.T) {
	const paras = "첫 문단\n둘째 줄\n\n둘째 문단\n"
	const twoBlank = "첫 문단\n\n\n둘째 문단\n"
	const code = "# 장\n\n```go\nx := 1\ny := 2\n```\n\n# 다음\n"
	const table = "| a | b |\n|---|---|\n| 1 | 2 |\n\nTable: 캡션\n\n뒤 문단\n"
	const cols = ":::columns\n\n:::column\n왼쪽\n:::\n\n:::column\n오른쪽\n:::\n\n:::\n다음 줄\n"
	const list = "- 하나\n- 둘\n  - 셋\n\n뒤\n"
	const head = "# 제목만\n## 다음 제목\n"
	const front = "---\ntitle: 시험\nslides: true\n---\n\n# 장\n"
	const unclosed = "# 장\n\n```\n코드\n"
	cases := []struct {
		name string
		src  string
		line int
		ok   bool
		want NoteSlot
	}{
		{"문단 뒤 — 문서 끝 빈 마지막 줄", "문단\n", 1, true, NoteSlot{Line: 2, PadBefore: true}},
		{"문단 뒤 — 끝 줄바꿈 없음", "문단", 1, true, NoteSlot{Line: 2, PadBefore: true}},
		{"두 문단 사이 빈 줄 하나 — 빈 줄 다음", paras, 2, true, NoteSlot{Line: 4, PadAfter: true}},
		{"두 문단 사이 빈 줄 둘 — 덧붙임 없음", twoBlank, 1, true, NoteSlot{Line: 3}},
		{"코드 블록 안 줄 — 닫는 펜스 다음", code, 4, true, NoteSlot{Line: 8, PadAfter: true}},
		{"표 + 캡션 뒤", table, 2, true, NoteSlot{Line: 7, PadAfter: true}},
		{"캡션 줄도 같다", table, 5, true, NoteSlot{Line: 7, PadAfter: true}},
		{":::columns 안 블록 — 컨테이너 닫는 줄 다음", cols, 8, true, NoteSlot{Line: 12, PadBefore: true, PadAfter: true}},
		{"목록 안 항목 — 목록 끝 다음", list, 3, true, NoteSlot{Line: 5, PadAfter: true}},
		{"제목 줄 — 다음 블록이 바로 이어짐", head, 1, true, NoteSlot{Line: 2, PadBefore: true, PadAfter: true}},
		{"line 0 + frontmatter — frontmatter 뒤 빈 줄 다음", front, 0, true, NoteSlot{Line: 6, PadAfter: true}},
		{"line 0 + frontmatter 바로 뒤 제목", "---\ntitle: 시험\n---\n# 장\n", 0, true, NoteSlot{Line: 4, PadBefore: true, PadAfter: true}},
		{"line 0 frontmatter 없음 — 1", "# 장\n", 0, true, NoteSlot{Line: 1, PadAfter: true}},
		{"닫히지 않은 펜스 — 문서 끝", unclosed, 4, true, NoteSlot{Line: 5, PadBefore: true}},
		{"블록 사이 빈 줄", paras, 3, false, NoteSlot{}},
		{"frontmatter 안", front, 2, false, NoteSlot{}},
		{"범위 밖", paras, 99, false, NoteSlot{}},
		{"음수", paras, -1, false, NoteSlot{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := NoteSlotAt([]byte(c.src), c.line)
			if ok != c.ok || got != c.want {
				t.Fatalf("NoteSlotAt(%d) = %+v, %v — 기대 %+v, %v", c.line, got, ok, c.want, c.ok)
			}
			if !ok || c.name == "닫히지 않은 펜스 — 문서 끝" {
				return
			}
			checkNoteSlotRoundTrip(t, c.src, got)
		})
	}
}

// 닫히지 않은 펜스 뒤에 넣은 글은 코드 안이다 — 화면은 NotesAt 재검사로 거부한다(이 왕복이 그 근거).
func TestNoteSlotAtUnclosedFenceSwallows(t *testing.T) {
	src := "# 장\n\n```\n코드\n"
	slot, ok := NoteSlotAt([]byte(src), 4)
	if !ok {
		t.Fatal("자리를 못 찾음")
	}
	next, open := insertNote(src, slot, "본문")
	if n, ok := NotesAt([]byte(next), open); ok && n.Inner == "본문" {
		t.Errorf("닫히지 않은 펜스 안에서 노트가 됨: %q", next)
	}
}

func TestNoteSlotAtDepthGuard(t *testing.T) {
	src := strings.Repeat(":::box\n", MaxNestingDepth+5) + "글\n"
	if got, ok := NoteSlotAt([]byte(src), MaxNestingDepth+6); ok {
		t.Errorf("깊이 상한을 넘었는데 %+v", got)
	}
}

// insertNote 는 화면(app.js finishNoteEditNow)이 넣는 글을 똑같이 만든다 — 돌려주는 open 은 여는 ::: 줄이다.
func insertNote(src string, slot NoteSlot, md string) (string, int) {
	text := ":::notes\n" + md + "\n:::\n"
	if slot.PadBefore {
		text = "\n" + text
	}
	if slot.PadAfter {
		text += "\n"
	}
	lines := strings.Split(src, "\n")
	pos := len(src)
	if slot.Line <= len(lines) {
		pos = 0
		for i := 0; i < slot.Line-1; i++ {
			pos += len(lines[i]) + 1
		}
	} else if !strings.HasSuffix(src, "\n") {
		text = "\n" + text
	}
	open := slot.Line
	if slot.PadBefore {
		open++
	}
	return src[:pos] + text + src[pos:], open
}

// checkNoteSlotRoundTrip 은 그 자리에 노트를 넣은 원문을 렌더해 노트가 정확히 하나 늘고(그 안 글은 「본문」), 노트 밖 요소의 꼴·차례가
// 그대로인지, NotesAt 이 여는 줄에서 그 노트를 읽는지 본다.
func checkNoteSlotRoundTrip(t *testing.T, src string, slot NoteSlot) {
	t.Helper()
	next, open := insertNote(src, slot, "본문")
	beforeNotes, beforeTags := noteOutline(t, src)
	afterNotes, afterTags := noteOutline(t, next)
	if len(afterNotes) != len(beforeNotes)+1 {
		t.Fatalf("노트 수 %d → %d: %q", len(beforeNotes), len(afterNotes), next)
	}
	if !slices.Contains(afterNotes, "본문") {
		t.Errorf("넣은 노트 글이 「본문」이 아님 %q: %q", afterNotes, next)
	}
	if !slices.Equal(beforeTags, afterTags) {
		t.Errorf("노트 밖 블록이 바뀜\n전 %v\n뒤 %v\n원문 %q", beforeTags, afterTags, next)
	}
	if n, ok := NotesAt([]byte(next), open); !ok || n.Inner != "본문" {
		t.Errorf("NotesAt(%d) = %+v, %v: %q", open, n, ok, next)
	}
}

// noteOutline 은 렌더한 HTML 의 노트 요소마다 안 글(앞뒤 공백 뺌)과, 노트 밖 요소의 이름 차례다.
func noteOutline(t *testing.T, src string) (notes, tags []string) {
	t.Helper()
	res, err := Render([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	root, err := html.Parse(strings.NewReader(res.HTML))
	if err != nil {
		t.Fatal(err)
	}
	var text func(*html.Node) string
	text = func(n *html.Node) string {
		if n.Type == html.TextNode {
			return n.Data
		}
		var b strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			b.WriteString(text(c))
		}
		return b.String()
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if isNotesEl(n) {
				notes = append(notes, strings.TrimSpace(text(n)))
				return
			}
			tags = append(tags, n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return notes, tags
}
