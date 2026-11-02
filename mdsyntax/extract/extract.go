// Package extract 는 안내서에서 문법 단일본을 모은다(mdsyntax 의 go generate 와 시험이 쓴다 — 넣은 글에 기대지 않게 따로 둔다).
package extract

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Topics 는 주제 이름이다. 차례는 "all" 로 이을 때의 차례다.
var Topics = []string{"index", "core", "charts", "math", "diagrams", "slides"}

// Sources 는 단일본을 모으는 안내서다(spec 아래). 이 차례로 읽고, 한 주제의 구간이 여럿이면 이 차례·문서 안 차례로 잇는다.
var Sources = []string{
	"markdown.md",
	"charts.md",
	"math.md",
	"diagrams.md",
	"slides.md",
}

var (
	regionRe = regexp.MustCompile(`^<!-- syntax:([a-z]+):(begin|end) -->$`)
	// 시험이 읽는 표지(<!-- chart-types:begin -->, <!-- chart-example warn="…" --> …)는 Claude 에게 뜻이 없다. pagebreak 같은
	// 문법 주석은 코드 블록 안에만 있으므로 코드 블록 밖의 이 꼴만 뺀다
	markerRe = regexp.MustCompile(`^<!-- [a-z0-9-]+(:begin|:end| [a-z]+="[^"]*")? -->$`)
	// 그림만 있는 줄 — 안내서의 화면 그림. 글로 읽는 쪽에는 뜻이 없다
	imageLineRe = regexp.MustCompile(`^!\[[^\]]*\]\([^)]*\)$`)
	fenceRe     = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
)

// Build 는 docsDir 의 안내서에서 주제별 단일본을 만든다. 주제마다 머리(출처·손으로 고치지 말 것)를 붙인다.
// 구간이 닫히지 않았거나 겹치거나, 모르는 주제이거나, 주제에 글이 없으면 오류다.
func Build(docsDir string) (map[string][]byte, error) {
	known := map[string]bool{}
	for _, t := range Topics {
		known[t] = true
	}
	bodies := map[string]*strings.Builder{}
	from := map[string][]string{}
	for _, src := range Sources {
		b, err := os.ReadFile(filepath.Join(docsDir, src))
		if err != nil {
			return nil, err
		}
		open, openLine := "", 0
		fence := ""
		for i, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
			if fence == "" && strings.Contains(line, "<!-- syntax:") && !regionRe.MatchString(line) {
				return nil, fmt.Errorf("%s:%d: syntax 표지 꼴이 틀림(줄 하나에 `<!-- syntax:주제:begin -->` 만, 소문자): %q", src, i+1, line)
			}
			if fence == "" {
				if m := regionRe.FindStringSubmatch(line); m != nil {
					switch {
					case !known[m[1]]:
						return nil, fmt.Errorf("%s:%d: 모르는 주제 %q", src, i+1, m[1])
					case m[2] == "begin" && open != "":
						return nil, fmt.Errorf("%s:%d: %s 구간 안에서 %s 구간을 엶(%d줄)", src, i+1, open, m[1], openLine)
					case m[2] == "begin":
						open, openLine = m[1], i+1
						if bodies[open] == nil {
							bodies[open] = &strings.Builder{}
						} else {
							bodies[open].WriteString("\n")
						}
						if len(from[open]) == 0 || from[open][len(from[open])-1] != src {
							from[open] = append(from[open], src)
						}
					case open != m[1]:
						return nil, fmt.Errorf("%s:%d: 열리지 않은 %s 구간을 닫음", src, i+1, m[1])
					default:
						open = ""
					}
					continue
				}
			}
			// 코드 블록 안팎: 여는 펜스와 같은 글자로 같거나 긴 줄이 닫는다(CommonMark)
			if m := fenceRe.FindStringSubmatch(line); m != nil {
				switch {
				case fence == "":
					fence = m[1]
				case m[1][0] == fence[0] && len(m[1]) >= len(fence) && strings.TrimSpace(line) == m[1]:
					fence = ""
				}
			} else if fence == "" && (markerRe.MatchString(line) || imageLineRe.MatchString(line)) {
				continue
			}
			if open != "" {
				bodies[open].WriteString(line)
				bodies[open].WriteString("\n")
			}
		}
		if open != "" {
			return nil, fmt.Errorf("%s:%d: %s 구간이 닫히지 않음", src, openLine, open)
		}
	}
	// Sources 밖의 안내서에 syntax 표지가 있으면 조용히 빠지지 않게 멈춘다
	others, _ := filepath.Glob(filepath.Join(docsDir, "*.md"))
	for _, p := range others {
		name := filepath.Base(p)
		if slices.Contains(Sources, name) {
			continue
		}
		if b, err := os.ReadFile(p); err == nil && strings.Contains(string(b), "<!-- syntax:") {
			return nil, fmt.Errorf("%s 에 syntax 표지가 있으나 extract.Sources 에 없음", name)
		}
	}
	out := map[string][]byte{}
	for _, t := range Topics {
		body := bodies[t]
		if body == nil || strings.TrimSpace(body.String()) == "" {
			return nil, fmt.Errorf("주제 %s 의 구간이 없음", t)
		}
		head := fmt.Sprintf("<!-- itda-hyve 마크다운 문법 · 주제 %s · 자동 생성(spec/%s) — 손으로 고치지 않는다. 장 번호(\"3장\")는 이 주제 안의 번호다 -->\n\n",
			t, strings.Join(from[t], "·spec/"))
		out[t] = []byte(head + collapseBlank(body.String()))
	}
	return out, nil
}

// collapseBlank 은 뺀 줄 때문에 생긴 빈 줄 여럿을 하나로 줄이고 앞뒤 빈 줄을 없앤다(코드 블록 안은 그대로).
func collapseBlank(s string) string {
	var b strings.Builder
	blank, fence := 0, ""
	for _, line := range strings.Split(strings.Trim(s, "\n"), "\n") {
		if m := fenceRe.FindStringSubmatch(line); m != nil {
			switch {
			case fence == "":
				fence = m[1]
			case m[1][0] == fence[0] && len(m[1]) >= len(fence) && strings.TrimSpace(line) == m[1]:
				fence = ""
			}
		}
		if fence == "" && strings.TrimSpace(line) == "" {
			blank++
			continue
		}
		if blank > 0 && b.Len() > 0 {
			b.WriteString("\n")
		}
		blank = 0
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}
