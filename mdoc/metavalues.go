package mdoc

import (
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// 편집 화면(#474 ②)이 쓰는 값 뽑기 — 폼이 보여 주는 값·출처·검사는 모두 ① 결과(정리·합치기·fill)
// 그대로다. 별도의 상속 계산을 두지 않는다(같은 종류 어긋남이 두 리뷰 회차에서 났다).

// MetaValues 는 합쳐진 Meta 를 정규 키 경로 → 값 글자로 낸다. 문서·폴더 어디에서도 정하지 않아
// 기본값으로 남은 키는 빠진다. 참·거짓 키는 거짓이 기본이라 「정함」과 구분이 없어 참일 때만 내고,
// toc-depth·papersize·ratio 처럼 기본값과 같은 값은 정하지 않은 것으로 뺀다(Meta 는 명시와 기본을
// 구분하지 않는다 — 화면 표시 「기본 3」이 정확해진다). code.line-numbers 는 껍데기가 포인터라
// false 도 「정한 값」으로 낸다. 스키마 시험이 파서의 열거·범위와 Values 를 맞출 때도 쓴다.
func MetaValues(m Meta) map[string]string {
	out := map[string]string{}
	put := func(k, v string) {
		if v != "" {
			out[k] = v
		}
	}
	put("title", m.Title)
	put("subtitle", m.Subtitle)
	if len(m.Author) > 0 {
		out["author"] = strings.Join(m.Author, ", ")
	}
	put("date", m.Date)
	put("lang", m.Lang)
	if m.TOC {
		out["toc"] = "true"
	}
	if m.TOCDepth != 3 {
		out["toc-depth"] = strconv.Itoa(m.TOCDepth)
	}
	if m.NumberSections {
		out["number-sections"] = "true"
	}
	if m.PaperSize != "a4" {
		out["papersize"] = m.PaperSize
	}
	put("theme", m.Theme)
	if m.Cover {
		out["cover"] = "true"
	}
	put("doc-no", m.DocNo)
	put("version", m.Version)
	put("classification", m.Classification)
	put("organization", m.Organization)
	put("list-style", m.ListStyle)
	if s := m.Slides; s != nil {
		put("slides.ratio", s.Ratio)
		if s.Level > 0 {
			out["slides.level"] = strconv.Itoa(s.Level)
		}
		put("slides.transition", s.Transition)
		if s.Incremental {
			out["slides.incremental"] = "true"
		}
		if s.FontScale > 0 {
			out["slides.font-scale"] = strconv.FormatFloat(s.FontScale, 'g', -1, 64)
		}
	}
	if c := m.Code; c != nil && c.LineNumbers != nil {
		out["code.line-numbers"] = strconv.FormatBool(*c.LineNumbers)
	}
	return out
}

// TidyFolderMetaNode 는 파일별 정리(tidyMetaNode — 같은 키는 마지막 승)를 마치고 별칭 키를 정규
// 이름으로 바꾼 매핑 노드 사본을 돌려준다. 편집 화면(#474 ②)의 쓰기가 ① 과 같은 정리 규칙으로 고치기
// 시작하게 한다(원본 노드는 바뀌지 않는다). ① 의 렌더는 이름을 바꾸지 않는다(경고가 적힌 표기로 — #504).
func TidyFolderMetaNode(n *yaml.Node) *yaml.Node { return tidyMetaNode(n, true) }

// FrontmatterKey 는 편집 화면이 보는 ① 결과의 키별 풀이다.
type FrontmatterKey struct {
	// Value 는 출처 노드의 원문 값 글자다(스칼라·목록은 이은 글자). 무시 여부와 무관하게 그 출처가
	// 적은 것이다 — 폼 표시와 저장 검사의 왕복 비교에 쓴다. 출처가 없으면 "".
	Value string
	// Applied 는 ① 이 그 값을 그대로 반영했는가다. 범위 밖·틀린 꼴로 무시되었거나(fill 의 값 경고),
	// 모르는 키이거나, 문서의 그룹 스칼러(slides: false·code: false)가 하위를 억제했으면 거짓.
	Applied bool
	// FromDoc 은 문서 frontmatter 가 이 키를 정했다(그룹 스칼러 억제로 하위를 정한 것도 포함).
	FromDoc bool
	// FromRel 은 이 키가 든 폴더 파일의 문서 기준 상대 경로다(문서가 이긴 키는 빈글자).
	FromRel string
}

// ResolveFrontmatter 는 문서와 폴더 파일을 ① 렌더와 한 길(frontmatterCore)로 풀어 키별 결과를
// 낸다(#474 ② — 폼의 「적용될 값·출처」와 저장 전 검사가 이 하나를 쓴다. 별도 재구현을 두지
// 않는다). 돌려주는 것: 합친 Meta(테마 고르기·테마 defaults 판단에 쓴다), 키별 풀(아는 키 경로만),
// ① 의 경고. 슬라이드 판단 규칙(문서 자신의 slides 값이 정한다)도 렌더와 같다 — 문서가 slides 를
// 정하지 않았으면 폴더 slides 하위 전체가 억제되고, slides: false 면 통째로 이기고, true 면 하위를
// 폴더 맵이 채운다. code: false 도 같이 하위를 억제한다(fill 이 통째로 이긴다).
func ResolveFrontmatter(src []byte, folder []FolderMeta) (Meta, map[string]FrontmatterKey, []Warning) {
	meta, keys, _, warns, _, _ := frontmatterCore(src, folder)
	return meta, keys, warns
}

// nodeValueText 는 스칼라(빈 값 null 은 제외)·목록(요소를 공백으로 이은 글자)의 값이다.
func nodeValueText(v *yaml.Node) string {
	switch v.Kind {
	case yaml.ScalarNode:
		if isNull(v) {
			return ""
		}
		return v.Value
	case yaml.SequenceNode:
		var parts []string
		for _, it := range v.Content {
			if it.Kind != yaml.ScalarNode {
				return ""
			}
			parts = append(parts, it.Value)
		}
		return strings.Join(parts, " ")
	}
	return ""
}

// NormalizeAliasKey 는 키를 묶음 경로(group ""·"slides"·"code")의 정규 이름으로 바꾼다(별칭이
// 아니면 그대로). 편집 화면(#474 ②)이 파일의 옛 표기를 정본 이름으로 다루기 위해 쓴다.
func NormalizeAliasKey(group, key string) string { return normalizeAlias(group, key) }

// schemaKnows 는 group 안의 정본 키(또는 그 별칭)가 스키마 표에 있는지다.
func schemaKnows(group, key string) bool {
	for _, k := range FrontmatterSchema {
		if k.Group != group {
			continue
		}
		if k.Key == key || slices.Contains(k.Aliases, key) {
			return true
		}
	}
	return false
}
