package mdoc

import (
	"fmt"
	"slices"

	"gopkg.in/yaml.v3"
)

// 폴더 기본 frontmatter `_metadata.yml` 상속(itda-hyve#474 ①). 문서 창(docview)이 경로를 알아 폴더 위로
// 파일을 찾아 읽고, 이 패키지는 받은 YAML 을 문서 frontmatter 와 합친다 — mdoc.Render 는 경로를
// 모르는 순수 함수 그대로다(값은 Options.FolderMeta 로 들어온다).
//
// 구조는 두 단계다(2026-10-31 접근 방식 정리 — 특례를 덧대지 않는다):
//  1. 파일별 정리(tidyMetaNode) — 폴더 파일이 적용될 때만 쓰는 규칙이다(문서도 함께 정리한다).
//     각 파일에서 같은 키(별칭은 정규 이름으로 견준다)는 마지막 하나만 남긴다 — 값이 틀려도 그 값이
//     이기고, 그 키는 경고와 함께 무시된다. 별칭 비교는 파서가 보는 수준(최상위, slides·code 바로 아래)
//     에서만 한다. 키 노드는 사용자가 적은 표기 그대로 둔다(경고가 적은 이름을 말하게 — itda-hyve#504).
//     폴더 파일이 하나도 적용되지 않으면 정리를 돌리지 않는다 — 폴더 없는 문서의 처리는
//     main(e65d27c) 과 완전히 같다(parseFrontmatter).
//  2. 파일 사이 합치기(mergeFolderInto) — 정리된 노드를 먼 폴더 → 가까운 폴더 → 문서 순으로 얹는다.
//     slides·code 가 양쪽 다 맵이면 하위 키 단위(정규 이름으로 견준다), 그 밖의 모든 값(모르는 키의
//     맵 포함)은 통째로 바꾼다.
//
// 경고 계약(itda-hyve#504 — frontmatterCore):
//   - 문서 frontmatter 의 경고는 폴더 파일이 있어도 없을 때와 같다: 문서 원본 노드(정리 전)를 따로
//     풀어 낸다 — 같은 키의 앞 값 경고·적은 별칭 표기·문서 줄(Line)이 그대로다. 값은 정리 규칙대로
//     마지막 키가 이긴다(앞 값이 맞고 뒤 값이 틀리면 폴더 없을 때와 값이 다를 수 있다 — 경고는 같다).
//   - 폴더 파일의 값 경고는 최종 값으로 쓰인 키만 낸다(Line 0, 문구 앞머리 "<rel> <n>줄: ") — 문서나
//     가까운 폴더가 덮은 키, 같은 파일 안 앞 값의 경고는 내지 않는다. 키 이름은 그 파일에 적힌 표기다.
//   - 폴더 파일 자체의 오류(읽기·YAML·모양)는 Line 0, 문구 "<rel>: … 건너뜀" 이다.
//   - 문서에 slides 키가 없어 폴더의 slides 설정(맵·true·빈 값)을 버리면 안내 경고 하나를 낸다(itda-hyve#543 —
//     folder-slides-unused, Line 0, 문구 앞머리 "<rel>: " — slides 를 정한 가장 가까운 파일, 줄 없음).
//   - 순서: 폴더 파일 오류 → 폴더 slides 안내 → 폴더 값 경고 → 문서 경고(Render 는 Line 으로 안정 정렬한다 — Line 0 이 앞).

// FolderMeta 는 문서 위 폴더의 `_metadata.yml` 하나다. docview 가 읽어 Options.FolderMeta 로 넘긴다.
type FolderMeta struct {
	// Rel 은 문서 기준 상대 경로다(예: "_metadata.yml", "../_metadata.yml"). 경고 문구·출처 표시에 쓴다.
	Rel string
	// YAML 은 파일 원문이다.
	YAML []byte
	// Err 는 파일을 읽지 못한 까닭이다(크기 상한 초과·권한). 비면 읽은 것이다. 이 파일만 건너뛰고 경고한다.
	Err string
}

// metaSrc 는 최종 노드에서 각 키가 어느 폴더 파일에서 왔는지다(정규화된 키 경로 → Rel).
// 키 경로는 최상위 키("theme")와 slides·code 하위 키("slides.ratio")다. 문서에서 온 키는 여기 없다.
type metaSrc map[string]string

// knownGroups 는 파일 사이 합치기에서 하위 키 단위로 얹는 맵 키다.
var knownGroups = map[string]bool{"slides": true, "code": true}

// tidyMetaNode 는 매핑 노드에서 같은 키를 마지막 하나로 줄인다(폴더 파일이 적용될 때만 쓴다).
//   - 같은 키(별칭은 정규 이름으로 견준 뒤)는 마지막 하나만 남긴다 — 값이 틀려도 그 값이 이기고,
//     그 키는 경고와 함께 무시된다. 폴더가 없는 문서는 정리를 하지 않아 지금까지(main) 그대로다.
//   - 별칭 비교·중복 정리는 파서가 보는 수준에서만 한다: 최상위 키와 slides·code 바로 아래 키.
//     목록 안 매핑·더 깊은 곳은 건드리지 않는다(거기엔 별칭도 중복 규칙도 없다).
//   - 키·값 노드는 원본을 그대로 재사용한다 — 노드의 줄(Line)이 온 파일 그대로여야 경고가 맞는 줄을
//     말하고, 키 이름도 적힌 표기 그대로라 경고가 그 표기로 난다(itda-hyve#504). rename 이 참이면(편집 화면의
//     쓰기 — TidyFolderMetaNode) 별칭 키를 정규 이름으로 바꾼 사본을 둔다.
func tidyMetaNode(n *yaml.Node, rename bool) *yaml.Node {
	return tidyLevel(n, "", rename)
}

// tidyLevel 은 매핑 하나(group "" 은 최상위, "slides"·"code" 는 그 바로 아래)를 줄인다. 최상위의
// slides·code 맵 값은 한 단계만 더 내려간다 — 그 아래(하위 값이 맵이어도)는 건드리지 않는다.
func tidyLevel(n *yaml.Node, group string, rename bool) *yaml.Node {
	content := make([]*yaml.Node, 0, len(n.Content))
	pos := map[string]int{} // 정규 키 → content 안 (키, 값) 쌍의 인덱스
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		canon := normalizeAlias(group, k.Value)
		if rename && canon != k.Value { // 키 이름을 바꾼 사본(원본 노드를 바꾸지 않는다)
			nk := *k
			nk.Value = canon
			k = &nk
		}
		if group == "" && v.Kind == yaml.MappingNode && knownGroups[canon] {
			v = tidyLevel(v, canon, rename)
		}
		if j, ok := pos[canon]; ok { // 같은 키 — 앞 것을 빼고 이 자리에 마지막 키·값을 둔다
			content[j], content[j+1] = k, v
		} else {
			pos[canon] = len(content)
			content = append(content, k, v)
		}
	}
	out := *n
	out.Content = content
	return &out
}

// mergeFolderMeta 는 폴더 파일을 먼 폴더부터 합친다. 돌려주는 것: 합친 매핑 노드(nil 이면 없음),
// 키 소스표, 경고, 적용된 파일의 Rel(먼 것부터 — Result.FolderMeta 는 가까운 것부터로 뒤집어 쓴다).
// 읽을 수 없거나(크기·권한) YAML 이 깨졌거나 매핑이 아닌 파일은 그 파일만 건너뛰고 경고한다.
// 파일 안의 중복 키는 tidyMetaNode 가 문서와 같이 마지막 키로 정리한다(파일을 버리지 않는다).
func mergeFolderMeta(folder []FolderMeta) (*yaml.Node, metaSrc, []Warning, []string) {
	var merged *yaml.Node
	srcs := metaSrc{}
	var warns []Warning
	var applied []string
	for _, f := range folder { // 먼 폴더부터 — 뒤(가까운 폴더)가 앞을 덮는다
		if f.Err != "" {
			warns = append(warns, Warning{Line: 0, Code: WarnFrontmatterInvalid,
				Message: fmt.Sprintf("%s: %s — 건너뜀", f.Rel, f.Err)})
			continue
		}
		var root yaml.Node
		if err := yaml.Unmarshal(f.YAML, &root); err != nil {
			warns = append(warns, Warning{Line: 0, Code: WarnFrontmatterInvalid,
				Message: fmt.Sprintf("%s: YAML 을 읽지 못해 건너뜀: %s", f.Rel, yamlErr(err))})
			continue
		}
		if root.Kind == 0 || len(root.Content) == 0 {
			continue // 빈 파일(주석만 포함) — 상속할 것이 없다
		}
		if root.Content[0].Kind != yaml.MappingNode {
			warns = append(warns, Warning{Line: 0, Code: WarnFrontmatterInvalid,
				Message: fmt.Sprintf("%s: 키: 값 모양이 아니라 건너뜀", f.Rel)})
			continue
		}
		n := tidyMetaNode(root.Content[0], false)
		merged = mergeFolderInto(merged, n)
		registerSrc(srcs, n, f.Rel)
		applied = append(applied, f.Rel)
	}
	return merged, srcs, warns, applied
}

// mergeFolderInto 는 매핑 노드 base 에 over 를 얹는다(over 가 이긴다). slides·code 가 양쪽 다
// 맵이면 하위 키 단위로 얹고, 그 밖의 모든 값 — 모르는 키의 맵 포함 — 은 over 값으로 통째로 바꾼다.
// 덮을 때는 키·값 노드를 함께 승자 것으로 바꾼다 — 모르는 키 경고는 키 노드의 줄로 나므로, 값을 승자
// 것으로 바꾸고 키를 패자 것에 두면 파일 이름과 줄이 서로 다른 원본에서 섞인다(itda-hyve#474 리뷰 3회차 [1]).
// 어느 쪽이 nil 이면 다른 쪽을 그대로 돌려준다. 입력은 파일별 정리(tidyMetaNode)를 마친 노드다.
func mergeFolderInto(base, over *yaml.Node) *yaml.Node {
	if base == nil || over == nil {
		if base == nil {
			return over
		}
		return base
	}
	out := *base
	content := make([]*yaml.Node, 0, len(base.Content)+len(over.Content))
	content = append(content, base.Content...)
	for i := 0; i+1 < len(over.Content); i += 2 {
		k, v := over.Content[i], over.Content[i+1]
		j := mappingIndex(content, k.Value)
		switch {
		case j < 0:
			content = append(content, k, v)
		case knownGroups[k.Value] && v.Kind == yaml.MappingNode && content[j+1].Kind == yaml.MappingNode:
			content[j+1] = mergeGroupInto(content[j+1], v, k.Value) // slides·code — 하위 키 단위
		default:
			content[j], content[j+1] = k, v // 통째로 — 키도 승자 것
		}
	}
	out.Content = content
	return &out
}

// mergeGroupInto 는 slides·code 하위를 키 단위로 얹는다(over 승 — 키·값 노드 함께, 키는 정규 이름으로
// 견준다 — 표기는 승자가 적은 그대로다). 하위 값은 맵이어도 통째로 바꾼다.
func mergeGroupInto(base, over *yaml.Node, group string) *yaml.Node {
	out := *base
	content := make([]*yaml.Node, 0, len(base.Content)+len(over.Content))
	content = append(content, base.Content...)
	for i := 0; i+1 < len(over.Content); i += 2 {
		k, v := over.Content[i], over.Content[i+1]
		if j := groupIndex(content, group, k.Value); j >= 0 {
			content[j], content[j+1] = k, v
		} else {
			content = append(content, k, v)
		}
	}
	out.Content = content
	return &out
}

// groupIndex 는 그룹(slides·code) 하위 content 에서 key 와 정규 이름이 같은 키의 인덱스다(없으면 -1).
func groupIndex(content []*yaml.Node, group, key string) int {
	canon := normalizeAlias(group, key)
	for i := 0; i+1 < len(content); i += 2 {
		if normalizeAlias(group, content[i].Value) == canon {
			return i
		}
	}
	return -1
}

func mappingIndex(content []*yaml.Node, key string) int {
	for i := 0; i+1 < len(content); i += 2 {
		if content[i].Value == key {
			return i
		}
	}
	return -1
}

// registerSrc 는 정리된 노드의 키를 소스 rel 로 등록한다 — 경고가 날 수 있는 수준(최상위 키와
// slides·code 하위 키)만, 정규 키 경로로. 이미 있는 키는 덮는다(나중에 부른, 가까운 파일이 이긴다).
func registerSrc(srcs metaSrc, n *yaml.Node, rel string) {
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		srcs[k.Value] = rel
		if v.Kind == yaml.MappingNode && knownGroups[k.Value] {
			for j := 0; j+1 < len(v.Content); j += 2 {
				srcs[k.Value+"."+normalizeAlias(k.Value, v.Content[j].Value)] = rel
			}
		}
	}
}

// unregisterSrc 는 문서(정리를 마친) 노드가 가진 키를 소스표에서 지운다 — 문서가 이긴 키의 경고는
// 폴더 파일 경로가 아니라 문서 줄로 나와야 한다. 최상위 키와 slides·code 하위 키만 본다.
func unregisterSrc(srcs metaSrc, n *yaml.Node) {
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		delete(srcs, k.Value)
		if v.Kind == yaml.MappingNode && knownGroups[k.Value] {
			for j := 0; j+1 < len(v.Content); j += 2 {
				delete(srcs, k.Value+"."+normalizeAlias(k.Value, v.Content[j].Value))
			}
		}
	}
}

// docSlidesValue 는 정리를 마친 문서 노드의 slides 값이다(키가 하나뿐이다 — 없으면 nil).
func docSlidesValue(doc *yaml.Node) *yaml.Node {
	if doc == nil {
		return nil
	}
	if j := mappingIndex(doc.Content, "slides"); j >= 0 {
		return doc.Content[j+1]
	}
	return nil
}

// folderHasSlideMap 은 합친 폴더 노드에 slides 맵이 있는지다(있어야 문서의 slides: true 스칼라를 빼고
// 폴더 맵으로 채운다).
func folderHasSlideMap(merged *yaml.Node) bool {
	if merged == nil {
		return false
	}
	j := mappingIndex(merged.Content, "slides")
	return j >= 0 && merged.Content[j+1].Kind == yaml.MappingNode
}

// folderSlidesUnused 는 문서가 slides 를 정하지 않아 폴더의 slides 설정을 버릴 때 내는 안내 경고다(itda-hyve#543).
// 렌더는 그대로다(슬라이드 여부는 문서 자신의 slides: 만 정한다 — frontmatterCore). 폴더의 slides 가
// 맵이거나 슬라이드를 켜는 값(true·빈 값)일 때만 낸다 — 폴더의 slides: false·틀린 스칼라는 버려도
// 잃는 설정이 없다. 경로는 slides 를 정한 가장 가까운 폴더 파일이다(srcs). 줄은 싣지 않는다 — 여러
// 파일의 slides 맵을 합치면 키 노드는 먼 파일 것이라 줄이 경로와 어긋난다.
func folderSlidesUnused(merged *yaml.Node, srcs metaSrc) (Warning, bool) {
	j := mappingIndex(merged.Content, "slides")
	if j < 0 {
		return Warning{}, false
	}
	v := merged.Content[j+1]
	var b bool
	if !(v.Kind == yaml.MappingNode || isNull(v) || v.Kind == yaml.ScalarNode && v.Decode(&b) == nil && b) {
		return Warning{}, false
	}
	return Warning{Line: 0, Code: WarnFolderSlidesUnused,
		Message: srcs["slides"] + ": 폴더의 slides 설정은 문서 frontmatter 에 slides: true 가 있어야 적용됨 — 이 문서는 슬라이드로 열리지 않음"}, true
}

// folderWarnPrefix 는 폴더 파일에서 온 키의 경고 앞머리다(예: "../_metadata.yml 3줄: "). 문서 줄이
// 아니므로 경고의 Line 은 0 이고, 그 파일의 줄은 문구로 말한다. path 는 정규 키 경로다(srcs 와 같다).
func folderWarnPrefix(srcs metaSrc, path string, v *yaml.Node) (string, bool) {
	rel, ok := srcs[path]
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%s %d줄: ", rel, v.Line), true
}

// inheritFolderList 는 적용된 폴더 파일 목록을 가까운 것부터로 뒤집는다.
func inheritFolderList(applied []string) []string {
	if len(applied) == 0 {
		return nil
	}
	out := slices.Clone(applied)
	slices.Reverse(out)
	return out
}
