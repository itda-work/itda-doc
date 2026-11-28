package mdoc

import (
	"strings"
	"testing"
)

// 보드 타일 잇기(itda-work/itda-hyve#1038 3단계 D): 제목 속성 to 의 검사(boardlink.go). 선을 그리는 일은 web/dashboard.js links 가 하고
// node 시험(web/testdata/board_links.cjs)이 본다.
func TestBoardLinks(t *testing.T) {
	const fm = "---\ndashboard: {cols: 4}\n---\n\n" // 본문은 5줄부터
	cases := []struct {
		name, body string
		warns      string   // 경고 코드@줄
		has, not   []string // HTML 에 있어야·없어야 하는 조각
	}{
		{"맞는 연결", "## 결제 {#pay to=settle}\n\n글\n\n## 정산 {#settle}\n\n글\n", "",
			[]string{`<h2 id="pay" data-to="settle"`}, []string{"{"}},
		{"여러 도착(적은 차례)", "## 가 {to=다,나}\n\n글\n\n## 나\n\n글\n\n## 다\n\n글\n", "",
			[]string{`<h2 id="가" data-to="다,나"`}, nil},
		{"구역 넘김", "# 위\n\n## 가 {to=나}\n\n글\n\n---\n\n# 아래\n\n## 나\n\n글\n", "",
			[]string{`<h2 id="가" data-to="나"`}, nil},
		{"저절로 붙은 id(한글·공백·기호·같은 제목)", "## 주문 {to=알림-발송,매출억-원,q3-sales,매출-1}\n\n글\n\n## 알림 발송\n\n글\n\n## 매출(억 원)\n\n글\n\n## Q3 Sales\n\n글\n\n## 매출\n\n글\n\n## 매출\n\n글\n", "",
			[]string{`data-to="알림-발송,매출억-원,q3-sales,매출-1"`, `<h2 id="매출-1"`}, nil},
		// id 가 겹칠 때(리뷰 1회차 [3] — processHeadings 의 차례 그대로): 직접 적은 id 를 먼저 등록해 자리와 상관없이 저절로 붙는 id 가 비킨다
		{"직접 적은 id 가 먼저(뒤에 있어도)", "## 다 {to=가}\n\n글\n\n## 가\n\n글\n\n## 나 {#가}\n\n글\n", "",
			[]string{`<h2 id="다" data-to="가"`, `<h2 id="가-1" data-line="9">가</h2>`, `<h2 id="가" data-line="13">나</h2>`}, nil},
		// 직접 적은 id 끼리 겹치면 둘 다 그대로(duplicate-id). 다 → same 은 남고(화면은 읽는 차례로 첫 타일로), 둘째 타일의 to=same 은 자기 자신
		{"직접 적은 id 끼리 겹침", "## 가 {#same}\n\n글\n\n## 나 {#same to=same,다}\n\n글\n\n## 다 {to=same}\n\n글\n", "duplicate-id@9,dashboard-link@9",
			[]string{`<h2 id="same" data-to="다"`, `<h2 id="다" data-to="same"`}, nil},
		{"마주 보는 연결·고리", "## 가 {to=나}\n\n글\n\n## 나 {to=가,다}\n\n글\n\n## 다 {to=가}\n\n글\n", "",
			[]string{`<h2 id="가" data-to="나"`, `<h2 id="나" data-to="가,다"`, `<h2 id="다" data-to="가"`}, nil},
		{"따옴표 값의 공백", "## 가 {to=\"나, 다\"}\n\n글\n\n## 나\n\n글\n\n## 다\n\n글\n", "", []string{`data-to="나, 다"`}, nil},
		// 깨지는 입력: 맞는 도착은 남고 틀린 도착만 버린다
		{"일부만 버림(없는 id)", "## 결제 {#pay to=settle,nope}\n\n글\n\n## 정산 {#settle}\n\n글\n", "dashboard-link@5",
			[]string{`<h2 id="pay" data-to="settle"`}, []string{"nope"}},
		{"없는 id 뿐", "## 가 {to=nope}\n\n글\n", "dashboard-link@5", nil, []string{"data-to"}},
		{"구역 제목 줄은 타일이 아님", "# 구역\n\n## 가 {to=구역}\n\n글\n", "dashboard-link@7", nil, []string{"data-to"}},
		{"타일 안 낮은 제목은 타일이 아님", "## 가 {to=작은,나}\n\n글\n\n### 작은\n\n글\n\n## 나\n\n글\n", "dashboard-link@5", []string{`data-to="나"`}, nil},
		{"상자 안 제목은 타일이 아님", "## 가 {to=안}\n\n:::box\n## 안\n\n글\n:::\n", "dashboard-link@5", nil, []string{"data-to"}},
		{"보드에 넣지 않는 제목은 타일이 아님", "## 숨김 {.pause}\n\n## 가 {to=숨김}\n\n글\n", "dashboard-link@7", nil, []string{"data-to"}},
		{"자기 자신", "## 가 {to=가,나}\n\n글\n\n## 나\n\n글\n", "dashboard-link@5", []string{`<h2 id="가" data-to="나"`}, nil},
		{"같은 도착 중복", "## 가 {to=나,나}\n\n글\n\n## 나\n\n글\n", "dashboard-link@5", []string{`<h2 id="가" data-to="나"`}, nil},
		{"빈 항목", "## 가 {to=나,,다}\n\n글\n\n## 나\n\n글\n\n## 다\n\n글\n", "dashboard-link@5", []string{`<h2 id="가" data-to="나,다"`}, nil},
		{"빈 값", "## 가 {to=}\n\n글\n", "dashboard-link@5", nil, []string{"data-to"}},
		// 타일을 여는 제목이 아닌 곳의 to 는 쓰지 않는다
		{"구역 제목 줄의 to", "# 구역 {to=가}\n\n## 가\n\n글\n", "dashboard-link@5", nil, []string{"data-to"}},
		{"타일 안 낮은 제목의 to", "## 가\n\n글\n\n### 작은 {to=가}\n\n글\n", "dashboard-link@9", nil, []string{"data-to"}},
		{"pause 제목의 to", "## 숨김 {.pause to=가}\n\n## 가\n\n글\n", "dashboard-link@5", nil, []string{"data-to"}},
		// 다른 속성(at·span·tone)은 그대로, to 를 버려도 자리는 남는다
		{"at·span 과 함께", "## 가 {at=2,1 span=2x1 to=나,없음 tone=up}\n\n글\n\n## 나\n\n글\n", "dashboard-link@5",
			[]string{`<h2 id="가" data-at="2,1" data-span="2x1" data-to="나" data-tone="up"`}, nil},
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
	// 경고 문구는 무엇을 왜 버렸는지 말한다 — 한 제목에 한 줄
	r := render(t, fm+"# 구역\n\n## 가 {to=가,없음,구역,나,나,}\n\n글\n\n## 나\n\n글\n")
	if len(r.Warnings) != 1 {
		t.Fatalf("한 줄이어야 함: %+v", r.Warnings)
	}
	for _, want := range []string{"to=가,없음,구역,나,나,", "#가(자기 자신)", "#없음(그 id 의 제목이 없음)", "#구역(타일을 여는 제목이 아님)", "#나(두 번 적음)", "빈 항목"} {
		if !strings.Contains(r.Warnings[0].Message, want) {
			t.Errorf("문구에 %q 없음: %s", want, r.Warnings[0].Message)
		}
	}
	if !strings.Contains(r.HTML, `<h2 id="가" data-to="나"`) {
		t.Errorf("맞는 도착이 남지 않음:\n%s", r.HTML)
	}
	// 머리 제목이 된 첫 높은 제목(frontmatter 제목 없음)은 구역 제목 줄과 같이 타일이 아니다
	r = render(t, "---\ndashboard: true\n---\n\n# 보드 이름\n\n## 가 {to=보드-이름}\n\n글\n")
	if boardWarns(r.Warnings) != "dashboard-link@7" {
		t.Errorf("머리 제목: %+v", r.Warnings)
	}
	// dashboard.level 이 타일 단계를 정한다 — level 3 이면 ## 는 구역 제목 줄이라 도착이 아니다
	r = render(t, "---\ndashboard: {level: 3}\n---\n\n## 구역\n\n### 가 {to=구역,나}\n\n글\n\n### 나\n\n글\n")
	if boardWarns(r.Warnings) != "dashboard-link@7" || !strings.Contains(r.HTML, `<h3 id="가" data-to="나"`) {
		t.Errorf("level 3: %+v\n%s", r.Warnings, r.HTML)
	}
}

// 보드 문서가 아니면(쪽·슬라이드) to 는 검사하지 않는다 — 경고 없이 보이지 않는 data-to 로 남는다.
func TestBoardLinksOtherViews(t *testing.T) {
	for _, fm := range []string{"", "---\nslides: true\n---\n\n"} {
		r := render(t, fm+"# 제목\n\n## 결제 {#pay to=settle,nope}\n\n글\n\n## 정산 {#settle}\n\n글\n\n### 작은 {to=}\n\n글\n")
		if len(r.Warnings) != 0 {
			t.Errorf("%q: 경고 %+v", fm, r.Warnings)
		}
		if strings.Contains(r.HTML, "{") || !strings.Contains(r.HTML, `<h2 id="pay" data-to="settle,nope"`) {
			t.Errorf("%q: 속성이 그대로 data-to 로 가지 않음:\n%s", fm, r.HTML)
		}
	}
}
