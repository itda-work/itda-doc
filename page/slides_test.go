package page

import (
	"strings"
	"testing"
)

// 슬라이드 설정(5장): frontmatter slides: + 문서에서 정한 슬라이드 단계. 늘 채운다(화면이 언제든 슬라이드로 바꿀 수 있다).
func TestSlideView(t *testing.T) {
	ts, err := BuiltinThemes()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		src  string
		want SlideView
	}{
		{"# 장\n\n글\n", SlideView{Ratio: "16:9", Level: 1, Transition: "none"}},
		{"---\ntitle: t\nslides: true\n---\n\n# 장\n\n## 절\n\n글\n", SlideView{Present: true, Ratio: "16:9", Level: 2, Transition: "none"}},
		{"---\nslides:\n  ratio: \"4:3\"\n  level: 1\n  transition: none\n  incremental: true\n---\n\n# 장\n\n## 절\n\n글\n",
			SlideView{Present: true, Ratio: "4:3", Level: 1, Transition: "none", Incremental: true}},
		{"글만\n", SlideView{Ratio: "16:9", Level: 0, Transition: "none"}},
		// 기본은 none 이지만 고른 전환은 그대로(itda-hyve#351)
		{"---\nslides:\n  transition: fade\n---\n\n# 장\n", SlideView{Present: true, Ratio: "16:9", Level: 1, Transition: "fade"}},
		{"---\nslides:\n  transition: slide\n---\n\n# 장\n", SlideView{Present: true, Ratio: "16:9", Level: 1, Transition: "slide"}},
		// 글자 배율(itda-hyve#430): 고른 값은 그대로, 틀린 값은 0(화면의 기본 0.9)
		{"---\nslides:\n  font-scale: 0.8\n---\n\n# 장\n", SlideView{Present: true, Ratio: "16:9", Level: 1, Transition: "none", FontScale: 0.8}},
		{"---\nslides:\n  font-scale: 2\n---\n\n# 장\n", SlideView{Present: true, Ratio: "16:9", Level: 1, Transition: "none"}},
	}
	for _, c := range cases {
		v := RenderView([]byte(c.src), "", "/_doc/", ts, "", false)
		if v.Slides != c.want {
			t.Errorf("%q → %+v, 기대 %+v", c.src, v.Slides, c.want)
		}
	}
	// 제목 슬라이드에 쓸 문서 정보는 글자 값이고 테마 defaults 를 따른다
	v := RenderView([]byte("---\ntitle: <b>제목</b>\nauthor: 홍길동\ndate: 2026-10-02\n---\n\n글\n"), "", "/_doc/", ts, "mono", false)
	if v.Info["title"] != "<b>제목</b>" || v.Info["author"] != "홍길동" || v.Info["date"] != "2026-10-02" {
		t.Errorf("info %+v", v.Info)
	}
}

// 보드 설정(itda-work/itda-hyve#362): frontmatter dashboard: + 문서에서 정한 타일 단계(슬라이드와 같은 규칙). 늘 채운다.
func TestBoardView(t *testing.T) {
	ts, err := BuiltinThemes()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		src  string
		want BoardView
	}{
		{"# 장\n\n글\n", BoardView{Cols: 4, Rows: 3, Ratio: "16:9", Level: 1}},
		{"---\ndashboard: true\n---\n\n# 판\n\n## 타일\n\n글\n", BoardView{Present: true, Cols: 4, Rows: 3, Ratio: "16:9", Level: 2}},
		{"---\ndashboard: {cols: 2, rows: 4, ratio: \"4:3\", level: 3}\n---\n\n## 타일\n", BoardView{Present: true, Cols: 2, Rows: 4, Ratio: "4:3", Level: 3}},
		// slides: 와 함께면 둘 다 Present — 처음 꼴은 보드(호스트 화면이 정한다), 슬라이드로 바꿔 볼 수 있다
		{"---\nslides: true\ndashboard: true\n---\n\n## 타일\n\n글\n", BoardView{Present: true, Cols: 4, Rows: 3, Ratio: "16:9", Level: 2}},
		// 회사 메타데이터로 쓰던 값은 틀린 값이라 보드가 아니다
		{"---\ndashboard: 영업팀\n---\n\n## 타일\n\n글\n", BoardView{Cols: 4, Rows: 3, Ratio: "16:9", Level: 2}},
	}
	for _, c := range cases {
		v := RenderView([]byte(c.src), "", "/_doc/", ts, "", false)
		if v.Board != c.want {
			t.Errorf("%q → %+v, 기대 %+v", c.src, v.Board, c.want)
		}
	}
	v := RenderView([]byte("---\nslides: true\ndashboard: true\n---\n\n## 타일\n\n글\n"), "", "/_doc/", ts, "", false)
	if !v.Slides.Present || !v.Board.Present {
		t.Errorf("둘 다 Present 여야 함: %+v %+v", v.Slides, v.Board)
	}
	// Check 의 meta.dashboard(앱의 검사 도구·itda-doc check 가 같은 값을 낸다)
	res, err := Check([]byte("---\ndashboard: {cols: 9}\n---\n\n## 타일\n\n:::kpi\n글\n:::\n"), "", ts)
	if err != nil || !res.Meta.Dashboard || res.Meta.Slides {
		t.Fatalf("check %+v %v", res, err)
	}
	var codes []string
	for _, w := range res.Warnings {
		codes = append(codes, w.Code)
	}
	if strings.Join(codes, ",") != "frontmatter-value,kpi-empty" {
		t.Errorf("check 경고 %v", codes)
	}
	if res, _ := Check([]byte("---\ndashboard: 영업팀\n---\n"), "", ts); res.Meta.Dashboard {
		t.Errorf("틀린 값인데 dashboard 참: %+v", res)
	}
}
