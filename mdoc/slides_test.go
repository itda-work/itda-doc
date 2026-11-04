package mdoc

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// 발표용 표기(1.12): . . . 멈춤, :::incremental·:::notes(보통 컨테이너), frontmatter slides:, 슬라이드 단계.
func TestPause(t *testing.T) {
	r := render(t, "# 제목\n\n- 하나\n\n. . .\n\n- 둘\n")
	if !strings.Contains(r.HTML, `<div class="pause" data-line="5"></div>`) {
		t.Errorf("멈춤\n%s", r.HTML)
	}
	for _, src := range []string{"글 . . .\n", ". . . 글\n", "...\n", ". .\n", "```\n. . .\n```\n"} {
		if strings.Contains(render(t, src).HTML, `class="pause"`) {
			t.Errorf("%q 는 멈춤이 아님", src)
		}
	}
	r = render(t, ":::incremental\n- a\n- b\n:::\n\n:::notes\n발표자만 본다\n:::\n")
	if !strings.Contains(r.HTML, `<div class="incremental"`) || !strings.Contains(r.HTML, `<div class="notes"`) || len(r.Warnings) != 0 {
		t.Errorf("컨테이너\n%s %+v", r.HTML, r.Warnings)
	}
}

func TestSlidesMeta(t *testing.T) {
	cases := []struct {
		fm    string
		want  *SlidesMeta
		warns int
	}{
		{"", nil, 0},
		{"slides: true", &SlidesMeta{}, 0},
		{"slides:", &SlidesMeta{}, 0},
		{"slides: false", nil, 0},
		{"slides:\n  ratio: \"4:3\"\n  level: 2\n  transition: slide\n  incremental: true", &SlidesMeta{Ratio: "4:3", Level: 2, Transition: "slide", Incremental: true}, 0},
		{"slides:\n  ratio: 16/9", &SlidesMeta{Ratio: "16:9"}, 0},
		{"slides:\n  ratio: \"21:9\"\n  level: 9\n  transition: zoom\n  incremental: 예\n  theme: dark", &SlidesMeta{}, 5},
		{"slides: 발표", nil, 1},
		// 소수·따옴표 숫자는 정수가 아니다 — 잘라 받지 않는다(yamlInt, itda-work/itda-hyve#362)
		{"slides:\n  level: 2.9", &SlidesMeta{}, 1},
		{"slides:\n  slide-level: 6.5", &SlidesMeta{}, 1},
		{"slides:\n  level: \"2\"", &SlidesMeta{}, 1},
		{"slides: [a, b]", nil, 1},
		// 글자 배율(itda-hyve#430): 0.6~1.5 의 숫자. 경계는 받고, 밖·숫자 아님·글자·목록은 경고하고 0(기본)
		{"slides:\n  font-scale: 0.8", &SlidesMeta{FontScale: 0.8}, 0},
		{"slides:\n  font-scale: 1", &SlidesMeta{FontScale: 1}, 0},
		{"slides:\n  fontscale: 0.6\n  ratio: \"4:3\"", &SlidesMeta{FontScale: 0.6, Ratio: "4:3"}, 0},
		{"slides:\n  font_scale: 1.5", &SlidesMeta{FontScale: 1.5}, 0},
		{"slides:\n  font-scale: 2", &SlidesMeta{}, 1},
		{"slides:\n  font-scale: 0.59", &SlidesMeta{}, 1},
		{"slides:\n  font-scale: 크게", &SlidesMeta{}, 1},
		{"slides:\n  font-scale: \"0.8\"", &SlidesMeta{}, 1},
		{"slides:\n  font-scale: .nan", &SlidesMeta{}, 1},
		{"slides:\n  font-scale: [1]", &SlidesMeta{}, 1},
		{"slides:\n  font-scale:", &SlidesMeta{}, 1},
	}
	for _, c := range cases {
		src := "---\ntitle: t\n" + c.fm + "\n---\n\n본문\n"
		r := render(t, src)
		got := r.Meta.Slides
		if (got == nil) != (c.want == nil) || (got != nil && *got != *c.want) {
			t.Errorf("%q → %+v, 기대 %+v", c.fm, got, c.want)
		}
		if len(r.Warnings) != c.warns {
			t.Errorf("%q 경고 %d, 기대 %d: %+v", c.fm, len(r.Warnings), c.warns, r.Warnings)
		}
	}
}

// 틀린 font-scale 은 frontmatter 값 경고(줄 번호는 값의 줄)이고, 모르는 키 경고는 키 목록에 font-scale 을 적는다(itda-hyve#430)
func TestSlidesFontScaleWarning(t *testing.T) {
	r := render(t, "---\ntitle: t\nslides:\n  font-scale: 2\n  size: big\n---\n\n본문\n")
	if len(r.Warnings) != 2 {
		t.Fatalf("경고 %+v", r.Warnings)
	}
	w := r.Warnings[0]
	if w.Code != WarnFrontmatterValue || w.Line != 4 || !strings.Contains(w.Message, "slides.font-scale") || !strings.Contains(w.Message, "0.6~1.5") {
		t.Errorf("범위 밖 경고 %+v", w)
	}
	if w := r.Warnings[1]; w.Code != WarnFrontmatterValue || !strings.Contains(w.Message, "incremental·font-scale)") {
		t.Errorf("모르는 키 경고 %+v", w)
	}
}

func TestSlideLevel(t *testing.T) {
	cases := []struct {
		src  string
		want int
	}{
		{"본문만\n", 0},
		{"# 장\n\n## 절\n\n내용\n\n## 절2\n\n내용\n", 2},
		{"# 장\n\n내용\n\n## 절\n\n내용\n", 1},
		{"# 장\n\n## 절\n\n### 소절\n\n내용\n", 3},
		{"## 제목만\n\n## 또 제목\n", 2},
		{"# 장\n\n---\n\n## 절\n\n<!-- pagebreak -->\n\n### 소절\n\n글\n", 3},
		{":::box\n# 상자 안\n\n글\n:::\n\n## 바깥\n\n글\n", 2}, // 맨 위 블록의 제목만 센다
	}
	for _, c := range cases {
		if got := render(t, c.src).SlideLevel; got != c.want {
			t.Errorf("%q → %d, 기대 %d", c.src, got, c.want)
		}
	}
}

var notesClassRe = regexp.MustCompile(`class="[^"]*\bnotes\b`)

// TestHeadingNotesClass 는 제목의 {.notes} 가 노트가 아닌지다(itda-hyve#344 — 발표자 노트는 :::notes 컨테이너만): DropNotes 와 상관없이
// 제목은 보통 제목으로 남고(글자·id·번호·목차·각주), class 의 notes 만 경고와 함께 빠진다. 다른 class 는 남는다.
func TestHeadingNotesClass(t *testing.T) {
	src := "---\nnumber-sections: true\n---\n\n# 공개\n\n## 첫째 {.notes #first-id}\n\n### 둘째 {.x .notes}\n\n## 셋째[^n] {.notes}\n\n[^n]: 제목 각주\n"
	for _, drop := range []bool{false, true} {
		r, err := Render([]byte(src), Options{DropNotes: drop})
		if err != nil {
			t.Fatal(err)
		}
		if notesClassRe.MatchString(r.HTML) {
			t.Errorf("drop=%v: 제목에 notes class 가 남음\n%s", drop, r.HTML)
		}
		for _, want := range []string{"첫째", `id="first-id"`, `class="x"`, "셋째", "제목 각주", `id="fn:1"`} {
			if !strings.Contains(r.HTML, want) {
				t.Errorf("drop=%v: %q 가 빠짐\n%s", drop, want, r.HTML)
			}
		}
		var nums []string
		var walk func([]TOCItem)
		walk = func(items []TOCItem) {
			for _, it := range items {
				nums = append(nums, it.Number)
				walk(it.Children)
			}
		}
		walk(r.TOC)
		if strings.Join(nums, ",") != "1,1.1,1.1.1,1.2" {
			t.Errorf("drop=%v: 제목 번호 %v", drop, nums)
		}
		n := 0
		for _, w := range r.Warnings {
			if w.Code == WarnBadAttribute && strings.Contains(w.Message, ":::notes") {
				n++
			}
		}
		if n != 3 {
			t.Errorf("drop=%v: 제목 .notes 경고 %d, 기대 3: %+v", drop, n, r.Warnings)
		}
	}
}

// TestDropNotesFootnoteChain 은 노트를 빼도 공개 본문에서 각주를 거쳐 닿는 각주(연쇄·순환)는 남고, 노트에서만 닿는 각주만 빠지며
// 번호가 0 이 되지 않는지다.
func TestDropNotesFootnoteChain(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
		not       []string
	}{
		{"연쇄", "공개[^a]\n\n:::notes\n비밀[^z]\n:::\n\n[^z]: 노트 각주 비밀\n[^a]: 공개각주 [^b]\n[^b]: 종속각주 [^c]\n[^c]: 셋째각주\n",
			[]string{"공개각주", "종속각주", "셋째각주", `id="fn:1"`, `id="fn:2"`, `id="fn:3"`, `href="#fn:2"`, `href="#fn:3"`},
			[]string{"비밀", "fn:0", "fn:4"}},
		{"순환", "공개[^a]\n\n:::notes\n비밀\n:::\n\n[^a]: 앞 [^b]\n[^b]: 뒤 [^a]\n",
			[]string{"앞", "뒤", `id="fn:1"`, `id="fn:2"`},
			[]string{"비밀", "fn:0", "fn:3"}},
		{"노트 전용 연쇄", "공개[^a]\n\n:::notes\n비밀[^n]\n:::\n\n[^a]: 공개각주\n[^n]: 노트각주 비밀 [^m]\n[^m]: 노트 종속 비밀\n",
			[]string{"공개각주", `id="fn:1"`},
			[]string{"비밀", "fn:0", "fn:2"}},
		{"노트와 공유", ":::notes\n비밀[^a]\n:::\n\n공개[^a]\n\n[^a]: 공유각주\n",
			[]string{"공유각주", `id="fn:1"`, `href="#fn:1"`},
			[]string{"비밀", "fn:0", "fn:2", `href="#fnref:1"`}}, // 노트 쪽 참조(fnref:1)로 돌아가는 되돌림 링크는 빠진다
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := Render([]byte(c.src), Options{DropNotes: true})
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range c.want {
				if !strings.Contains(r.HTML, w) {
					t.Errorf("%q 없음\n%s", w, r.HTML)
				}
			}
			for _, w := range c.not {
				if strings.Contains(r.HTML, w) {
					t.Errorf("%q 가 남음\n%s", w, r.HTML)
				}
			}
			// 목록은 새 번호 차례다
			for i, prev := 2, strings.Index(r.HTML, `id="fn:1"`); strings.Contains(r.HTML, `id="fn:`+strconv.Itoa(i)+`"`); i++ {
				at := strings.Index(r.HTML, `id="fn:`+strconv.Itoa(i)+`"`)
				if at < prev {
					t.Errorf("각주 목록 차례가 번호와 다름\n%s", r.HTML)
				}
				prev = at
			}
		})
	}
}

// DropNotes(itda-hyve#344 결정 7 — 공유의 「노트 함께 보이기」 꺼짐): :::notes·{.notes} 컨테이너의 안에 든 것을 빼고 빈 자리만 남긴다(상자 안·중첩 포함). 노트 안
// 제목은 TOC 에도 없고, 노트에서만 가리킨 각주는 목록에서 빠지며 남은 각주는 1부터 다시 매긴다. 끄면(기본) 그대로 낸다.
func TestDropNotes(t *testing.T) {
	src := "# 제목\n\n앞 문단[^p]\n\n:::notes\n### 노트제목비밀\n노트 비밀 문장[^n] ![그림](비밀.png) [링크](http://e.com/비밀)\n\n" +
		":::notes\n안쪽 노트 비밀\n:::\n\n[^n]: 노트 각주 비밀\n:::\n\n::: {.notes .x}\n둘째 노트 비밀\n:::\n\n:::box\n상자 공개\n\n:::notes\n상자 안 노트 비밀\n:::\n:::\n\n" +
		"뒤 문단[^q]\n\n[^p]: 앞 각주 공개\n[^q]: 뒤 각주 공개\n"
	keep, err := Render([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"노트제목비밀", "노트 각주 비밀", "둘째 노트 비밀", "상자 안 노트 비밀", `class="notes"`} {
		if !strings.Contains(keep.HTML, want) {
			t.Errorf("DropNotes 없이 %q 가 빠짐", want)
		}
	}
	r, err := Render([]byte(src), Options{DropNotes: true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.HTML, "비밀") {
		t.Errorf("노트가 남음\n%s", r.HTML)
	}
	// 노트는 빈 자리로 남는다(맨 위 둘·상자 안 하나) — 호스트와 구조가 같다
	if leaks := notesLeak(t, r.HTML, "비밀"); len(leaks) > 0 || strings.Count(r.HTML, `<div class="notes"`) != 3 {
		t.Errorf("노트 자리가 비지 않음 %q\n%s", leaks, r.HTML)
	}
	sameStructure(t, src)
	for _, want := range []string{"앞 문단", "뒤 문단", "상자 공개", "앞 각주 공개", "뒤 각주 공개", `<div class="box"`} {
		if !strings.Contains(r.HTML, want) {
			t.Errorf("%q 가 빠짐\n%s", want, r.HTML)
		}
	}
	// 각주: 노트의 [^n] 이 2번이었다 — 빼면 뒤 각주가 2번이 되고 목록에도 둘뿐이다(되돌림 링크도 같은 번호)
	for _, want := range []string{`href="#fn:1"`, `href="#fn:2"`, `id="fn:1"`, `id="fn:2"`, `href="#fnref:2"`} {
		if !strings.Contains(r.HTML, want) {
			t.Errorf("각주 번호 %s 없음\n%s", want, r.HTML)
		}
	}
	if strings.Contains(r.HTML, "fn:3") || strings.Count(r.HTML, "<li id=\"fn:") != 2 {
		t.Errorf("각주를 다시 매기지 않음\n%s", r.HTML)
	}
	for _, it := range r.TOC {
		if strings.Contains(it.Text, "비밀") || len(it.Children) > 0 {
			t.Errorf("노트 안 제목이 목차에 남음: %+v", r.TOC)
		}
	}
	// {.notes .x} 의 .x 는 DropNotes 와 상관없이 같은 경고 하나와 함께 버린다(transform.go notes) — 노트 자리는 늘 class="notes" 하나
	if len(r.Warnings) != 1 || r.Warnings[0].Code != WarnBadAttribute || !slices.Equal(r.Warnings, keep.Warnings) ||
		strings.Contains(keep.HTML, "notes x") {
		t.Errorf("노트 속성 경고: 공유 %+v, 호스트 %+v\n%s", r.Warnings, keep.Warnings, keep.HTML)
	}
	// 노트에서만 각주를 가리키면 각주 목록이 비어 빈 자리(가로줄·목록 없는 div)만 남는다
	only := "본문\n\n:::notes\n노트[^a]\n:::\n\n[^a]: 노트 각주 비밀\n"
	r, _ = Render([]byte(only), Options{DropNotes: true})
	if strings.Contains(r.HTML, "비밀") || strings.Contains(r.HTML, "<hr>") || strings.Contains(r.HTML, "<li") ||
		!strings.Contains(r.HTML, `<div class="footnotes" role="doc-endnotes"></div>`) {
		t.Errorf("노트 각주 목록이 남음\n%s", r.HTML)
	}
	sameStructure(t, only)
}
