package mdoc

import (
	"reflect"
	"strings"
	"testing"
)

// 코드 블록 표시(itda-hyve#435): frontmatter code: — line-numbers(별칭 linenumbers·line_numbers). 기본은 켬이라
// "정하지 않음"(nil)과 false 를 갈라 화면이 정한다(정하지 않음 = 켬).
func TestCodeMeta(t *testing.T) {
	f := func(b bool) *bool { return &b }
	cases := []struct {
		fm    string
		want  *CodeMeta
		warns int
	}{
		{"", nil, 0},
		{"code: true", &CodeMeta{}, 0},
		{"code:", &CodeMeta{}, 0},
		{"code: false", &CodeMeta{LineNumbers: f(false)}, 0},
		{"code:\n  line-numbers: false", &CodeMeta{LineNumbers: f(false)}, 0},
		{"code: {line-numbers: false}", &CodeMeta{LineNumbers: f(false)}, 0}, // 안내서 예시(markdown.md 8장) 그대로 — 문서 예시 = 동작
		{"code:\n  line-numbers: true", &CodeMeta{LineNumbers: f(true)}, 0},
		{"code:\n  linenumbers: false", &CodeMeta{LineNumbers: f(false)}, 0},
		{"code:\n  line_numbers: false", &CodeMeta{LineNumbers: f(false)}, 0},
		{"code:\n  line-numbers: 아니오", &CodeMeta{}, 1},
		{"code:\n  line-numbers: 1", &CodeMeta{}, 1},
		{"code:\n  line-numbers:", &CodeMeta{}, 0}, // 빈 값은 정하지 않음(boolean 과 같다)
		{"code:\n  size: big", &CodeMeta{}, 1},
		{"code: 발표", nil, 1},
		{"code: [a, b]", nil, 1},
	}
	for _, c := range cases {
		src := "---\ntitle: t\n" + c.fm + "\n---\n\n본문\n"
		r := render(t, src)
		got := r.Meta.Code
		if (got == nil) != (c.want == nil) || (got != nil && !reflect.DeepEqual(got, c.want)) {
			t.Errorf("%q → %+v, 기대 %+v", c.fm, got, c.want)
		}
		if len(r.Warnings) != c.warns {
			t.Errorf("%q 경고 %d, 기대 %d: %+v", c.fm, len(r.Warnings), c.warns, r.Warnings)
		}
	}
}

// 틀린 line-numbers 는 frontmatter 값 경고(줄 번호는 값의 줄)이고, 모르는 키 경고는 키 목록에 line-numbers 를 적는다
func TestCodeLineNumbersWarning(t *testing.T) {
	r := render(t, "---\ntitle: t\ncode:\n  line-numbers: 끔\n  size: big\n---\n\n본문\n")
	if len(r.Warnings) != 2 {
		t.Fatalf("경고 %+v", r.Warnings)
	}
	w := r.Warnings[0]
	if w.Code != WarnFrontmatterValue || w.Line != 4 || !strings.Contains(w.Message, "code.line-numbers") || !strings.Contains(w.Message, "true 또는 false") {
		t.Errorf("값 꼴 경고 %+v", w)
	}
	if w := r.Warnings[1]; w.Code != WarnFrontmatterValue || !strings.Contains(w.Message, "line-numbers)") {
		t.Errorf("모르는 키 경고 %+v", w)
	}
}
