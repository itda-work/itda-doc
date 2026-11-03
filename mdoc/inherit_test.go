package mdoc

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// 폴더 기본 frontmatter 상속(itda-hyve#474 ①) — Options.FolderMeta 로 폴더 값을 넣어 합침 규칙을 본다.
// 우선순위: 문서 frontmatter > 가까운 폴더 > 먼 폴더 > (테마 defaults·기본값은 문서 창 쪽).

func fm(rel, yaml string) FolderMeta   { return FolderMeta{Rel: rel, YAML: []byte(yaml)} }
func fmErr(rel, msg string) FolderMeta { return FolderMeta{Rel: rel, Err: msg} }
func renderFM(t *testing.T, src string, folder ...FolderMeta) Result {
	t.Helper()
	r, err := Render([]byte(src), Options{FolderMeta: folder})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// 인수 조건 ①: 폴더의 slides.font-scale + 문서의 slides.ratio — 슬라이드로 열리고 둘 다 적용된다.
func TestInheritSlidesMerged(t *testing.T) {
	r := renderFM(t, "---\nslides:\n  ratio: \"4:3\"\n---\n\n# a\n", fm("_metadata.yml", "slides:\n  font-scale: 0.8\n"))
	if r.Meta.Slides == nil {
		t.Fatal("slides 가 nil")
	}
	if r.Meta.Slides.FontScale != 0.8 || r.Meta.Slides.Ratio != "4:3" {
		t.Errorf("font-scale %v·ratio %q", r.Meta.Slides.FontScale, r.Meta.Slides.Ratio)
	}
}

// 인수 조건 ②: 문서에 slides: 가 없으면 폴더의 slides 값은 버린다(nil).
func TestInheritSlidesNeedsOwnKey(t *testing.T) {
	r := renderFM(t, "# b\n", fm("_metadata.yml", "slides:\n  font-scale: 0.8\n"))
	if r.Meta.Slides != nil {
		t.Errorf("폴더 slides 가 문서에 얹힘: %+v", r.Meta.Slides)
	}
	// 버린 폴더 slides 값은 값 경고 대신 안내 경고 하나로 알린다(itda-hyve#543).
	if len(r.Warnings) != 1 || r.Warnings[0].Code != WarnFolderSlidesUnused || r.Warnings[0].Line != 0 ||
		!strings.HasPrefix(r.Warnings[0].Message, "_metadata.yml: ") {
		t.Errorf("안내 경고 하나가 아님: %v", r.Warnings)
	}
}

// itda-hyve#543: 폴더 slides 를 버릴 때만 안내한다 — 문서가 slides 를 정했거나(true·false·맵) 폴더 slides 가
// 꺼짐·틀린 값이면 내지 않는다. slides 를 정한 가장 가까운 폴더 파일의 경로로 알린다(줄 없음).
func TestFolderSlidesUnusedHint(t *testing.T) {
	cases := []struct {
		doc    string
		folder []FolderMeta
		want   string // 기대 경고 앞머리("" 이면 경고 없음)
	}{
		{"# a\n", []FolderMeta{fm("_metadata.yml", "theme: bold\nslides:\n  ratio: \"16:9\"\n")}, "_metadata.yml: "},
		{"---\ntitle: t\n---\n\n# a\n", []FolderMeta{fm("_metadata.yml", "slides: true\n")}, "_metadata.yml: "},
		{"# a\n", []FolderMeta{fm("_metadata.yml", "slides:\n")}, "_metadata.yml: "},
		{"# a\n", []FolderMeta{fm("../_metadata.yml", "slides:\n  level: 2\n"), fm("_metadata.yml", "x: 1\nslides:\n  font-scale: 0.8\n")}, "_metadata.yml: "},
		{"---\nslides: true\n---\n\n# a\n", []FolderMeta{fm("_metadata.yml", "slides:\n  level: 2\n")}, ""},
		{"---\nslides: false\n---\n\n# a\n", []FolderMeta{fm("_metadata.yml", "slides:\n  level: 2\n")}, ""},
		{"---\nslides:\n  level: 3\n---\n\n# a\n", []FolderMeta{fm("_metadata.yml", "slides:\n  level: 2\n")}, ""},
		{"# a\n", []FolderMeta{fm("_metadata.yml", "slides: false\n")}, ""},
		{"# a\n", []FolderMeta{fm("_metadata.yml", "slides: 발표\n")}, ""},
		{"# a\n", []FolderMeta{fm("_metadata.yml", "theme: bold\n")}, ""},
		{"# a\n", []FolderMeta{fm("../_metadata.yml", "slides:\n  level: 2\n"), fm("_metadata.yml", "slides: false\n")}, ""},
		{"# a\n", []FolderMeta{fm("../_metadata.yml", "slides:\n  level: 2\n"), fm("_metadata.yml", "slides: true\n")}, "_metadata.yml: "},
	}
	for _, c := range cases {
		r := renderFM(t, c.doc, c.folder...)
		var got []Warning
		for _, w := range r.Warnings {
			if w.Code == WarnFolderSlidesUnused {
				got = append(got, w)
			}
		}
		if c.want == "" {
			if len(got) != 0 {
				t.Errorf("%q: 경고가 나면 안 됨: %v", c.doc, got)
			}
			continue
		}
		if len(got) != 1 || got[0].Line != 0 || !strings.HasPrefix(got[0].Message, c.want) {
			t.Errorf("%q: 앞머리 %q 인 경고 하나를 기대, %v", c.doc, c.want, got)
		}
		if r.Meta.Slides != nil {
			t.Errorf("%q: 렌더가 바뀜 — slides %+v", c.doc, r.Meta.Slides)
		}
	}
}

// 문서의 slides: false 는 폴더의 slides 맵을 통째로 대체한다(노드 병합 — 스칼라는 통째로 바꾼다).
func TestInheritSlidesFalseOverridesFolder(t *testing.T) {
	r := renderFM(t, "---\nslides: false\n---\n\n# b\n", fm("_metadata.yml", "slides:\n  font-scale: 0.8\n"))
	if r.Meta.Slides != nil {
		t.Errorf("slides: false 인데 %+v", r.Meta.Slides)
	}
}

// 인수 조건 ③: 먼 폴더 < 가까운 폴더 < 문서.
func TestInheritThemePriority(t *testing.T) {
	r := renderFM(t, "---\n---\n\n# c\n",
		fm("../_metadata.yml", "theme: classic\n"), fm("_metadata.yml", "theme: bold\n"))
	if r.Meta.Theme != "bold" {
		t.Errorf("가까운 폴더가 이겨야 함: %q", r.Meta.Theme)
	}
	r = renderFM(t, "---\ntheme: mono\n---\n\n# c\n",
		fm("../_metadata.yml", "theme: classic\n"), fm("_metadata.yml", "theme: bold\n"))
	if r.Meta.Theme != "mono" {
		t.Errorf("문서가 이겨야 함: %q", r.Meta.Theme)
	}
}

// 인수 조건 ④: 문서가 명시한 false 는 폴더의 true 를 이긴다(0 값과 "안 정함" 이 섞이지 않는다).
func TestInheritExplicitFalseWins(t *testing.T) {
	r := renderFM(t, "---\ntoc: false\n---\n\n# a\n", fm("_metadata.yml", "toc: true\n"))
	if r.Meta.TOC {
		t.Error("문서의 toc: false 가 폴더의 true 에 묻힘")
	}
	if !strings.Contains(r.HTML, `<section class="toc-page">`) == false { // 목차가 없어야 한다
		t.Error("목차가 생김")
	}
	r = renderFM(t, "---\n---\n\n# a\n", fm("_metadata.yml", "toc: true\n"))
	if !r.Meta.TOC {
		t.Error("폴더의 toc: true 가 문서에 얹히지 않음")
	}
}

// code: 매핑은 키 단위, 스칼라는 통째로 바꾼다. 문서 code: true 는 폴더의 line-numbers 를 덮어 기본(켬)이다.
func TestInheritCodeMergeRules(t *testing.T) {
	r := renderFM(t, "---\n---\n\n# a\n", fm("_metadata.yml", "code:\n  line-numbers: false\n"))
	if r.Meta.Code == nil || r.Meta.Code.LineNumbers == nil || *r.Meta.Code.LineNumbers {
		t.Errorf("폴더 line-numbers: false 가 안 얹힘: %+v", r.Meta.Code)
	}
	// 문서가 빈 맵(code: {})이면 폴더의 키가 남는다.
	r = renderFM(t, "---\ncode: {}\n---\n\n# a\n", fm("_metadata.yml", "code:\n  line-numbers: false\n"))
	if r.Meta.Code == nil || r.Meta.Code.LineNumbers == nil || *r.Meta.Code.LineNumbers {
		t.Errorf("빈 맵에서 폴더 키가 사라짐: %+v", r.Meta.Code)
	}
	// 문서의 스칼라 code: true 는 폴더 맵을 통째로 대체한다.
	r = renderFM(t, "---\ncode: true\n---\n\n# a\n", fm("_metadata.yml", "code:\n  line-numbers: false\n"))
	if r.Meta.Code == nil || r.Meta.Code.LineNumbers != nil {
		t.Errorf("스칼라가 맵을 대체하지 못함: %+v", r.Meta.Code)
	}
}

// 모르는 키(Extra)도 물려받는다. 목록 값은 통째로 바뀐다.
func TestInheritExtraKeys(t *testing.T) {
	r := renderFM(t, "---\n---\n\n# a\n", fm("_metadata.yml", "수신: 감사관실\n"))
	if f, ok := r.Meta.Field("수신"); !ok || f.Value != "감사관실" {
		t.Errorf("모르는 키가 안 물려받음: %+v %v", f, ok)
	}
	r = renderFM(t, "---\n수신: 기획실\n---\n\n# a\n", fm("_metadata.yml", "수신: 감사관실\n"))
	if f, _ := r.Meta.Field("수신"); f.Value != "기획실" {
		t.Errorf("문서의 Extra 가 폴더를 못 이김: %q", f.Value)
	}
	r = renderFM(t, "---\n참조: [a, b]\n---\n\n# a\n", fm("_metadata.yml", "참조: [x]\n"))
	if f, _ := r.Meta.Field("참조"); !reflect.DeepEqual(f.List, []string{"a", "b"}) {
		t.Errorf("목록 값이 통째로 안 바뀜: %+v", f)
	}
}

// 별칭 키도 같은 규칙으로 합친다(폴더 fontscale + 문서 font-scale — 문서가 뒤에 오니 이긴다).
func TestInheritAliasKeys(t *testing.T) {
	r := renderFM(t, "---\nslides:\n  font-scale: 1.2\n---\n\n# a\n", fm("_metadata.yml", "slides:\n  fontscale: 0.8\n"))
	if r.Meta.Slides.FontScale != 1.2 {
		t.Errorf("문서의 font-scale 이 폴더의 fontscale 을 못 이김: %v", r.Meta.Slides.FontScale)
	}
}

// 인수 조건 ⑤: 폴더 파일의 YAML 이 깨지면 그 파일만 건너뛰고 경고한다 — 문서는 렌더되고,
// 경고의 Line 은 0(문서 줄이 아님), 문구에 파일 상대 경로가 있다.
func TestInheritBrokenFolderYAML(t *testing.T) {
	r := renderFM(t, "---\ntheme: classic\n---\n\n# a\n",
		fm("../_metadata.yml", "toc: [겹침\n"), fm("_metadata.yml", "papersize: a4-landscape\n"))
	if r.Meta.PaperSize != "a4-landscape" {
		t.Errorf("깨진 파일 때문에 다른 파일이 건너뛰어짐: %q", r.Meta.PaperSize)
	}
	if len(r.Warnings) != 1 || r.Warnings[0].Code != WarnFrontmatterInvalid || r.Warnings[0].Line != 0 {
		t.Fatalf("경고 %v", r.Warnings)
	}
	if !strings.Contains(r.Warnings[0].Message, "../_metadata.yml") || !strings.Contains(r.Warnings[0].Message, "건너뜀") {
		t.Errorf("경고 문구에 파일 경로가 없음: %q", r.Warnings[0].Message)
	}
}

// 읽지 못한 파일(ErR)·매핑이 아닌 파일도 그 파일만 건너뛰고 경고한다. 빈 파일(주석만)은 조용히 지나간다.
func TestInheritUnreadableAndNonMapping(t *testing.T) {
	r := renderFM(t, "---\n---\n\n# a\n",
		fmErr("../_metadata.yml", "파일이 너무 큼(64KB 넘음)"), fm("_metadata.yml", "- a\n- b\n"), fm("s/_metadata.yml", "# 주석만\n"))
	if len(r.Warnings) != 2 {
		t.Fatalf("경고 %v", r.Warnings)
	}
	for _, w := range r.Warnings {
		if w.Line != 0 || w.Code != WarnFrontmatterInvalid {
			t.Errorf("경고 %v", w)
		}
	}
	if !strings.Contains(r.Warnings[0].Message, "파일이 너무 큼") || !strings.Contains(r.Warnings[1].Message, "키: 값 모양") {
		t.Errorf("경고 문구 %q / %q", r.Warnings[0].Message, r.Warnings[1].Message)
	}
	if r.FolderMeta != nil {
		t.Errorf("적용 목록이 비어야 함: %v", r.FolderMeta)
	}
}

// 폴더에서 온 키의 값 경고는 문서 줄 번호가 아니라 그 파일의 경로·줄로 낸다(Line 0).
func TestInheritFolderValueWarningLabel(t *testing.T) {
	r := renderFM(t, "---\nslides:\n  font-scale: 1.2\n---\n\n# a\n",
		fm("_metadata.yml", "toc: 안녕\nslides:\n  ratio: \"16:10\"\n"))
	var folderWarns []Warning
	for _, w := range r.Warnings {
		if strings.HasPrefix(w.Message, "_metadata.yml ") {
			folderWarns = append(folderWarns, w)
		}
	}
	if len(folderWarns) != 2 {
		t.Fatalf("폴더 값 경고가 2개여야 함: %v", r.Warnings)
	}
	for _, w := range folderWarns {
		if w.Line != 0 || w.Code != WarnFrontmatterValue {
			t.Errorf("폴더 경고의 줄·코드: %+v", w)
		}
	}
	if !strings.Contains(folderWarns[0].Message, "_metadata.yml 1줄: frontmatter toc:") {
		t.Errorf("toc 경고 문구: %q", folderWarns[0].Message)
	}
	if !strings.Contains(folderWarns[1].Message, "_metadata.yml 3줄: frontmatter slides.ratio:") {
		t.Errorf("slides.ratio 경고 문구: %q", folderWarns[1].Message)
	}
}

// 문서가 이긴 키의 폴더 값 경고는 나오지 않는다(문서 줄 경고로 바뀌지도 않는다).
func TestInheritDocOverrideDropsFolderWarning(t *testing.T) {
	r := renderFM(t, "---\ntoc: false\nslides:\n  ratio: \"4:3\"\n---\n\n# a\n",
		fm("_metadata.yml", "toc: 안녕\nslides:\n  ratio: \"16:10\"\n"))
	if len(r.Warnings) != 0 {
		t.Errorf("문서가 이긴 키의 폴더 경고가 남음: %v", r.Warnings)
	}
}

// 문서의 YAML 이 깨졌으면 지금처럼 본문으로 렌더한다 — 폴더 상속은 하지 않는다(파일 자체 오류 경고는 낸다).
func TestInheritBrokenDocYAML(t *testing.T) {
	r := renderFM(t, "---\ntoc: [겹침\n---\n\n# a\n", fm("_metadata.yml", "toc: true\n"))
	if r.Meta.TOC {
		t.Error("문서가 깨진데 폴더 값이 얹힘")
	}
	var codes []string
	for _, w := range r.Warnings {
		codes = append(codes, w.Code)
	}
	if len(codes) != 1 || codes[0] != WarnFrontmatterInvalid {
		t.Errorf("경고 %v", r.Warnings)
	}
	if !strings.Contains(r.HTML, "toc: [겹침") {
		t.Error("본문으로 렌더하지 않음")
	}
}

// Result.FolderMeta 는 적용된 파일만, 가까운 것부터다. 깨진 파일은 빠진다.
func TestInheritFolderList(t *testing.T) {
	r := renderFM(t, "# a\n",
		fm("../../_metadata.yml", "toc: true\n"), fm("../_metadata.yml", "toc: [깨짐\n"), fm("_metadata.yml", "theme: classic\n"))
	if !reflect.DeepEqual(r.FolderMeta, []string{"_metadata.yml", "../../_metadata.yml"}) {
		t.Errorf("적용 목록: %v", r.FolderMeta)
	}
}

// FolderMeta 가 비면(붙여넣은 원문 등) 지금까지의 결과와 같다.
func TestInheritNoneUnchanged(t *testing.T) {
	src := "---\ntheme: classic\ntoc: true\n---\n\n# a\n"
	r, err := Render([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Meta.Theme != "classic" || !r.Meta.TOC || r.FolderMeta != nil || len(r.Warnings) != 0 {
		t.Errorf("상속 없는 렌더가 달라짐: %+v %v", r.Meta, r.Warnings)
	}
}

// 별칭 정규화(리뷰 R1 [2] 반례 이전): 세 층에서 별칭 표기(font_scale)가 다른 키로 어긋나 폴더 값이
// 문서를 이기지 않는다 — 합치기 전에 모든 키를 정규 이름으로 바꾼다.
func TestInheritAliasNormalized(t *testing.T) {
	r := renderFM(t, "---\nslides:\n  font-scale: 1.2\n---\n\n# a\n",
		fm("../_metadata.yml", "slides:\n  font-scale: 0.8\n"), fm("_metadata.yml", "slides:\n  font_scale: 0.9\n"))
	if r.Meta.Slides == nil || r.Meta.Slides.FontScale != 1.2 {
		t.Fatalf("문서의 1.2 가 폴더 별칭 0.9 에 밀림: %+v", r.Meta.Slides)
	}
	// 문서 쪽 별칭도 정규 이름으로 합친다(문서 fontscale 이 폴더 font-scale 을 이긴다).
	r = renderFM(t, "---\nslides:\n  fontscale: 1.2\n---\n\n# a\n", fm("_metadata.yml", "slides:\n  font-scale: 0.8\n"))
	if r.Meta.Slides.FontScale != 1.2 {
		t.Fatalf("문서의 별칭이 폴더를 못 이김: %v", r.Meta.Slides.FontScale)
	}
}

// 문서가 이긴 별칭 키의 폴더 경고는 남지 않는다(정규화 뒤 같은 키라 unregister 가 지운다).
func TestInheritAliasOverrideWarningGone(t *testing.T) {
	r := renderFM(t, "---\nslides:\n  font-scale: 1.2\n---\n\n# a\n", fm("_metadata.yml", "slides:\n  font_scale: bad\n"))
	if len(r.Warnings) != 0 {
		t.Fatalf("덮인 폴더 경고가 남음: %v", r.Warnings)
	}
}

// 폴더 파일의 중복 키는 문서와 같이 마지막 키가 이긴다(2026-10-31 접근 방식 정리 — 파일을 버리지
// 않는다). 리뷰 1회차 [3] 반례(중복이 우선순위·슬라이드 판단을 무너뜨림)도 이 규칙으로 막힌다.
func TestInheritDuplicateFolderLastKeyWins(t *testing.T) {
	// 리뷰 반례: 폴더 theme 중복 + 문서 theme — 문서가 이긴다(옛 버그: 폴더 마지막 값이 문서를 밀침).
	r := renderFM(t, "---\ntheme: mono\n---\n\n# a\n", fm("_metadata.yml", "theme: classic\ntheme: bold\n"))
	if r.Meta.Theme != "mono" {
		t.Fatalf("문서 값: %q", r.Meta.Theme)
	}
	if len(r.Warnings) != 0 {
		t.Fatalf("중복 키 경고가 남음(파일을 버리지 않는다): %v", r.Warnings)
	}
	if !slices.Equal(r.FolderMeta, []string{"_metadata.yml"}) {
		t.Fatalf("적용 목록: %v", r.FolderMeta)
	}
	// 폴더 안 중복 자체는 마지막 키 승: classic 다음 bold → bold.
	r = renderFM(t, "# a\n", fm("_metadata.yml", "theme: classic\ntheme: bold\n"))
	if r.Meta.Theme != "bold" {
		t.Fatalf("폴더 중복의 마지막 키: %q", r.Meta.Theme)
	}
	// 폴더 slides false/true 중복: 마지막 true 라도 문서에 slides: 가 없으면 슬라이드로 열지 않는다.
	r = renderFM(t, "# plain\n", fm("_metadata.yml", "slides: false\nslides: true\n"))
	if r.Meta.Slides != nil {
		t.Fatalf("문서 slides 없는데 상속됨: %+v", r.Meta.Slides)
	}
	// slides 하위의 별칭 중복도 마지막이 이긴다(ratio 4:3 뒤 aspect 16:9 → 16:9).
	r = renderFM(t, "---\nslides: {}\n---\n\n# a\n", fm("_metadata.yml", "slides:\n  ratio: \"4:3\"\n  aspect: \"16:9\"\n"))
	if r.Meta.Slides == nil || r.Meta.Slides.Ratio != "16:9" {
		t.Fatalf("하위 별칭 중복: %+v", r.Meta.Slides)
	}
}

// 문서가 slides: true(또는 빈 값)로 슬라이드임만 정했다면 하위 설정은 폴더 매핑이 채운다(itda-hyve#474 의 뜻 —
// 조정 정리). slides: false 는 슬라이드 아님을 정한 것이다(폴더 slides 값도 쓰지 않는다).
func TestInheritSlidesTrueFillsFromFolder(t *testing.T) {
	r := renderFM(t, "---\nslides: true\n---\n\n# a\n", fm("_metadata.yml", "slides:\n  font-scale: 0.8\n  ratio: \"4:3\"\n"))
	if r.Meta.Slides == nil || r.Meta.Slides.FontScale != 0.8 || r.Meta.Slides.Ratio != "4:3" {
		t.Fatalf("slides: true + 폴더 매핑: %+v", r.Meta.Slides)
	}
	r = renderFM(t, "---\nslides:\n---\n\n# a\n", fm("_metadata.yml", "slides:\n  font-scale: 0.8\n")) // 빈 값(null)
	if r.Meta.Slides == nil || r.Meta.Slides.FontScale != 0.8 {
		t.Fatalf("slides 빈 값 + 폴더 매핑: %+v", r.Meta.Slides)
	}
	r = renderFM(t, "---\nslides: true\n---\n\n# a\n", fm("_metadata.yml", "slides:\n  font-scale: bad\n")) // 폴더 경고 라벨 유지
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0].Message, "_metadata.yml 2줄: frontmatter slides.font-scale") {
		t.Fatalf("폴더 하위 경고 라벨: %v", r.Warnings)
	}
}

// 파일별 정리(tidyMetaNode)는 폴더 파일이 적용될 때만 쓰는 규칙이다 — 여기는 그 정리 노드를 검사한다:
// 원본 노드를 fill 로 읽은 것(정리 없이)과 정리한 노드를 fill 로 읽은 것이 이 표의 입력에서 Meta·경고
// (줄 번호 포함)가 같다(유효값끼리 중복·별칭 혼합·깨진 값 단독). 정리가 달라지는 입력(유효값→틀린 값·
// null·앞 경고→뒤 유효값, 모르는 키 중복)은 두지 않는다 — 폴더 없는 문서는 정리를 하지 않아 main 과
// 같다(TestNoFolderMatchesMain).
func TestTidyMatchesParser(t *testing.T) {
	for _, tc := range []struct{ name, fm string }{
		{"최상위 중복", "theme: classic\ntheme: bold\n"},
		{"최상위 중복·값 혼합", "toc: true\ntoc: false\ntheme: classic\n"},
		{"slides 중복", "slides: true\nslides: false\n"},
		{"slides 중복 맵", "slides:\n  ratio: \"4:3\"\nslides:\n  font-scale: 1.2\n"},
		{"slides 안 중복", "slides:\n  font-scale: 1.2\n  font-scale: 0.8\n"},
		{"slides 안 별칭 혼합", "slides:\n  font-scale: 1.2\n  fontscale: 0.8\n"},
		{"별칭 순서", "slides:\n  font_scale: 0.8\n  font-scale: 1.2\n"},
		{"code 별칭", "code:\n  line-numbers: true\n  line_numbers: false\n"},
		{"깨진 값", "toc: 안녕\ntheme: classic\n"},
		{"빈 값", "slides:\n"},
	} {
		var root yaml.Node
		if err := yaml.Unmarshal([]byte(tc.fm), &root); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		orig := root.Content[0]
		metaA, metaB := defaultMeta(), defaultMeta()
		wA := metaA.fill(orig, nil)
		wB := metaB.fill(tidyMetaNode(orig, false), nil)
		if !reflect.DeepEqual(metaA, metaB) {
			t.Errorf("%s: Meta 정리 전 %+v / 후 %+v", tc.name, metaA, metaB)
		}
		if !reflect.DeepEqual(wA, wB) {
			t.Errorf("%s: 경고 정리 전 %v / 후 %v", tc.name, wA, wB)
		}
	}
}

// 모르는 키의 중복은 정리가 마지막 하나로 줄인다(지금 파서는 Extra 에 둘 다 쌓고 첫 값을 찾는다 —
// 폴더 상속은 마지막 하나만 넘겨 물려받는 값이 하나가 된다).
func TestTidyExtraDuplicateToOne(t *testing.T) {
	r := renderFM(t, "# a\n", fm("_metadata.yml", "수신: 감사관실\n수신: 기획실\n"))
	if f, ok := r.Meta.Field("수신"); !ok || f.Value != "기획실" {
		t.Fatalf("마지막 키가 안 이김: %+v %v", f, ok)
	}
	if n := len(r.Meta.Extra); n != 1 {
		t.Fatalf("Extra 가 %d개", n)
	}
}

// 파일 사이 합치기에서 키 단위로 얹는 것은 아는 맵 slides·code 뿐이다 — 모르는 키의 맵은 통째로
// 바꾼다(2026-10-31 접근 방식 정리. 깨지는 입력: 폴더 custom {a} + 문서 custom {b} → b 만).
func TestInheritUnknownMapReplaced(t *testing.T) {
	r := renderFM(t, "---\ncustom:\n  b: 2\n---\n\n# a\n", fm("_metadata.yml", "custom:\n  a: 1\n"))
	if f, ok := r.Meta.Field("custom"); !ok || strings.Contains(f.Value, "a: 1") || !strings.Contains(f.Value, "b: 2") {
		t.Fatalf("모르는 맵이 키 단위로 합쳐짐: %+v", f)
	}
	r = renderFM(t, "# a\n", fm("_metadata.yml", "custom:\n  a: 1\n"))
	if f, _ := r.Meta.Field("custom"); !strings.Contains(f.Value, "a: 1") {
		t.Fatalf("폴더의 모르는 맵이 안 물려받음: %+v", f)
	}
}

// 목록 안 매핑의 중복 키는 건드리지 않는다 — 파서가 보는 수준(최상위·slides/code 바로 아래) 밖이다.
// 파일을 버리지도 않는다(리뷰 2회차 [2] 반례 이전).
func TestInheritSequenceMappingDuplicateNotDisturbed(t *testing.T) {
	r := renderFM(t, "# body\n", fm("_metadata.yml", "theme: mono\nauthor:\n  - name: first\n    name: second\n"))
	if r.Meta.Theme != "mono" || !slices.Equal(r.Meta.Author, []string{"first"}) || len(r.Warnings) != 0 {
		t.Fatalf("meta=%+v warnings=%v", r.Meta, r.Warnings)
	}
	if !slices.Equal(r.FolderMeta, []string{"_metadata.yml"}) {
		t.Fatalf("파일이 버려짐: %v", r.FolderMeta)
	}
}

// 문서의 slides 중복은 마지막 값이 이긴다 — true 다음 false 면 슬라이드가 아니고, true 다음 맵이면
// 그 맵이 문서 값이다(리뷰 2회차 [1] 반례 이전).
func TestInheritSlidesLastKeyWins(t *testing.T) {
	r := renderFM(t, "---\nslides: true\nslides: false\n---\n\n# a\n", fm("_metadata.yml", "slides:\n  font-scale: 0.8\n"))
	if r.Meta.Slides != nil {
		t.Fatalf("마지막 false 가 버려짐: %+v", r.Meta.Slides)
	}
	r = renderFM(t, "---\nslides: true\nslides:\n  font-scale: 1.2\n---\n\n# a\n", fm("_metadata.yml", "slides:\n  font-scale: 0.8\n"))
	if r.Meta.Slides == nil || r.Meta.Slides.FontScale != 1.2 {
		t.Fatalf("마지막 문서 맵이 버려짐: %+v", r.Meta.Slides)
	}
}

// 같은 파일 안 두 slides 맵을 합치지 않는다(마지막 맵만) — 폴더 파일이 있든 없든 문서 단독 해석과
// 같다(리뷰 2회차 [1] 세 번째 반례 이전).
func TestInheritDocDuplicateSemanticsStable(t *testing.T) {
	src := "---\nslides:\n  ratio: \"4:3\"\nslides:\n  font-scale: 1.2\n---\n\n# a\n"
	plain := renderFM(t, src)
	folder := renderFM(t, src, fm("_metadata.yml", "theme: mono\n"))
	if plain.Meta.Slides.Ratio != "" || folder.Meta.Slides.Ratio != "" {
		t.Fatalf("마지막 slides 맵만 써야 함: 단독 %+v / 폴더 %+v", plain.Meta.Slides, folder.Meta.Slides)
	}
	if folder.Meta.Theme != "mono" {
		t.Fatalf("폴더 theme: %q", folder.Meta.Theme)
	}
}

// 별칭 정규화는 정확한 경로(최상위, slides·code 바로 아래)에서만 한다 — slides.custom 안의
// aspect/ratio 는 별칭이 아니라 모르는 키다(리뷰 2회차 [3] 반례 이전).
func TestInheritAliasOnlyDirectChildren(t *testing.T) {
	r := renderFM(t, "---\nslides: true\n---\n\n# a\n", fm("_metadata.yml", "theme: mono\nslides:\n  custom:\n    aspect: x\n    ratio: y\n"))
	if r.Meta.Theme != "mono" {
		t.Fatalf("theme 까지 사라짐: %+v %v", r.Meta, r.Warnings)
	}
	// 문서가 slides: true 라 슬라이드는 열리지만 custom 은 모르는 키 경고가 난다.
	var msg string
	for _, w := range r.Warnings {
		msg += w.Message
	}
	if !strings.Contains(msg, "slides.custom 는 모르는 키") {
		t.Fatalf("custom 경고: %v", r.Warnings)
	}
}

// 경고 계약(itda-hyve#504): 폴더 파일이 적용돼도 문서 frontmatter 의 경고는 폴더가 없을 때와 같다 — 같은 키의
// 앞 값 경고가 남는다(정리 규칙이 값은 마지막 키로 정해도). 값은 정리 규칙대로 마지막 키다.
func TestTidyKeepsEarlierDuplicateWarnings(t *testing.T) {
	r := renderFM(t, "---\ntoc: 안녕\ntoc: true\n---\n\n# a\n", fm("_metadata.yml", "theme: classic\n"))
	if !r.Meta.TOC || len(r.Warnings) != 1 || r.Warnings[0].Line != 2 {
		t.Fatalf("유효값은 마지막 키, 앞 값의 경고는 문서 2줄로 남는다: %+v %v", r.Meta, r.Warnings)
	}
	r = renderFM(t, "---\ntoc: 안녕\ntoc: true\n---\n\n# a\n")
	if !r.Meta.TOC || len(r.Warnings) != 1 || r.Warnings[0].Line != 2 {
		t.Fatalf("폴더 없는 중복: %+v %v", r.Meta, r.Warnings)
	}
}

// 경고 계약(itda-hyve#504): 문서 경고는 폴더가 있을 때와 없을 때 JSON 까지 같다 — 무관한 폴더 파일(모르는 키
// 하나)을 더해 견준다. 입력은 리뷰 3회차 기준판 비교 표의 사례와 별칭·모르는 하위 키·틀린 그룹 값이다.
func TestFolderWarningsMatchNoFolder(t *testing.T) {
	warned := 0
	for _, fmText := range []string{
		"toc: true\ntoc: bad",
		"toc: bad\ntoc: true",
		"toc: true\ntoc:",
		"slides:\n  font-scale: 1.2\n  font-scale: bad",
		"slides:\n  font_scale: bad",
		"slides:\n  fontscale: 0.1\n  font-scale: 2",
		"slides:\n  aspect: \"5:4\"\n  custom: 1",
		"code:\n  line_numbers: 켬\n  line-numbers: false",
		"code:\n  linenumbers: 켬",
		"slides: 발표\ncode: 켬",
		"slides: true\nslides: false",
		"수신: 감사관실\n수신: 기획실\ntoc-depth: 9",
		"slides:\n  level: 0\nslides:\n  transition: wipe",
	} {
		src := "---\n" + fmText + "\n---\n\n# a\n"
		none := renderFM(t, src)
		with := renderFM(t, src, fm("../_metadata.yml", "수신: 폴더\n"))
		if len(with.FolderMeta) != 1 {
			t.Fatalf("%q: 폴더 파일이 적용되지 않음", fmText)
		}
		a, _ := json.Marshal(none.Warnings)
		b, _ := json.Marshal(with.Warnings)
		if string(a) != string(b) {
			t.Errorf("%q: 문서 경고가 폴더 유무로 달라짐\n없음 %s\n있음 %s", fmText, a, b)
		}
		if len(none.Warnings) > 0 {
			warned++
		}
	}
	if warned < 10 { // 사례 대부분이 경고를 내야 계약을 시험한다(경고 없는 사례는 값만 다른 꼴)
		t.Fatalf("경고를 낸 사례가 %d 개뿐", warned)
	}
}

// 경고 계약(itda-hyve#504): 폴더 파일의 값 경고는 그 파일에 적힌 표기(별칭 그대로)·그 파일의 줄로 난다.
// Key 는 정규 키 경로다(편집 화면의 키별 풀이 정규 경로로 본다).
func TestFolderWarningKeepsWrittenAlias(t *testing.T) {
	r := renderFM(t, "---\nslides: true\n---\n\n# a\n", fm("_metadata.yml", "theme: classic\nslides:\n  font_scale: bad\n"))
	if len(r.Warnings) != 1 {
		t.Fatalf("경고 1개여야: %v", r.Warnings)
	}
	w := r.Warnings[0]
	if w.Line != 0 || w.Key != "slides.font-scale" || !strings.Contains(w.Message, "_metadata.yml 3줄: frontmatter slides.font_scale:") {
		t.Fatalf("폴더 별칭 경고: %+v", w)
	}
	// 같은 파일 안 앞 값(font-scale: 0.8)은 마지막 키(font_scale: bad)가 이겨 경고는 마지막 키 하나다.
	r = renderFM(t, "---\nslides: true\n---\n\n# a\n", fm("_metadata.yml", "slides:\n  font-scale: 0.8\n  font_scale: bad\n"))
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0].Message, "_metadata.yml 3줄: frontmatter slides.font_scale:") || r.Meta.Slides.FontScale != 0 {
		t.Fatalf("폴더 안 중복의 마지막 키: %+v %v", r.Meta.Slides, r.Warnings)
	}
	// 가까운 폴더가 별칭으로 먼 폴더를 덮으면 경고는 가까운 파일의 표기·줄이다.
	r = renderFM(t, "---\nslides: true\n---\n\n# a\n",
		fm("../_metadata.yml", "slides:\n  font-scale: 0.8\n"), fm("_metadata.yml", "\nslides:\n  fontscale: 9\n"))
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0].Message, "_metadata.yml 3줄: frontmatter slides.fontscale:") || strings.HasPrefix(r.Warnings[0].Message, "../") {
		t.Fatalf("가까운 폴더 별칭 경고: %v", r.Warnings)
	}
	// 문서의 별칭 경고도 적힌 표기·문서 줄이다(폴더 없을 때와 같다).
	r = renderFM(t, "---\nslides:\n  font_scale: bad\n---\n\n# a\n", fm("_metadata.yml", "slides:\n  font-scale: 0.8\n"))
	if len(r.Warnings) != 1 || r.Warnings[0].Line != 3 || !strings.Contains(r.Warnings[0].Message, "slides.font_scale") {
		t.Fatalf("문서 별칭 경고: %v", r.Warnings)
	}
}

// 모르는 하위 키 경고는 키 노드의 줄로 나므로, 병합에서 덮을 때 키·값 노드를 함께 승자 것으로
// 바꾼다 — 파일 이름과 줄이 서로 다른 원본에서 섞이지 않는다(리뷰 3회차 [1] 반례 이전).
func TestInheritUnknownKeyWarningLines(t *testing.T) {
	// 문서의 slides.custom(문서 6줄)이 폴더 것을 이기면 경고도 문서 6줄.
	r := renderFM(t, "---\nslides:\n\n\n\n  custom: document\n---\n\n# body\n", fm("_metadata.yml", "slides:\n  custom: folder\n"))
	if len(r.Warnings) != 1 || r.Warnings[0].Line != 6 {
		t.Fatalf("문서 6줄이어야: %v", r.Warnings)
	}
	// 가까운 폴더(5줄)가 먼 폴더(2줄)를 이기면 경고 문구도 가까운 파일 5줄.
	r = renderFM(t, "---\nslides: true\n---\n\n# body\n",
		fm("../_metadata.yml", "slides:\n  custom: far\n"), fm("_metadata.yml", "slides:\n\n\n\n  custom: near\n"))
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0].Message, "_metadata.yml 5줄:") {
		t.Fatalf("가까운 폴더 5줄이어야: %v", r.Warnings)
	}
	// 아는 키 값 경고(vv.Line)와 본문 data-line 도 밀리지 않는다(리뷰 3회차 확인 시험).
	r = renderFM(t, "---\nslides:\n\n\n  font-scale: bad\n---\n\n# body\n", fm("_metadata.yml", "slides:\n  font-scale: 0.8\n"))
	if len(r.Warnings) != 1 || r.Warnings[0].Line != 5 {
		t.Fatalf("문서 5줄이어야: %v", r.Warnings)
	}
	if !strings.Contains(r.HTML, `data-line="8"`) {
		t.Fatalf("본문 줄 밀림: %s", r.HTML)
	}
}

// 폴더 파일이 하나도 적용되지 않으면(경로 없음·파일 없음·모두 읽기 실패) frontmatter 처리는
// main(e65d27c) 과 완전히 같다 — 정리를 돌리지 않는다. 기대값은 리뷰 3회차의 기준판 비교 표(main 열).
func TestNoFolderMatchesMain(t *testing.T) {
	render := func(fm string) Result {
		r, err := Render([]byte("---\n"+strings.ReplaceAll(fm, "·", "\n")+"\n---\n\n# a\n"), Options{})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	// toc: true 다음 toc: bad — main: TOC=true·마지막 값 경고 1개.
	r := render("toc: true·toc: bad")
	if !r.Meta.TOC || len(r.Warnings) != 1 {
		t.Fatalf("①: %+v %v", r.Meta, r.Warnings)
	}
	// toc: true 다음 toc:(null) — main: TOC=true·경고 없음.
	r = render("toc: true·toc:")
	if !r.Meta.TOC || len(r.Warnings) != 0 {
		t.Fatalf("②: %+v %v", r.Meta, r.Warnings)
	}
	// toc: bad 다음 toc: true — main: TOC=true·앞 값 경고 1개(문서 2줄).
	r = render("toc: bad·toc: true")
	if !r.Meta.TOC || len(r.Warnings) != 1 || r.Warnings[0].Line != 2 {
		t.Fatalf("③: %+v %v", r.Meta, r.Warnings)
	}
	// slides {font-scale 1.2} 다음 {font-scale bad} — main: FontScale=1.2·경고 1개.
	r = render("slides:·  font-scale: 1.2·  font-scale: bad")
	if r.Meta.Slides == nil || r.Meta.Slides.FontScale != 1.2 || len(r.Warnings) != 1 {
		t.Fatalf("④: %+v %v", r.Meta.Slides, r.Warnings)
	}
	// code {line-numbers false} 다음 빈 line-numbers: — main: LineNumbers=false.
	r = render("code:·  line-numbers: false·  line-numbers:")
	if r.Meta.Code == nil || r.Meta.Code.LineNumbers == nil || *r.Meta.Code.LineNumbers {
		t.Fatalf("⑤: %+v", r.Meta.Code)
	}
	// 모르는 키 중복 — main: Extra 두 개·Field 는 첫 값.
	r = render("수신: 감사관실·수신: 기획실")
	if n := len(r.Meta.Extra); n != 2 {
		t.Fatalf("⑥ Extra %d개", n)
	}
	if f, _ := r.Meta.Field("수신"); f.Value != "감사관실" {
		t.Fatalf("⑥ Field 첫 값: %+v", f)
	}
	// 단독 별칭 값 오류 — main: 경고 이름 slides.font_scale(정규화 안 함).
	r = render("slides:·  font_scale: bad")
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0].Message, "slides.font_scale") {
		t.Fatalf("⑦: %v", r.Warnings)
	}
}
