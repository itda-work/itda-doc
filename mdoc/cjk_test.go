package mdoc

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/testutil"
	"github.com/yuin/goldmark/util"
)

// 이 파일의 비교는 goldmark 의 테스트 방식을 따라 html.WithUnsafe() 를 쓴다(raw HTML 이 든 예제도 같게 비교하려고).
// 테스트 안에서만 쓰고, 패키지의 렌더러는 이것을 켜지 않는다.

// plainGoldmark 는 보정 전(goldmark 기본 강조·취소선)의 한 문단 결과다.
func plainGoldmark(t *testing.T, src string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := goldmark.New(goldmark.WithExtensions(extension.Strikethrough)).Convert([]byte(src), &buf); err != nil {
		t.Fatal(err)
	}
	return strings.TrimSuffix(strings.TrimPrefix(buf.String(), "<p>"), "</p>\n")
}

func cjkFriendlyGoldmark(exts ...goldmark.Extender) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(exts...),
		goldmark.WithRendererOptions(html.WithUnsafe()),
		goldmark.WithParserOptions(parser.WithInlineParsers(
			util.Prioritized(cjkEmphasisParser{}, 499),
			util.Prioritized(cjkStrikeParser{}, 499),
		)),
	)
}

type specExample struct {
	Markdown string `json:"markdown"`
	HTML     string `json:"html"`
	Example  int    `json:"example"`
	Section  string `json:"section"`
}

// skipOrFatalf 는 CommonMark 예제를 읽을 수 없을 때 건너뛴다. ITDA_DOC_STRICT=1 이면(just test·CI) 건너뛰지 않고 실패한다 —
// 652 예제 시험이 조용히 빠진 채 통과하지 않게.
func skipOrFatalf(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv("ITDA_DOC_STRICT") == "1" {
		t.Fatalf(format, args...)
	}
	t.Skipf(format, args...)
}

// goldmarkDir 는 모듈 캐시의 goldmark 폴더다. spec.json 은 거기 _test/ 에 있다.
func goldmarkDir(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/yuin/goldmark").Output()
	if err != nil {
		skipOrFatalf(t, "goldmark 모듈 폴더를 찾지 못함: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func hasCJK(s string) bool {
	for _, r := range s {
		if c := classify(r); c.cjk() || c == clsIVS || c == clsSVS {
			return true
		}
	}
	return false
}

// TestCommonMarkSpecUnchanged 는 보정이 CommonMark 동작을 깨지 않는지 본다.
// CommonMark 0.31.2 명세 예제 전체(652개) 중 CJK 글자가 없는 것은 결과가 goldmark 기본과 글자 하나까지 같아야 한다.
// 강조 절(132개)은 명세의 기대 HTML 과도 맞춰 본다.
func TestCommonMarkSpecUnchanged(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(goldmarkDir(t), "_test", "spec.json"))
	if err != nil {
		skipOrFatalf(t, "spec.json 없음: %v", err)
	}
	var examples []specExample
	if err := json.Unmarshal(raw, &examples); err != nil {
		t.Fatal(err)
	}
	plain := goldmark.New(goldmark.WithRendererOptions(html.WithUnsafe()))
	cjk := cjkFriendlyGoldmark()
	conv := func(m goldmark.Markdown, src string) string {
		var buf bytes.Buffer
		if err := m.Convert([]byte(src), &buf); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}

	var total, same, emph, emphSame, emphSpecPlain, emphSpecCJK int
	for _, ex := range examples {
		if hasCJK(ex.Markdown) {
			continue
		}
		total++
		a, b := conv(plain, ex.Markdown), conv(cjk, ex.Markdown)
		if a == b {
			same++
		} else {
			t.Errorf("예제 %d (%s) 결과가 달라짐\n입력 %q\n기본 %q\n보정 %q", ex.Example, ex.Section, ex.Markdown, a, b)
		}
		if ex.Section == "Emphasis and strong emphasis" {
			emph++
			if a == b {
				emphSame++
			}
			if a == ex.HTML {
				emphSpecPlain++
			}
			if b == ex.HTML {
				emphSpecCJK++
			}
		}
	}
	t.Logf("CJK 없는 예제 %d/%d 가 goldmark 기본과 같음 (전체 %d)", same, total, len(examples))
	t.Logf("강조 절: %d/%d 가 기본과 같음, 명세 기대 HTML 일치 — 기본 %d, 보정 %d", emphSame, emph, emphSpecPlain, emphSpecCJK)
	if emph != 132 {
		t.Errorf("강조 절 예제 수가 132 가 아님: %d", emph)
	}
}

// TestGoldmarkStrikethroughCases 는 goldmark 의 취소선 테스트 파일을 보정한 파서로 그대로 통과하는지 본다.
func TestGoldmarkStrikethroughCases(t *testing.T) {
	path := filepath.Join(goldmarkDir(t), "extension", "_test", "strikethrough.txt")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("strikethrough.txt 없음: %v", err)
	}
	testutil.DoTestCaseFile(cjkFriendlyGoldmark(extension.Strikethrough), path, t)
}

// 명세 「Tips for Implementers」의 표. 바깥(Out)·표시(M)·안쪽(In) 글자로 열 수 있는지(Start)·닫을 수 있는지(End).
// https://github.com/tats-u/markdown-cjk-friendly/blob/main/implementers-tips.md
func TestCJKFlankingTable(t *testing.T) {
	cases := []struct {
		out, marker, in string
		end, start      bool
	}{
		{" ", "*", " ", false, false},
		{" ", "*", `"`, false, true},
		{" ", "*", "a", false, true},
		{" ", "*", "～", false, true},
		{" ", "*", "字", false, true},
		{" ", "_", " ", false, false},
		{" ", "_", `"`, false, true},
		{" ", "_", "a", false, true},
		{" ", "_", "～", false, true},
		{" ", "_", "字", false, true},
		{`"`, "*", `"`, true, true},
		{`"`, "*", "a", false, true},
		{`"`, "*", "字", true, true},
		{"～", "*", "a", true, true},
		{`"`, "_", `"`, true, true},
		{`"`, "_", "a", false, true},
		{`"`, "_", "字", false, true},
		{"～", "_", "a", false, true},
		{"a", "*", "a", true, true},
		{"a", "_", "a", false, false},
	}
	for _, c := range cases {
		// 표의 Out 은 표시 앞 글자, In 은 뒤 글자다. 같은 자리에서 열 수 있는지(Start)·닫을 수 있는지(End)를 본다.
		before, after := classify([]rune(c.out)[0]), classify([]rune(c.in)[0])
		left, right := flanking(before, after)
		start, end := left, right
		if c.marker == "_" {
			start = left && (!right || before.spaceOrPunct())
			end = right && (!left || after.spaceOrPunct())
		}
		if start != c.start || end != c.end {
			t.Errorf("%q %s %q: start=%v end=%v, 표는 start=%v end=%v", c.out, c.marker, c.in, start, end, c.start, c.end)
		}
	}
}

func TestIsCJK(t *testing.T) {
	for r, want := range map[rune]bool{
		'가': true, '힣': true, 'ㄱ': true, 'ᄀ': true, '字': true, 'あ': true, 'ア': true, '。': true, '「': true,
		'～': true, '（': true, '〰': true, '㊗': true, 0x20000: true, 0x1F202: true,
		'a': false, '"': false, '(': false, '%': false, '‘': false, '·': false, '😀': false, '⌚': false, 0x1F37A: false,
	} {
		if isCJK(r) != want {
			t.Errorf("isCJK(%U %q) = %v", r, r, !want)
		}
	}
}
