package mdoc

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/texmath"
)

// 수식 안내서(spec/math.md)를 코드와 맞춘다 — 상한 표는 상수와, 지원 범위 표의 예는 그려지는지,
// 쓰지 않는 명령 표의 명령은 막히는지, 1장 예제는 경고 없이 그려지는지.
func TestMathGuide(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "spec", "math.md"))
	if err != nil {
		t.Fatal(err)
	}
	g := string(b)
	section := func(name string) string {
		i, j := strings.Index(g, "<!-- "+name+":begin -->"), strings.Index(g, "<!-- "+name+":end -->")
		if i < 0 || j < i {
			t.Fatalf("안내서에 %s 표지가 없음", name)
		}
		return g[i:j]
	}

	want := map[string]int{
		"수식 하나의 길이(바이트)": texmath.MaxInputBytes, "괄호·환경의 중첩": texmath.MaxDepth, "매크로 확장 횟수": texmath.MaxExpand,
		"간격·크기(em)": texmath.MaxLengthEm, "문서 하나의 수식 수": MaxMathPerDoc,
	}
	got := map[string]int{}
	for _, m := range regexp.MustCompile(`(?m)^\| ([^|]+?) \| (\d+) \|`).FindAllStringSubmatch(section("math-limits"), -1) {
		n, _ := strconv.Atoi(m[2])
		got[m[1]] = n
	}
	if len(got) != len(want) {
		t.Errorf("상한 표 %v, 코드 %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("상한 %q: 안내서 %d, 코드 %d", k, got[k], v)
		}
	}

	// 지원 범위: 표의 `…` 마다 블록 수식으로 그려 본다(환경 이름만 적힌 칸은 그 환경으로 감싼다)
	code := regexp.MustCompile("`([^`]+)`")
	n := 0
	for _, line := range strings.Split(section("math-support"), "\n") {
		if !strings.HasPrefix(line, "| ") || strings.HasPrefix(line, "| 분류") {
			continue
		}
		cells := strings.Split(line, "|")
		for _, m := range code.FindAllStringSubmatch(cells[2], -1) {
			tex := m[1]
			if regexp.MustCompile(`^[a-z]+$`).MatchString(tex) {
				tex = `\begin{` + tex + `}a & b \\ c & d\end{` + tex + `}`
				if strings.TrimSpace(cells[1]) == "정렬" {
					tex = `\begin{` + m[1] + `}a &= b \\ c &= d\end{` + m[1] + `}`
					if m[1] == "gather" || m[1] == "multline" || m[1] == "equation" {
						tex = `\begin{` + m[1] + `}a = b\end{` + m[1] + `}`
					}
				}
			}
			tex = strings.NewReplacer(" … ", " x ", "…", "x").Replace(tex)
			r := render(t, "$$\n"+tex+"\n$$")
			if len(r.Warnings) > 0 || !strings.Contains(r.HTML, "<math") {
				t.Errorf("지원 범위 %s 의 %q 를 그리지 못함: %+v", strings.TrimSpace(cells[1]), tex, r.Warnings)
			}
			n++
		}
	}
	if n < 60 {
		t.Errorf("지원 범위 예가 %d 개뿐 — 표를 읽지 못함?", n)
	}

	// 쓰지 않는 명령: 5장 표의 명령마다 막는다
	i, j := strings.Index(g, "## 5. 쓰지 않는 명령"), strings.Index(g, "모르는 명령(") // 표만(뒤의 글은 미지원·문법 오류 예)
	blockedN := 0
	for _, m := range code.FindAllStringSubmatch(g[i:j], -1) {
		cmd := m[1]
		if !strings.HasPrefix(cmd, `\`) {
			continue
		}
		tex := cmd + "{a}{b}{c}"
		if cmd == `\tiny` {
			tex = `\tiny x` // \tiny … \Huge 칸
		}
		r := render(t, "$"+tex+"$")
		if strings.Contains(r.HTML, "<math") || len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0].Message, "쓰지 않는 명령 "+cmd) {
			t.Errorf("%s 가 막히지 않음: %+v", cmd, r.Warnings)
		}
		blockedN++
	}
	if r := render(t, "$$\n\\begin{CD}A @>>> B\\end{CD}\n$$"); !hasWarn(r, WarnMath) {
		t.Error("CD 환경이 막히지 않음")
	}
	if blockedN < 20 {
		t.Errorf("쓰지 않는 명령 %d 개뿐", blockedN)
	}

	// 1장 예제
	ex := regexp.MustCompile("(?s)<!-- math-example -->\n```markdown\n(.*?)```").FindStringSubmatch(g)
	if ex == nil {
		t.Fatal("안내서에 math-example 이 없음")
	}
	if r := render(t, ex[1]); len(r.Warnings) > 0 || strings.Count(r.HTML, "<math") != 2 {
		t.Errorf("1장 예제: %+v\n%s", r.Warnings, r.HTML)
	}
}
