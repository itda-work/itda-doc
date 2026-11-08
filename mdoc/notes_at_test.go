package mdoc

import (
	"strings"
	"testing"
)

func TestNotesAt(t *testing.T) {
	const fenced = "# 제목\n\n본문\n\n:::notes\n첫 줄\n\n둘째 줄\n:::\n\n뒤 문단\n"
	const front = "---\ntitle: 시험\nslides: true\n---\n\n# 제목\n\n:::notes\n노트\n:::\n"
	const attr = "::: {.notes}\n속성꼴\n:::\n"
	const named = "::: box {.notes}\n이름과 클래스\n:::\n"
	const inBox = ":::box\n밖\n\n:::notes\n상자 안\n:::\n\n밖 끝\n:::\n"
	const nested = ":::notes\n바깥 위\n\n:::notes\n안쪽\n:::\n\n바깥 아래\n:::\n"
	const empty = ":::notes\n:::\n"
	const crlf = ":::notes\r\n윈도 줄\r\n:::\r\n"
	const fence = ":::notes\n```\n:::\n```\n:::\n"
	cases := []struct {
		name string
		src  string
		line int
		ok   bool
		want Notes
	}{
		{"펜스꼴 여는 줄", fenced, 5, true, Notes{Open: 5, Close: 9, InnerStart: 6, InnerEnd: 8, Inner: "첫 줄\n\n둘째 줄"}},
		{"펜스꼴 안쪽 빈 줄", fenced, 7, true, Notes{Open: 5, Close: 9, InnerStart: 6, InnerEnd: 8, Inner: "첫 줄\n\n둘째 줄"}},
		{"펜스꼴 닫는 줄", fenced, 9, true, Notes{Open: 5, Close: 9, InnerStart: 6, InnerEnd: 8, Inner: "첫 줄\n\n둘째 줄"}},
		{"앞 줄은 노트가 아니다", fenced, 4, false, Notes{}},
		{"뒤 줄은 노트가 아니다", fenced, 10, false, Notes{}},
		{"frontmatter 를 포함한 원문 줄", front, 9, true, Notes{Open: 8, Close: 10, InnerStart: 9, InnerEnd: 9, Inner: "노트"}},
		{"frontmatter 줄은 노트가 아니다", front, 2, false, Notes{}},
		{"속성꼴", attr, 2, true, Notes{Open: 1, Close: 3, InnerStart: 2, InnerEnd: 2, Inner: "속성꼴"}},
		{"이름과 클래스", named, 2, true, Notes{Open: 1, Close: 3, InnerStart: 2, InnerEnd: 2, Inner: "이름과 클래스"}},
		{"상자 안", inBox, 5, true, Notes{Open: 4, Close: 6, InnerStart: 5, InnerEnd: 5, Inner: "상자 안"}},
		{"상자의 다른 줄은 노트가 아니다", inBox, 2, false, Notes{}},
		{"중첩 — 안쪽", nested, 5, true, Notes{Open: 4, Close: 6, InnerStart: 5, InnerEnd: 5, Inner: "안쪽"}},
		{"중첩 — 바깥 위", nested, 2, true, Notes{Open: 1, Close: 9, InnerStart: 2, InnerEnd: 8, Inner: "바깥 위\n\n:::notes\n안쪽\n:::\n\n바깥 아래"}},
		{"중첩 — 바깥 아래", nested, 8, true, Notes{Open: 1, Close: 9, InnerStart: 2, InnerEnd: 8, Inner: "바깥 위\n\n:::notes\n안쪽\n:::\n\n바깥 아래"}},
		{"안쪽이 빈 노트", empty, 1, true, Notes{Open: 1, Close: 2, InnerStart: 2, InnerEnd: 1}},
		{"CRLF 는 정규화한다", crlf, 2, true, Notes{Open: 1, Close: 3, InnerStart: 2, InnerEnd: 2, Inner: "윈도 줄"}},
		{"코드 안의 ::: 는 닫는 줄이 아니다", fence, 3, true, Notes{Open: 1, Close: 5, InnerStart: 2, InnerEnd: 4, Inner: "```\n:::\n```"}},
		{"줄 0", fenced, 0, false, Notes{}},
		{"음수 줄", fenced, -1, false, Notes{}},
		{"줄 밖", fenced, 999, false, Notes{}},
		{"닫히지 않은 펜스", ":::notes\n안 닫음\n", 2, false, Notes{}},
		{"닫는 줄은 가장 안쪽 것 — 상자가 안 닫혀도 노트는 닫힌다", ":::box\n:::notes\n글\n:::\n", 3, true, Notes{Open: 2, Close: 4, InnerStart: 3, InnerEnd: 3, Inner: "글"}},
		{"닫는 줄을 안쪽에 준 바깥 노트는 닫히지 않았다", ":::notes\n:::notes\n글\n:::\n", 1, false, Notes{}},
		{"닫히지 않은 안쪽은 바깥으로 올라가지 않는다", ":::notes\n> :::notes\n> 안 닫음\n\n바깥\n:::\n", 3, false, Notes{}},
		{"닫히지 않은 안쪽의 뒤 줄은 바깥 노트다", ":::notes\n> :::notes\n> 안 닫음\n\n바깥\n:::\n", 5, true, Notes{Open: 1, Close: 6, InnerStart: 2, InnerEnd: 5, Inner: "> :::notes\n> 안 닫음\n\n바깥"}},
		{"닫히지 않은 인용 속 노트의 뒤 문단", "> :::notes\n> 안 닫음\n\n뒤 문단\n", 4, false, Notes{}},
		{"다른 컨테이너", ":::box\n글\n:::\n", 2, false, Notes{}},
		{"제목의 .notes 는 노트가 아니다", "# 제목 {.notes}\n\n글\n", 1, false, Notes{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := NotesAt([]byte(c.src), c.line)
			if ok != c.ok || got != c.want {
				t.Errorf("NotesAt(%d) = %+v, %v — 기대 %+v, %v", c.line, got, ok, c.want, c.ok)
			}
		})
	}
}

// 깊이 상한을 넘는 문서는 렌더가 원문으로 보이므로(renderRaw) 노트 자리도 없다 — TableAt 과 같은 가드다.
func TestNotesAtDepthGuard(t *testing.T) {
	src := strings.Repeat(":::box\n", MaxNestingDepth+5) + ":::notes\n글\n:::\n"
	if got, ok := NotesAt([]byte(src), MaxNestingDepth+7); ok {
		t.Errorf("깊이 상한을 넘었는데 %+v", got)
	}
}

// 렌더의 노트 자리(data-line)를 그대로 넘기면 그 노트를 찾는다 — 화면이 노트 칸의 줄 번호로 부르는 길이다(itda-hyve#763).
func TestNotesAtMatchesRenderedLine(t *testing.T) {
	src := "---\nslides: true\n---\n\n# 장\n\n본문\n\n:::notes\n노트 글\n:::\n"
	res, err := Render([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.HTML, `class="notes" data-line="9"`) {
		t.Fatalf("노트 자리 줄이 9 가 아님: %s", res.HTML)
	}
	n, ok := NotesAt([]byte(src), 9)
	if !ok || n.Open != 9 || n.Close != 11 || n.Inner != "노트 글" {
		t.Errorf("NotesAt(9) = %+v, %v", n, ok)
	}
}
