package mdoc

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 표 편집 그리드(itda-hyve#355)의 읽기(TableAt)·쓰기(FormatTable).

func mustTable(t *testing.T, src string, line int) Table {
	t.Helper()
	tb, ok := TableAt([]byte(src), line)
	if !ok {
		t.Fatalf("%d 줄에서 표를 찾지 못함:\n%s", line, src)
	}
	return tb
}

func TestTableAtRead(t *testing.T) {
	src := "앞 문단\n\n| 이름 | 값 | 비고 | 끝 |\n|:--|--:|:-:|---|\n| a \\| b | `x | y` | **굵게** [링크](https://e.com) | 1<br>2 |\n| 짧은 |\n\nTable: 표 제목\n\n뒤 문단\n"
	tb := mustTable(t, src, 5)
	// 첫 본문 행은 칸이 다섯이다 — 코드 스팬 안의 | 도 칸을 나눈다(GFM). 머리 행보다 길어 열이 다섯이 되고(빈 머리 칸·정렬 없음),
	// 짧은 행은 빈 칸으로 채운다. 칸 글은 원문 그대로(\| ·**·<br> 그대로)
	want := Table{
		Start: 3, End: 6,
		Header:  []string{"이름", "값", "비고", "끝", ""},
		Align:   []string{"left", "right", "center", "", ""},
		Rows:    [][]string{{`a \| b`, "`x", "y`", "**굵게** [링크](https://e.com)", "1<br>2"}, {"짧은", "", "", "", ""}},
		Caption: "표 제목", CaptionPrefix: "Table:", CaptionStart: 8, CaptionEnd: 8,
		Notes: []string{"머리 행보다 긴 행 1개 — 열을 늘려 보입니다"},
	}
	if !reflect.DeepEqual(tb, want) {
		t.Fatalf("읽기\n 얻음 %#v\n 기대 %#v", tb, want)
	}
	// 커서가 구분 행·캡션 줄·사이 빈 줄에 있어도 같은 표, 표 밖이면 없음
	for _, ln := range []int{3, 4, 6, 7, 8} {
		if got := mustTable(t, src, ln); got.Start != 3 {
			t.Errorf("%d 줄: 시작 %d", ln, got.Start)
		}
	}
	for _, ln := range []int{0, 1, 2, 9, 10, 99} {
		if _, ok := TableAt([]byte(src), ln); ok {
			t.Errorf("%d 줄: 표 밖인데 찾음", ln)
		}
	}
}

func TestTableAtVariants(t *testing.T) {
	cases := []struct {
		name, src                string
		line                     int
		header                   []string
		align                    []string
		rows                     [][]string
		caption, prefix, head, p string
		chart                    bool
	}{
		{name: "앞뒤 | 없음", src: "a | b\n--- | :---\n1 | 2\n", line: 1,
			header: []string{"a", "b"}, align: []string{"", "left"}, rows: [][]string{{"1", "2"}}},
		{name: "본문 행 없음", src: "| a |\n|---|\n", line: 2, header: []string{"a"}, align: []string{""}, rows: [][]string{}},
		{name: ": 캡션", src: "| a |\n|--:|\n| 1 |\n\n: 짧은 캡션\n", line: 5,
			header: []string{"a"}, align: []string{"right"}, rows: [][]string{{"1"}}, caption: "짧은 캡션", prefix: ":"},
		{name: "chart 안", src: ":::chart{type=bar}\n| 분기 | 매출 |\n|:--|--:|\n| 1분기 | 3 |\n\n: 분기 매출\n:::\n", line: 3,
			header: []string{"분기", "매출"}, align: []string{"left", "right"}, rows: [][]string{{"1분기", "3"}}, caption: "분기 매출", prefix: ":", chart: true},
		{name: "approval 안(여는 줄 바로 아래)", src: ":::approval\n| 담당 | 팀장 |\n|:--:|:--:|\n| 홍 |  |\n:::\n", line: 2,
			header: []string{"담당", "팀장"}, align: []string{"center", "center"}, rows: [][]string{{"홍", ""}}},
		{name: "actions 안", src: ":::actions\n| 할 일 | 담당 |\n|---|---|\n| 보고 | 김 |\n:::\n", line: 4,
			header: []string{"할 일", "담당"}, align: []string{"", ""}, rows: [][]string{{"보고", "김"}}},
		{name: "인용 안", src: "> 인용\n>\n> | a | b |\n> |---|---|\n> | 1 | 2 |\n>\n> Table: 인용 표\n", line: 4,
			header: []string{"a", "b"}, align: []string{"", ""}, rows: [][]string{{"1", "2"}}, caption: "인용 표", prefix: "Table:", head: "> ", p: "> "},
		{name: "목록 안", src: "- 항목\n\n  | a | b |\n  |---|---|\n  | 1 | 2 |\n", line: 5,
			header: []string{"a", "b"}, align: []string{"", ""}, rows: [][]string{{"1", "2"}}, head: "  ", p: "  "},
		// 머리 행에 | 가 없는 한 열 표 — 구분 행에 | 나 콜론이 있으면 goldmark 가 표로 읽는다(화면은 판정하지 않고 Go 에 맡긴다, itda-hyve#355 접근 방식 결정)
		{name: "머리 행 | 없음(한 열)", src: "a\n|---|\nx\n", line: 1, header: []string{"a"}, align: []string{""}, rows: [][]string{{"x"}}},
		{name: "| 없는 구분 행(:---)", src: "a\n:---\nx\n", line: 3, header: []string{"a"}, align: []string{"left"}, rows: [][]string{{"x"}}},
		{name: "frontmatter 뒤", src: "---\ntitle: x\n---\n\n| a |\n|---|\n| 1 |\n", line: 6,
			header: []string{"a"}, align: []string{""}, rows: [][]string{{"1"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tb := mustTable(t, c.src, c.line)
			if !reflect.DeepEqual(tb.Header, c.header) || !reflect.DeepEqual(tb.Align, c.align) || !reflect.DeepEqual(tb.Rows, c.rows) ||
				tb.Caption != c.caption || tb.CaptionPrefix != c.prefix || tb.InChart != c.chart || tb.HeadPrefix != c.head || tb.Prefix != c.p {
				t.Fatalf("얻음 %#v", tb)
			}
		})
	}
	// 머리 행 줄 번호는 frontmatter 를 센 원문 줄이다
	if tb := mustTable(t, cases[len(cases)-1].src, 5); tb.Start != 5 || tb.End != 7 {
		t.Errorf("frontmatter 뒤 줄 범위 %d~%d", tb.Start, tb.End)
	}
	for _, src := range []string{"a\n|---|\nx\n", "a\n:---\nx\n"} {
		if tb := mustTable(t, src, 2); tb.Start != 1 || tb.End != 3 {
			t.Errorf("%q 줄 범위 %d~%d", src, tb.Start, tb.End)
		}
	}
	// 맨 --- 는 표가 아니다(setext 제목)
	if _, ok := TableAt([]byte("a\n---\nx\n"), 1); ok {
		t.Error("setext 제목을 표로 읽음")
	}
	if _, ok := TableAt([]byte("# 제목\n\n문단 | 파이프\n"), 3); ok {
		t.Error("표가 아닌 | 줄에서 찾음")
	}
	// | 가 없는 줄도 표에 붙어 있으면 본문 행이다(GFM — 첫 칸 x, 둘째 칸 빈 칸). 화면은 판정하지 않고 Go 에 맡긴다
	if tb := mustTable(t, "| a | b |\n|---|---|\nx\n", 3); !reflect.DeepEqual(tb.Rows, [][]string{{"x", ""}}) || tb.End != 3 {
		t.Errorf("| 없는 본문 행: %#v", tb)
	}
}

// 여러 줄 캡션의 강제 줄바꿈(줄 끝 공백 둘)은 읽고 쓸 때 지우지 않는다(itda-hyve#355 리뷰 [2]). 첫 줄·뒤 줄, 인용 안.
func TestTableCaptionHardBreak(t *testing.T) {
	for _, src := range []string{
		"| a |\n|---|\n| x |\n\nTable: first  \nsecond\n",
		"| a |\n|---|\n| x |\n\nTable: first\nsecond  \nthird\n",
		"> | a |\n> |---|\n> | x |\n>\n> Table: first  \n> second\n",
	} {
		tb := mustTable(t, src, 1)
		if !strings.Contains(tb.Caption, "  \n") {
			t.Errorf("캡션의 줄 끝 공백을 뗌: %q", tb.Caption)
		}
		out := replaceLines(src, tb.Start, max(tb.End, tb.CaptionEnd), FormatTable(tb))
		a, _ := Render([]byte(src), Options{})
		b, _ := Render([]byte(out), Options{})
		if !strings.Contains(a.HTML, "<br>") || a.HTML != b.HTML {
			t.Errorf("강제 줄바꿈 캡션 왕복\n원문 %q\n쓴 글 %q\n%s", src, out, firstDiff(a.HTML, b.HTML))
		}
	}
}

// 쓰기 예산(itda-hyve#355 리뷰 [3]): 맞춘 표가 상한을 넘을 것이면 맞춤 없이 쓰고 알린다, 그것도 넘거나 칸·입력이 많으면 거부한다.
// 상한은 패키지 변수를 작게 바꿔 시험한다(거대 글을 만들지 않는다).
func TestWriteTableBudget(t *testing.T) {
	defer func(c, i, o int) { tableMaxCells, tableMaxIn, tableMaxOut = c, i, o }(tableMaxCells, tableMaxIn, tableMaxOut)
	tb := Table{Header: []string{"긴 칸", "b"}, Align: []string{"right", "center"}, Caption: "c  \nd", Prefix: "> ", HeadPrefix: "> "}
	tb.Rows = append(tb.Rows, []string{strings.Repeat("가", 50), "y"})
	for range 40 {
		tb.Rows = append(tb.Rows, []string{"x", "y"})
	}
	aligned := FormatTable(tb)
	// size 는 쓰지 않고 센 바이트 수 — 실제 글과 같다(맞춤·맞춤 없음 둘 다)
	g := newTableGrid(tb, tableCols(tb))
	if g.size(true) != len(aligned) || g.size(false) != len(g.format(false)) {
		t.Fatalf("크기 셈이 틀림: %d/%d, %d/%d", g.size(true), len(aligned), g.size(false), len(g.format(false)))
	}
	if r, err := WriteTable(tb); err != nil || r.Markdown != aligned || r.Note != "" {
		t.Fatalf("예산 안인데 맞추지 않음: %v %q", err, r.Note)
	}
	tableMaxOut = len(aligned) - 1
	r, err := WriteTable(tb)
	if err != nil || r.Note != "표가 커서 열 너비를 맞추지 않음" || len(r.Markdown) > tableMaxOut {
		t.Fatalf("맞춤 없이 쓰지 않음: %v %q %d", err, r.Note, len(r.Markdown))
	}
	if !strings.HasPrefix(r.Markdown, "> | 긴 칸 | b |\n> | --: | :-: |\n> | "+strings.Repeat("가", 50)+" | y |\n> | x | y |\n") {
		t.Errorf("맞춤 없는 꼴:\n%s", r.Markdown)
	}
	// 맞춤 없는 표도 렌더는 같다
	a, _ := Render([]byte(aligned), Options{})
	b, _ := Render([]byte(r.Markdown), Options{})
	if a.HTML != b.HTML {
		t.Errorf("맞춤 없는 표의 렌더가 다름\n%s", firstDiff(a.HTML, b.HTML))
	}
	tableMaxOut = len(r.Markdown) - 1
	if _, err := WriteTable(tb); err != ErrTableTooBig {
		t.Errorf("맞춤 없이도 넘는데 오류가 아님: %v", err)
	}
	tableMaxOut, tableMaxCells = 1<<20, 2*len(tb.Rows)+1 // 2열 × (행+1) = 상한 + 1
	if _, err := WriteTable(tb); err != ErrTableTooBig {
		t.Errorf("칸 수 상한: %v", err)
	}
	tableMaxCells++
	if _, err := WriteTable(tb); err != nil {
		t.Errorf("칸 수 상한 딱 맞음: %v", err)
	}
	tableMaxIn = 100
	if _, err := WriteTable(tb); err != ErrTableTooBig {
		t.Errorf("입력 바이트 상한: %v", err)
	}
}

func TestFormatTable(t *testing.T) {
	tb := Table{
		Header: []string{"구분", "Name", "가운데", "없음"},
		Align:  []string{"left", "right", "center", ""},
		Rows:   [][]string{{"한글칸", "x", "a|b", " 앞뒤 공백 "}, {"줄\n바꿈", `이미 \| 막음`, "", "①"}},
	}
	got := FormatTable(tb)
	want := strings.Join([]string{
		// 한글은 두 칸, ① 같은 East Asian ambiguous 는 한 칸. 오른쪽 정렬은 왼쪽을, 가운데 정렬은 양쪽을 채운다
		"| 구분       |         Name | 가운데 | 없음      |",
		"| :--------- | -----------: | :----: | --------- |",
		"| 한글칸     |            x |  a\\|b  | 앞뒤 공백 |",
		"| 줄<br>바꿈 | 이미 \\| 막음 |        | ①         |",
		"",
		"Table: 새 캡션",
	}, "\n")
	tb.Caption = "새 캡션"
	got = FormatTable(tb)
	if got != want {
		t.Fatalf("쓰기\n얻음:\n%s\n기대:\n%s", got, want)
	}
	// 캡션 접두: 원래 것 유지, 차트 안 새 캡션은 ":", 비우면 캡션 문단 없음
	tb.CaptionPrefix = "table:"
	if s := FormatTable(tb); !strings.HasSuffix(s, "\n\ntable: 새 캡션") {
		t.Errorf("원래 접두를 지키지 않음:\n%s", s)
	}
	tb.CaptionPrefix, tb.InChart = "", true
	if s := FormatTable(tb); !strings.HasSuffix(s, "\n\n: 새 캡션") {
		t.Errorf("차트 안 새 캡션 접두가 : 가 아님:\n%s", s)
	}
	tb.Caption = "  "
	if s := FormatTable(tb); strings.Contains(s, "새 캡션") || strings.HasSuffix(s, "\n") {
		t.Errorf("빈 캡션인데 캡션 문단을 씀:\n%s", s)
	}
	// 인용 안 표: 줄마다 > 를 잇고 캡션 앞 빈 줄도 인용 안(>)
	q := Table{Header: []string{"a"}, Rows: [][]string{{"1"}}, Caption: "c", HeadPrefix: "> ", Prefix: "> "}
	if s := FormatTable(q); s != "> | a   |\n> | --- |\n> | 1   |\n>\n> Table: c" {
		t.Errorf("인용 안 표:\n%s", s)
	}
	// 열이 하나도 없으면 한 열
	if s := FormatTable(Table{}); s != "|     |\n| --- |" {
		t.Errorf("빈 표: %q", s)
	}
	if c := FormatCell("a\r\nb | c \\| d"); c != `a<br>b \| c \| d` {
		t.Errorf("칸 변환 %q", c)
	}
}

// 읽은 표를 그대로 쓰면 렌더 결과(HTML)가 같다 — 열 맞춤만 바뀐다(itda-hyve#355 시험 「왕복」). 견본·golden 의 모든 표.
func TestTableRoundTrip(t *testing.T) {
	var files []string
	for _, g := range []string{"testdata/*.md", "../spec/samples/*.md", "testdata/tables/*.md"} {
		m, _ := filepath.Glob(g)
		files = append(files, m...)
	}
	total := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(normalize(b))
		n := strings.Count(src, "\n") + 1
		for ln := 1; ln <= n; ln++ {
			tb, ok := TableAt([]byte(src), ln)
			if !ok || tb.Start != ln {
				continue
			}
			total++
			if len(tb.Notes) > 0 {
				t.Logf("%s:%d: %v — 긴 행은 넣으면 열이 생기므로 왕복에서 뺀다", f, ln, tb.Notes)
				continue
			}
			out := replaceLines(src, tb.Start, max(tb.End, tb.CaptionEnd), FormatTable(tb))
			a, err1 := Render([]byte(src), Options{})
			b, err2 := Render([]byte(out), Options{})
			if err1 != nil || err2 != nil {
				t.Fatal(err1, err2)
			}
			if a.HTML != b.HTML {
				t.Errorf("%s:%d 표: 읽고 바로 쓰면 렌더가 달라짐\n%s", f, ln, firstDiff(a.HTML, b.HTML))
			}
			// 한 번 더 읽어 쓰면 글이 그대로다(열 맞춤이 굳는다)
			again, _ := TableAt([]byte(out), ln)
			if out2 := replaceLines(out, again.Start, max(again.End, again.CaptionEnd), FormatTable(again)); out2 != out {
				t.Errorf("%s:%d 표: 두 번째 쓰기가 첫 번째와 다름", f, ln)
			}
		}
	}
	if total < 20 {
		t.Fatalf("왕복한 표가 %d 개뿐 — 견본을 찾지 못했나", total)
	}
	t.Logf("표 %d 개 왕복", total)
}

func replaceLines(src string, from, to int, text string) string {
	lines := strings.Split(src, "\n")
	out := append([]string{}, lines[:from-1]...)
	out = append(out, strings.Split(text, "\n")...)
	return strings.Join(append(out, lines[to:]...), "\n")
}

func firstDiff(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	lo := max(0, i-80)
	return "기존: " + a[lo:min(len(a), i+120)] + "\n새것: " + b[lo:min(len(b), i+120)]
}

// 렌더는 표마다(최상위·컨테이너·목록·인용 안) data-table-line 에 머리 행 원문 줄을 싣는다 — TableAt 의 Start 와 같다(itda-hyve#355, 미리보기에서
// 표 편집으로 가는 길). 제목 다음의 구분 행 꼴 문단(:---)에는 붙지 않는다(리뷰 3회차 [6]).
func TestTableLineAttr(t *testing.T) {
	re := regexp.MustCompile(`<table[^>]*data-table-line="(\d+)"`)
	for _, c := range []struct {
		name, src string
		want      []int
	}{
		{"최상위·캡션", "# 제목\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\nTable: 캡션\n\n| c |\n|---|\n", []int{3, 9}},
		{"approval", ":::approval\n| 담당 | 팀장 |\n|:--:|:--:|\n| 홍 |  |\n:::\n", []int{2}},
		{"actions 둘째 표", ":::actions\n| 할 일 | 담당 |\n|---|---|\n| 보고 | 김 |\n\n| x |\n|---|\n| y |\n:::\n", []int{2, 6}},
		{"box 안 제목 뒤 :---", ":::box\n# heading\n:---\n\na\n:---\nx\n:::\n", []int{5}},
		{"목록", "- 항목\n\n  | a | b |\n  |---|---|\n  | 1 | 2 |\n", []int{3}},
		{"인용", "> 인용\n>\n> | a | b |\n> |---|---|\n> | 1 | 2 |\n", []int{3}},
		{"frontmatter", "---\ntitle: x\n---\n\n| a |\n|---|\n| 1 |\n\n:::box\n| b |\n|---|\n:::\n", []int{5, 10}},
	} {
		t.Run(c.name, func(t *testing.T) {
			r, err := Render([]byte(c.src), Options{})
			if err != nil {
				t.Fatal(err)
			}
			var got []int
			for _, m := range re.FindAllStringSubmatch(r.HTML, -1) {
				n, _ := strconv.Atoi(m[1])
				got = append(got, n)
				if tb, ok := TableAt([]byte(c.src), n); !ok || tb.Start != n {
					t.Errorf("data-table-line %d 이 TableAt Start 와 다름: %v %d", n, ok, tb.Start)
				}
			}
			if !reflect.DeepEqual(got, c.want) || strings.Count(r.HTML, "<table") != len(c.want) {
				t.Errorf("data-table-line %v, 기대 %v\n%s", got, c.want, r.HTML)
			}
		})
	}
}
