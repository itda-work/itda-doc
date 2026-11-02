package page

import (
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
		// 기본은 none 이지만 고른 전환은 그대로(#351)
		{"---\nslides:\n  transition: fade\n---\n\n# 장\n", SlideView{Present: true, Ratio: "16:9", Level: 1, Transition: "fade"}},
		{"---\nslides:\n  transition: slide\n---\n\n# 장\n", SlideView{Present: true, Ratio: "16:9", Level: 1, Transition: "slide"}},
		// 글자 배율(#430): 고른 값은 그대로, 틀린 값은 0(화면의 기본 0.9)
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
