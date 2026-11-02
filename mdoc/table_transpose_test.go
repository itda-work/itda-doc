package mdoc

import (
	"strings"
	"testing"
)

// 표 전치(#468 — 미리보기 표 오른쪽 클릭 메뉴 「행/열 바꾸기」). TransposeTable 은 [Header, ...Rows] 를 뒤집는다:
// 왼쪽 위 칸은 제자리, 새 머리 행 = 옛 첫 열. 열 정렬·캡션·접두 규칙을 여기 본다.

// equalTable 은 칸·정렬의 구조만 본다(빈 칸과 nil 을 같게 본다).
func equalTable(a, b Table) bool {
	if len(a.Header) != len(b.Header) || len(a.Rows) != len(b.Rows) {
		return false
	}
	cols := len(b.Header)
	row := func(s []string) string { return strings.Join(pad(s, cols), "\x00") }
	if row(a.Header) != row(b.Header) {
		return false
	}
	for r := range a.Rows {
		if row(a.Rows[r]) != row(b.Rows[r]) {
			return false
		}
	}
	return row(a.Align) == row(b.Align)
}

func TestTransposeTable(t *testing.T) {
	// 2행(머리+본문 1) × 3열 → 3행(머리+본문 2) × 2열 — 왼쪽 위 칸 a 는 제자리다
	tb := mustTable(t, "| a | b | c |\n|---|---|---|\n| 1 | 2 | 3 |\n", 1)
	want := Table{Start: 1, End: 3,
		Header: []string{"a", "1"}, Align: []string{"", ""},
		Rows: [][]string{{"b", "2"}, {"c", "3"}}}
	if got := TransposeTable(tb); !equalTable(got, want) || got.Start != want.Start || got.End != want.End {
		t.Fatalf("전치\n 얻음 %#v\n 기대 %#v", got, want)
	}
	// 줄 범위·캡션·접두·차트 판정은 그대로다(화면이 그 표 줄만 바꿔 넣는다)
	c := mustTable(t, ":::chart{type=bar}\n| 분기 | 매출 |\n|:--|--:|\n| 1분기 | 3 |\n\n: 분기 매출\n:::\n", 2)
	tc := TransposeTable(c)
	if tc.Start != c.Start || tc.End != c.End || tc.CaptionStart != c.CaptionStart || tc.CaptionEnd != c.CaptionEnd ||
		tc.Caption != c.Caption || tc.CaptionPrefix != c.CaptionPrefix || tc.InChart != c.InChart ||
		tc.HeadPrefix != c.HeadPrefix || tc.Prefix != c.Prefix {
		t.Fatalf("전치가 줄 범위·캡션·접두·차트 판정을 바꿈\n 얻음 %#v\n 원본 %#v", tc, c)
	}
}

func TestTransposeTableAlign(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string // 전치한 표의 열 정렬
	}{
		{"옛 2열 이후가 섞이면 없음", "| a | b | c |\n|:--|--:|:-:|\n| 1 | 2 | 3 |\n", []string{"left", ""}},
		{"옛 2열 이후가 모두 같으면 그 값", "| a | b | c |\n|:--|--:|--:|\n| 1 | 2 | 3 |\n", []string{"left", "right"}},
		{"정렬 없음", "| a | b |\n|---|---|\n| 1 | 2 |\n", []string{"", ""}},
		{"옛 첫 열만 정렬", "| a | b |\n|:-:|---|\n| 1 | 2 |\n", []string{"center", ""}},
		// 본문 행이 없으면 새 표는 한 열(머리 a)이다
		{"본문 행 없음", "| a | b |\n|:--|---|\n", []string{"left"}},
	}
	for _, c := range cases {
		if got := TransposeTable(mustTable(t, c.src, 1)); strings.Join(got.Align, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s: 정렬 얻음 %v 기대 %v", c.name, got.Align, c.want)
		}
	}
}

// 왕복 — 캡션·인용·목록 접두·차트 안 표를 읽어 전치해 끼워 넣으면(FormatTable — 화면이 replace 하는 모양) 다시 읽어
// 같은 구조가 나온다.
func TestTransposeRoundTrip(t *testing.T) {
	cases := []struct {
		name, src string
		line      int
	}{
		{"캡션", "| a | b |\n|---|---|\n| 1 | 2 |\n\nTable: 제목\n", 1},
		{"인용", "> | a | b |\n> |---|---|\n> | 1 | 2 |\n>\n> Table: 인용 표\n", 3},
		{"목록", "- 항목\n\n  | a | b |\n  |---|---|\n  | 1 | 2 |\n", 5},
		{"차트", ":::chart{type=bar}\n| 분기 | 매출 |\n|:--|--:|\n| 1분기 | 3 |\n\n: 분기 매출\n:::\n", 2},
		{"머리 행보다 긴 행(열이 늘어남)", "| a |\n|---|\n| 1 | 2 |\n", 1},
	}
	for _, c := range cases {
		tb := mustTable(t, c.src, c.line)
		tr := TransposeTable(tb)
		whole := spliceTable(c.src, tb, FormatTable(tr))
		got, ok := TableAt([]byte(whole), tb.Start)
		if !ok {
			t.Errorf("%s: 전치한 표를 다시 읽지 못함\n%s", c.name, whole)
			continue
		}
		if !equalTable(got, tr) {
			t.Errorf("%s: 왕복\n 얻음 %#v\n 기대 %#v\n원문:\n%s", c.name, got, tr, whole)
		}
	}
}

// spliceTable 는 src 의 표 줄 범위(캡션 포함)를 md 로 바꿔 넣는다(화면의 replace 와 같은 계산).
func spliceTable(src string, tb Table, md string) string {
	lines := strings.Split(src, "\n")
	end := max(tb.End, tb.CaptionEnd)
	out := append([]string{}, lines[:tb.Start-1]...)
	out = append(out, strings.Split(md, "\n")...)
	return strings.Join(append(out, lines[end:]...), "\n")
}

func TestTransposeTableMarkdown(t *testing.T) {
	// 쓴 원문 — 열 너비 맞춤·캡션 문단 유지
	tb := mustTable(t, "| a | b | c |\n|---|---|---|\n| 1 | 2 | 3 |\n\nTable: 제목\n", 1)
	if got, want := FormatTable(TransposeTable(tb)), "| a   | 1   |\n| --- | --- |\n| b   | 2   |\n| c   | 3   |\n\nTable: 제목"; got != want {
		t.Fatalf("쓴 원문\n 얻음 %q\n 기대 %q", got, want)
	}
}
