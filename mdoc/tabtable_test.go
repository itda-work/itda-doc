package mdoc

import (
	"reflect"
	"strings"
	"testing"
)

// 탭으로 나뉜 표(itda-hyve#579) — TabTable(글만)·TabBlockAt(문서의 탭 줄)·TabPasteSpot(붙여 넣은 자리).

const tabSample = "모델\t이런 질문이면\t맞는 일\t비용 감각\n" +
	"Haiku\t실수해도 금방 다시 하면 되나?\t분류 · 추출 · 짧은 정리\t×1\n" +
	"Sonnet\t늘 하던 일반 업무인가?\t회의록 · 보고서 · 자료 비교\t×2\n" +
	"Opus\t계획이 필요하거나 결과 품질이 중요한가?\t계획 · 정밀 검토 · 구현\t×5\n" +
	"Fable\t가장 어려운 문제인가?\t최상위 — 플랜에 따라 크레딧\t×10\n"

func TestTabTableSample(t *testing.T) {
	tb, err := TabTable(tabSample)
	if err != nil {
		t.Fatal("이슈 예를 표로 읽지 못함")
	}
	if !tb.Tabs || len(tb.Header) != 4 || len(tb.Rows) != 4 || !reflect.DeepEqual(tb.Align, []string{"", "", "", ""}) {
		t.Fatalf("모양: %#v", tb)
	}
	md := FormatTable(tb)
	want := "| 모델   | 이런 질문이면                           | 맞는 일                     | 비용 감각 |\n" +
		"| ------ | --------------------------------------- | --------------------------- | --------- |\n" +
		"| Haiku  | 실수해도 금방 다시 하면 되나?           | 분류 · 추출 · 짧은 정리     | ×1        |\n" +
		"| Sonnet | 늘 하던 일반 업무인가?                  | 회의록 · 보고서 · 자료 비교 | ×2        |\n" +
		"| Opus   | 계획이 필요하거나 결과 품질이 중요한가? | 계획 · 정밀 검토 · 구현     | ×5        |\n" +
		"| Fable  | 가장 어려운 문제인가?                   | 최상위 — 플랜에 따라 크레딧 | ×10       |"
	if md != want {
		t.Fatalf("표 원문\n얻음\n%s\n기대\n%s", md, want)
	}
	// 쓴 표는 렌더와 같은 파서로 다시 읽으면 같은 칸이다
	back, ok := TableAt([]byte(md+"\n"), 1)
	if !ok || !reflect.DeepEqual(back.Header, tb.Header) || !reflect.DeepEqual(back.Rows, tb.Rows) {
		t.Fatalf("다시 읽기: %v %#v", ok, back)
	}
}

func TestTabTableRules(t *testing.T) {
	yes := map[string][2]int{ // 글 → [열, 본문 행]
		"a\tb\nc\td":             {2, 1},
		"a\tb\r\nc\td\r\n":       {2, 1}, // \r\n 정리, 끝 줄바꿈 하나 무시
		"a\tb\rc\td":             {2, 1},
		"a\t\tc\n\tb\t":          {3, 1}, // 빈 칸도 탭은 남는다
		"\tQ1\tQ2\nA\t1\t2":      {3, 1}, // 머리 행의 빈 첫 칸(피벗) — 모든 줄이 탭으로 시작하지는 않음
		"a\tb\nc\td\ne\tf\ng\th": {2, 3},
		"a\tb\n\t\nc\td":         {2, 2}, // 모든 칸이 빈 행(리뷰 1회차 [3])
		"a\tb\n \t \nc\td":       {2, 2}, // 공백·탭만 있는 행도 칸 수가 맞으면 행
		"#\tName\n1\tA":          {2, 1}, // 마크다운처럼 보이는 첫 칸은 글자일 뿐
	}
	for in, w := range yes {
		tb, err := TabTable(in)
		if err != nil || len(tb.Header) != w[0] || len(tb.Rows) != w[1] {
			t.Errorf("%q: 표여야 함 — %v %#v", in, err, tb)
		}
	}
	no := []string{
		"",
		"a\tb",             // 한 줄
		"a\tb\n",           // 한 줄 + 끝 줄바꿈
		"a\tb\nc\td\ne",    // 탭 없는 줄
		"a\tb\nc\td\te",    // 칸 수 불일치
		"a\tb\n\nc\td",     // 빈 줄
		"a\tb\n \nc\td",    // 탭 없는 공백 줄
		"a\tb\n\t\t\nc\td", // 모든 칸이 빈 행이어도 칸 수가 다르면 아님
		"가\n나\n",           // 탭이 없는 줄들(칸 하나)
		"a\tb\n\n",         // 끝 줄바꿈 둘 — 빈 줄
		"\tfoo()\n\tbar()", // 탭 들여쓴 코드
		"\t- 하나\n\t- 둘",    // 탭 들여쓴 목록
		"- 하나\n\t- 둘",      // 목록과 탭 들여쓴 하위 항목
	}
	for _, in := range no {
		if tb, err := TabTable(in); err != ErrNotTabTable {
			t.Errorf("%q: 표가 아니어야 함 — %v %#v", in, err, tb)
		}
	}
}

func TestTabTableCells(t *testing.T) {
	tb, err := TabTable("  이름 \t a|b \n x\\|y\t**굵게**  \n")
	if err != nil {
		t.Fatal("표로 읽지 못함")
	}
	// 앞뒤 공백 뗌, | → \|(이미 \| 면 그대로), 다른 글자는 원문
	if !reflect.DeepEqual(tb.Header, []string{"이름", `a\|b`}) || !reflect.DeepEqual(tb.Rows, [][]string{{`x\|y`, "**굵게**"}}) {
		t.Fatalf("칸: %#v %#v", tb.Header, tb.Rows)
	}
	md := FormatTable(tb)
	back, ok := TableAt([]byte(md), 3)
	if !ok || back.Rows[0][1] != "**굵게**" || back.Header[1] != `a\|b` || len(back.Header) != 2 {
		t.Fatalf("| 이스케이프 뒤 다시 읽기: %v %#v\n%s", ok, back, md)
	}
}

func TestTabBlockAt(t *testing.T) {
	src := "---\ntitle: 시험\n---\n\n# 제목\n\n앞 문단\n\n이름\t값\nA\t1\nB\t2\n\n뒤 문단\n"
	for _, line := range []int{9, 10, 11} {
		tb, ok := TabBlockAt([]byte(src), line)
		if !ok || tb.Start != 9 || tb.End != 11 || !reflect.DeepEqual(tb.Header, []string{"이름", "값"}) || len(tb.Rows) != 2 || !tb.Tabs {
			t.Fatalf("%d 줄: %v %#v", line, ok, tb)
		}
	}
	for _, line := range []int{0, 2, 5, 7, 8, 12, 13, 99} {
		if tb, ok := TabBlockAt([]byte(src), line); ok {
			t.Errorf("%d 줄: 탭 표가 아니어야 함 %#v", line, tb)
		}
	}
	// 제목 바로 아래 붙은 탭 줄도 문단이다. ::: 컨테이너 안도 받는다
	if tb, ok := TabBlockAt([]byte("## 표\na\tb\nc\td\n"), 2); !ok || tb.Start != 2 || tb.End != 3 {
		t.Errorf("제목 아래: %v %#v", ok, tb)
	}
	if tb, ok := TabBlockAt([]byte(":::note\na\tb\nc\td\n:::\n"), 3); !ok || tb.Start != 2 || tb.End != 3 {
		t.Errorf("컨테이너 안: %v %#v", ok, tb)
	}
	no := map[string]int{
		"```\na\tb\nc\td\n```\n":               2, // 코드 블록 안
		"앞\na\tb\nc\td\n":                      2, // 탭 없는 줄이 든 문단
		"a\tb\nc\td\te\n":                      1, // 칸 수 불일치
		"a\tb\n":                               1, // 한 줄
		"- 항목\n  a\tb\n  c\td\n":               2, // 목록 안
		"> a\tb\n> c\td\n":                     1, // 인용 안
		"| x | y |\n|---|---|\n| a\tb | c |\n": 3, // 기존 표
		"\n\tQ1\tQ2\nA\t1\t2\nB\t3\t4\n":       3, // 머리 행이 들여 쓴 코드가 됨 — 앞 줄이 탭 줄
		"a\tb\nc\td\n\t\ne\tf\n":               1, // 모든 칸이 빈 행이 문단을 나눔 — 뒤 줄이 탭 줄
		"#\tName\n1\tA\n2\tB\n":                2, // 첫 줄이 제목이 됨 — 앞 줄이 탭 줄(알려진 한계, GUIDE)
	}
	for src, line := range no {
		if tb, ok := TabBlockAt([]byte(src), line); ok {
			t.Errorf("%q %d 줄: 탭 표가 아니어야 함 %#v", src, line, tb)
		}
	}
}

// 따옴표 칸(엑셀·시트의 TSV 클립보드, 리뷰 1회차 [4]): " 로 시작한 칸은 짝 따옴표까지 한 칸, "" → ", 칸 안 줄바꿈 → <br>
func TestTabTableQuoted(t *testing.T) {
	tb, err := TabTable("a\t\"x\ny\"\tb\n1\t\"말 \"\"그대로\"\"\"\t\"탭\t안\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tb.Header, []string{"a", "x<br>y", "b"}) || !reflect.DeepEqual(tb.Rows, [][]string{{"1", `말 "그대로"`, "탭\t안"}}) {
		t.Fatalf("칸: %#v %#v", tb.Header, tb.Rows)
	}
	// 줄 첫머리가 아닌 " 는 글자다
	if tb, err := TabTable("키\t5\"\n길이\t3\"\n"); err != nil || tb.Rows[0][1] != `3"` {
		t.Fatalf("가운데 따옴표: %v %#v", err, tb)
	}
	// 따옴표 칸은 안에 탭·줄바꿈·"" 가 있을 때만이다(엑셀 규칙, 리뷰 2회차 [1]) — 그 밖의 " 는 글자 그대로 탭·줄바꿈으로 나눈다
	for _, c := range []struct {
		in   string
		rows [][]string
	}{
		{"\"hi\" there\tb\nc\td\n", [][]string{{`"hi" there`, "b"}, {"c", "d"}}}, // 닫는 따옴표 뒤에 글
		{"\"abc\"\tb\nc\t\"d\"\n", [][]string{{`"abc"`, "b"}, {"c", `"d"`}}},     // 칸 전체가 따옴표째 값
		{"a\t\"x\nc\td\n", [][]string{{"a", `"x`}, {"c", "d"}}},
		{"a\t\"x\ny\tb\nc\td", [][]string{{"a", `"x`}, {"y", "b"}, {"c", "d"}}},                    // 짝 없는 " 는 뒤 줄을 삼키지 않는다
		{"a\t\"x\" 뒤\nc\td\n", [][]string{{"a", `"x" 뒤`}, {"c", "d"}}},                             // 가운데 줄바꿈 없는 짝, 뒤에 글
		{"a\t\"x\ty\" 뒤\nc\td\te\n", [][]string{{"a", `"x`, `y" 뒤`}, {"c", "d", "e"}}},             // 특수 글자가 있어도 닫는 따옴표 뒤에 글이면 글자 그대로
		{"a\t\"한 줄\n둘째 줄\"\tb\nc\td\te\n", [][]string{{"a", "한 줄<br>둘째 줄", "b"}, {"c", "d", "e"}}}, // 여러 줄 칸
		// 짝 없이 " 로 시작한 칸이 뒤 행의 " 로 끝나는 칸(12")과 짝이 되면 그 사이가 한 칸이다 — 엑셀 형식의 모호함, 현재 동작 고정(리뷰 3회차 [1])
		{"\"a\tb\n12\"\tc\nd\te", [][]string{{"a\tb<br>12", "c"}, {"d", "e"}}},
	} {
		tb, err := TabTable(c.in)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if got := append([][]string{tb.Header}, tb.Rows...); !reflect.DeepEqual(got, c.rows) {
			t.Errorf("%q: %#v", c.in, got)
		}
	}
	for _, in := range []string{
		"a\t\"x\ny\"\nc\td\te", // 따옴표를 풀면 칸 수 불일치
	} {
		if tb, err := TabTable(in); err != ErrNotTabTable {
			t.Errorf("%q: 표가 아니어야 함 — %v %#v", in, err, tb)
		}
	}
}

// 칸 수 상한(리뷰 1회차 [8]) — 판정은 끝까지 하고(표가 아니면 ErrNotTabTable), 표면 ErrTableTooBig
func TestTabTableTooBig(t *testing.T) {
	old := tableMaxCells
	tableMaxCells = 6
	defer func() { tableMaxCells = old }()
	if _, err := TabTable(strings.Repeat("a\tb\n", 3)); err != nil {
		t.Fatalf("6칸: %v", err)
	}
	if _, err := TabTable(strings.Repeat("a\tb\n", 4)); err != ErrTableTooBig {
		t.Fatalf("8칸: %v", err)
	}
	if _, err := TabTable(strings.Repeat("a\tb\n", 4) + "c\n"); err != ErrNotTabTable {
		t.Fatalf("넘었지만 표 아님: %v", err)
	}
}

func TestTabPasteSpot(t *testing.T) {
	type c struct {
		src, text   string
		first, last int
	}
	yes := []c{
		{"앞\n\na\tb\nc\td\n", "a\tb\nc\td\n", 3, 4},
		{"a\tb\nc\td", "a\tb\nc\td", 1, 2},
		{"앞 글a\tb\nc\td뒤 글\n", "a\tb\nc\td", 1, 2},                // 줄 가운데 붙여넣기(앞뒤 글과 한 문단)
		{"앞\n\n\tQ1\tQ2\nA\t1\t2\n", "\tQ1\tQ2\nA\t1\t2\n", 3, 4}, // 빈 첫 칸 머리 행(그대로면 들여 쓴 코드)
		{":::chart\na\tb\nc\td\n:::\n", "a\tb\nc\td\n", 2, 3},
		// 붙여 넣은 글 자체의 마크다운에 휘둘리지 않는다(리뷰 1회차 [1])
		{"#\tName\tScore\n1\tA\t90\n", "#\tName\tScore\n1\tA\t90\n", 1, 2},
		{"x\ty\n-\tz\n", "x\ty\n-\tz\n", 1, 2},
		{"No\tv\n1.\tz\n", "No\tv\n1.\tz\n", 1, 2},
		{">\tq\na\tb\n", ">\tq\na\tb\n", 1, 2},
		{"a\tb\n=\t=\n", "a\tb\n=\t=\n", 1, 2},
		{"a\tb\n---\t-\n", "a\tb\n---\t-\n", 1, 2},
		{"a\tb\n```\tc\n", "a\tb\n```\tc\n", 1, 2},
		{"앞\n\na\tb\n\t\nc\td\n", "a\tb\n\t\nc\td\n", 3, 5},           // 모든 칸이 빈 행(리뷰 1회차 [3])
		{"앞\n\na\t\"x\n\ny\"\nc\td\n", "a\t\"x\n\ny\"\nc\td\n", 3, 6}, // 따옴표 칸 안 빈 줄
		{"    code\n\tQ\tR\nc\td\n", "\tQ\tR\nc\td\n", 2, 3},          // 들여 쓴 코드 뒤 빈 줄 자리(붙여 넣기 전 그 줄은 코드 밖)
	}
	for _, x := range yes {
		if !TabPasteSpot([]byte(x.src), x.text, x.first, x.last) {
			t.Errorf("%q %d-%d: 받아야 함", x.src, x.first, x.last)
		}
	}
	no := []c{
		{"```\na\tb\nc\td\n```\n", "a\tb\nc\td\n", 2, 3},             // 펜스 코드 안
		{"~~~go\n\ta\tb\n\tc\td\n~~~\n", "\ta\tb\n\tc\td\n", 2, 3},   // 펜스 안 탭 들여쓰기
		{"| x | y |\n|---|---|\na\tb\nc\td\n", "a\tb\nc\td\n", 3, 4}, // 기존 표에 붙음(행이 됨)
		{"# 제목a\tb\nc\td\n", "a\tb\nc\td\n", 1, 2},                   // 제목 줄
		{"- 항목\n  a\tb\n  c\td\n", "a\tb\n  c\td\n", 2, 3},           // 목록 안
		{"> a\tb\n> c\td\n", "a\tb\n> c\td\n", 1, 2},                 // 인용 안
		{"<div>\na\tb\nc\td\n</div>\n", "a\tb\nc\td\n", 2, 3},        // HTML 블록
		{"---\na\tb\nc\td\n---\n", "a\tb\nc\td\n", 2, 3},             // frontmatter
		// 기존 들여 쓴 코드 첫 줄 끝(리뷰 1회차 [2]) — 코드 블록이 쪼개진다
		{"para\n\n    x := 1a\tb\nc\td\n    y := 2\n", "a\tb\nc\td", 3, 4},
		// 기존 들여 쓴 코드 사이 빈 줄 — 붙여 넣기 전 그 줄은 코드 블록 안
		{"para\n\n    x := 1\n\tQ\tR\nc\td\n    y := 2\n", "\tQ\tR\nc\td", 4, 5},
		{"para\n\n    x := 1\na\tb\nc\td\n\n    y := 2\n", "a\tb\nc\td\n", 4, 5},
		// 자리가 글과 맞지 않음
		{"a\tb\nc\td\n", "a\tb\nc\td\n", 1, 1},
		{"a\tb\nc\td\n", "a\tb\nc\td\n", 0, 1},
		{"a\tb\nc\td\n", "a\tb\nc\td", 2, 3},
		{"a\tb\nX\td\n", "a\tb\nc\td", 1, 2},
		{"a\tb\nX\tY\nc\td\n", "a\tb\nc\tq\nc\td\n", 1, 3}, // 가운데 줄이 다름
		{"a\tb\n", "a\tb", 1, 1},                           // 한 줄
		// 인용·목록·표 바로 아래 줄(빈 줄 없이) — 자리표시가 게으른 이음줄·표 행이 된다(리뷰 2회차 [2])
		{"> q\na\tb\nc\td\n", "a\tb\nc\td\n", 2, 3},
		{"- item\na\tb\nc\td\n", "a\tb\nc\td\n", 2, 3},
		{"| h | i |\n|---|---|\na\tb\nc\td\n", "a\tb\nc\td\n", 3, 4},
		// 끝 줄 다음이 --- · === — 자리표시 문단이 setext 제목이 된다
		{"p\n\na\tb\nc\td\n---\n", "a\tb\nc\td\n", 3, 4},
		{"p\n\na\tb\nc\td\n===\n", "a\tb\nc\td\n", 3, 4},
	}
	for _, x := range no {
		if TabPasteSpot([]byte(x.src), x.text, x.first, x.last) {
			t.Errorf("%q %d-%d: 받지 않아야 함", x.src, x.first, x.last)
		}
	}
}

// 탭 표 → 쓴 표가 열 너비 예산(WriteTable)을 그대로 따른다 — 붙여넣기 경로가 부르는 것
func TestTabTableWrite(t *testing.T) {
	tb, _ := TabTable(strings.Repeat("a\tb\n", 3))
	out, err := WriteTable(tb)
	if err != nil || out.Markdown != "| a   | b   |\n| --- | --- |\n| a   | b   |\n| a   | b   |" {
		t.Fatalf("%v %q", err, out.Markdown)
	}
}
