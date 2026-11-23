package mdoc

import (
	"fmt"
	"strings"
	"testing"
)

// 보드 타일의 자리·크기(itda-work/itda-hyve#1038): 제목 속성 at·span 의 검사(boardplace.go). 놓는 일은 web/dashboard.js pack 이 하고
// node 시험(web/testdata/board_pack.cjs)이 본다.

func boardWarns(ws []Warning) string {
	var out []string
	for _, w := range ws {
		out = append(out, fmt.Sprintf("%s@%d", w.Code, w.Line))
	}
	return strings.Join(out, ",")
}

func TestBoardPlace(t *testing.T) {
	const fm = "---\ndashboard: {cols: 4}\n---\n\n" // 본문은 5줄부터
	cases := []struct {
		name, body string
		warns      string   // 경고 코드@줄
		has, not   []string // HTML 에 있어야·없어야 하는 조각
	}{
		{"맞는 값은 data-* 로", "## 매출 {at=3,1 span=2x2}\n\n글\n", "",
			[]string{`<h2 id="매출" data-at="3,1" data-span="2x2"`}, []string{"{"}},
		{"at·span 하나씩", "## 가 {at=1,1}\n\n글\n\n## 나 {span=4x1}\n\n글\n", "",
			[]string{`data-at="1,1"`, `data-span="4x1"`}, nil},
		{"따옴표 값의 공백", "## 가 {at=\"2, 1\"}\n\n글\n", "", []string{`data-at="2, 1"`}, nil},
		{"꼴이 틀린 at", "## 가 {at=3}\n\n글\n", "dashboard-place@5", nil, []string{"data-at"}},
		{"0 행", "## 가 {at=1,0}\n\n글\n", "dashboard-place@5", nil, []string{"data-at"}},
		{"격자 밖 열", "## 가 {at=5,1}\n\n글\n", "dashboard-place@5", nil, []string{"data-at"}},
		{"at + span 이 격자 밖", "## 가 {at=4,1 span=2x1}\n\n글\n", "dashboard-place@5", []string{`data-span="2x1"`}, []string{"data-at"}},
		{"대문자 X", "## 가 {span=3X4}\n\n글\n", "dashboard-place@5", nil, []string{"data-span"}},
		{"너비가 cols 보다 큼", "## 가 {at=1,1 span=5x1}\n\n글\n", "dashboard-place@5", []string{`data-at="1,1"`}, []string{"data-span"}},
		{"높이 0", "## 가 {span=2x0}\n\n글\n", "dashboard-place@5", nil, []string{"data-span"}},
		{"높이 상한", "## 가 {span=1x100}\n\n글\n", "dashboard-place@5", nil, []string{"data-span"}},
		{"행 상한", "## 가 {at=1,99 span=1x2}\n\n글\n", "dashboard-place@5", []string{`data-span="1x2"`}, []string{"data-at"}},
		// 겹침: 읽는 차례로 뒤의 것이 경고, 그 at 만 뗀다(span 은 남는다)
		{"겹침", "## 가 {at=1,1 span=2x2}\n\n글\n\n## 나 {at=2,2 span=2x1}\n\n글\n", "dashboard-place@9",
			[]string{`data-at="1,1"`, `data-span="2x1"`}, []string{`data-at="2,2"`}},
		{"겹침: 떨어진 칸은 괜찮다", "## 가 {at=1,1 span=2x2}\n\n글\n\n## 나 {at=3,2}\n\n글\n", "", []string{`data-at="3,2"`}, nil},
		// 행은 구역 안에서 센다 — 가로줄·구역 제목 줄·쪽 나눔 뒤의 같은 자리는 겹치지 않는다
		{"구역이 다르면 겹치지 않음", "## 가 {at=1,1}\n\n글\n\n---\n\n## 나 {at=1,1}\n\n글\n\n# 구역\n\n## 다 {at=1,1}\n\n글\n\n\\newpage\n\n## 라 {at=1,1}\n\n글\n", "",
			[]string{`<h2 id="나" data-at="1,1"`, `<h2 id="다" data-at="1,1"`, `<h2 id="라" data-at="1,1"`}, nil},
		// 타일 제목이 아닌 곳: 구역 제목 줄·타일 안 낮은 제목·컨테이너 안 제목 — 경고하고 뗀다
		{"구역 제목", "# 구역 {at=1,1}\n\n## 가\n\n글\n", "dashboard-place@5", nil, []string{"data-at"}},
		{"타일 안 낮은 제목", "## 가\n\n글\n\n### 작은 {span=2x1}\n\n글\n", "dashboard-place@9", nil, []string{"data-span"}},
		{"컨테이너 안 제목", "## 가\n\n:::box\n## 안 {at=1,1}\n\n글\n:::\n", "dashboard-place@8", nil, []string{"data-at"}},
		// 건너뛰기는 화면 groups 와 같다 — class pause 가 붙은 제목·컨테이너는 종류를 가리지 않고 보드에 없다(itda-hyve#1038 리뷰 1회차)
		{"pause 제목은 타일이 아니다", "## 숨김 {.pause at=1,1}\n\n## 자동\n\n글\n\n## 보임 {at=1,1}\n\n글\n", "dashboard-place@5",
			[]string{`<h2 id="보임" data-at="1,1"`, `<h2 id="숨김" class="pause" data-line="5">`}, nil},
		{"pause·page-break 컨테이너는 건너뜀(경계 아님)", "## 가 {at=1,1}\n\n글\n\n::: {.pause .page-break}\n:::\n\n## 나 {at=1,1}\n\n글\n", "dashboard-place@12",
			[]string{`<h2 id="가" data-at="1,1"`}, []string{`<h2 id="나" data-at`}},
		{"page-break 컨테이너는 경계", "## 가 {at=1,1}\n\n글\n\n::: {.page-break}\n:::\n\n## 나 {at=1,1}\n\n글\n", "",
			[]string{`<h2 id="나" data-at="1,1"`}, nil},
		{"page-break 제목은 경계(타일 아님)", "## 가 {at=1,1}\n\n글\n\n## 끊기 {.page-break}\n\n## 나 {at=1,1}\n\n글\n", "",
			[]string{`<h2 id="나" data-at="1,1"`}, nil},
		// 다른 key=value 는 그대로
		{"다른 속성은 그대로", "## 가 {at=9,9 tone=up}\n\n글\n", "dashboard-place@5", []string{`data-tone="up"`}, []string{"data-at"}},
	}
	for _, c := range cases {
		r := render(t, fm+c.body)
		if got := boardWarns(r.Warnings); got != c.warns {
			t.Errorf("%s: 경고 %s, 기대 %q: %+v", c.name, got, c.warns, r.Warnings)
		}
		for _, s := range c.has {
			if !strings.Contains(r.HTML, s) {
				t.Errorf("%s: HTML 에 %q 없음:\n%s", c.name, s, r.HTML)
			}
		}
		for _, s := range c.not {
			if strings.Contains(r.HTML, s) {
				t.Errorf("%s: HTML 에 %q 가 남음:\n%s", c.name, s, r.HTML)
			}
		}
	}
	// 경고 문구는 무엇을 버렸는지 말한다
	r := render(t, fm+"## 가 {at=5,1}\n\n글\n")
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0].Message, "at=5,1") || !strings.Contains(r.Warnings[0].Message, "4열") {
		t.Errorf("문구: %+v", r.Warnings)
	}
	// 폴더 값 cols 도 격자 폭이다, 기본 cols 는 4
	r = render(t, "---\ndashboard: true\n---\n\n## 가 {at=6,1}\n\n글\n", Options{FolderMeta: []FolderMeta{{Rel: "_metadata.yml", YAML: []byte("dashboard: {cols: 6}\n")}}})
	if len(r.Warnings) != 0 {
		t.Errorf("폴더 cols 6: %+v", r.Warnings)
	}
	r = render(t, "---\ndashboard: true\n---\n\n## 가 {at=5,1}\n\n글\n")
	if boardWarns(r.Warnings) != "dashboard-place@5" {
		t.Errorf("기본 cols 4: %+v", r.Warnings)
	}
	// dashboard.level 이 타일 단계를 정한다 — level 3 이면 ## 는 구역 제목 줄
	r = render(t, "---\ndashboard: {level: 3}\n---\n\n## 구역 {at=1,1}\n\n### 가 {at=1,1}\n\n글\n")
	if boardWarns(r.Warnings) != "dashboard-place@5" || !strings.Contains(r.HTML, `<h3 id="가" data-at="1,1"`) {
		t.Errorf("level 3: %+v\n%s", r.Warnings, r.HTML)
	}
}

// 쪽·슬라이드 문서(dashboard: 없음)에서 at·span 은 글자로 남지 않고 경고도 없다 — 보이지 않는 data-* 다.
func TestBoardPlaceOtherViews(t *testing.T) {
	for _, fm := range []string{"", "---\nslides: true\n---\n\n"} {
		r := render(t, fm+"# 제목\n\n## 매출 {at=3,1 span=2x2}\n\n글\n\n## 틀린 {at=0 span=9X9}\n\n글\n")
		if len(r.Warnings) != 0 {
			t.Errorf("%q: 경고 %+v", fm, r.Warnings)
		}
		if strings.Contains(r.HTML, "{") || strings.Contains(r.HTML, "at=3,1") || strings.Contains(r.HTML, "span=2x2") || !strings.Contains(r.HTML, ">매출</h2>") {
			t.Errorf("%q: 속성이 글자로 남음:\n%s", fm, r.HTML)
		}
	}
}
