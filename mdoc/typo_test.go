package mdoc

import (
	"reflect"
	"strings"
	"testing"
)

// frontmatter 모르는 키의 오타 경고(itda-doc#14, typo.go).

func typoWarns(ws []Warning) []Warning {
	var out []Warning
	for _, w := range ws {
		if w.Code == WarnFrontmatterTypo {
			out = append(out, w)
		}
	}
	return out
}

func renderSrc(t *testing.T, src string) Result {
	t.Helper()
	r, err := Render([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// 인수 조건 1: slide: true 하나 — 경고 1개(줄은 키 줄), 값은 Extra, 렌더는 가까운 키가 없는 모르는 키와 같다.
func TestTypoTopLevelSlide(t *testing.T) {
	r := renderSrc(t, "---\ntitle: 발표\nslide: true\n---\n\n# a\n\n본문\n")
	ws := typoWarns(r.Warnings)
	if len(ws) != 1 {
		t.Fatalf("frontmatter-typo 경고가 1개여야 함: %v", r.Warnings)
	}
	w := ws[0]
	if w.Line != 3 || w.Key != "slide" || w.Message != "frontmatter slide 는 모르는 키 — slides 의 오타?" {
		t.Errorf("경고 꼴: %+v", w)
	}
	if len(r.Warnings) != 1 {
		t.Errorf("다른 경고가 생김: %v", r.Warnings)
	}
	if f, ok := r.Meta.Field("slide"); !ok || f.Value != "true" {
		t.Errorf("slide 가 Extra 에 남지 않음: %+v %v", f, ok)
	}
	if r.Meta.Slides != nil {
		t.Errorf("오타 키가 슬라이드를 켬: %+v", r.Meta.Slides)
	}
	// 비교 기준: 아는 키와 먼 모르는 키 — 경고 없이 같은 렌더여야 한다
	base := renderSrc(t, "---\ntitle: 발표\nqqqqq: true\n---\n\n# a\n\n본문\n")
	if len(base.Warnings) != 0 {
		t.Errorf("먼 모르는 키에 경고: %v", base.Warnings)
	}
	if r.HTML != base.HTML || !reflect.DeepEqual(r.TOC, base.TOC) || r.SlideLevel != base.SlideLevel {
		t.Errorf("경고 말고 렌더가 달라짐:\n%s\n---\n%s", r.HTML, base.HTML)
	}
	rm, bm := r.Meta, base.Meta
	rm.Extra, bm.Extra = nil, nil
	if !reflect.DeepEqual(rm, bm) {
		t.Errorf("Extra 말고 Meta 가 달라짐: %+v / %+v", rm, bm)
	}
}

// 인수 조건 2·3: 최상위 키 표 — 제안할 정본 키(빈 글자는 경고 없음).
func TestTypoTopLevelTable(t *testing.T) {
	for _, c := range []struct{ key, want string }{
		{"slide", "slides"},
		{"toc_depth", "toc-depth"},
		{"Toc_Depth", "toc-depth"},
		{"Number-Sections", "number-sections"},
		{"numbersections", "number-sections"},
		{"papersiz", "papersize"},
		{"subtitel", "subtitle"}, // 인접 전치
		{"TOC", "toc"},           // 길이 3 — 정규화 일치만
		{"covers", "cover"},      // 테마 자유 키라도 가까우면 경고(동작은 같다)
		{"data", "date"},         // 길이 4 거리 1 — 경고됨(수용, 보고서)
		{"Doc_No", "doc-no"},
		{"수신", ""},
		{"참조", ""},
		{"시행일", ""},
		{"foo", ""},
		{"tic", ""}, // 길이 3 — toc 와 거리 1 이어도 경고 없음
		{"tit", ""},
		{"organisation-unit", ""},
		{"background", ""},
		{"custom", ""},
		{"slidez-x", ""},
		{"titl", "title"},
		{"versoin", "version"},
		{"Title", "title"},
		{"title2", "title"},   // 숫자 접미도 경고됨(수용 — 일부러 쓴 자유 키면 오탐, 동작은 같다)
		{"authors", "author"}, // 복수형
		{"1", ""},
		{"true", ""},
	} {
		src := "---\n" + c.key + ": x\n---\n\n본문\n"
		ws := typoWarns(renderSrc(t, src).Warnings)
		if c.want == "" {
			if len(ws) != 0 {
				t.Errorf("%s: 경고가 없어야 함: %v", c.key, ws)
			}
			continue
		}
		if len(ws) != 1 || ws[0].Line != 2 || ws[0].Key != c.key ||
			ws[0].Message != "frontmatter "+c.key+" 는 모르는 키 — "+c.want+" 의 오타?" {
			t.Errorf("%s: %s 제안 경고 1개여야 함: %v", c.key, c.want, ws)
		}
	}
}

// 빈 키·비스칼라 키는 견주지 않는다.
func TestTypoOddKeys(t *testing.T) {
	for _, src := range []string{
		"---\n\"\": x\n---\n\n본문\n",
		"---\n? [a]\n: x\n---\n\n본문\n",
		"---\n? {title: a}\n: x\n---\n\n본문\n",
	} {
		if ws := typoWarns(renderSrc(t, src).Warnings); len(ws) != 0 {
			t.Errorf("%q: 경고가 없어야 함: %v", src, ws)
		}
	}
}

// 판단: 아는 키가 같은 frontmatter 에 함께 있어도 경고한다.
func TestTypoWithKnownKeyPresent(t *testing.T) {
	r := renderSrc(t, "---\nslides: true\nslide: true\n---\n\n# a\n")
	if ws := typoWarns(r.Warnings); len(ws) != 1 || ws[0].Line != 3 {
		t.Errorf("slides 와 함께 있어도 경고 1개: %v", r.Warnings)
	}
	if r.Meta.Slides == nil {
		t.Errorf("slides: true 가 그대로여야 함")
	}
}

// 인수 조건 4: 묶음 안 모르는 키 — 코드·줄·Key 는 그대로, 문구 괄호 안 끝에 「— X 의 오타?」.
func TestTypoGroupHint(t *testing.T) {
	for _, c := range []struct{ src, key, msg string }{
		{"---\nslides:\n  ratoi: \"4:3\"\n---\n\n# a\n", "slides.ratoi",
			"frontmatter slides.ratoi 는 모르는 키라 무시함(ratio·level·transition·incremental·font-scale — ratio 의 오타?)"},
		{"---\nslides:\n  fontscal: 1.1\n---\n\n# a\n", "slides.fontscal",
			"frontmatter slides.fontscal 는 모르는 키라 무시함(ratio·level·transition·incremental·font-scale — font-scale 의 오타?)"},
		{"---\nslides:\n  Slide_Level: 2\n---\n\n# a\n", "slides.Slide_Level",
			"frontmatter slides.Slide_Level 는 모르는 키라 무시함(ratio·level·transition·incremental·font-scale — level 의 오타?)"},
		{"---\nslides:\n  speed: 2\n---\n\n# a\n", "slides.speed",
			"frontmatter slides.speed 는 모르는 키라 무시함(ratio·level·transition·incremental·font-scale)"},
		{"---\ncode:\n  linenumber: false\n---\n\n# a\n", "code.linenumber",
			"frontmatter code.linenumber 는 모르는 키라 무시함(line-numbers — line-numbers 의 오타?)"},
		{"---\ncode:\n  wrap: true\n---\n\n# a\n", "code.wrap",
			"frontmatter code.wrap 는 모르는 키라 무시함(line-numbers)"},
	} {
		r := renderSrc(t, c.src)
		if len(r.Warnings) != 1 {
			t.Errorf("%s: 경고 1개여야 함: %v", c.key, r.Warnings)
			continue
		}
		w := r.Warnings[0]
		if w.Code != WarnFrontmatterValue || w.Line != 3 || w.Key != c.key || w.Message != c.msg {
			t.Errorf("%s: 경고 꼴\n got %+v\nwant msg %q", c.key, w, c.msg)
		}
	}
}

// 인수 조건 5: 폴더 _metadata.yml 의 slide: true — 문서로 물려받아 Extra 에 들고, 경고는 기존 폴더 경고 꼴(Line 0·앞머리).
func TestTypoFromFolder(t *testing.T) {
	r := renderFM(t, "---\ntitle: a\n---\n\n# a\n", fm("../_metadata.yml", "theme: mono\nslide: true\n"))
	if _, ok := r.Meta.Field("slide"); !ok {
		t.Fatalf("폴더의 모르는 키가 문서로 안 넘어옴: %+v", r.Meta.Extra)
	}
	ws := typoWarns(r.Warnings)
	if len(ws) != 1 || ws[0].Line != 0 || ws[0].Key != "slide" ||
		ws[0].Message != "../_metadata.yml 2줄: frontmatter slide 는 모르는 키 — slides 의 오타?" {
		t.Errorf("폴더 오타 경고 꼴: %v", r.Warnings)
	}
	// 폴더 묶음 안 오타도 기존 폴더 경고 꼴 그대로 문구만 덧붙는다
	r = renderFM(t, "---\nslides: true\n---\n\n# a\n", fm("_metadata.yml", "slides:\n  ratoi: \"4:3\"\n"))
	found := false
	for _, w := range r.Warnings {
		if w.Line == 0 && w.Code == WarnFrontmatterValue && strings.HasPrefix(w.Message, "_metadata.yml 2줄: frontmatter slides.ratoi ") &&
			strings.HasSuffix(w.Message, " — ratio 의 오타?)") {
			found = true
		}
	}
	if !found {
		t.Errorf("폴더 slides.ratoi 경고: %v", r.Warnings)
	}
	// 문서가 같은 오타 키로 덮으면 폴더 경고는 없고 문서 줄 경고 하나
	r = renderFM(t, "---\nslide: false\n---\n\n# a\n", fm("_metadata.yml", "slide: true\n"))
	if ws := typoWarns(r.Warnings); len(ws) != 1 || ws[0].Line != 2 {
		t.Errorf("문서가 덮은 오타 키: %v", r.Warnings)
	}
}

func TestOSADistance(t *testing.T) {
	for _, c := range []struct {
		a, b string
		d    int
	}{
		{"", "", 0}, {"abc", "", 3}, {"slide", "slides", 1}, {"subtitel", "subtitle", 1},
		{"ratoi", "ratio", 1}, {"kitten", "sitting", 3}, {"ca", "abc", 3},
	} {
		if got := osaDistance(c.a, c.b); got != c.d {
			t.Errorf("osaDistance(%q,%q)=%d, want %d", c.a, c.b, got, c.d)
		}
	}
}
