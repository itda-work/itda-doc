// Package mdsyntax 는 문서 창 마크다운 문법의 단일본이다(itda-hyve#134).
//
// 정본은 사람이 읽는 안내서(spec/*.md)다. 안내서의 `<!-- syntax:주제:begin -->` … `end` 구간을
// extract.Build 가 모아 topics/<주제>.md 로 쓰고(go generate — gen/), 바이너리에 넣는다. 손으로 고치지 않는다:
// TestBundleFresh 가 안내서에서 다시 만든 것과 넣은 것이 바이트까지 같은지 본다.
// 같은 안내서의 표지 표(컨테이너·알림·frontmatter 키·차트 속성·차트 종류)는 문서 창 편집기의 자동 완성 후보가 된다
// (completions.json — 같은 gen 이 쓰고 같은 시험이 묶는다, itda-hyve#434).
// Claude 는 앱의 문법 도구(예: MCP md_syntax)나 `itda-doc syntax` 로 지금 실행 중인 판의 문법을 받는다 — 판과 Hash 가 함께 간다.
package mdsyntax

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/itda-work/itda-doc/mdsyntax/extract"
)

//go:generate go run ./gen

//go:embed topics/*.md
var topicFS embed.FS

//go:embed completions.json
var completionsJSON []byte

// Completion 은 편집기 자동 완성 후보 하나다(extract.Completion).
type Completion = extract.Completion

// Completions 는 넣은 자동 완성 후보다(안내서 표 차례). 받은 쪽이 고쳐도 넣은 것은 바뀌지 않게 매번 새로 푼다.
func Completions() []Completion {
	var out []Completion
	if err := json.Unmarshal(completionsJSON, &out); err != nil {
		panic("mdsyntax: 넣은 후보를 풀지 못함: " + err.Error()) // TestBundleFresh 가 막는다
	}
	return out
}

// Topics 는 주제 이름이다. 차례는 "all" 로 이을 때의 차례다.
var Topics = extract.Topics

// TopicAll 은 모든 주제를 이은 것이다.
const TopicAll = "all"

// Topic 은 주제 하나의 글이다. 모르는 주제는 오류다(받는 이름을 함께 알린다).
func Topic(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = "index"
	}
	if name == TopicAll {
		var b strings.Builder
		for i, t := range Topics {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(mustTopic(t))
		}
		return b.String(), nil
	}
	for _, t := range Topics {
		if t == name {
			return mustTopic(t), nil
		}
	}
	return "", fmt.Errorf("모르는 주제 %q — %s 또는 %s", name, strings.Join(Topics, "·"), TopicAll)
}

func mustTopic(name string) string {
	b, err := topicFS.ReadFile("topics/" + name + ".md")
	if err != nil {
		panic("mdsyntax: 넣은 주제가 없음: " + name) // TestBundleFresh 가 막는다
	}
	// Windows 체크아웃(autocrlf)에서 CRLF 로 들어와도 같은 글·같은 Hash 가 되게 줄 끝을 맞춘다(.gitattributes 도 LF 로 묶는다)
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// Hash 는 넣은 문법 전체의 해시(sha256 앞 12자리)다. 주제 차례대로 이름과 글을 이어 잰다 — 같은 판이라도 문법이 바뀌면 바뀐다.
func Hash() string {
	h := sha256.New()
	for _, t := range Topics {
		fmt.Fprintf(h, "%s\x00%s\x00", t, mustTopic(t))
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}
