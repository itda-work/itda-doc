package mdoc

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// 스키마 표(FrontmatterSchema)와 파서가 어긋나지 않게 막는다(itda-hyve#474 ① — 뮤테이션으로 확인:
// meta.go 의 case 를 하나 더하거나 별칭을 지우면 이 시험이 실패한다). 파서가 아는 키·별칭은
// meta.go 의 `switch …Value` case 에서 뽑는다(guidemd_test.go caseKeys 와 같은 방법).

func TestSchemaMatchesParser(t *testing.T) {
	for _, tc := range []struct{ group, fn string }{
		{"", "fill"},
		{"slides", "parseSlides"},
		{"code", "parseCode"},
	} {
		sameSet(t, "스키마 "+tc.group, schemaNames(tc.group), caseKeys(t, tc.fn))
	}
}

// 최상위 키에는 별칭이 없다 — 별칭은 slides·code 하위 키에만 있다(안내서 표와 같은 모양).
func TestSchemaTopLevelHasNoAliases(t *testing.T) {
	for _, k := range FrontmatterSchema {
		if k.Group == "" && len(k.Aliases) > 0 {
			t.Errorf("최상위 키 %s 에 별칭 %v", k.Key, k.Aliases)
		}
	}
}

// 표의 키·별칭이 서로 겹치지 않는다(같은 이름이 두 그룹에 있으면 편집 화면이 잘못 그린다).
func TestSchemaKeysUnique(t *testing.T) {
	var seen []string
	for _, k := range FrontmatterSchema {
		for _, n := range append([]string{k.Key}, k.Aliases...) {
			if slices.Contains(seen, n) {
				t.Errorf("키 %s 가 겹침", n)
			}
			seen = append(seen, n)
		}
	}
}

// 표의 별칭은 파서가 실제로 받는다 — 별칭으로 값을 주면 그 필드가 채워진다(정본 이름과 같은 결과).
func TestSchemaAliasesParse(t *testing.T) {
	r, err := Render([]byte("---\nslides:\n  aspect: \"4:3\"\n  slide-level: 2\n  font_scale: 1.1\ncode:\n  line_numbers: false\n---\n\n# a\n"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Meta.Slides == nil || r.Meta.Slides.Ratio != "4:3" || r.Meta.Slides.Level != 2 || r.Meta.Slides.FontScale != 1.1 {
		t.Errorf("slides 별칭: %+v", r.Meta.Slides)
	}
	if r.Meta.Code == nil || r.Meta.Code.LineNumbers == nil || *r.Meta.Code.LineNumbers {
		t.Errorf("code 별칭: %+v", r.Meta.Code)
	}
	if len(r.Warnings) != 0 {
		t.Errorf("별칭 경고: %v", r.Warnings)
	}
}

// ---- 편집 화면용 필드(itda-hyve#474 ②) — Input·Values·Min·Max·Step·PerDoc 이 파서와 맞는지 막는다.

// Input 이 있는 키의 사람용 스키마(Kind)도 함께 있다(기계용 필드만 덩그러니 있는 키가 없게).
func TestSchemaInputHasKind(t *testing.T) {
	for _, k := range FrontmatterSchema {
		if k.Input != "" && k.Kind == "" {
			t.Errorf("%s: Input 이 있는데 Kind 가 비었다", k.Key)
		}
		switch k.Input {
		case "", "text", "bool", "int", "number", "enum", "theme", "list-style":
		default:
			t.Errorf("%s: 모르는 Input %q", k.Key, k.Input)
		}
		if k.Input == "enum" || k.Input == "list-style" {
			if len(k.Values) == 0 {
				t.Errorf("%s: %s 인데 Values 가 비었다", k.Key, k.Input)
			}
		} else if len(k.Values) > 0 {
			t.Errorf("%s: Input %s 에는 Values 가 없다", k.Key, k.Input)
		}
		switch k.Input {
		case "int", "number":
			if k.Min <= 0 || k.Max <= k.Min {
				t.Errorf("%s: Input %s 인데 Min·Max 가 없거나 순서가 틀리다(%v~%v)", k.Key, k.Input, k.Min, k.Max)
			}
		case "text", "bool", "enum", "theme", "list-style", "":
			if k.Min != 0 || k.Max != 0 {
				t.Errorf("%s: Input %s 인데 Min·Max 가 있다", k.Key, k.Input)
			}
		}
	}
}

// theme 후보는 화면이 그때그때 테마 목록으로 채운다 — 표에는 두지 않는다.
func TestSchemaThemeInputHasNoValues(t *testing.T) {
	for _, k := range FrontmatterSchema {
		if k.Input == "theme" && len(k.Values) > 0 {
			t.Errorf("theme Input 에 Values 가 있다 — 화면이 테마 목록으로 채운다")
		}
	}
}

// PerDoc(문서마다 다른 값 — 화면이 접어 두는 묶음)는 이 다섯이다.
func TestSchemaPerDocKeys(t *testing.T) {
	want := []string{"title", "subtitle", "date", "doc-no", "version"}
	var got []string
	for _, k := range FrontmatterSchema {
		if k.PerDoc {
			got = append(got, k.Key)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("PerDoc 키 %v, 기대 %v", got, want)
	}
}

// enum·list-style 후보(Values)는 파서가 받는 표기다 — 넣으면 그 값이 Meta 에 들고 경고가 없다.
// papersize a4 는 Meta 가 기본값(a4)과 명시를 구분하지 않아 MetaValues 가 「정하지 않음」으로
// 빼므로 거기만 기대를 달리 한다(slides.ratio·transition 은 SlidesMeta 가 구분한다). list-style
// 조합("gaejosik gongmun")은 파서가 목록 표기로 받는다 — YAML 쓰는 쪽이 그 표기로 쓴다(저장 규칙, docview).
func TestSchemaValuesParse(t *testing.T) {
	for _, tc := range []struct {
		path    string // 키 경로(그룹.키)
		keys    []string
		yamlOf  func(v string) string // Values 값을 YAML 표기로
		skipped map[string]bool       // 기본값과 같아 MetaValues 에 빠지는 후보
	}{
		{path: "papersize", keys: []string{"papersize"}, skipped: map[string]bool{"a4": true}},
		{path: "list-style", keys: []string{"list-style"},
			yamlOf: func(v string) string {
				if strings.Contains(v, " ") {
					return "[" + strings.ReplaceAll(v, " ", ", ") + "]"
				}
				return v
			}},
		{path: "slides.ratio", keys: []string{"slides", "ratio"}, yamlOf: quoted},
		{path: "slides.transition", keys: []string{"slides", "transition"}},
	} {
		schema := schemaByPath(tc.path)
		yamlOf := tc.yamlOf
		if yamlOf == nil {
			yamlOf = func(v string) string { return v }
		}
		for _, v := range schema.Values {
			src := frontmatterFor(tc.keys, yamlOf(v))
			r, err := Render(src, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Warnings) != 0 {
				t.Errorf("%s = %s: 경고 %v", tc.path, v, r.Warnings)
			}
			got := MetaValues(r.Meta)[tc.path]
			if tc.skipped[v] {
				if got != "" {
					t.Errorf("%s = %s(기본값): MetaValues 에 %q 가 들었다 — 빠져야 한다", tc.path, v, got)
				}
				continue
			}
			if got != v {
				t.Errorf("%s = %s: Meta 에는 %q", tc.path, v, got)
			}
		}
	}
}

func quoted(v string) string { return `"` + v + `"` }

// frontmatterFor 는 keys 경로에 값 v 를 둔 문서 원문다.
func frontmatterFor(keys []string, v string) []byte {
	fm := "---\n"
	if len(keys) == 2 {
		fm += keys[0] + ":\n  " + keys[1] + ": " + v + "\n"
	} else {
		fm += keys[0] + ": " + v + "\n"
	}
	return []byte(fm + "---\n\n# a\n")
}

// int·number 의 Min·Max 는 파서가 받는 경계와 같다 — 경계값은 들고 그 바로 바깥은 경고가 난다.
func TestSchemaMinMaxMatchParser(t *testing.T) {
	step := func(input string) float64 {
		if input == "int" {
			return 1
		}
		return 0.1
	}
	for _, path := range []string{"toc-depth", "slides.level", "slides.font-scale"} {
		k := schemaByPath(path)
		if k.Input != "int" && k.Input != "number" {
			t.Fatalf("%s: int·number 가 아님", path)
		}
		n := step(k.Input)
		num := func(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }
		for _, tc := range []struct {
			v    string
			want bool // 파서가 받는가
		}{
			{num(k.Min), true}, {num(k.Max), true},
			{num(k.Min - n), false}, {num(k.Max + n), false},
		} {
			src := frontmatterFor(strings.Split(path, "."), tc.v)
			r, err := Render(src, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if got := len(r.Warnings) == 0; got != tc.want {
				t.Errorf("%s = %s: 경고 %v(받기 기대 %v)", path, tc.v, r.Warnings, tc.want)
			}
		}
	}
}

// schemaByPath 는 키 경로("slides.font-scale")의 표 항목이다.
func schemaByPath(path string) MetaKey {
	group, key := "", path
	if g, k, ok := strings.Cut(path, "."); ok {
		group, key = g, k
	}
	for _, k := range FrontmatterSchema {
		if k.Group == group && k.Key == key {
			return k
		}
	}
	panic("스키마에 없는 경로: " + path)
}

// ① 의 경고 JSON 은 34cd175 와 같은 모양이다 — Warning.Key 는 편집 화면의 내부 보조 정보라
// 직렬화에서 뺀다(itda-hyve#474 ② 리뷰 3회차 [6], baseline 비교: Key 만 빼면 8표본 전부 동일).
func TestWarningKeyNotSerialized(t *testing.T) {
	r, err := Render([]byte("---\ntoc-depth: bad\n---\n\n# a\n"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Warnings) == 0 {
		t.Fatal("경고 없음")
	}
	b, err := json.Marshal(r.Warnings[0])
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["Key"]; ok {
		t.Errorf("경고 JSON 에 Key 가 실림: %s", b)
	}
	if _, ok := m["key"]; ok {
		t.Errorf("경고 JSON 에 key 가 실림: %s", b)
	}
	// 34cd175 의 모양 — Line·Code·Message 만
	if len(m) != 3 {
		t.Errorf("경고 JSON 필드 수: %v", m)
	}
}
