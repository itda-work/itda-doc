package mdoc

import (
	"fmt"
	"strings"
	"testing"
)

const (
	rsComment = `<!-- itda-data {"v":1} -->`
	rsTable   = "| a | b |\n| - | -: |\n| x | 1 |\n\nTable: 판1 기준"
	rsNew     = rsComment + "\n| a | b |\n| - | -: |\n| x | 2 |\n| y | 3 |\n\nTable: 판2 기준"
	rsChart   = ":::chart{type=bar}\n\n| a | b |\n| - | -: |\n| x | 1 |\n\n: 판1 기준\n\n:::"
)

// 인수 조건: 최상위 자료 블록(기록 주석 + 표·차트)의 범위만 바꿀 수 있다 — 목록·인용·다단·각주 안, 범위가 블록 끝과 다름, 주석만 있고
// 블록 없음은 거부한다. 바꾼 뒤에도 앞뒤 글은 그대로다.
func TestReplaceFitsAt(t *testing.T) {
	block := rsComment + "\n" + rsTable
	cases := []struct {
		name     string
		doc      string
		from, to int
		repl     string
		want     bool
	}{
		{"최상위 표", "# 제목\n\n" + block + "\n\n뒤 문단\n", 3, 8, rsNew, true},
		{"최상위 표 — 같은 글", "# 제목\n\n" + block + "\n\n뒤 문단\n", 3, 8, block, true},
		{"문서 처음·끝 줄바꿈 없음", block, 1, 6, rsNew, true},
		{"frontmatter 뒤", "---\ntitle: 보고\n---\n\n" + block + "\n", 5, 10, rsNew, true},
		{"최상위 차트", "앞\n\n" + rsComment + "\n" + rsChart + "\n\n뒤\n", 3, 12, rsComment + "\n" + strings.Replace(rsChart, "| x | 1 |", "| x | 9 |", 1), true},
		{"범위가 블록 끝과 다름(캡션 빼고)", block + "\n", 1, 4, rsNew, false},
		{"범위가 블록보다 김", block + "\n\n뒤\n", 1, 8, rsNew, false},
		{"주석만 있고 블록 없음", rsComment + "\n\n문단\n", 1, 3, rsNew, false},
		{"주석이 아닌 줄에서 시작", "문단\n" + rsTable + "\n", 1, 6, rsNew, false},
		{"목록 안", "- " + strings.ReplaceAll(block, "\n", "\n  ") + "\n", 1, 6, rsNew, false},
		{"인용 안", "> " + strings.ReplaceAll(block, "\n", "\n> ") + "\n", 1, 6, rsNew, false},
		{"다단 안", ":::columns\n" + block + "\n:::\n", 2, 7, rsNew, false},
		{"각주 정의 안", "본문[^a]\n\n[^a]: 각주\n\n    " + strings.ReplaceAll(block, "\n", "\n    ") + "\n", 5, 10, rsNew, false},
		{"바꿀 글이 블록이 아님", block + "\n", 1, 6, rsComment + "\n\n문단", false},
		{"바꿀 글이 닫히지 않은 코드로 끝남", block + "\n\n뒤\n", 1, 6, rsNew + "\n\n```", false},
		{"바꿀 글에 CR", block + "\n", 1, 6, strings.ReplaceAll(rsNew, "\n", "\r\n"), false},
		{"줄 범위 밖", block + "\n", 1, 99, rsNew, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ReplaceFitsAt([]byte(c.doc), c.from, c.to, c.repl); got != c.want {
				t.Fatalf("ReplaceFitsAt(%d~%d) = %v, 바람 %v\n%s", c.from, c.to, got, c.want, c.doc)
			}
		})
	}
}

// FuzzReplaceFitsAt 은 임의 문서에서 판정이 멈추지 않고, 받으면 바꾼 글의 그 자리가 최상위 HTML 한 줄 + 표·차트이며, 같은 글로 바꾸기는
// 범위가 자료 블록 꼴이면 늘 받는지 본다.
func FuzzReplaceFitsAt(f *testing.F) {
	block := rsComment + "\n" + rsTable
	for _, s := range []string{
		block, "# a\n\n" + block + "\n\n뒤\n", "- " + strings.ReplaceAll(block, "\n", "\n  "), ":::columns\n" + block + "\n:::\n",
		"> " + block, rsComment + "\n" + rsChart, "[^a]: x\n" + block, "본문[^a]\n\n[^a]:\n    " + strings.ReplaceAll(block, "\n", "\n    "),
		"---\nt: 1\n---\n" + block, block + "\n```\n", "<div>\n" + block + "\n</div>\n",
	} {
		f.Add(s, 1, 6)
	}
	f.Fuzz(func(t *testing.T, doc string, from, to int) {
		if len(doc) > 4096 || from < 1 || from > 64 || to < from || to > 80 {
			return
		}
		if ReplaceFitsAt([]byte(doc), from, to, rsNew) {
			s := string(normalize([]byte(doc)))
			out := s[:lineStart(s, from)] + rsNew + s[min(len(s), max(lineStart(s, from), lineStart(s, to+1)-1)):]
			if !recordRange([]byte(out), from, from+strings.Count(rsNew, "\n")) {
				t.Fatalf("받았는데 바꾼 글의 자리가 자료 블록이 아님\n%q", out)
			}
		}
		if recordRange(normalize([]byte(doc)), from, to) {
			s := string(normalize([]byte(doc)))
			end := lineStart(s, to+1)
			if end > 0 && end <= len(s) && s[end-1] == '\n' {
				end--
			}
			same := s[lineStart(s, from):end]
			p, ok := parseSpots([]byte(doc))
			overlap := false
			for _, sp := range p.spans {
				overlap = overlap || (sp.start <= to && from <= sp.end)
			}
			if ok && !overlap && !p.inFrontmatter(from) && !ReplaceFitsAt([]byte(doc), from, to, same) {
				t.Fatalf("같은 글로 바꾸기를 거부 %d~%d\n%q", from, to, doc)
			}
		}
	})
}

// ChartValues 는 렌더와 같은 판정으로 읽은 계열 값이다 — 축마다 단위(unit 왼쪽·unit2 오른쪽)를 뗀다. 표로 남는 블록은 ok 가 거짓.
func TestChartValues(t *testing.T) {
	const tbl = "\n\n| m | a | b |\n| - | -: | -: |\n| x | 10 | 20 |\n| y |  | 3 |\n\n:::"
	for _, c := range []struct {
		open string
		want string
		ok   bool
	}{
		{":::chart{type=bar}", "[{1 [10 0] [true false]} {2 [20 3] [true true]}]", true},
		{`:::chart{type=bar axis2="b" unit2="0"}`, "[{1 [10 0] [true false]} {2 [2 3] [true true]}]", true},
		{`:::chart{type=bar unit="0" axis2="b"}`, "[{1 [1 0] [true false]} {2 [20 3] [true true]}]", true},
		{":::chart{type=pie}", "[{1 [10 0] [true false]}]", true}, // 원은 첫 계열만 — 렌더러가 정한 대응
		{":::chart{type=nope}", "", false},
	} {
		s, ok := ChartValues([]byte(c.open + tbl))
		if got := fmt.Sprint(s); ok != c.ok || (ok && got != c.want) {
			t.Errorf("%s → %s %v, 바람 %s %v", c.open, got, ok, c.want, c.ok)
		}
	}
}
