package mdoc

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// frontmatter list-style(itda-hyve#141): 글자 하나 또는 목록, 정규형으로. 틀린 값은 경고하고 버린다(테마 기본).
func TestListStyleFrontmatter(t *testing.T) {
	cases := []struct {
		front, want string
		warn        bool
	}{
		{"list-style: gaejosik", "gaejosik", false},
		{"list-style: GONGMUN", "gongmun", false},
		{"list-style: plain", "plain", false},
		{"list-style: [gongmun, gaejosik]", "gaejosik gongmun", false},
		{"list-style:\n  - gaejosik\n  - gaejosik", "gaejosik", false},
		{"list-style:", "", false},
		{"list-style: bullets", "", true},
		{"list-style: [plain, gongmun]", "", true},
		{"list-style: []", "", true},
		{"list-style: {a: 1}", "", true},
		{"list-style: [[gaejosik]]", "", true},
	}
	for _, c := range cases {
		r := render(t, "---\n"+c.front+"\n---\n\n- 항목\n")
		if r.Meta.ListStyle != c.want {
			t.Errorf("%q: %q, 기대 %q", c.front, r.Meta.ListStyle, c.want)
		}
		if got := countWarn(r, WarnFrontmatterValue) > 0; got != c.warn {
			t.Errorf("%q: 경고 %v, 기대 %v — %+v", c.front, got, c.warn, r.Warnings)
		}
		if _, ok := r.Meta.Field("list-style"); ok {
			t.Errorf("%q: 아는 키가 Extra 에 남음", c.front)
		}
	}
}

// 글자로 적은 항목 기호·번호(itda-hyve#141)는 경고만 — 목록마다 하나, 줄은 첫 자리. HTML 은 그대로다.
func TestListMarkerWarning(t *testing.T) {
	cases := []struct {
		name, src string
		lines     []int // 경고 줄(목록마다 하나)
	}{
		{"개조식 두 단계", "- ○ 내용\n  - ○ 하위", []int{1, 2}},
		{"한 목록 여러 항목", "- □ 하나\n- □ 둘\n- □ 셋", []int{1}},
		{"빈칸 없는 기호", "- □추진", []int{1}},
		{"■ ● ◦ ▸ •", "- ■ 가\n\n* ● 나\n\n+ ◦ 다\n\n---\n\n- ▸ 라\n\n---\n\n- • 마", []int{1, 3, 5, 9, 13}},
		{"ㅇ 자모", "- ㅇ 내용", []int{1}},
		{"순서 목록 안 가.", "1. 가. 내용", []int{1}},
		{"게으른 이어쓰기", "1. 관련\n2. 본문\n   가. 선정\n   나. 기간", []int{3}},
		{"가) (가) (1) (12)", "- 가) 하나\n\n---\n\n- (가) 둘\n\n---\n\n- (1) 셋\n\n---\n\n- (12) 넷", []int{1, 5, 9, 13}},
		{"거. 허)", "- 거. 하나\n\n---\n\n- 허) 둘", []int{1, 5}},
		{"이어 붙은 줄의 기호", "- 내용\n  ○ 하위", []int{2}},
		{"이어 붙은 줄의 (가)", "1. 본문\n   (가) 하위", []int{2}},
		{"둘째 문단", "- 내용\n\n  ○ 둘째 문단", []int{3}},
		{"인용 안", "> - ○ 인용 안", []int{1}},
		{"둘째 항목부터", "- 보통\n- ○ 기호", []int{2}},
		// 음성
		{"보통 목록", "- 내용\n  - 하위\n1. 하나\n   1. 둘", nil},
		{"※ 보충", "- ※ 보충", nil},
		{"ㅇ 이름", "- ㅇㅇ 이름", nil},
		{"가.나다", "- 가.나다", nil},
		{"가 뒤 빈칸 없음", "- 가)나다", nil},
		{"사다리 밖 한글", "- 고. 하나\n\n---\n\n- (고) 둘", nil},
		{"세 자리 괄호 숫자", "- (100) 하나", nil},
		{"닫히지 않은 괄호", "- (1 하나", nil},
		{"원문자(단계를 가리키는 글로 흔함)", "- ① 하나\n- ③ 뒤 캐시\n- ㉮ 셋", nil},
		{"이어 붙은 줄의 (1)", "- 순서: (1) 먼저\n  (2) 다음", nil},
		{"둘째 문단의 (1)", "- 내용\n\n  (1) 둘째 문단", nil},
		{"코드", "- `가.` 코드\n- `○` 코드", nil},
		{"이스케이프", "- \\(1) 하나\n- 가\\. 둘", nil},
		{"강조 안", "- **○** 굵게", nil},
		{"줄 가운데", "- 내용 ○ 가운데\n- 내용 (1) 가운데", nil},
		{"문단", "○ 목록 아닌 문단\n\n가. 문단", nil},
		{"표 칸", "| a |\n|---|\n| ○ b |", nil},
		{"코드 블록", "```\n- ○ 코드\n```", nil},
		{"작업 목록", "- [ ] ○ 할 일", nil},
	}
	for _, c := range cases {
		r := render(t, c.src)
		var got []int
		for _, w := range r.Warnings {
			if w.Code == WarnListMarker {
				got = append(got, w.Line)
			}
		}
		if len(got) != len(c.lines) {
			t.Errorf("%s %q: 경고 줄 %v, 기대 %v", c.name, c.src, got, c.lines)
			continue
		}
		for i := range got {
			if got[i] != c.lines[i] {
				t.Errorf("%s %q: 경고 줄 %v, 기대 %v", c.name, c.src, got, c.lines)
				break
			}
		}
	}
	// HTML 은 경고가 없을 때와 같다(기호를 알아보고 숨기지 않는다)
	r := render(t, "- ○ 내용")
	contains(t, r.HTML, "<li>○ 내용</li>")
	// 메시지는 고칠 길을 적는다
	if m := render(t, "- ○ 내용").Warnings[0].Message; !strings.Contains(m, "list-style: gaejosik") || !strings.Contains(m, "○") {
		t.Errorf("기호 메시지: %s", m)
	}
	if m := render(t, "1. 가. 내용").Warnings[0].Message; !strings.Contains(m, "list-style: gongmun") || !strings.Contains(m, "가.") {
		t.Errorf("번호 메시지: %s", m)
	}
}

// 저장소의 마크다운 문서에는 글자로 적은 항목 기호가 없다(itda-hyve#141 — 재현 샘플도 list-style 로 다시 썼다). 조사 자료의
// 이슈 초안처럼 기호 문법 자체를 보이는 글은 코드로 적는다.
func TestListMarkerRepoDocs(t *testing.T) {
	err := filepath.WalkDir("..", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p != ".." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, w := range render(t, string(b)).Warnings {
			if w.Code == WarnListMarker {
				t.Errorf("%s:%d %s", strings.TrimPrefix(p, "../"), w.Line, w.Message)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
