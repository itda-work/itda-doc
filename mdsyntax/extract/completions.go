package extract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Completion 은 문서 창 편집기의 자동 완성 후보 하나다(#434). 안내서의 표지 표에서 뽑는다 — 이름·뜻을 손으로 따로 적지 않는다.
type Completion struct {
	// Kind 는 후보가 뜨는 자리다: container(줄 첫머리 `:::` 다음), chart-attr(`:::chart{` 안), chart-type(`type=` 다음),
	// alert(`> [!` 다음), fm-key(frontmatter 의 줄 첫머리).
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Detail  string `json:"detail"`            // 한 줄 뜻(표의 뜻 칸 — 마크다운 링크·백틱은 글자만 남긴다)
	Example string `json:"example,omitempty"` // 짧은 예(표의 대표 표기 칸 등)
	Topic   string `json:"topic"`             // 도움말 주제(Topics 가운데 하나)
	Section string `json:"section,omitempty"` // 그 주제 안의 제목 글(백틱을 뺀 글자 — 화면이 같은 글의 제목을 찾는다)
}

// completionSource 는 후보를 뽑는 표 하나다: 안내서·표지·종류와, 표 한 줄(칸 목록)에서 후보를 만드는 법.
type completionSource struct {
	file, mark, kind string
	row              func(cells []string) (Completion, bool)
}

var completionSources = []completionSource{
	{"markdown.md", "containers", "container", func(c []string) (Completion, bool) {
		return Completion{Name: code(c[0]), Detail: plain(c[1])}, len(c) >= 2
	}},
	{"markdown.md", "alerts", "alert", func(c []string) (Completion, bool) {
		return Completion{Name: code(c[0]), Detail: c[1] + " — " + plain(c[3])}, len(c) >= 4
	}},
	{"markdown.md", "fm-keys", "fm-key", func(c []string) (Completion, bool) {
		if len(c) < 4 {
			return Completion{}, false
		}
		return Completion{Name: code(c[0]), Detail: plain(c[3]), Example: "값: " + plain(c[1]) + " · 기본: " + plain(c[2])}, true
	}},
	{"charts.md", "chart-attrs", "chart-attr", func(c []string) (Completion, bool) {
		n := code(c[0])
		if strings.HasPrefix(n, "#") || strings.HasPrefix(n, ".") {
			return Completion{}, true // #id·.class 는 이름이 아니라 꼴이다 — 후보가 아니다(빈 이름)
		}
		if len(c) < 3 {
			return Completion{}, false
		}
		return Completion{Name: n, Detail: plain(c[1]), Example: code(c[2])}, true
	}},
	{"charts.md", "chart-types", "chart-type", func(c []string) (Completion, bool) {
		return Completion{Name: code(c[0]), Detail: plain(c[1]) + " — " + plain(c[2])}, len(c) >= 3
	}},
}

var (
	rowRe      = regexp.MustCompile("^\\| `[^`]+` \\|")
	headingRe  = regexp.MustCompile(`^(#{1,6}) (.+)$`)
	mdLinkRe   = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]+)\)`)
	backtickRe = regexp.MustCompile("`([^`]*)`")
)

// code 는 칸의 첫 `…` 안 글자다(없으면 칸 그대로).
func code(cell string) string {
	if m := backtickRe.FindStringSubmatch(cell); m != nil {
		return m[1]
	}
	return cell
}

// plain 은 칸의 마크다운 링크·백틱을 글자만 남긴다.
func plain(cell string) string {
	return strings.TrimSpace(backtickRe.ReplaceAllString(mdLinkRe.ReplaceAllString(cell, "$1"), "$1"))
}

// docLine 은 안내서 한 줄과 그 자리의 제목·syntax 주제다(코드 블록 밖에서만 센다).
type docLine struct {
	text, section, topic string
}

func scanDoc(docsDir, file string) ([]docLine, error) {
	b, err := os.ReadFile(filepath.Join(docsDir, file))
	if err != nil {
		return nil, err
	}
	var out []docLine
	section, topic, fence := "", "", ""
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if m := fenceRe.FindStringSubmatch(line); m != nil {
			switch {
			case fence == "":
				fence = m[1]
			case m[1][0] == fence[0] && len(m[1]) >= len(fence) && strings.TrimSpace(line) == m[1]:
				fence = ""
			}
		} else if fence == "" {
			if m := regionRe.FindStringSubmatch(line); m != nil {
				if m[2] == "begin" {
					topic = m[1]
				} else {
					topic = ""
				}
			} else if m := headingRe.FindStringSubmatch(line); m != nil && len(m[1]) >= 2 {
				section = strings.TrimSpace(backtickRe.ReplaceAllString(m[2], "$1"))
			}
		}
		out = append(out, docLine{line, section, topic})
	}
	return out, nil
}

// fileTopic 은 안내서의 첫 syntax 주제다 — 표의 뜻 칸이 다른 안내서로 링크하면 그 후보의 도움말은 그 주제다.
func fileTopic(docsDir, file string) string {
	lines, err := scanDoc(docsDir, file)
	if err != nil {
		return ""
	}
	for _, l := range lines {
		if l.topic != "" {
			return l.topic
		}
	}
	return ""
}

// Completions 는 안내서의 표지 표에서 자동 완성 후보를 뽑는다. 표지가 없거나, 표지가 syntax 구간 밖이거나(도움말에 없는 글),
// 줄을 읽지 못하거나, 표가 비었으면 오류다 — 안내서를 고쳐 후보가 조용히 빠지지 않게.
func Completions(docsDir string) ([]Completion, error) {
	var out []Completion
	for _, src := range completionSources {
		lines, err := scanDoc(docsDir, src.file)
		if err != nil {
			return nil, err
		}
		begin, end := "<!-- "+src.mark+":begin -->", "<!-- "+src.mark+":end -->"
		in, found, n := false, false, 0
		for _, l := range lines {
			switch l.text {
			case begin:
				in, found = true, true
				continue
			case end:
				in = false
			}
			if !in || !strings.HasPrefix(l.text, "| ") || strings.HasPrefix(l.text, "|---") {
				continue
			}
			if !rowRe.MatchString(l.text) {
				continue // 머리 줄(백틱 없음)
			}
			if l.topic == "" {
				return nil, fmt.Errorf("%s 의 %s 표가 syntax 구간 밖에 있음(도움말에 없는 글)", src.file, src.mark)
			}
			cells := splitRow(l.text)
			c, ok := src.row(cells)
			if !ok {
				return nil, fmt.Errorf("%s 의 %s 표 줄을 읽지 못함: %s", src.file, src.mark, l.text)
			}
			if c.Name == "" {
				continue
			}
			c.Kind, c.Topic, c.Section = src.kind, l.topic, l.section
			// 뜻 칸이 다른 주제의 안내서로 링크하면(차트·발표) 도움말은 그 주제의 처음이다
			for _, m := range mdLinkRe.FindAllStringSubmatch(strings.Join(cells, " "), -1) {
				if t := fileTopic(docsDir, m[2]); t != "" && t != c.Topic {
					c.Topic, c.Section = t, ""
					break
				}
			}
			// 차트 종류는 종류마다 절이 있다(### 3.1 `bar` — 세로 막대)
			if src.kind == "chart-type" {
				for _, h := range lines {
					if m := headingRe.FindStringSubmatch(h.text); m != nil && h.topic == c.Topic && strings.Contains(m[2], "`"+c.Name+"`") {
						c.Section = h.section
						break
					}
				}
			}
			out = append(out, c)
			n++
		}
		if !found || in {
			return nil, fmt.Errorf("%s 에 %s 표지가 없거나 닫히지 않음", src.file, src.mark)
		}
		if n == 0 {
			return nil, fmt.Errorf("%s 의 %s 표에 후보가 없음", src.file, src.mark)
		}
	}
	return out, nil
}

// splitRow 는 `| a | b |` 를 칸으로 나눈다(칸 안의 \| 는 없다고 본다 — 표지 표는 그렇게 쓴다).
func splitRow(line string) []string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// CompletionsJSON 은 넣을 후보 파일의 바이트다(줄마다 후보 하나 — 차이가 읽히게).
func CompletionsJSON(docsDir string) ([]byte, error) {
	cs, err := Completions(docsDir)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.WriteString("[\n")
	for i, c := range cs {
		var j bytes.Buffer
		enc := json.NewEncoder(&j)
		enc.SetEscapeHTML(false) // 뜻 칸의 < > & 를 글자 그대로(파일 차이가 읽히게)
		if err := enc.Encode(c); err != nil {
			return nil, err
		}
		b.Write(bytes.TrimSuffix(j.Bytes(), []byte("\n")))
		if i < len(cs)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("]\n")
	return b.Bytes(), nil
}
