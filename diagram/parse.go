package diagram

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// frontmatter(--- … ---)·접근성 문장(accTitle·accDescr) 읽기 — 그림 종류가 함께 쓴다(flowchart 의 parse.go 에서 꺼냈다).

// SplitFrontmatter 는 줄들의 맨 앞이 --- … --- 이면 그 사이(frontmatter 소스)와 본문 시작 줄 차례를 갈라 돌려준다.
// 첫 줄이 --- 가 아니면 ok 가 거짓이다. --- 이 닫히지 않았으면 closed 가 거짓이다(부르는 쪽이 오류로 알린다).
func SplitFrontmatter(lines []string) (fm string, body int, ok, closed bool) {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", 0, false, true
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[1:i], "\n"), i + 1, true, true
		}
	}
	return "", 0, true, false
}

// FMKey 는 frontmatter 의 title 밖 최상위 키 하나다. Line 은 frontmatter 소스 안의 줄(1부터).
type FMKey struct {
	Key  string
	Line int
}

// Frontmatter 는 읽은 frontmatter 다. Title 은 옛 DecodeLabel 을 지난 title 글(없으면 "" — 길이 상한 검사에만
// 쓰고, 여섯 종류의 캡션은 모두 Raw 를 Caption 으로 읽는다), HasTitle 은
// 글인 title 이 있었다는 뜻. Raw 는 YAML 이 읽은 title 글 그대로(정책 B 의 Caption 이 읽는다), TitleLine 은 title 키의 줄
// (frontmatter 소스 기준, 1부터).
type Frontmatter struct {
	Title     string
	HasTitle  bool
	Raw       string
	TitleLine int
	Others    []FMKey
}

// ParseFrontmatter 는 frontmatter 소스(--- 사이)를 YAML(Mermaid 와 같다)로 읽는다 — 블록 스칼라(|·>)·따옴표·주석·
// 엔티티 모두 YAML 규칙대로. title 은 글(스칼라)이면 캡션이다. 나머지 최상위 키(config·displayMode …)는 Others 로 돌려
// 부르는 쪽이 쓰지 않는다고 알린다. 읽을 수 없는 YAML, 매핑이 아닌 문서, 글이 아닌 title, 라벨 상한을 넘는 title 은
// *Error 다(원문 그대로 — 캡션을 잘못 적지 않게). *Error 의 Line 은 frontmatter 소스 기준(1부터)이라 부르는 쪽이
// 원문 줄로 바꾼다.
func ParseFrontmatter(src string) (Frontmatter, error) {
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(src), &root); err != nil {
		return Frontmatter{}, &Error{Msg: fmt.Sprintf("frontmatter YAML 을 읽지 못함(%s)", strings.TrimPrefix(err.Error(), "yaml: "))}
	}
	var fm Frontmatter
	if len(root.Content) == 0 {
		return fm, nil // 비었거나 주석만
	}
	m := root.Content[0]
	if m.Kind != yaml.MappingNode {
		return Frontmatter{}, &Error{Line: m.Line, Msg: "frontmatter 가 키: 값 꼴(YAML 매핑)이 아님"}
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		k, v := m.Content[i], m.Content[i+1]
		if k.Value != "title" {
			fm.Others = append(fm.Others, FMKey{Key: k.Value, Line: k.Line})
			continue
		}
		if v.Kind == yaml.AliasNode {
			v = v.Alias
		}
		if v.Kind != yaml.ScalarNode {
			return Frontmatter{}, &Error{Line: k.Line, Msg: "frontmatter 의 title 이 글이 아님"}
		}
		if v.Tag == "!!null" {
			continue
		}
		t := DecodeLabel(v.Value)
		if n := len([]rune(t)); n > MaxLabel {
			// 줄은 title 키 줄이다(공통화 전 flowchart 와 같다) — 여러 줄 제목(블록 스칼라)이나 별칭으로 값이
			// 다른 줄에 정의된 경우에도 값 노드의 줄(v.Line)이 아니라 키 줄을 보고한다.
			return Frontmatter{}, &Error{Line: k.Line, Msg: fmt.Sprintf("라벨이 %d 자를 넘음", MaxLabel)}
		}
		fm.Title, fm.HasTitle = strings.ReplaceAll(t, "\n", " "), true
		fm.Raw, fm.TitleLine = v.Value, k.Line
	}
	return fm, nil
}

// AccLine 은 "accTitle: 글"·"accDescr { …" 꼴을 본다. kind 는 ':' 또는 '{', v 는 그 뒤의 글이다.
func AccLine(s, kw string) (byte, string, bool) {
	if !strings.HasPrefix(s, kw) {
		return 0, "", false
	}
	r := strings.TrimLeft(s[len(kw):], " \t")
	if r == "" || (r[0] != ':' && r[0] != '{') {
		return 0, "", false
	}
	return r[0], strings.TrimSpace(r[1:]), true
}
