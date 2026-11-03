package mdoc

import (
	"strings"
	"testing"
)

func TestEscapeCell(t *testing.T) {
	for in, want := range map[string]string{
		"  a  ":       "a",
		"x\ny":        "x<br>y",
		"x\r\ny":      "x<br>y",
		"a\tb":        "a b",
		"a\x01b\x7f":  "ab",
		"<br>":        `\<br\>`,
		"a | b":       `a \| b`,
		"**b** _i_":   `\*\*b\*\* \_i\_`,
		"[l](u) ![i]": `\[l\](u) \!\[i\]`,
		"`c` $x$ ~s~": "\\`c\\` \\$x\\$ \\~s\\~",
		"&amp; \\":    `\&amp; \\`,
		"-->":         `--\>`,
		"가·①":         "가·①",
	} {
		if got, _, _ := EscapeCell(in); got != want {
			t.Errorf("EscapeCell(%q) = %q, 기대 %q", in, got, want)
		}
	}
	if _, brs, dropped := EscapeCell("a\nb\nc\x02"); brs != 2 || dropped != 1 {
		t.Errorf("brs=%d dropped=%d", brs, dropped)
	}
}

func TestTopBlockAt(t *testing.T) {
	src := strings.Join([]string{
		"---", "title: t", "---", // 1-3
		"",                              // 4
		"<!-- c -->",                    // 5
		"| a | b |",                     // 6
		"|---|--:|",                     // 7
		"| 1 | 2 |",                     // 8
		"",                              // 9
		"Table: 캡션",                     // 10
		"",                              // 11
		":::chart{type=bar unit=\"원\"}", // 12
		"| k | v |",                     // 13
		"|---|---|",                     // 14
		"| x | 1 |",                     // 15
		":::",                           // 16
		"",                              // 17
		"- | a |",                       // 18
		"  |---|",                       // 19
		"",                              // 20
		":::note",                       // 21
		"| n |",                         // 22
		"|---|",                         // 23
		":::",                           // 24
		"",                              // 25
		":::chart",                      // 26
		"| k | v |",                     // 27
		"|---|---|",                     // 28
	}, "\n")
	type want struct {
		kind             string
		start, end       int
		closed, hasTable bool
	}
	for line, w := range map[int]want{
		5: {"html", 5, 5, true, false}, 6: {"table", 6, 10, true, true}, 9: {"table", 6, 10, true, true}, 10: {"table", 6, 10, true, true},
		12: {"chart", 12, 16, true, true}, 15: {"chart", 12, 16, true, true}, 16: {"chart", 12, 16, true, true},
		26: {"chart", 26, 0, false, true}, 28: {"chart", 26, 0, false, true},
	} {
		b, ok := TopBlockAt([]byte(src), line)
		if !ok || b.Kind != w.kind || b.Start != w.start || b.End != w.end || b.Closed != w.closed || b.HasTable != w.hasTable {
			t.Errorf("줄 %d: %+v %v, 기대 %+v", line, b, ok, w)
		}
	}
	for _, line := range []int{0, 1, 2, 4, 11, 17, 18, 19, 21, 22, 99} {
		if b, ok := TopBlockAt([]byte(src), line); ok {
			t.Errorf("줄 %d: 최상위 표·차트가 아닌데 %+v", line, b)
		}
	}
	b, _ := TopBlockAt([]byte(src), 13)
	if b.Table.Start != 13 || !b.Table.InChart || len(b.ChartAttrs) != 2 || b.ChartAttrs[1] != [2]string{"unit", "원"} {
		t.Errorf("차트 안 표·속성: %+v", b)
	}
	b, _ = TopBlockAt([]byte(src), 8)
	if b.Table.Caption != "캡션" || b.Table.Header[1] != "b" || b.Table.Align[1] != "right" {
		t.Errorf("표: %+v", b.Table)
	}
	// 여러 줄 HTML 블록, CRLF
	b, ok := TopBlockAt([]byte("<!-- a\nb -->\n\n문단"), 2)
	if !ok || b.Kind != "html" || b.Start != 1 || b.End != 2 {
		t.Errorf("여러 줄 주석: %+v %v", b, ok)
	}
	b, ok = TopBlockAt([]byte("x\r\n\r\n| a |\r\n|---|\r\n"), 3)
	if !ok || b.Kind != "table" || b.Start != 3 {
		t.Errorf("CRLF: %+v %v", b, ok)
	}
}

// 자료 표 넣기(itda-hyve#365 ③ 조각 C)의 자리: 최상위 빈 줄은 그 줄, 최상위 블록 안은 그 블록 끝 다음 줄(표는 캡션까지), 목록·인용·컨테이너·
// frontmatter 안은 거부.
func TestInsertSpotAt(t *testing.T) {
	src := strings.Join([]string{
		"---", "title: t", "---", // 1-3
		"",                 // 4
		"첫 문단 한 줄", "둘째 줄", // 5-6
		"",                         // 7
		"- 목록", "  이어짐", "", "- 둘", // 8-11
		"",            // 12
		"> 인용", "> 둘", // 13-14
		"",                       // 15
		":::columns", "왼", ":::", // 16-18
		"",                                                 // 19
		"| a | b |", "|---|---|", "| 1 | 2 |", "Table: 캡션", // 20-23
		"",                               // 24
		"```", "code", "", "more", "```", // 25-29
		"",      // 30
		"## 제목", // 31
		"끝 문단",  // 32
	}, "\n")
	for _, c := range []struct {
		line int
		want int
		kind string
	}{
		{1, 0, "frontmatter"}, {3, 0, "frontmatter"},
		{4, 4, ""}, {5, 7, ""}, {6, 7, ""}, {7, 7, ""},
		{8, 0, "list"}, {9, 0, "list"}, {10, 0, "list"}, {11, 0, "list"},
		{12, 12, ""},
		{13, 0, "quote"}, {14, 0, "quote"},
		{16, 0, "container"}, {17, 0, "container"}, {18, 0, "container"},
		{20, 24, ""}, {22, 24, ""}, {23, 24, ""},
		{26, 30, ""}, {27, 30, ""},
		{31, 32, ""}, {32, 33, ""},
	} {
		got, ok := InsertSpotAt([]byte(src), c.line)
		if ok != (c.kind == "") || got.Line != c.want || got.Kind != c.kind {
			t.Errorf("줄 %d: %+v %v, 기대 줄 %d 꼴 %q", c.line, got, ok, c.want, c.kind)
		}
	}
	for _, line := range []int{0, 33} {
		if _, ok := InsertSpotAt([]byte(src), line); ok {
			t.Errorf("범위 밖 줄 %d 를 받음", line)
		}
	}
	// 끝 줄바꿈이 있는 문서의 마지막(빈) 줄, 빈 문서
	if got, ok := InsertSpotAt([]byte("a\n"), 2); !ok || got.Line != 2 {
		t.Errorf("끝 빈 줄: %+v %v", got, ok)
	}
	if got, ok := InsertSpotAt(nil, 1); !ok || got.Line != 1 {
		t.Errorf("빈 문서: %+v %v", got, ok)
	}
	// 다단 안 표·목록 안 표도 컨테이너·목록이다
	if got, ok := InsertSpotAt([]byte(":::columns\n| a |\n|---|\n| 1 |\n:::\n"), 3); ok || got.Kind != "container" {
		t.Errorf("다단 안 표: %+v %v", got, ok)
	}
	if got, ok := InsertSpotAt([]byte("- x\n\n  | a |\n  |---|\n"), 3); ok || got.Kind != "list" {
		t.Errorf("목록 안 표: %+v %v", got, ok)
	}
}
