package mdoc

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Meta 는 frontmatter 다(1.2). 아는 키는 필드로, 모르는 키는 Extra 에 순서대로 둔다.
// 값은 글자 그대로다 — HTML 로 해석하지 않는다. 화면에 낼 때는 받는 쪽이 이스케이프한다.
type Meta struct {
	// Present 는 frontmatter 블록이 있었고 읽혔는지다.
	Present bool

	Title    string
	Subtitle string
	Author   []string
	Date     string // YAML 이 날짜로 읽어도 적힌 글자 그대로 둔다
	Lang     string

	TOC            bool
	TOCDepth       int // 기본 3
	NumberSections bool
	PaperSize      string // a4(기본) · a4-landscape

	// 여기부터 Pandoc 에 없는 키
	Theme          string
	Cover          bool
	DocNo          string
	Version        string
	Classification string
	Organization   string
	// ListStyle 은 목록 기호 사다리다(itda-hyve#141, liststyle.go). "" 이면 테마 기본.
	ListStyle string

	// Slides 는 frontmatter slides: 다(1.12). 있으면 문서 창이 슬라이드 미리보기로 연다. 없으면 nil.
	Slides *SlidesMeta

	// Code 는 frontmatter code: 다(itda-hyve#435 — 코드 블록 표시). 없으면 nil.
	Code *CodeMeta

	// Extra 는 모르는 키다. 공문의 수신·참조·시행일처럼 테마가 표지·머리글에 쓴다.
	Extra []MetaField
}

// SlidesMeta 는 발표 설정이다. 모두 선택이고, 0 값은 "정하지 않음" 이다(화면이 기본을 쓴다).
type SlidesMeta struct {
	Ratio       string `json:"ratio"`       // "16:9"(기본) · "4:3"
	Level       int    `json:"level"`       // 슬라이드를 여는 제목 단계(1~6). 0 이면 문서에서 정한다(Result.SlideLevel)
	Transition  string `json:"transition"`  // none(기본) · fade · slide
	Incremental bool   `json:"incremental"` // 모든 목록을 차례로
	// FontScale 은 슬라이드 글자 배율(0.6~1.5)이다. 0 이면 기본(0.9 — base.css --slide-scale). 1 이면 글자 토큰 그대로(본문 23pt)
	FontScale float64 `json:"font_scale"`
}

// CodeMeta 는 코드 블록 표시 설정이다(itda-hyve#435). 모두 선택이고, nil 은 "정하지 않음" 이다(화면이 기본을 쓴다).
type CodeMeta struct {
	// LineNumbers 는 코드 블록 줄 번호다. nil 이면 정하지 않음(기본 켬), false 면 끔. &true 는 정하지 않음과 같은 결과다.
	LineNumbers *bool `json:"line_numbers,omitempty"`
}

// MetaField 는 모르는 키 하나다.
type MetaField struct {
	Key string
	// Value 는 글자 값이다. 목록이면 ", " 로 이은 것, 그 밖의 구조면 YAML 글자다.
	Value string
	// List 는 값이 글자 목록일 때만 채운다.
	List []string
}

// Field 는 모르는 키의 값을 찾는다.
func (m Meta) Field(key string) (MetaField, bool) {
	for _, f := range m.Extra {
		if f.Key == key {
			return f, true
		}
	}
	return MetaField{}, false
}

func defaultMeta() Meta {
	return Meta{TOCDepth: 3, PaperSize: "a4"}
}

// parseFrontmatter 는 맨 앞의 --- 블록을 읽고 폴더 기본값(_metadata.yml)을 합친다(itda-hyve#474 — folder 는
// 먼 폴더부터). 돌려주는 본문은 frontmatter 자리를 빈 줄로 채운 것이다 — 줄 번호가 원문과 같아야
// data-line·경고가 맞는다. 넷째 값은 적용된 폴더 파일의 문서 기준 상대 경로다(가까운 것부터).
// 폴더 파일이 적용될 때만 문서도 파일별 정리(tidyMetaNode)·파일 사이 합치기(mergeFolderInto)를 하고,
// 하나도 적용되지 않으면(경로 없음·파일 없음·모두 읽기 실패) 정리 없이 지금까지 그대로 fill 한다 —
// 폴더 없는 문서의 처리는 main(e65d27c) 과 완전히 같다.
// 문서의 YAML 이 깨졌거나 키: 값 모양이 아니면 폴더 상속 없이 지금까지처럼 본문으로 렌더한다(폴더 파일
// 자체의 오류 경고는 그대로 낸다 — 그 파일을 고쳐야 하니까).
func parseFrontmatter(src []byte, folder []FolderMeta) (Meta, []byte, []Warning, []string) {
	meta, _, body, warns, applied, _ := frontmatterCore(src, folder)
	return meta, body, warns, applied
}

// frontmatterCore 는 frontmatter 풀이의 하나뿐인 길이다: 폴더 합치기(mergeFolderMeta) → 문서 노드
// 읽기 → (폴더가 적용된 경우) 정리·slides 판단·합치기 → fill. 렌더(parseFrontmatter)와 편집 화면
// (ResolveFrontmatter, itda-hyve#474 ②)가 함께 부른다 — 별도 재구현이 어긋난 값을 내지 않게. 편집 화면용 키별
// 풀(keys)도 여기서 낸다 — 출처는 srcs, 무시는 fill 경고의 Key(Warning.Key), 그룹 억제는 문서의 스칼라
// false 로 본다.
func frontmatterCore(src []byte, folder []FolderMeta) (Meta, map[string]FrontmatterKey, []byte, []Warning, []string, bool) {
	meta := defaultMeta()
	keys := map[string]FrontmatterKey{}
	merged, srcs, warns, applied := mergeFolderMeta(folder)

	var docRaw *yaml.Node // 문서 frontmatter 매핑 노드(블록이 없으면 nil) — 정리 전 원본
	yamlSrc, end, ok := splitFrontmatter(src)
	if ok {
		var root yaml.Node
		if err := yaml.Unmarshal(yamlSrc, &root); err != nil {
			return meta, keys, src, append(warns, Warning{Line: 1, Code: WarnFrontmatterInvalid,
				Message: "frontmatter YAML 을 읽지 못해 본문으로 렌더함: " + yamlErr(err)}), nil, false
		}
		switch {
		case root.Kind == 0 || len(root.Content) == 0:
			// 빈 블록(--- 다음 바로 ---)은 정보 없는 frontmatter 다
		case root.Content[0].Kind == yaml.MappingNode:
			docRaw = root.Content[0]
		default:
			return meta, keys, src, append(warns, Warning{Line: 1, Code: WarnFrontmatterInvalid,
				Message: "frontmatter 가 키: 값 모양이 아니라 본문으로 렌더함"}), nil, false
		}
		meta.Present = true
	}

	body := func() []byte { // frontmatter 자리를 빈 줄로 채운 본문 — 줄 번호가 원문과 같아야 한다
		b := make([]byte, 0, len(src))
		b = append(b, bytes.Repeat([]byte("\n"), bytes.Count(src[:end], []byte("\n")))...)
		return append(b, src[end:]...)
	}

	if merged == nil { // 적용된 폴더 파일이 없다 — 정리 없이 지금까지 그대로
		if docRaw != nil {
			warns = append(warns, meta.fill(docRaw, srcs)...)
			collectKeys(keys, docRaw, srcs, warns, nil)
		}
		return meta, keys, body(), warns, nil, meta.Present
	}

	var docNode *yaml.Node
	if docRaw != nil {
		docNode = tidyMetaNode(docRaw, false)
	}

	// 슬라이드로 열지는 정리된 문서 노드의 slides 값 하나로 정한다(정리 뒤에는 키가 하나뿐이다).
	//   없음 → 정하지 않은 것 — 폴더 slides 값은 버린다.
	//   false → 「슬라이드 아님」을 정한 것 — 폴더 slides 값도 통째로 이긴다(문서 값은 그대로 — fill 이 nil).
	//   true·null(빈 값) → 슬라이드임만 정한 것 — 폴더 맵이 있으면 하위 설정을 폴더 맵이 채운다(문서의
	//     스칼라를 빼고 폴더 맵을 살린다 — 소스도 폴더로 남아 폴더 파일 경고가 난다). 폴더 맵이 없으면
	//     문서 스칼라 그대로(fill 이 빈 SlidesMeta 를 만든다 — 단일 문서 동작 그대로).
	//   틀린 스칼라(예: slides: 발표)·맵 → 문서 값 그대로(fill 이 경고를 내거나 맵을 폴더 맵 위에 얹는다).
	if sl := docSlidesValue(docNode); sl != nil {
		var b bool
		slideOn := isNull(sl) || sl.Kind == yaml.ScalarNode && sl.Decode(&b) == nil && b
		slideOff := sl.Kind == yaml.ScalarNode && !isNull(sl) && sl.Decode(&b) == nil && !b
		if slideOn && folderHasSlideMap(merged) {
			dropKey(docNode, "slides")
		} else if slideOff {
			dropKey(merged, "slides") // 문서가 슬라이드 아님을 정했다 — 폴더 하위 값을 억제한다(suppressedGroups 도 참고)
		}
	} else {
		if w, ok := folderSlidesUnused(merged, srcs); ok { // 버리는 폴더 slides 값을 알린다(itda-hyve#543)
			warns = append(warns, w)
		}
		dropKey(merged, "slides")
	}

	final := mergeFolderInto(merged, docNode) // 문서 노드가 마지막에 이긴다(여기서 merged 는 nil 이 아니다)
	if docNode != nil {
		unregisterSrc(srcs, docNode)
	}
	finalWarns := meta.fill(final, srcs)
	collectKeys(keys, final, srcs, finalWarns, suppressedGroups(docNode))
	// 경고 계약(itda-hyve#504, inherit.go 머리): 폴더 값 경고(Line 0)는 최종 값으로 쓰인 폴더 키에서만, 문서
	// 경고는 문서 원본 노드를 폴더 없이 따로 풀어 — 폴더가 없을 때와 같은 경고(앞 값·적은 표기·줄)다.
	// 최종 fill 의 문서 줄 경고는 이 원본 풀이의 부분집합이라 버린다(문서 노드는 같은 함수로 검사된다).
	for _, w := range finalWarns {
		if w.Line == 0 {
			warns = append(warns, w)
		}
	}
	if docRaw != nil {
		scratch := defaultMeta()
		warns = append(warns, scratch.fill(docRaw, nil)...)
	}
	return meta, keys, body(), warns, inheritFolderList(applied), meta.Present
}

// suppressedGroups 는 문서가 스칼라 false 로 통째로 정한 그룹이다(하위 키를 억제 — 폴더 값이
// 쓰이지 않는다). slides 는 위 판단에서, code 는 문서 스칼라가 폴더 맵을 통째로 이기는 ① 규칙에서 온다.
func suppressedGroups(docNode *yaml.Node) map[string]bool {
	out := map[string]bool{}
	for _, g := range []string{"slides", "code"} {
		v := docGroupValue(docNode, g)
		if v == nil || v.Kind != yaml.ScalarNode || isNull(v) {
			continue
		}
		var b bool
		if v.Decode(&b) == nil && !b {
			out[g] = true
		}
	}
	return out
}

// docGroupValue 는 매핑 노드의 그룹(slides·code) 값이다(없으면 nil).
func docGroupValue(n *yaml.Node, group string) *yaml.Node {
	if n == nil {
		return nil
	}
	if j := mappingIndex(n.Content, group); j >= 0 {
		return n.Content[j+1]
	}
	return nil
}

// collectKeys 는 풀이가 끝난 노드에서 편집 화면용 키별 풀을 채운다(itda-hyve#474 ②). 노드는 ① 이 fill 한
// 것과 같은 노드다(폴더가 적용되지 않았으면 정리 전 문서 노드). 무시된 값은 fill 경고의 Key 로
// 가려내고, 억제 그룹의 하위 키는 문서가 정한 것(값 없음)으로 둔다.
func collectKeys(keys map[string]FrontmatterKey, n *yaml.Node, srcs metaSrc, warns []Warning, suppressed map[string]bool) {
	if n == nil {
		return
	}
	ignored := map[string]bool{}
	for _, w := range warns {
		if w.Key != "" && w.Code == WarnFrontmatterValue {
			ignored[w.Key] = true
		}
	}
	set := func(path string, v *yaml.Node) {
		rel := srcs[path]
		keys[path] = FrontmatterKey{Value: nodeValueText(v), Applied: !ignored[path], FromDoc: rel == "", FromRel: rel}
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if knownGroups[k.Value] {
			if v.Kind != yaml.MappingNode {
				continue // 그룹 스칼러 — 하위 키는 없다
			}
			for j := 0; j+1 < len(v.Content); j += 2 {
				set(k.Value+"."+normalizeAlias(k.Value, v.Content[j].Value), v.Content[j+1]) // 정규 키 경로(표기는 파일 그대로)
			}
			continue
		}
		if !schemaKnows("", k.Value) {
			continue
		}
		set(k.Value, v)
	}
	for g := range suppressed { // 억제된 그룹의 하위 — 문서가 정했다(끔). 값은 없다(적용되는 하위 값이 아니다)
		for _, k := range FrontmatterSchema {
			if k.Group == g && k.Input != "" {
				keys[g+"."+k.Key] = FrontmatterKey{Applied: true, FromDoc: true}
			}
		}
	}
}

// dropKey 는 매핑에서 키 쌍 하나를 뺀다(정리 뒤에는 키가 하나뿐이다).
func dropKey(n *yaml.Node, key string) {
	if n == nil {
		return
	}
	if j := mappingIndex(n.Content, key); j >= 0 {
		n.Content = append(n.Content[:j:j], n.Content[j+2:]...)
	}
}

// splitFrontmatter 는 첫 줄이 --- 이고 뒤에 --- 또는 ... 줄이 있을 때 그 사이를 돌려준다.
// 닫는 줄이 없으면 frontmatter 가 아니다(문서 첫 줄의 가로줄일 수 있다).
func splitFrontmatter(src []byte) (yamlSrc []byte, end int, ok bool) {
	first, rest, found := bytes.Cut(src, []byte("\n"))
	if !found || string(bytes.TrimRight(first, " \t")) != "---" {
		return nil, 0, false
	}
	off := len(first) + 1
	for len(rest) > 0 {
		line, next, _ := bytes.Cut(rest, []byte("\n"))
		t := string(bytes.TrimRight(line, " \t"))
		lineEnd := off + len(line)
		if t == "---" || t == "..." {
			if lineEnd < len(src) {
				lineEnd++ // 줄 끝 \n 까지
			}
			return src[len(first)+1 : off], lineEnd, true
		}
		off = lineEnd + 1
		rest = next
	}
	return nil, 0, false
}

// yamlErr 는 yaml.v3 오류에서 "yaml: " 머리를 뗀다.
func yamlErr(err error) string {
	return strings.TrimPrefix(err.Error(), "yaml: ")
}

// fill 은 매핑 노드를 필드로 옮긴다. 형식이 틀린 아는 키는 경고하고 기본값을 둔다.
// srcs 는 폴더 파일에서 온 키의 출처다(inherit.go) — 그 키의 경고는 문서 줄 번호 대신 그 파일의
// 상대 경로와 줄로 낸다(Line 0). 문서에서 온 키는 지금처럼 문서 줄 번호다.
func (m *Meta) fill(n *yaml.Node, srcs metaSrc) []Warning {
	var warns []Warning
	bad := func(k, v *yaml.Node, want string) {
		if p, ok := folderWarnPrefix(srcs, k.Value, v); ok {
			warns = append(warns, Warning{Line: 0, Code: WarnFrontmatterValue, Key: k.Value,
				Message: p + fmt.Sprintf("frontmatter %s: %s 이어야 해서 무시함", k.Value, want)})
			return
		}
		warns = append(warns, Warning{Line: v.Line + 1, Code: WarnFrontmatterValue, Key: k.Value,
			Message: fmt.Sprintf("frontmatter %s: %s 이어야 해서 무시함", k.Value, want)})
	}
	str := func(k, v *yaml.Node, dst *string) {
		if s, ok := scalar(v); ok {
			*dst = s
			return
		}
		bad(k, v, "글자")
	}
	boolean := func(k, v *yaml.Node, dst *bool) {
		if isNull(v) {
			return
		}
		var b bool
		if v.Kind != yaml.ScalarNode || v.Decode(&b) != nil {
			bad(k, v, "true 또는 false")
			return
		}
		*dst = b
	}

	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		switch k.Value {
		case "title":
			str(k, v, &m.Title)
		case "subtitle":
			str(k, v, &m.Subtitle)
		case "author":
			if list, ok := authors(v); ok {
				m.Author = list
			} else {
				bad(k, v, "글자 또는 글자 목록")
			}
		case "date":
			str(k, v, &m.Date)
		case "lang":
			str(k, v, &m.Lang)
		case "toc":
			boolean(k, v, &m.TOC)
		case "toc-depth":
			var d int
			if v.Kind != yaml.ScalarNode || v.Decode(&d) != nil || d < 1 || d > 6 {
				bad(k, v, "1~6 의 정수")
				continue
			}
			m.TOCDepth = d
		case "number-sections":
			boolean(k, v, &m.NumberSections)
		case "papersize":
			s, ok := scalar(v)
			s = strings.ToLower(strings.TrimSpace(s))
			if !ok || (s != "a4" && s != "a4-landscape") {
				bad(k, v, "a4 또는 a4-landscape")
				continue
			}
			m.PaperSize = s
		case "theme":
			str(k, v, &m.Theme)
		case "cover":
			boolean(k, v, &m.Cover)
		case "doc-no":
			str(k, v, &m.DocNo)
		case "version":
			str(k, v, &m.Version)
		case "classification":
			str(k, v, &m.Classification)
		case "organization":
			str(k, v, &m.Organization)
		case "list-style":
			if isNull(v) {
				continue
			}
			ls, ok := parseListStyle(v)
			if !ok {
				bad(k, v, listStyleWant)
				continue
			}
			m.ListStyle = ls
		case "slides":
			sm, ws := parseSlides(v, srcs)
			m.Slides = sm
			warns = append(warns, ws...)
		case "code":
			cm, ws := parseCode(v, srcs)
			m.Code = cm
			warns = append(warns, ws...)
		default:
			m.Extra = append(m.Extra, extraField(k.Value, v))
			// 아는 키와 아주 가까운 모르는 키는 오타일 수 있어 알린다 — 값은 위처럼 Extra 에 그대로(itda-doc#14, typo.go).
			// 그 아는 키가 같은 frontmatter 에 함께 있어도 낸다(둘 다 적었어도 한쪽은 오타일 수 있다).
			if s, ok := typoSuggest("", k.Value); ok {
				msg := fmt.Sprintf("frontmatter %s 는 모르는 키 — %s 의 오타?", k.Value, s)
				if p, ok := folderWarnPrefix(srcs, k.Value, k); ok {
					warns = append(warns, Warning{Line: 0, Code: WarnFrontmatterTypo, Key: k.Value, Message: p + msg})
				} else {
					warns = append(warns, Warning{Line: k.Line + 1, Code: WarnFrontmatterTypo, Key: k.Value, Message: msg})
				}
			}
		}
	}
	return warns
}

func isNull(v *yaml.Node) bool {
	return v.Kind == yaml.ScalarNode && v.Tag == "!!null"
}

// scalar 는 글자 값을 꺼낸다. null 은 빈 글자다.
func scalar(v *yaml.Node) (string, bool) {
	if v.Kind != yaml.ScalarNode {
		return "", false
	}
	if isNull(v) {
		return "", true
	}
	return v.Value, true
}

// authors 는 글자, 글자 목록, Pandoc 식 {name: …} 목록을 받는다.
func authors(v *yaml.Node) ([]string, bool) {
	if s, ok := scalar(v); ok {
		if s == "" {
			return nil, true
		}
		return []string{s}, true
	}
	if v.Kind != yaml.SequenceNode {
		return nil, false
	}
	var out []string
	for _, it := range v.Content {
		if s, ok := scalar(it); ok {
			out = append(out, s)
			continue
		}
		if it.Kind == yaml.MappingNode {
			if name, ok := mapValue(it, "name"); ok {
				out = append(out, name)
				continue
			}
		}
		return nil, false
	}
	return out, true
}

func mapValue(m *yaml.Node, key string) (string, bool) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return scalar(m.Content[i+1])
		}
	}
	return "", false
}

func extraField(key string, v *yaml.Node) MetaField {
	f := MetaField{Key: key}
	if s, ok := scalar(v); ok {
		f.Value = s
		return f
	}
	if v.Kind == yaml.SequenceNode {
		list := make([]string, 0, len(v.Content))
		for _, it := range v.Content {
			s, ok := scalar(it)
			if !ok {
				list = nil
				break
			}
			list = append(list, s)
		}
		if list != nil {
			f.List = list
			f.Value = strings.Join(list, ", ")
			return f
		}
	}
	// 그 밖의 구조는 테마가 해석할 수 있게 YAML 글자로 넘긴다
	out, err := yaml.Marshal(v)
	if err == nil {
		f.Value = strings.TrimSpace(string(out))
	}
	return f
}

// parseSlides 는 slides: 를 읽는다. true(또는 빈 값)·매핑을 받는다. false 면 없는 것과 같다.
// 틀린 값은 그 키만 버리고 경고한다(나머지 설정은 쓴다). srcs 는 폴더 출처다(fill 과 같다).
func parseSlides(v *yaml.Node, srcs metaSrc) (*SlidesMeta, []Warning) {
	var warns []Warning
	bad := func(n *yaml.Node, key, want string) {
		if p, ok := folderWarnPrefix(srcs, "slides."+normalizeAlias("slides", key), n); ok {
			warns = append(warns, Warning{Line: 0, Code: WarnFrontmatterValue, Key: "slides." + normalizeAlias("slides", key),
				Message: p + fmt.Sprintf("frontmatter slides.%s: %s 이어야 해서 무시함", key, want)})
			return
		}
		warns = append(warns, Warning{Line: n.Line + 1, Code: WarnFrontmatterValue, Key: "slides." + normalizeAlias("slides", key),
			Message: fmt.Sprintf("frontmatter slides.%s: %s 이어야 해서 무시함", key, want)})
	}
	whole := func(n *yaml.Node) {
		if p, ok := folderWarnPrefix(srcs, "slides", n); ok {
			warns = append(warns, Warning{Line: 0, Code: WarnFrontmatterValue, Key: "slides", Message: p + "frontmatter slides: true 또는 설정 목록이어야 해서 무시함"})
			return
		}
		warns = append(warns, Warning{Line: n.Line + 1, Code: WarnFrontmatterValue, Key: "slides", Message: "frontmatter slides: true 또는 설정 목록이어야 해서 무시함"})
	}
	sm := &SlidesMeta{}
	switch {
	case isNull(v):
		return sm, nil
	case v.Kind == yaml.ScalarNode:
		var b bool
		if v.Decode(&b) != nil {
			whole(v)
			return nil, warns
		}
		if !b {
			return nil, nil
		}
		return sm, nil
	case v.Kind != yaml.MappingNode:
		whole(v)
		return nil, warns
	}
	for i := 0; i+1 < len(v.Content); i += 2 {
		kk, vv := v.Content[i], v.Content[i+1]
		s, _ := scalar(vv)
		s = strings.ToLower(strings.TrimSpace(s))
		switch kk.Value {
		case "ratio", "aspect", "aspectratio":
			switch s {
			case "16:9", "16/9", "4:3", "4/3":
				sm.Ratio = strings.ReplaceAll(s, "/", ":")
			default:
				bad(vv, kk.Value, `"16:9" 또는 "4:3"`)
			}
		case "level", "slide-level":
			var n int
			if vv.Kind != yaml.ScalarNode || vv.Decode(&n) != nil || n < 1 || n > 6 {
				bad(vv, kk.Value, "1~6 의 정수")
				continue
			}
			sm.Level = n
		case "transition":
			switch s {
			case "fade", "slide", "none":
				sm.Transition = s
			default:
				bad(vv, kk.Value, "fade·slide·none")
			}
		case "incremental":
			var b bool
			if vv.Kind != yaml.ScalarNode || vv.Decode(&b) != nil {
				bad(vv, kk.Value, "true 또는 false")
				continue
			}
			sm.Incremental = b
		case "font-scale", "fontscale", "font_scale":
			var f float64
			if vv.Kind != yaml.ScalarNode || vv.Decode(&f) != nil || !(f >= 0.6 && f <= 1.5) {
				bad(vv, kk.Value, "0.6~1.5 의 숫자")
				continue
			}
			sm.FontScale = f
		default:
			if p, ok := folderWarnPrefix(srcs, "slides."+kk.Value, kk); ok {
				warns = append(warns, Warning{Line: 0, Code: WarnFrontmatterValue, Key: "slides." + normalizeAlias("slides", kk.Value),
					Message: p + fmt.Sprintf("frontmatter slides.%s 는 모르는 키라 무시함(ratio·level·transition·incremental·font-scale%s)", kk.Value, typoHint("slides", kk.Value))})
				continue
			}
			warns = append(warns, Warning{Line: kk.Line + 1, Code: WarnFrontmatterValue, Key: "slides." + normalizeAlias("slides", kk.Value),
				Message: fmt.Sprintf("frontmatter slides.%s 는 모르는 키라 무시함(ratio·level·transition·incremental·font-scale%s)", kk.Value, typoHint("slides", kk.Value))})
		}
	}
	return sm, warns
}

// parseCode 는 code: 를 읽는다(itda-hyve#435). true(또는 빈 값)·매핑을 받고, false 는 line-numbers: false 와 같다(줄 번호 끔).
// 틀린 값은 그 키만 버리고 경고한다(나머지 설정은 쓴다). srcs 는 폴더 출처다(fill 과 같다).
func parseCode(v *yaml.Node, srcs metaSrc) (*CodeMeta, []Warning) {
	var warns []Warning
	bad := func(n *yaml.Node, key, want string) {
		if p, ok := folderWarnPrefix(srcs, "code."+normalizeAlias("code", key), n); ok {
			warns = append(warns, Warning{Line: 0, Code: WarnFrontmatterValue, Key: "code." + normalizeAlias("code", key),
				Message: p + fmt.Sprintf("frontmatter code.%s: %s 이어야 해서 무시함", key, want)})
			return
		}
		warns = append(warns, Warning{Line: n.Line + 1, Code: WarnFrontmatterValue, Key: "code." + normalizeAlias("code", key),
			Message: fmt.Sprintf("frontmatter code.%s: %s 이어야 해서 무시함", key, want)})
	}
	whole := func(n *yaml.Node) {
		if p, ok := folderWarnPrefix(srcs, "code", n); ok {
			warns = append(warns, Warning{Line: 0, Code: WarnFrontmatterValue, Key: "code", Message: p + "frontmatter code: true 또는 설정 목록이어야 해서 무시함"})
			return
		}
		warns = append(warns, Warning{Line: n.Line + 1, Code: WarnFrontmatterValue, Key: "code", Message: "frontmatter code: true 또는 설정 목록이어야 해서 무시함"})
	}
	cm := &CodeMeta{}
	switch {
	case isNull(v):
		return cm, nil
	case v.Kind == yaml.ScalarNode:
		var b bool
		if v.Decode(&b) != nil {
			whole(v)
			return nil, warns
		}
		if !b {
			cm.LineNumbers = &b // code: false — 줄 번호 끔
		}
		return cm, nil
	case v.Kind != yaml.MappingNode:
		whole(v)
		return nil, warns
	}
	for i := 0; i+1 < len(v.Content); i += 2 {
		kk, vv := v.Content[i], v.Content[i+1]
		switch kk.Value {
		case "line-numbers", "linenumbers", "line_numbers":
			if isNull(vv) {
				continue // 빈 값은 정하지 않음(기본 켬) — boolean() 과 같다
			}
			var b bool
			if vv.Kind != yaml.ScalarNode || vv.Decode(&b) != nil {
				bad(vv, kk.Value, "true 또는 false")
				continue
			}
			cm.LineNumbers = &b
		default:
			if p, ok := folderWarnPrefix(srcs, "code."+kk.Value, kk); ok {
				warns = append(warns, Warning{Line: 0, Code: WarnFrontmatterValue, Key: "code." + normalizeAlias("code", kk.Value),
					Message: p + fmt.Sprintf("frontmatter code.%s 는 모르는 키라 무시함(line-numbers%s)", kk.Value, typoHint("code", kk.Value))})
				continue
			}
			warns = append(warns, Warning{Line: kk.Line + 1, Code: WarnFrontmatterValue, Key: "code." + normalizeAlias("code", kk.Value),
				Message: fmt.Sprintf("frontmatter code.%s 는 모르는 키라 무시함(line-numbers%s)", kk.Value, typoHint("code", kk.Value))})
		}
	}
	return cm, warns
}
