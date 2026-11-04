package mdoc

import (
	"fmt"
	"strings"
	"testing"
)

// 보드용 표기(itda-work/itda-hyve#362): frontmatter dashboard:, 표준 어휘 kpi 의 구조 게이트.

func TestDashboardMeta(t *testing.T) {
	cases := []struct {
		fm    string
		want  *DashboardMeta
		codes []string
	}{
		{"", nil, nil},
		{"dashboard: true", &DashboardMeta{}, nil},
		{"dashboard:", &DashboardMeta{}, nil},
		{"dashboard: false", nil, nil},
		{"dashboard:\n  cols: 3\n  rows: 2\n  ratio: \"4:3\"\n  level: 3", &DashboardMeta{Cols: 3, Rows: 2, Ratio: "4:3", Level: 3}, nil},
		{"dashboard: {cols: 4, rows: 3, aspect: 16/9}", &DashboardMeta{Cols: 4, Rows: 3, Ratio: "16:9"}, nil},
		{"dashboard:\n  cols: 7\n  rows: 1\n  ratio: \"21:9\"\n  level: 0\n  size: big", &DashboardMeta{},
			[]string{WarnFrontmatterValue, WarnFrontmatterValue, WarnFrontmatterValue, WarnFrontmatterValue, WarnFrontmatterValue}},
		// 실시간 소스는 ② — 키가 둘이어도 경고 한 줄, 나머지 설정은 쓴다
		{"dashboard:\n  cols: 2\n  refresh: 30s\n  sources:\n    매출: { file: ./sales.csv }", &DashboardMeta{Cols: 2}, []string{WarnDashboardSources}},
		// 회사 메타데이터로 쓰던 값(옛 판은 Extra 에 두었다) — 새 판에서는 틀린 값이라 없는 것과 같다
		{"dashboard: 영업팀", nil, []string{WarnFrontmatterValue}},
		{"dashboard: [a, b]", nil, []string{WarnFrontmatterValue}},
		// 소수는 정수가 아니다 — yaml.v3 Decode(&int) 의 절삭(6.9 → 6)을 받지 않는다(yamlInt). 상한 직후 소수·범위 안 소수·따옴표 숫자
		{"dashboard: {cols: 6.9, rows: 4.9, level: 2.9}", &DashboardMeta{},
			[]string{WarnFrontmatterValue, WarnFrontmatterValue, WarnFrontmatterValue}},
		{"dashboard: {cols: 3.0, rows: 2.5, level: \"2\"}", &DashboardMeta{},
			[]string{WarnFrontmatterValue, WarnFrontmatterValue, WarnFrontmatterValue}},
		{"dashboard: {cols: 6, rows: 4, level: 6}", &DashboardMeta{Cols: 6, Rows: 4, Level: 6}, nil},
	}
	for _, c := range cases {
		src := "---\ntitle: t\n" + c.fm + "\n---\n\n본문\n"
		r := render(t, src)
		got := r.Meta.Dashboard
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%q → %+v, 기대 %+v", c.fm, got, c.want)
		}
		var codes []string
		for _, w := range r.Warnings {
			codes = append(codes, w.Code)
		}
		if strings.Join(codes, ",") != strings.Join(c.codes, ",") {
			t.Errorf("%q 경고 %v, 기대 %v: %+v", c.fm, codes, c.codes, r.Warnings)
		}
		if _, ok := r.Meta.Field("dashboard"); ok {
			t.Errorf("%q: dashboard 가 Extra 에 남음", c.fm)
		}
	}
}

func TestKPI(t *testing.T) {
	dl := ":::kpi\n매출\n: 12.4억\n: ▲ 8% 전월 대비\n\n이탈률\n: 3.1%\n: ▼ 0.4%p\n\n방문\n: 1,204\n: 지난주와 같음\n:::\n"
	r := render(t, dl)
	want := `<div class="kpi kpi-tiles" data-line="1">
<dl>
<div class="kpi-item">
<dt>매출</dt>
`
	if !strings.HasPrefix(r.HTML, want) || len(r.Warnings) != 0 {
		t.Fatalf("정의 목록 꼴\n%s %+v", r.HTML, r.Warnings)
	}
	if strings.Count(r.HTML, `class="kpi-item"`) != 3 || !strings.Contains(r.HTML, `<dd class="kpi-up">▲ 8% 전월 대비</dd>`) ||
		!strings.Contains(r.HTML, `<dd class="kpi-down">▼ 0.4%p</dd>`) || !strings.Contains(r.HTML, "<dd>지난주와 같음</dd>") {
		t.Errorf("정의 목록 꼴 항목·증감\n%s", r.HTML)
	}

	tbl := ":::kpi\n| 지표 | 값 | 증감 |\n|---|--:|---|\n| **매출** | 12.4억 | +8% |\n| 이탈률 | 3.1% | −0.4%p |\n| 방문 | 1,204 | |\n:::\n"
	r = render(t, tbl)
	if strings.Contains(r.HTML, "<table") || strings.Count(r.HTML, `class="kpi-item"`) != 3 || len(r.Warnings) != 0 ||
		!strings.Contains(r.HTML, "<dt><strong>매출</strong></dt>") || !strings.Contains(r.HTML, `<dd class="kpi-up">+8%</dd>`) ||
		!strings.Contains(r.HTML, `<dd class="kpi-down">−0.4%p</dd>`) || !strings.Contains(r.HTML, "<dt>방문</dt>\n<dd>1,204</dd>\n</div>") {
		t.Errorf("3열 표 꼴\n%s %+v", r.HTML, r.Warnings)
	}

	// 둘 다: 정의 목록이 KPI, 표는 그대로 + 경고
	r = render(t, ":::kpi\n매출\n: 12억\n\n| a | b | c |\n|---|---|---|\n| 1 | 2 | 3 |\n:::\n")
	if !strings.Contains(r.HTML, `class="kpi-item"`) || !strings.Contains(r.HTML, "<table") || len(r.Warnings) != 1 || r.Warnings[0].Code != WarnKPIAmbiguous {
		t.Errorf("둘 다\n%s %+v", r.HTML, r.Warnings)
	}

	// 둘 다 없음: 종전 그대로(바이트까지) — 보드 문서에서만 경고
	plain := ":::kpi\n회사 어휘로 쓴 문단\n\n| a | b |\n|---|---|\n| 1 | 2 |\n:::\n"
	r = render(t, plain)
	if strings.Contains(r.HTML, "kpi-item") || strings.Contains(r.HTML, "kpi-tiles") || len(r.Warnings) != 0 || !strings.HasPrefix(r.HTML, `<div class="kpi" data-line="1">`+"\n<p>회사 어휘로 쓴 문단</p>") {
		t.Errorf("구조 없음\n%s %+v", r.HTML, r.Warnings)
	}
	r = render(t, "---\ndashboard: true\n---\n"+plain)
	if len(r.Warnings) != 1 || r.Warnings[0].Code != WarnKPIEmpty || r.Warnings[0].Line != 4 {
		t.Errorf("보드 문서의 빈 kpi: %+v", r.Warnings)
	}
	// 클래스로만 단 kpi 는 이름이 아니라 손대지 않는다(chart 와 같은 규칙)
	if r := render(t, "::: {.kpi}\n매출\n: 12억\n:::\n"); strings.Contains(r.HTML, "kpi-item") {
		t.Errorf("{.kpi}\n%s", r.HTML)
	}
}

// 폴더 _metadata.yml 의 dashboard(itda-work/itda-hyve#362, 사용자 결정 2026-11-04 — slides: 와 똑같이): 보드로 열지는 문서의
// dashboard: 가 정하고(없으면 folder-dashboard-unused 안내), 문서가 true 면 폴더 맵이 하위 설정을 채우고, 문서 맵의 키가 이기고,
// false 면 폴더 값을 쓰지 않는다. 폴더 값의 경고는 slides 폴더 값처럼 Line 0·「<rel> <n>줄: 」, 문서 값의 경고는 문서 줄.
func TestDashboardFromFolder(t *testing.T) {
	fold := func(y string) Options {
		return Options{FolderMeta: []FolderMeta{{Rel: "_metadata.yml", YAML: []byte(y)}}}
	}
	codes := func(ws []Warning) string {
		var out []string
		for _, w := range ws {
			out = append(out, fmt.Sprintf("%s@%d", w.Code, w.Line))
		}
		return strings.Join(out, ",")
	}
	// 문서에 dashboard 가 없다 — 보드가 아니고 안내 하나(틀린 폴더 값 rows: 9 는 쓰지 않으니 값 경고도 없다)
	r := render(t, "---\ntitle: t\n---\n\n본문\n", fold("dashboard: {cols: 3, rows: 9}\norganization: 잇다\n"))
	if r.Meta.Dashboard != nil || r.Meta.Organization != "잇다" || codes(r.Warnings) != WarnFolderDashboardUnused+"@0" ||
		!strings.HasPrefix(r.Warnings[0].Message, "_metadata.yml: ") || !strings.Contains(r.Warnings[0].Message, "보드로 열리지 않음") {
		t.Errorf("문서에 dashboard 없음: %+v %+v", r.Meta.Dashboard, r.Warnings)
	}
	// 문서 true — 폴더 맵이 채운다, 틀린 폴더 값은 폴더 파일 경로·줄로
	r = render(t, "---\ndashboard: true\n---\n\n본문\n", fold("dashboard: {cols: 3, rows: 9}\n"))
	if r.Meta.Dashboard == nil || *r.Meta.Dashboard != (DashboardMeta{Cols: 3}) || codes(r.Warnings) != WarnFrontmatterValue+"@0" ||
		!strings.HasPrefix(r.Warnings[0].Message, "_metadata.yml 1줄: frontmatter dashboard.rows") {
		t.Errorf("문서 true + 폴더 맵: %+v %+v", r.Meta.Dashboard, r.Warnings)
	}
	// 빈 값도 true 와 같다, 폴더 refresh 는 폴더 위치로 dashboard-sources
	r = render(t, "---\ndashboard:\n---\n\n본문\n", fold("dashboard:\n  rows: 2\n  refresh: 60\n"))
	if r.Meta.Dashboard == nil || r.Meta.Dashboard.Rows != 2 || codes(r.Warnings) != WarnDashboardSources+"@0" ||
		!strings.HasPrefix(r.Warnings[0].Message, "_metadata.yml 3줄: ") {
		t.Errorf("문서 빈 값 + 폴더 refresh: %+v %+v", r.Meta.Dashboard, r.Warnings)
	}
	// 문서 맵 — 하위 키 단위로 얹고 문서 키가 이긴다(별칭 aspect ↔ ratio 도 같은 키로 견준다)
	r = render(t, "---\ndashboard: {rows: 2, ratio: \"16:9\"}\n---\n\n본문\n", fold("dashboard: {cols: 3, rows: 4, aspect: \"4:3\"}\n"))
	if r.Meta.Dashboard == nil || *r.Meta.Dashboard != (DashboardMeta{Cols: 3, Rows: 2, Ratio: "16:9"}) || len(r.Warnings) != 0 {
		t.Errorf("문서 맵 + 폴더 맵: %+v %+v", r.Meta.Dashboard, r.Warnings)
	}
	// 문서 false — 폴더 값을 쓰지 않는다(경고 없음)
	r = render(t, "---\ndashboard: false\n---\n\n본문\n", fold("dashboard: {cols: 3}\n"))
	if r.Meta.Dashboard != nil || len(r.Warnings) != 0 {
		t.Errorf("문서 false: %+v %+v", r.Meta.Dashboard, r.Warnings)
	}
	// 문서의 틀린 값은 폴더가 있어도 문서 줄로
	r = render(t, "---\ndashboard: {cols: 9}\n---\n\n본문\n", fold("dashboard: {rows: 3}\n"))
	if codes(r.Warnings) != WarnFrontmatterValue+"@2" || r.Meta.Dashboard == nil || r.Meta.Dashboard.Rows != 3 {
		t.Errorf("문서의 틀린 값: %+v %+v", r.Meta.Dashboard, r.Warnings)
	}
	// 편집 화면 풀: 문서가 보드면 폴더에서 온 하위 키의 출처가 보인다
	_, keys, _ := ResolveFrontmatter([]byte("---\ndashboard: true\n---\n"), []FolderMeta{{Rel: "_metadata.yml", YAML: []byte("dashboard: {cols: 3}\n")}})
	if k := keys["dashboard.cols"]; k.FromRel != "_metadata.yml" || k.Value != "3" || !k.Applied {
		t.Errorf("편집 화면 풀: %+v", k)
	}
}

// dashboard 안 모르는 키의 오타 짐작(typo.go dashboardTypoHint): 가까우면 정본 이름을 덧붙인다.
func TestDashboardTypo(t *testing.T) {
	r := render(t, "---\ndashboard: {colls: 3, aspec: \"4:3\", zzz: 1}\n---\n")
	var msgs []string
	for _, w := range r.Warnings {
		msgs = append(msgs, w.Message)
	}
	got := strings.Join(msgs, "\n")
	if !strings.Contains(got, "dashboard.colls 는 모르는 키라 무시함(cols·rows·ratio·level — cols 의 오타?)") ||
		!strings.Contains(got, "dashboard.aspec 는 모르는 키라 무시함(cols·rows·ratio·level — ratio 의 오타?)") ||
		!strings.Contains(got, "dashboard.zzz 는 모르는 키라 무시함(cols·rows·ratio·level)") {
		t.Errorf("오타 짐작\n%s", got)
	}
	// 최상위 dashboard 의 오타는 frontmatter-typo(스키마 표의 최상위 키)
	if r := render(t, "---\ndashbord: true\n---\n"); len(r.Warnings) != 1 || r.Warnings[0].Code != WarnFrontmatterTypo || !strings.Contains(r.Warnings[0].Message, "dashboard 의 오타?") {
		t.Errorf("최상위 오타: %+v", r.Warnings)
	}
}
