package mdoc

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yuin/goldmark/ast"
)

// 각주 정의의 원문 구간(footnote_spans.go)과 그것으로 정하는 넣을 자리(InsertSpotAt) — itda-hyve#365 ③ 조각 C 접근 리뷰 ⓐ.

// 구간은 파서가 받아들인 정의마다 하나다 — 참조되지 않아 최종 AST 에서 지워진 정의, 같은 이름 둘째 정의도 따로 남는다. 끝은 정의 뒤
// 빈 줄을 뺀 줄이고 문서 끝이면 마지막 줄이다(끝 줄바꿈이 없어도). topLevel 은 Open 때의 부모가 문서인지다.
func TestFootnoteSpans(t *testing.T) {
	type span struct {
		start, end int
		top        bool
	}
	for _, c := range []struct {
		name, src string
		want      []span
	}{
		{"참조 없음", "[^a]:\n    - 하나\n    - 둘\n\n본문\n", []span{{1, 3, true}}},
		{"같은 이름 둘", "[^a]: 앞\n\n문단\n\n[^a]:\n    - 하나\n    - 둘\n\n본문[^a]\n", []span{{1, 1, true}, {5, 7, true}}},
		{"참조됨", "[^a]:\n    - 하나\n    - 둘\n\n본문[^a]\n", []span{{1, 3, true}}},
		{"인용 안", "> [^a]: 글\n>     더\n\n본문[^a]\n", []span{{1, 2, false}}},
		{"목록 안", "- [^a]: 글\n      더\n\n본문[^a]\n", []span{{1, 2, false}}},
		{"끝 줄바꿈 없음", "[^a]:\n    글", []span{{1, 2, true}}},
		{"안의 빈 줄 여럿", "[^a]: 하나\n\n\n\n    둘\n\n본문\n", []span{{1, 5, true}}},
		{"코드 안 가짜", "```\n[^a]: 글\n```\n", nil},
		{"CRLF", "[^a]: 하나\r\n    둘\r\n\r\n본문\r\n", []span{{1, 2, true}}},
		{"CR", "[^a]: 하나\r    둘\r\r본문\r", []span{{1, 2, true}}},
		{"탭 이어짐", "[^a]: 하나\n\t탭\n\n본문\n", []span{{1, 2, true}}},
		{"세 칸은 밖", "[^a]: 하나\n\n   세 칸\n\n본문\n", []span{{1, 1, true}}},
		{"네 칸은 안", "[^a]: 하나\n\n    네 칸\n\n본문\n", []span{{1, 3, true}}},
		{"빈 정의", "[^a]:\n\n본문\n", []span{{1, 1, true}}},
		{"게으른 이어짐", "[^a]: 하나\n게으른 줄\n\n본문\n", []span{{1, 2, true}}},
		{"안의 코드", "[^a]: 하나\n\n    ```\n    code\n    ```\n\n본문\n", []span{{1, 5, true}}},
		{"안의 HTML", "[^a]: 하나\n\n    <div>\n    x\n    </div>\n\n본문\n", []span{{1, 5, true}}},
		{"안의 다단", "[^a]: 하나\n\n    :::columns\n    왼\n    :::\n\n본문\n", []span{{1, 5, true}}},
		{"frontmatter 뒤", "---\ntitle: t\n---\n[^a]: 하나\n    둘\n\n본문\n", []span{{4, 5, true}}},
		{"문단을 끊음", "본문\n[^a]: 끊음\n뒤\n", []span{{2, 3, true}}},
		{"다단 안", "::: columns\n[^a]: 안\n:::\n\n끝\n", []span{{2, 2, false}}},
	} {
		p, ok := parseSpots([]byte(c.src))
		var got []span
		for _, sp := range p.spans {
			got = append(got, span{sp.start, sp.end, sp.topLevel})
		}
		if !ok || fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s: %v %v, 기대 %v", c.name, got, ok, c.want)
		}
	}
	// 원래 부모는 Close 가 정의를 옮기기 전의 것이다(인용·목록 항목·컨테이너)
	for src, want := range map[string]ast.NodeKind{"> [^a]: 글\n": ast.KindBlockquote, "- [^a]: 글\n": ast.KindListItem, ":::columns\n[^a]: 글\n:::\n": KindContainer} {
		if p, _ := parseSpots([]byte(src)); len(p.spans) != 1 || p.spans[0].parent != want {
			t.Errorf("%q 부모: %+v, 기대 %v", src, p.spans, want)
		}
	}
}

// 커서 줄마다의 자리: F 각주 정의 안·L 목록·Q 인용·C 컨테이너·M frontmatter 는 거부, 숫자는 넣을 줄이다. 정의 안·바로 앞·바로 뒤를 다 본다.
func TestInsertSpotFootnotes(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      string
	}{
		{"참조 없는 정의 안 목록(리뷰 2회차)", "[^a]:\n    - 하나\n    - 둘\n\n본문\n", "F F F 4 6 6"},
		{"같은 이름 둘째 정의(리뷰 2회차)", "[^a]: 앞\n\n문단\n\n[^a]:\n    - 하나\n    - 둘\n\n본문[^a]\n", "F 2 4 4 F F F 8 10 10"},
		{"참조된 정의 안 목록(리뷰 1회차)", "[^a]:\n    - 하나\n    - 둘\n\n본문[^a]\n", "F F F 4 6 6"},
		{"정의 앞 문단(리뷰 1회차 탐침)", "본문[^a]\n\n[^a]: 각주\n", "2 2 F 4"},
		{"인용 안", "> [^a]: 글\n>     더\n\n본문[^a]\n", "F F 3 5 5"},
		{"목록 안", "- [^a]: 글\n      더\n\n본문[^a]\n", "F F 3 5 5"},
		{"목록 안 둘째 항목 뒤 정의", "- x\n\n  [^c]: 목록 안\n\n본문[^c]\n", "L L F 4 6 6"},
		{"끝 줄바꿈 없음(깨지는 입력)", "[^a]:\n    글", "F F"},
		{"안의 빈 줄 여럿", "[^a]: 하나\n\n\n\n    둘\n\n본문\n", "F F F F F 6 8 8"},
		{"코드 안 가짜", "```\n[^a]: 글\n```\n", "4 4 4 4"},
		{"CRLF", "[^a]: 하나\r\n    둘\r\n\r\n본문\r\n", "F F 3 5 5"},
		{"CR", "[^a]: 하나\r    둘\r\r본문\r", "F F 3 5 5"},
		{"탭 이어짐", "[^a]: 하나\n\t탭\n\n본문\n", "F F 3 5 5"},
		{"세 칸은 밖", "[^a]: 하나\n\n   세 칸\n\n본문\n", "F 2 4 4 6 6"},
		{"네 칸은 안", "[^a]: 하나\n\n    네 칸\n\n본문\n", "F F F 4 6 6"},
		{"빈 정의", "[^a]:\n\n본문\n", "F 2 4 4"},
		{"게으른 이어짐", "[^a]: 하나\n게으른 줄\n\n본문\n", "F F 3 5 5"},
		{"안의 코드", "[^a]: 하나\n\n    ```\n    code\n    ```\n\n본문\n", "F F F F F 6 8 8"},
		{"안의 HTML", "[^a]: 하나\n\n    <div>\n    x\n    </div>\n\n본문\n", "F F F F F 6 8 8"},
		{"안의 다단", "[^a]: 하나\n\n    :::columns\n    왼\n    :::\n\n본문\n", "F F F F F 6 8 8"},
		{"frontmatter 뒤", "---\ntitle: t\n---\n[^a]: 하나\n    둘\n\n본문\n", "M M M F F 6 8 8"},
		{"문단을 끊는 정의", "본문\n[^a]: 끊음\n뒤\n", "2 F F 4"},
		{"다단 안 정의", "::: columns\n[^a]: 안\n:::\n\n끝\n", "C F C 4 6 6"},
		{"정의 둘 붙음", "[^a]: 하나\n[^b]: 둘\n\n본문[^a][^b]\n", "F F 3 5 5"},
		{"뒤가 정의 표지(퍼징 발견)", "# 제목\n\n\n: 정의 아님\n", "L L L 5 5"},
		{"링크 참조 정의 뒤 들여 쓴 이어짐(퍼징 발견)", "[a]: /u\n    이어짐\n\n본문\n", "3 3 3 5 5"},
	} {
		if g := spotString(c.src, strings.Count(normalizeString(c.src), "\n")+1); g != c.want {
			t.Errorf("%s: %s, 기대 %s", c.name, g, c.want)
		}
	}
}

// spotString 은 커서 1~n 줄의 자리를 TestInsertSpotFootnotes 의 표기(F L Q C M X B ?·숫자)로 적는다.
func spotString(src string, n int) string {
	var got []string
	for line := 1; line <= n; line++ {
		spot, ok := InsertSpotAt([]byte(src), line)
		switch {
		case ok:
			got = append(got, strconv.Itoa(spot.Line))
		case spot.Kind == "":
			got = append(got, "?")
		default:
			got = append(got, map[string]string{"footnote": "F", "list": "L", "quote": "Q", "container": "C", "frontmatter": "M", "complex": "X", "boundary": "B"}[spot.Kind])
		}
	}
	return strings.Join(got, " ")
}

// 리뷰 3회차 [2]: 끝 줄바꿈 없는 마지막 줄을 새 블록이 먹으면 Close 때 PeekLine 이 nil 이다 — 그 줄을 정의 안으로 읽지 않는다.
// 줄바꿈 유무로 판정이 달라지지 않는다(같은 줄 번호).
func TestInsertSpotEOFPairs(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      string
	}{
		{"들여 쓴 이어짐 — 안", "[^a]:\n    text", "F F"},
		{"빈 줄 뒤 문단 — 밖", "[^a]: note\n\nbody", "F 2 4"},
		{"빈 줄 뒤 제목 — 밖", "[^a]: note\n\n# heading", "F 2 4"},
		{"안의 정의 뒤 문단 — 밖", "[^a]:\n    [^b]: inner\n\nbody", "F F 3 5"},
		{"게으른 이어짐 — 안", "[^a]: note\nlazy", "F F"},
	} {
		n := strings.Count(c.src, "\n") + 1
		for _, eol := range []string{"", "\n"} {
			if g := spotString(c.src+eol, n); g != c.want {
				t.Errorf("%s(끝 %q): %s, 기대 %s", c.name, eol, g, c.want)
			}
		}
	}
	// 게으른 이어짐 줄은 렌더에서도 각주 글이다(본문 문단이 아니다)
	r, _ := Render([]byte("[^a]: note\nlazy\n\n본문[^a]"), Options{})
	html := r.HTML
	if strings.Count(html, "lazy") != 1 || !strings.Contains(html, "note\nlazy") || strings.Index(html, "lazy") < strings.Index(html, "footnotes") {
		t.Errorf("게으른 이어짐 렌더: %s", html)
	}
}

func normalizeString(s string) string { return string(normalize([]byte(s))) }

// checkInsert 는 InsertSpotAt 의 세 약속을 본다: 각주 정의 안이면 거부, 받으면 넣은 기록 주석 바로 다음이 최상위 표, 기존 글의
// 구조(블록 꼴·조상·순서·글, 숨은 정의 구간)가 그대로.
func checkInsert(t *testing.T, src string) {
	t.Helper()
	p, ok := parseSpots([]byte(src))
	if !ok {
		return
	}
	norm := string(p.src)
	before, _ := p.outline(nil)
	// 실제로 넣는 세 층(datareport 가 만드는 꼴 — 기록 주석·표·빈 줄·캡션, 표 칸 글은 자리 판정의 spotProbe 와 다르게)
	const block = "<!-- itda-data {\"v\":1,\"dataset\":\"s\"} -->\n| 지점 | 매출(원) |\n| :-- | --: |\n| 강남 | 100 |\n\nTable: 지점 매출 · 판1 기준\n"
	for line := 1; line <= len(p.st.lines); line++ {
		spot, ok := InsertSpotAt([]byte(src), line)
		for _, sp := range p.spans {
			if ok && sp.start <= line && line <= sp.end {
				t.Errorf("각주 정의 안 커서 %d 를 받음 %+v\n%q", line, spot, src)
			}
		}
		if !ok {
			continue
		}
		pos, text, _ := InsertText(norm, spot.Line, block)
		out := norm[:pos] + text + norm[pos:]
		comment := strings.Count(norm[:pos]+text[:len(text)-len(strings.TrimLeft(text, "\n"))], "\n") + 1
		if tb, ok := TopBlockAt([]byte(out), comment+1); !ok || tb.Kind != "table" || tb.Start != comment+1 {
			t.Errorf("커서 %d → 줄 %d: 넣은 표가 최상위가 아님 %+v %v\n%q", line, spot.Line, tb, ok, out)
			continue
		}
		q, ok := parseSpots([]byte(out))
		if !ok {
			t.Errorf("커서 %d: 넣은 글을 읽지 못함\n%q", line, out)
			continue
		}
		after, ok := q.outline(insertedLines(norm, pos, text))
		if !ok || strings.Join(after, "\n") != strings.Join(before, "\n") {
			t.Errorf("커서 %d → 줄 %d: 기존 글이 바뀜\n원문 %q\n전 %q\n뒤 %q", line, spot.Line, src, before, after)
		}
	}
}

// 대표 입력에서 기존 글 보존 속성(넣은 세 층을 빼고 원문 구간을 넣은 길이만큼 맞추면 블록 꼴·조상·순서·글이 같다).
func TestInsertSpotPreserves(t *testing.T) {
	for _, src := range []string{
		"[^a]:\n    - 하나\n    - 둘\n\n본문\n",
		"[^a]: 앞\n\n문단\n\n[^a]:\n    - 하나\n    - 둘\n\n본문[^a]\n",
		"본문[^a]\n\n[^a]: 각주\n",
		"[^a]:\n    글",
		"[^a]: 하나\n\n\n\n    둘\n\n본문\n",
		"# 제목\n\n- 목록\n- 둘\n\n> 인용\n\n| a |\n|---|\n| 1 |\nTable: 캡\n\n```\n[^a]: 가짜\n```\n\n끝[^a]\n\n[^a]: 진짜\n",
		"---\ntitle: t\n---\n[^a]: 하나\n    둘\n\n본문[^a]\n",
		"본문\n[^a]: 끊음\n뒤\n",
		"[a]: /u\n    이어짐\n\n본문\n", // 퍼징 발견 — 링크 참조 정의를 떼고 남은 문단은 정의와 한 블록
		"# 제목\n\n\n: 정의 아님\n",      // 퍼징 발견 — 넣을 자리 뒤 첫 줄이 정의 표지면 거부
	} {
		checkInsert(t, src)
	}
}

// 짧은 제한 조합 퍼징: 참조 유무·같은 이름 정의 수·빈 줄 수·감싸는 블록·정의 내용·문서 끝(줄바꿈 유무)을 엮어 모든 커서 줄에서
// checkInsert 의 세 약속을 본다. 시간 상한 3초 — 넘으면 거기까지 본 것으로 끝낸다.
func TestInsertSpotFootnoteCombos(t *testing.T) {
	deadline := time.Now().Add(3 * time.Second)
	defs := []string{"[^a]: 글", "[^a]:\n    - 하나\n    - 둘", "[^a]: 하나\n\n    둘", "[^a]:", "[^a]: 하나\n게으른 줄"}
	wraps := []func(string) string{
		func(s string) string { return s },
		func(s string) string { return "> " + strings.ReplaceAll(s, "\n", "\n> ") },
		func(s string) string { return "- " + strings.ReplaceAll(s, "\n", "\n  ") },
		func(s string) string { return ":::columns\n" + s + "\n:::" },
	}
	n := 0
	for _, def := range defs {
		for wi, wrap := range wraps {
			for _, ref := range []string{"본문", "본문[^a]"} {
				for dup := 1; dup <= 2; dup++ {
					for blanks := 1; blanks <= 3; blanks++ {
						for _, first := range []bool{true, false} {
							var seqs []string // 끝 줄바꿈 유무로 판정이 달라지지 않는다(리뷰 3회차 [2])
							for _, eol := range []string{"\n", ""} {
								sep := strings.Repeat("\n", blanks+1)
								var parts []string
								for d := 0; d < dup; d++ {
									parts = append(parts, wrap(def))
									if d+1 < dup {
										parts = append(parts, "문단")
									}
								}
								doc := strings.Join(parts, sep)
								if first {
									doc += sep + ref
								} else {
									doc = ref + sep + doc
								}
								if time.Now().After(deadline) {
									t.Logf("시간 상한 — %d 문서까지 봄", n)
									return
								}
								checkInsert(t, doc+eol)
								seqs = append(seqs, doc+" → "+spotString(doc+eol, strings.Count(doc, "\n")+1))
								n++
								if t.Failed() {
									t.Fatalf("조합 %d(감쌈 %d) 에서 멈춤", n, wi)
								}
							}
							if seqs[0] != seqs[1] {
								t.Fatalf("끝 줄바꿈 유무로 판정이 다름\n%q\n%q", seqs[0], seqs[1])
							}
						}
					}
				}
			}
		}
	}
	t.Logf("조합 %d 문서", n)
}

// 기본 실행은 시드만 돈다. 짧은 퍼징: go test -run '^$' -fuzz FuzzInsertSpotFootnotes -fuzztime 10s ./mdoc
func FuzzInsertSpotFootnotes(f *testing.F) {
	for _, s := range []string{
		"[^a]:\n    - 하나\n    - 둘\n\n본문\n",
		"[^a]: 앞\n\n문단\n\n[^a]:\n    - 하나\n    - 둘\n\n본문[^a]\n",
		"[^a]:\n    글",
		"[^a]: 하나\n\n\n\n    둘\n\n본문\n",
		"본문[^a]\n\n[^a]: 각주\n",
		"> [^a]: 글\n>     더\n\n본문[^a]\n",
		"[00]:    000\n    0000000", // 퍼징 발견 — 링크 참조 정의 뒤 들여 쓴 이어짐
		"\n: 0",                     // 퍼징 발견 — 뒤 정의 표지 줄을 캡션이 용어로 삼킨다
		"0\n$$000",                  // 퍼징 발견(비교 잡음) — 끝 줄바꿈 없는 마지막 줄
		"0\n\v",                     // 퍼징 발견 — \v 만 있는 줄은 빈 줄이 아니다(문단 이어짐)
		"*\n\t$$0",                  // 퍼징 발견 — 원문 범위가 아닌 줄 조각에서 panic
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 400 || strings.Count(src, "\n") > 40 {
			return
		}
		checkInsert(t, src)
	})
}

// 리뷰 3회차 시험의 힘: 관찰이 끝나지 않았으면(complete 거짓) 구간을 믿지 않고 complex 로 거부한다 — 관찰기 미설치, 두 번 설치,
// 닫히지 않은 정의. 중첩 상한을 넘은 문서도 같다. InsertFitsAt 도 같다.
func TestInsertSpotIncompleteWatch(t *testing.T) {
	const src = "본문\n\n[^a]: 각주\n"
	if _, ok := InsertSpotAt([]byte(src), 1); !ok {
		t.Fatal("대조 — 보통 문서를 거부함")
	}
	for name, watch := range map[string]func(*state) *footnoteWatch{
		"미설치": func(st *state) *footnoteWatch { return &footnoteWatch{st: st, open: map[ast.Node]footnoteSpan{}} },
		"두 번 설치": func(st *state) *footnoteWatch {
			w := watchFootnotes(st)
			w.installed++
			return w
		},
		"닫히지 않은 정의": func(st *state) *footnoteWatch {
			w := watchFootnotes(st)
			w.open[ast.NewParagraph()] = footnoteSpan{}
			return w
		},
	} {
		t.Run(name, func(t *testing.T) {
			old := watchSpots
			watchSpots = watch
			defer func() { watchSpots = old }()
			if spot, ok := InsertSpotAt([]byte(src), 1); ok || spot.Kind != "complex" {
				t.Errorf("InsertSpotAt = %+v %v", spot, ok)
			}
			if spot, ok := InsertFitsAt([]byte(src), 2, spotProbe); ok || spot.Kind != "complex" {
				t.Errorf("InsertFitsAt = %+v %v", spot, ok)
			}
		})
	}
	// 옵션을 두 번 붙여도 덮개는 한 겹이다(이미 감싼 파서는 nestingGate 가 아니다)
	st := newState([]byte(src), Meta{}, Options{})
	w := watchFootnotes(st)
	st.md.Parser().AddOptions(footnoteWatchOption{w})
	if w.installed != 1 {
		t.Errorf("두 번 붙인 옵션: installed %d", w.installed)
	}
	deep := strings.Repeat("> ", 2000) + "x\n\n본문\n"
	if spot, ok := InsertSpotAt([]byte(deep), strings.Count(deep, "\n")); ok || spot.Kind != "complex" {
		t.Errorf("중첩 상한 초과: %+v %v", spot, ok)
	}
}

// 리뷰 3회차 시험의 힘: 다른 규칙은 통과하고 넣어 보기(keepsAt)만이 거부하는 입력(퍼징 넷째 발견). 끝 줄바꿈 유무로 같다.
func TestInsertSpotKeepsOnly(t *testing.T) {
	for src, want := range map[string]string{
		"0\n    - \n00\n-":   "B B B L",
		"0\n    - \n00\n-\n": "B B B L 5",
	} {
		if g := spotString(src, strings.Count(src, "\n")+1); g != want {
			t.Errorf("%q: %s, 기대 %s", src, g, want)
		}
		checkInsert(t, src)
	}
}

// insertedLines 는 넣은 바이트 범위로 새 글의 줄을 가르고, 그 뒤 줄은 원문 줄로 되돌린다 — 끝 줄바꿈 없는 글이 원문 빈 줄과 한 줄이
// 되어도(리뷰 4회차 [1]) 그 줄은 새 글이다.
func TestInsertedLines(t *testing.T) {
	src := "# h\n\n\nbody\n" // 1 # h, 2 빈, 3 빈, 4 body, 5 끝
	pos, text, _ := InsertText(src, 2, "<!-- c -->\n| a |\n|---|\n\nTable: x")
	if pos != 4 || text != "\n<!-- c -->\n| a |\n|---|\n\nTable: x" {
		t.Fatalf("InsertText %d %q", pos, text) // 앞은 줄바꿈 하나 뒤라 빈 줄 하나를 더하고, 뒤는 빈 줄 둘이라 더하지 않는다
	}
	m := insertedLines(src, pos, text)
	var got []string
	for l := 0; l <= 10; l++ {
		o, ok := m(l)
		got = append(got, fmt.Sprintf("%d:%d/%v", l, o, ok))
	}
	// 넣은 글: 2~7 줄(앞 빈 줄 2, 7 줄 「Table: x」 은 원문 2 줄의 줄바꿈으로 끝난다 — 줄바꿈 수 5 로는 6 줄까지만 센다),
	// 8 = 원문 3(빈), 9 = 원문 4(body), 10 = 원문 5. 0 은 자리를 모르는 블록이라 그대로
	if g := strings.Join(got, " "); g != "0:0/true 1:1/true 2:0/false 3:0/false 4:0/false 5:0/false 6:0/false 7:0/false 8:3/true 9:4/true 10:5/true" {
		t.Errorf("대응 %s", g)
	}
}

// InsertFitsAt 은 자리를 옮기지 않고 그 줄에 그 글을 넣어 본다(리뷰 3회차 [1]) — 각주 정의 안·들여 쓴 이어짐 앞·뒤 정의 표지·
// frontmatter·범위 밖은 거부, 정의 첫 줄 앞·보통 경계는 받는다. 차트 글(:::chart)도 최상위 블록으로 본다.
func TestInsertFitsAt(t *testing.T) {
	const table = "<!-- itda-data -->\n| a |\n|---|\n| 1 |\n\nTable: 캡\n"
	const chart = "<!-- itda-data -->\n:::chart{type=bar}\n\n| a | b |\n|---|--:|\n| x | 1 |\n\n: 캡\n\n:::\n"
	for _, c := range []struct {
		src   string
		line  int
		block string
		want  string
	}{
		{":::chart{type=bar}\n| a | b |\n|---|--:|\n| x | 1 |\n:::\n\n: orphan\n", 6, table, "L"},
		{":::chart{type=bar}\n| a | b |\n|---|--:|\n| x | 1 |\n:::\n\n: orphan\n", 6, chart, "L"},
		{":::chart{type=bar}\n| a | b |\n|---|--:|\n| x | 1 |\n:::\n\n본문\n", 6, chart, "6"},
		{"본문\n[^a]: 끊음\n뒤\n", 2, table, "2"},
		{"본문\n[^a]: 끊음\n뒤\n", 3, table, "F"},
		{"[^a]: note\n\nbody", 3, table, "3"},
		{"[a]: /u\n    이어짐\n\n본문\n", 2, table, "B"},
		{"---\ntitle: t\n---\n본문\n", 2, table, "M"},
		{"본문\n", 3, table, "3"},
		{"본문\n", 4, table, "?"},
		{"본문\n", 0, table, "?"},
		// 받은 글을 그대로 넣어 본다(리뷰 4회차 — 탐침으로 바꿔 보면 받는 자리): 표·차트가 아닌 글은 거부
		{"body\n", 2, "plain text", "B"},
		{"body\n", 2, "<!-- itda-data -->\n- 목록\n", "B"},
		{"body\n", 2, spotProbe, "2"},
		// 끝 줄바꿈 없는 실제 표 글(캡션 끝)을 빈 줄 둘 앞에 넣는다 — 마지막 줄은 원문 빈 줄의 줄바꿈으로 끝나도 새 글이다(리뷰 4회차 [1])
		{"# heading\n\n\nbody\n", 2, strings.TrimSuffix(table, "\n"), "2"},
		{"# heading\n\n\n\nbody\n", 2, strings.TrimSuffix(table, "\n"), "2"},
		{"# heading\n\n\n", 2, strings.TrimSuffix(table, "\n"), "2"},
	} {
		got := "?"
		switch spot, ok := InsertFitsAt([]byte(c.src), c.line, c.block); {
		case ok:
			got = strconv.Itoa(spot.Line)
		case spot.Kind != "":
			got = map[string]string{"footnote": "F", "list": "L", "frontmatter": "M", "boundary": "B", "complex": "X"}[spot.Kind]
		}
		if got != c.want {
			t.Errorf("%q 줄 %d: %s, 기대 %s", c.src, c.line, got, c.want)
		}
	}
}
