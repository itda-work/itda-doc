package mdsyntax

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/mdsyntax/extract"
)

// 넣은 단일본은 안내서에서 다시 만든 것과 바이트까지 같다 — 안내서를 고치고 `just syntax`(go generate)를 잊으면 여기서 실패한다.
func TestBundleFresh(t *testing.T) {
	want, err := extract.Build(filepath.Join("..", "spec"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range Topics {
		got, err := topicFS.ReadFile("topics/" + name + ".md")
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want[name]) {
			t.Errorf("topics/%s.md 가 안내서와 다름 — just syntax 로 다시 만든다", name)
		}
	}
	comp, err := extract.CompletionsJSON(filepath.Join("..", "spec"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(completionsJSON, comp) {
		t.Error("completions.json 이 안내서 표와 다름 — just syntax 로 다시 만든다")
	}
	entries, _ := fs.ReadDir(topicFS, "topics")
	for _, e := range entries {
		if !slices.Contains(Topics, strings.TrimSuffix(e.Name(), ".md")) {
			t.Errorf("모르는 주제 파일 topics/%s", e.Name())
		}
	}
}

func TestTopic(t *testing.T) {
	idx, err := Topic("")
	if err != nil || !strings.Contains(idx, "주제 index") {
		t.Fatalf("빈 주제는 index: %v", err)
	}
	all, err := Topic("ALL")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range Topics {
		s, err := Topic(name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(all, s) {
			t.Errorf("all 에 %s 가 없음", name)
		}
		// 시험이 읽는 표지·화면 그림은 빠진다
		for _, bad := range []string{"<!-- syntax:", "<!-- chart-example", "<!-- md-example", ":begin -->", "](images/charts-"} {
			if strings.Contains(s, bad) {
				t.Errorf("%s 에 %q 가 남음", name, bad)
			}
		}
	}
	if _, err := Topic("tables"); err == nil || !strings.Contains(err.Error(), "core") {
		t.Errorf("모르는 주제: %v", err)
	}
	if h := Hash(); len(h) != 12 || h != Hash() {
		t.Errorf("Hash %q", h)
	}
}

// 코드 블록 안의 주석(<!-- pagebreak -->)은 문법이라 남는다. 구간이 겹치거나 닫히지 않으면 오류다.
func TestBuildRules(t *testing.T) {
	core, _ := Topic("core")
	if !strings.Contains(core, "<!-- pagebreak -->") {
		t.Error("core 에서 쪽 나눔 표기가 빠짐")
	}
	if !strings.Contains(core, "## 9. 경고 코드") || !strings.Contains(core, "`list-marker`") {
		t.Error("core 에 경고 코드 표가 없음")
	}
}

// 컨테이너 표에 줄이 더해지면 후보가 따라 늘고 이름·뜻이 표 칸 그대로다(#434 인수 조건 — 안내서를 고치고
// just syntax 를 잊으면 TestBundleFresh 가 completions.json 차이로 실패한다는 것의 뽑는 쪽 반을 여기서 본다).
func TestCompletionsFollowGuideTable(t *testing.T) {
	dir := t.TempDir()
	for _, src := range extract.Sources {
		b, err := os.ReadFile(filepath.Join("..", "spec", src))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, src), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(dir, "markdown.md")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	added := strings.Replace(string(b), "<!-- containers:end -->", "| `sidebar` | 옆 칸 |\n<!-- containers:end -->", 1)
	if added == string(b) {
		t.Fatal("컨테이너 표지를 찾지 못함 — 시험 준비가 깨졌다")
	}
	if err := os.WriteFile(p, []byte(added), 0o644); err != nil {
		t.Fatal(err)
	}
	cs, err := extract.Completions(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got *extract.Completion
	for i, c := range cs {
		if c.Name == "sidebar" {
			got = &cs[i]
		}
	}
	if got == nil || got.Kind != "container" || got.Detail != "옆 칸" || got.Topic != "core" {
		t.Errorf("표에 더한 줄이 후보로 나오지 않음: %+v", got)
	}
	for _, bad := range []string{"|", "`"} {
		if got != nil && strings.ContainsAny(got.Detail, bad) {
			t.Errorf("후보 뜻에 마크다운 표기가 남음: %q", got.Detail)
		}
	}
}

// 표지 꼴이 틀리거나(대문자·꼬리 공백) Sources 밖 안내서에 표지가 있으면 조용히 빠지지 않고 오류다(리뷰 R1 P3-15).
func TestBuildRejectsStrayMarkers(t *testing.T) {
	for name, extra := range map[string]string{
		"markdown.md":       "\n<!-- syntax:Core:begin -->\n",
		"markdown-extra.md": "\n<!-- syntax:core:begin -->\nx\n<!-- syntax:core:end -->\n",
		"slides.md":         "\n<!-- syntax:slides:end --> \n",
	} {
		dir := t.TempDir()
		for _, src := range extract.Sources {
			b, err := os.ReadFile(filepath.Join("..", "spec", src))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, src), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := extract.Build(dir); err != nil {
			t.Fatalf("복사본 Build: %v", err)
		}
		p := filepath.Join(dir, name)
		b, _ := os.ReadFile(p)
		if err := os.WriteFile(p, append(b, extra...), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := extract.Build(dir); err == nil {
			t.Errorf("%s 에 %q 를 더해도 오류가 아님", name, extra)
		}
	}
}
