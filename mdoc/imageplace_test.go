package mdoc

import (
	"strings"
	"testing"
)

// 슬라이드 그림 배치(#550): 이미지 속성 .fit·.fill·.left·.right·split 과 Deckset alt 지시. 결과는 class·data-split 뿐이고
// style 은 여전히 width 에서만 나온다.
func TestImagePlace(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string // 결과 HTML 에 들어 있어야 할 조각
		not       []string // 들어 있으면 안 되는 조각
		warns     int
	}{
		{"fit", "![그림](a.png){.fit}\n", []string{`<img src="a.png" alt="그림" class="img-fit">`}, nil, 0},
		{"fit+width", "![](a.png){.fit width=50%}\n", []string{`class="img-fit" style="width:50%"`}, nil, 0},
		{"right", "![](a.png){.right}\n", []string{`class="img-right"`}, []string{"data-split"}, 0},
		{"left fill split", "![](a.png){.left split=1:2 .fill}\n", []string{`class="img-left img-fill" data-split="1:2"`}, nil, 0},
		{"split 1:1 은 내지 않음", "![](a.png){.left split=1:1}\n", []string{`class="img-left"`}, []string{"data-split"}, 0},
		{"split 틀림", "![](a.png){.right split=1:4}\n", []string{`class="img-right"`}, []string{"data-split"}, 1},
		{"split 만", "![](a.png){split=1:2}\n", nil, []string{"data-split", "class="}, 1},
		{"fill 만", "![](a.png){.fill}\n", nil, []string{"class="}, 1},
		{"fit fill", "![](a.png){.fit .fill}\n", []string{`class="img-fit"`}, []string{"img-fill"}, 1},
		{"left right", "![](a.png){.left .right}\n", []string{`class="img-left"`}, []string{"img-right"}, 1},
		{"left fit", "![](a.png){.fit .left}\n", []string{`class="img-left"`}, []string{"img-fit"}, 0},
		{"모르는 클래스", "![](a.png){.fit .big}\n", []string{`class="img-fit"`}, []string{"big"}, 1},
		{"모르는 키", "![](a.png){width=10% height=20%}\n", []string{`style="width:10%"`}, []string{"height", "class="}, 1},
		{"full", "![](a.png){.full}\n", []string{`class="img-full"`}, []string{"img-fill"}, 0},
		{"full fill", "![](a.png){.full .fill}\n", []string{`class="img-full img-fill"`}, nil, 0},
		{"full fit", "![](a.png){.fit .full}\n", []string{`class="img-full"`}, []string{"img-fit"}, 0},
		{"full left 는 먼저 적은 쪽", "![](a.png){.full .left}\n", []string{`class="img-full"`}, []string{"img-left"}, 1},
		{"full split", "![](a.png){.full split=1:2}\n", []string{`class="img-full"`}, []string{"data-split"}, 1},
		{"속성 full 이 alt 지시를 덮음", "![left](a.png){.full}\n", []string{`alt="" class="img-full"`}, []string{"img-fill", "img-left"}, 0},
		{"id", "![](a.png){#x .left}\n", []string{`class="img-left"`}, []string{`id="x"`}, 1},
		{"width 만은 그대로", "![](a.png){width=40%}\n", []string{`<img src="a.png" alt="" style="width:40%">`}, nil, 0},
		// Deckset alt 지시 — alt 전체가 지시 낱말일 때만, alt 는 비운다
		{"deckset left", "![left](a.png)\n", []string{`alt="" class="img-left img-fill"`}, nil, 0},
		{"deckset right", "![right](a.png)\n", []string{`alt="" class="img-right img-fill"`}, nil, 0},
		{"deckset left fit", "![left fit](a.png)\n", []string{`alt="" class="img-left"`}, []string{"img-fill"}, 0},
		{"deckset fit right", "![fit right](a.png)\n", []string{`alt="" class="img-right"`}, nil, 0},
		{"deckset inline", "![inline](a.png)\n", []string{`<img src="a.png" alt="">`}, []string{"class="}, 0},
		{"deckset fit 단독은 배경(#569)", "![fit](a.png)\n", []string{`<img src="a.png" alt="" class="img-full">`}, nil, 0},
		{"deckset 대문자 Fit 은 대체 글", "![Fit](a.png)\n", []string{`alt="Fit"`}, []string{"class=", "data-slide-bg"}, 0},
		{"deckset fit + fill 속성", "![fit](a.png){.fill}\n", []string{`alt="" class="img-full img-fill"`}, nil, 0},
		{"deckset fit filtered", "![fit filtered](a.png)\n", []string{`<img src="a.png" alt="">`}, []string{"class="}, 1},
		{"deckset filtered", "![left filtered](a.png)\n", []string{`<img src="a.png" alt="">`}, []string{"class="}, 1},
		{"deckset 50%", "![50%](a.png)\n", []string{`<img src="a.png" alt="">`}, nil, 1},
		{"보통 alt", "![left side](a.png)\n", []string{`alt="left side"`}, []string{"class="}, 0},
		{"대문자는 지시 아님", "![Left](a.png)\n", []string{`alt="Left"`}, []string{"class="}, 0},
		{"속성이 alt 를 덮음", "![left](a.png){.right}\n", []string{`alt="" class="img-right"`}, []string{"img-fill"}, 0},
		{"alt 지시 + fill 속성", "![left fit](a.png){.fill}\n", []string{`alt="" class="img-left img-fill"`}, nil, 0},
		{"alt 지시 + fit 속성", "![left](a.png){.fit}\n", []string{`alt="" class="img-fit"`}, []string{"img-left"}, 0},
	}
	for _, c := range cases {
		r := render(t, c.src)
		for _, w := range c.want {
			if !strings.Contains(r.HTML, w) {
				t.Errorf("%s: %q 없음\n%s", c.name, w, r.HTML)
			}
		}
		for _, w := range c.not {
			if strings.Contains(r.HTML, w) {
				t.Errorf("%s: %q 있음\n%s", c.name, w, r.HTML)
			}
		}
		if len(r.Warnings) != c.warns {
			t.Errorf("%s: 경고 %d개, 기대 %d: %+v", c.name, len(r.Warnings), c.warns, r.Warnings)
		}
		for _, w := range r.Warnings {
			if w.Code != WarnBadAttribute || w.Line != 1 {
				t.Errorf("%s: 경고 %+v", c.name, w)
			}
		}
	}
}

// 분할 그림의 패널 판정(#550): 맨 위 블록이 그림 하나뿐인 줄(그림 문단·그림 하나만 감싼 링크 문단)만 패널 후보이고 슬라이드당 하나.
// 그 밖의 자리는 경고하고 제자리 .fit. 슬라이드 경계는 slides.js groups 와 같다(selftest 11b-B4 ⑥ 이 화면 묶음으로 대조한다).
func TestImagePlacePerSlide(t *testing.T) {
	const notSole, second = "그림 하나뿐인 줄에서만", "슬라이드당 하나"
	cases := []struct {
		name, src string
		panels    int      // data-slide-panel 을 받은 맨 위 블록 수
		warns     []string // 경고 문구 조각(차례대로) — 수가 곧 .fit 으로 바뀐 수
	}{
		{"그림 문단", "## 가\n\n![](a.png){.left}\n", 1, nil},
		{"Deckset 그림 문단", "## 가\n\n![right fit](a.png)\n", 1, nil},
		{"캡션 문단과 함께", "## 가\n\n![](a.png){.left}\n\nFigure: 캡션\n", 1, nil},
		{"링크 문단", "## 가\n\n[![설명](a.png){.left}](https://example.com)\n", 1, nil},
		{"막은 링크 문단", "## 가\n\n[![설명](a.png){.left}](javascript:alert(1))\n", 1, nil},
		{"한 슬라이드 둘", "## 가\n\n![](a.png){.left}\n\n![](b.png){.right}\n", 1, []string{second}},
		{"다른 슬라이드", "## 가\n\n![](a.png){.left}\n\n## 나\n\n![](b.png){.right}\n", 2, nil},
		{"가로줄", "## 가\n\n![](a.png){.left}\n\n---\n\n![](b.png){.right}\n", 2, nil},
		{"쪽 나눔", "## 가\n\n![](a.png){.left}\n\n\\newpage\n\n![](b.png){.right}\n", 2, nil},
		{"하위 제목은 같은 슬라이드", "## 가\n\n![](a.png){.left}\n\n### 작은\n\n![](b.png){.right}\n\n## 나\n\n글\n", 1, []string{second}},
		{"셋", "## 가\n\n![left](a.png)\n\n![](b.png){.right}\n\n![right fit](c.png)\n", 1, []string{second, second}},
		{"slides.level", "---\nslides:\n  level: 1\n---\n\n# 가\n\n## 작\n\n![](a.png){.left}\n\n## 작2\n\n![](b.png){.left}\n", 1, []string{second}},
		{"page-break 컨테이너는 경계", "## 가\n\n![](a.png){.left}\n\n:::page-break\n:::\n\n![](b.png){.right}\n", 2, nil},
		{"page-break 제목은 경계", "## 가\n\n![](a.png){.left}\n\n#### 작 {.page-break}\n\n![](b.png){.right}\n", 2, nil},
		{"목차 자리는 경계 아님", "## 가\n\n![](a.png){.left}\n\n[TOC]\n\n![](b.png){.right}\n", 1, []string{second}},
		// 그림 하나뿐인 줄이 아니면 경고하고 제자리 .fit — 슬라이드의 패널 자리를 쓰지 않는다
		{"목록", "## 가\n\n- ![](a.png){.left}\n\n![](b.png){.right}\n", 1, []string{notSole}},
		{"인용", "## 가\n\n> ![](a.png){.left}\n", 0, []string{notSole}},
		{"다단", "## 가\n\n::::columns\n:::column\n![](a.png){.left}\n:::\n::::\n", 0, []string{notSole}},
		{"상자", "## 가\n\n:::box\n![](a.png){.right}\n:::\n", 0, []string{notSole}},
		{"체크박스 + 링크 그림(리뷰 2회차 [1])", "## 확인\n\n- [x] [![](a.png){.left}](https://example.com)\n\n  검토 완료 설명\n", 0, []string{notSole}},
		{"글과 섞인 문단", "## 가\n\n글 ![](a.png){.left} 과 ![](b.png){.right}\n", 0, []string{notSole, notSole}},
		{"링크에 글이 섞임", "## 가\n\n[![](a.png){.left} 설명](https://example.com)\n", 0, []string{notSole}},
		{"그림 둘 문단", "## 가\n\n![](a.png){.left} ![](b.png)\n", 0, []string{notSole}},
		{"노트", "## 가\n\n:::notes\n![](n.png){.left}\n:::\n\n![](a.png){.right}\n", 1, []string{notSole}},
		{"다단 칸 안 노트", "## 가\n\n::::columns\n:::column\n:::notes\n![](n.png){.left}\n:::\n:::\n::::\n", 0, []string{notSole}},
		{"본문 제목 안", "## 가\n\n### 작 ![](b.png){.right} 끝\n", 0, []string{notSole}},
		{"각주", "## 가\n\n글[^1]\n\n[^1]: ![](b.png){.left}\n", 0, []string{notSole}},
		{"page-break 컨테이너 안", "## 가\n\n:::page-break\n![](b.png){.left}\n:::\n", 0, []string{notSole}},
	}
	for _, c := range cases {
		r := render(t, c.src)
		panels := strings.Count(r.HTML, `data-slide-panel="1"`)
		fits := strings.Count(r.HTML, "img-fit")
		var ws []Warning // 배치 경고만 — 막은 링크의 unsafe-url 경고 같은 다른 경고는 세지 않는다
		for _, w := range r.Warnings {
			if w.Code == WarnBadAttribute {
				ws = append(ws, w)
			}
		}
		ok := panels == c.panels && fits == len(c.warns) && len(ws) == len(c.warns)
		for i := 0; ok && i < len(c.warns); i++ {
			ok = strings.Contains(ws[i].Message, c.warns[i])
		}
		if !ok {
			t.Errorf("%s: 패널 %d·fit %d, 기대 패널 %d·경고 %q\n%s %+v", c.name, panels, fits, c.panels, c.warns, r.HTML, r.Warnings)
		}
		// 패널 후보 블록의 그림은 분할 class 를 갖는다
		if c.panels > 0 && !strings.Contains(r.HTML, "img-left") && !strings.Contains(r.HTML, "img-right") {
			t.Errorf("%s: 패널 그림 class 없음\n%s", c.name, r.HTML)
		}
	}
	// 막은 링크 단독 줄(리뷰 3회차): <a> 는 빠지고 표시된 문단에 그림만 남는다 — slides.js 가 이 문단을 통째로 패널로 옮긴다
	if h := render(t, "## 가\n\n[![설명](a.png){.left}](javascript:alert(1))\n").HTML; !strings.Contains(h, `<p data-slide-panel="1" data-line="3"><img src="a.png" alt="설명" class="img-left"></p>`) || strings.Contains(h, "<a ") {
		t.Errorf("막은 링크 단독 줄\n%s", h)
	}
	// 체크박스 재현 입력은 체크박스·링크·설명을 그대로 낸다(그림만 .fit)
	r := render(t, "## 확인\n\n- [x] [![](a.png){.left}](https://example.com)\n\n  검토 완료 설명\n")
	for _, w := range []string{`type="checkbox"`, `checked`, `<a href="https://example.com"><img src="a.png" alt="" class="img-fit"></a>`, "검토 완료 설명"} {
		if !strings.Contains(r.HTML, w) {
			t.Errorf("체크박스 입력: %q 없음\n%s", w, r.HTML)
		}
	}
}

// 배경 그림(.full, #569)의 판정: 분할 패널과 같은 「단독 줄만」 규칙이고, 그림 자리(패널·배경)는 합쳐 슬라이드당 하나 — 먼저 적은 쪽.
// 가로 배치(fit-row)는 맨 위 문단이 .fit 그림 2장 이상(링크로 감싼 그림 포함)만으로 되어 있을 때다.
func TestSlideBackgroundAndRow(t *testing.T) {
	const notSole, second, mixed, row = "그림 하나뿐인 줄에서만 배경", "배경 그림(.full)은 슬라이드당 하나", "합쳐 슬라이드당 하나", "가로 배치는"
	cases := []struct {
		name, src     string
		bgs, panels   int
		rows          int
		warns         []string // 배치 경고 조각(차례대로)
		fits          int      // 결과의 img-fit 수
		want, notWant []string
	}{
		{"그림 문단", "## 가\n\n![](a.png){.full}\n\n- 글\n", 1, 0, 0, nil, 0, []string{`<figure data-slide-bg="1" data-line="3">`}, nil},
		{"cover", "## 가\n\n![](a.png){.full .fill}\n", 1, 0, 0, nil, 0, []string{`class="img-full img-fill"`}, nil},
		{"Deckset fit", "## 가\n\n![fit](a.png)\n", 1, 0, 0, nil, 0, []string{`alt="" class="img-full"`}, nil},
		{"대문자 Fit 은 보통 그림", "## 가\n\n![Fit](a.png)\n", 0, 0, 0, nil, 0, []string{`alt="Fit"`}, []string{"img-full"}},
		{"캡션 문단과 함께", "## 가\n\n![](a.png){.full}\n\nFigure: 캡션\n", 1, 0, 0, nil, 0, []string{"<figcaption>캡션</figcaption>"}, nil},
		{"링크 문단", "## 가\n\n[![](a.png){.full}](https://example.com)\n", 1, 0, 0, nil, 0, []string{`<p data-slide-bg="1" data-line="3"><a href="https://example.com">`}, nil},
		{"제목 없는 장(가로줄 뒤)", "## 가\n\n글\n\n---\n\n![](a.png){.full}\n", 1, 0, 0, nil, 0, nil, nil},
		{"둘째 배경", "## 가\n\n![](a.png){.full}\n\n![](b.png){.full .fill}\n", 1, 0, 0, []string{second}, 1, nil, nil},
		{"다른 슬라이드", "## 가\n\n![](a.png){.full}\n\n## 나\n\n![](b.png){.full}\n", 2, 0, 0, nil, 0, nil, nil},
		{"분할 뒤 배경", "## 가\n\n![](a.png){.left}\n\n![](b.png){.full}\n", 0, 1, 0, []string{mixed}, 1, nil, nil},
		{"배경 뒤 분할", "## 가\n\n![](a.png){.full}\n\n![](b.png){.right}\n", 1, 0, 0, []string{mixed}, 1, nil, nil},
		{"목록 안", "## 가\n\n- ![](a.png){.full}\n", 0, 0, 0, []string{notSole}, 1, nil, nil},
		{"글과 섞인 문단", "## 가\n\n글 ![](a.png){.full}\n", 0, 0, 0, []string{notSole}, 1, nil, nil},
		{"다단 칸 안", "## 가\n\n::::columns\n:::column\n![](a.png){.full}\n:::\n::::\n", 0, 0, 0, []string{notSole}, 1, nil, nil},
		{"노트 안", "## 가\n\n:::notes\n![](n.png){.full}\n:::\n\n![](a.png){.full}\n", 1, 0, 0, []string{notSole}, 1, nil, nil},
		{"각주 안", "## 가\n\n글[^1]\n\n[^1]: ![](b.png){.full}\n", 0, 0, 0, []string{notSole}, 1, nil, nil},
		// 가로 배치
		{"가로 둘", "## 가\n\n![](a.png){.fit} ![](b.png){.fit}\n", 0, 0, 1, nil, 2, []string{`<p class="fit-row" data-line="3">`}, nil},
		{"가로 셋 줄바꿈·링크·캡션", "## 가\n\n![](a.png \"가\"){.fit}\n[![](b.png){.fit}](https://example.com)\n![](c.png \"다\"){.fit}\n", 0, 0, 1, nil, 3, []string{`title="가"`, `<a href="https://example.com"><img src="b.png" alt="" class="img-fit"></a>`}, nil},
		{"가로 강제 줄바꿈", "## 가\n\n![](a.png){.fit}  \n![](b.png){.fit}\n", 0, 0, 1, nil, 2, nil, nil},
		{"Deckset 은 가로 아님(배경 둘)", "## 가\n\n![fit](a.png) ![fit](b.png)\n", 0, 0, 0, []string{notSole, notSole}, 2, nil, []string{"fit-row"}},
		{"글이 섞임", "## 가\n\n![](a.png){.fit} 글 ![](b.png){.fit}\n", 0, 0, 0, []string{row}, 2, nil, []string{"fit-row"}},
		{"보통 그림이 섞임", "## 가\n\n![](a.png){.fit} ![](b.png){.fit} ![](c.png)\n", 0, 0, 0, []string{row}, 2, nil, []string{"fit-row"}},
		{"분할 그림이 섞임", "## 가\n\n![](a.png){.fit} ![](b.png){.fit} ![](c.png){.left}\n", 0, 0, 0, []string{row, "그림 하나뿐인 줄에서만 패널"}, 3, nil, []string{"fit-row"}},
		{"배경 그림이 섞임", "## 가\n\n![](a.png){.fit} ![](b.png){.fit} ![](c.png){.full}\n", 0, 0, 0, []string{row, notSole}, 3, nil, []string{"fit-row"}},
		{"링크에 글", "## 가\n\n![](a.png){.fit} [![](b.png){.fit} 설명](https://example.com)\n", 0, 0, 0, []string{row}, 2, nil, []string{"fit-row"}},
		{"한 장은 가로 아님", "## 가\n\n[![](a.png){.fit}](https://example.com) 글\n", 0, 0, 0, nil, 1, nil, []string{"fit-row"}},
		{"다단 칸 안은 가로 아님", "## 가\n\n::::columns\n:::column\n![](a.png){.fit} ![](b.png){.fit}\n:::\n::::\n", 0, 0, 0, nil, 2, nil, []string{"fit-row"}},
		{"가로 + 배경 한 장", "## 가\n\n![](bg.png){.full .fill}\n\n![](a.png){.fit} ![](b.png){.fit}\n", 1, 0, 1, nil, 2, nil, nil},
	}
	for _, c := range cases {
		r := render(t, c.src)
		var ws []Warning
		for _, w := range r.Warnings {
			if w.Code == WarnBadAttribute {
				ws = append(ws, w)
			}
		}
		bgs := strings.Count(r.HTML, `data-slide-bg="1"`)
		panels := strings.Count(r.HTML, `data-slide-panel="1"`)
		rows := strings.Count(r.HTML, `class="fit-row"`)
		fits := strings.Count(r.HTML, `class="img-fit"`)
		ok := bgs == c.bgs && panels == c.panels && rows == c.rows && fits == c.fits && len(ws) == len(c.warns)
		for i := 0; ok && i < len(c.warns); i++ {
			ok = strings.Contains(ws[i].Message, c.warns[i])
		}
		for _, w := range c.want {
			ok = ok && strings.Contains(r.HTML, w)
		}
		for _, w := range c.notWant {
			ok = ok && !strings.Contains(r.HTML, w)
		}
		if !ok {
			t.Errorf("%s: 배경 %d·패널 %d·가로 %d·fit %d, 기대 %d·%d·%d·%d, 경고 %q\n%s %+v", c.name, bgs, panels, rows, fits, c.bgs, c.panels, c.rows, c.fits, c.warns, r.HTML, r.Warnings)
		}
	}
}
