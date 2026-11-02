package mdoc

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 이슈 #140 의 시험 입력표 그대로 — 각 입력이 완전한 문서, 양성은 그 코드의 경고 하나(줄 1), 음성은 0.
// 다른 코드의 경고도 없어야 한다(한 입력이 두 함정에 걸리지 않는다).
func TestPitfallIssueTables(t *testing.T) {
	cases := []struct {
		name, src, code string
		warn            bool
	}{
		{"W1 양1", "06시~22시 → 06시~다음 날 01시", WarnTildeRange, true},
		{"W1 양2", "3~5명, 10~20명", WarnTildeRange, true},
		{"W1 음1", "~~취소~~ 글", WarnTildeRange, false},
		{"W1 음2", "글 ~취소~ 글", WarnTildeRange, false},
		{"W1 음3", "3~5명 모집", WarnTildeRange, false},
		{"W1 음4", `06시\~22시 → 06시\~다음 날 01시`, WarnTildeRange, false},

		{"W2 양1", "2026. 10. 14.(수) / 교통정책과", WarnDateList, true},
		{"W2 양2", "2099. 10. 14.", WarnDateList, true},
		{"W2 양3", "1900. 1. 1.", WarnDateList, true},
		{"W2 양4", "2026. 12. 31.", WarnDateList, true},
		{"W2 음1", "2100. 10. 14.", WarnDateList, false},
		{"W2 음2", "1899. 10. 14.", WarnDateList, false},
		{"W2 음3", "2026. 13. 1.", WarnDateList, false},
		{"W2 음4", "2099) 10. 14.", WarnDateList, false},
		{"W2 음5", "2026. 10) 14.", WarnDateList, false},
		{"W2 음6", "1000. 항목", WarnDateList, false},
		{"W2 음7", "2026. 첫째 항목", WarnDateList, false},
		{"W2 음8", "일정 안내\n2026. 10. 14.", WarnDateList, false},
		{"W2 음9", "회의는 2026. 10. 14. 에 열린다.", WarnDateList, false},
		{"W2 음10", `2026\. 10. 14.(수)`, WarnDateList, false},
		{"W2 음11", "| 날짜 |\n|---|\n| 2026. 10. 14. |", WarnDateList, false},
	}
	for _, c := range cases {
		r := render(t, c.src)
		want := 0
		if c.warn {
			want = 1
		}
		if len(r.Warnings) != want {
			t.Errorf("%s %q: 경고 %d 개, 기대 %d — %+v", c.name, c.src, len(r.Warnings), want, r.Warnings)
			continue
		}
		if c.warn && (r.Warnings[0].Code != c.code || r.Warnings[0].Line != 1) {
			t.Errorf("%s %q: 경고 %+v, 기대 코드 %s 줄 1", c.name, c.src, r.Warnings[0], c.code)
		}
	}
}

// 이스케이프한 입력은 경고 없이 글자 그대로다(받아들임).
func TestPitfallEscapedLiteral(t *testing.T) {
	r := render(t, `06시\~22시 → 06시\~다음 날 01시`)
	contains(t, r.HTML, "06시~22시 → 06시~다음 날 01시")
	notContains(t, r.HTML, "<del>")
	r = render(t, `2026\. 10. 14.(수)`)
	contains(t, r.HTML, ">2026. 10. 14.(수)</p>")
	notContains(t, r.HTML, "<ol")
}

// 경계와 AST 에서 빠져야 하는 자리. 양성은 경고 하나, 그 줄 번호는 문제 표기가 있는 원문 줄.
func TestPitfallBoundaries(t *testing.T) {
	cases := []struct {
		name, src, code string
		line            int // 0 이면 경고 없음
	}{
		// W1: 여는 ~ 앞뒤 글자
		{"숫자~숫자", "3~5 그리고 7~9", WarnTildeRange, 1},
		{"한글~한글", "월~금 근무, 토~일 휴무", WarnTildeRange, 1},
		{"숫자~한글", "10시~정오 그리고 2시~오후", WarnTildeRange, 1},
		{"여는 ~ 뒤 공백", "3 ~5 그리고 7~ 9", WarnTildeRange, 0},
		{"여는 ~ 앞 라틴", "a~5 그리고 7~9", WarnTildeRange, 0},
		{"여는 ~ 뒤 라틴", "3~b 그리고 7~9", WarnTildeRange, 0},
		{"물결 둘 범위", "3~~5 그리고 7~~9", WarnTildeRange, 0},
		{"셋째 줄", "제목 줄\n\n둘째 문단\n06시~22시 → 06시~01시", WarnTildeRange, 4},
		{"줄 넘어 닫음", "06시~22시\n→ 06시~01시", WarnTildeRange, 1},
		{"문단 둘 둘 다", "3~5명, 10~20명\n\n1~2시, 3~4시", WarnTildeRange, 1}, // 아래에서 둘을 따로 센다
		{"코드", "`3~5명, 10~20명`", WarnTildeRange, 0},
		{"코드 블록", "```\n3~5명, 10~20명\n```", WarnTildeRange, 0},
		{"수식", "$3~5 + 10~20$", WarnTildeRange, 0},
		{"HTML 블록", "<div>\n3~5명, 10~20명\n</div>", WarnTildeRange, 0},
		{"링크 주소", "[보기](http://a.b/3~5/10~20)", WarnTildeRange, 0},
		{"표 칸", "| 기간 |\n|---|\n| 3~5명, 10~20명 |", WarnTildeRange, 3}, // 칸 안에서도 실제로 취소선이 된다
		{"목록 안", "- 3~5명, 10~20명", WarnTildeRange, 1},
		{"인용 안", "> 06시~22시 → 06시~01시", WarnTildeRange, 1},

		// W2: 연 1899·1900·2099·2100, 월 0·1·12·13, 구분자
		{"연 1900 월 12", "1900. 12. 1.", WarnDateList, 1},
		{"연 2099 월 1", "2099. 1. 1.", WarnDateList, 1},
		{"월 0", "2026. 0. 1.", WarnDateList, 0},
		{"둘 다 )", "2026) 10) 14.", WarnDateList, 0},
		{"날짜 아닌 중첩(알려진 오탐)", "2026. 10. 항목", WarnDateList, 1},
		{"빈 줄 뒤", "일정 안내\n\n2026. 10. 14.(수)", WarnDateList, 3},
		{"인용 안", "> 2026. 10. 14.", WarnDateList, 1},
		{"코드", "```\n2026. 10. 14.\n```", WarnDateList, 0},
		{"들여쓴 코드", "    2026. 10. 14.", WarnDateList, 0},
		{"HTML 블록", "<div>\n2026. 10. 14.\n</div>", WarnDateList, 0},
		{"둘째 항목만", "2026. 첫째\n2027. 10. 14.", WarnDateList, 0},
	}
	for _, c := range cases {
		r := render(t, c.src)
		var got []Warning
		for _, w := range r.Warnings {
			if w.Code == c.code {
				got = append(got, w)
			}
		}
		switch {
		case c.line == 0 && len(got) != 0:
			t.Errorf("%s %q: 경고가 없어야 함 — %+v", c.name, c.src, got)
		case c.line > 0 && (len(got) == 0 || got[0].Line != c.line):
			t.Errorf("%s %q: 줄 %d 경고 기대 — %+v", c.name, c.src, c.line, got)
		case c.line > 0 && c.name != "문단 둘 둘 다" && len(got) != 1:
			t.Errorf("%s %q: 경고 하나 기대 — %+v", c.name, c.src, got)
		}
	}
	// 취소선마다 하나
	r := render(t, "3~5명, 10~20명\n\n1~2시, 3~4시")
	if n := countWarn(r, WarnTildeRange); n != 2 {
		t.Errorf("문단 둘: tilde-range %d 개, 기대 2 — %+v", n, r.Warnings)
	}
	// frontmatter 가 있어도 원문 줄
	r = render(t, "---\ntitle: 안내\n---\n\n2026. 10. 14.(수)\n\n3~5명, 10~20명")
	if !hasWarnAt(r, WarnDateList, 5) || !hasWarnAt(r, WarnTildeRange, 7) {
		t.Errorf("frontmatter 뒤 줄 번호: %+v", r.Warnings)
	}
}

// 메시지는 조건부("범위 표기면"·"날짜면")이고 고칠 표기를 적는다.
func TestPitfallMessages(t *testing.T) {
	r := render(t, "3~5명, 10~20명")
	if m := r.Warnings[0].Message; !strings.Contains(m, "범위 표기") || !strings.Contains(m, `\~`) {
		t.Errorf("W1 메시지: %s", m)
	}
	r = render(t, "2026. 10. 14.")
	if m := r.Warnings[0].Message; !strings.Contains(m, "날짜면") || !strings.Contains(m, `2026\.`) {
		t.Errorf("W2 메시지: %s", m)
	}
}

// 저장소의 마크다운 문서(문서 창이 여는 docs·조사 자료·재현 샘플·README 등 모두)에는 함정 경고가 없다 — 범위·날짜는
// \~·2026\. 로 적는다. 처음 켰을 때 28곳이 실제로 취소선이 되어 있었다(#140).
func TestPitfallRepoDocs(t *testing.T) {
	var files []string
	err := filepath.WalkDir("..", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p != ".." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".md") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 20 {
		t.Fatalf("문서 %d 개", len(files))
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range render(t, string(b)).Warnings {
			if w.Code == WarnTildeRange || w.Code == WarnDateList {
				t.Errorf("%s:%d %s %s", strings.TrimPrefix(f, "../"), w.Line, w.Code, w.Message)
			}
		}
	}
}

func countWarn(r Result, code string) int {
	n := 0
	for _, w := range r.Warnings {
		if w.Code == code {
			n++
		}
	}
	return n
}

func hasWarnAt(r Result, code string, line int) bool {
	for _, w := range r.Warnings {
		if w.Code == code && w.Line == line {
			return true
		}
	}
	return false
}
