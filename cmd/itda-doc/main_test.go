package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/itda-work/itda-doc/mdoc"
	"github.com/itda-work/itda-doc/page"
)

const (
	chartsGuide = "../../spec/samples/charts-guide.md"
	report      = "../../mdoc/testdata/report.md"
)

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := run(args, &out, &errOut)
	return out.String(), err
}

// render 의 기본 출력은 mdoc.Render(Options{}) 의 HTML 조각 그대로다.
func TestRender(t *testing.T) {
	out, err := runCLI(t, "render", report)
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	r, err := mdoc.Render(src, mdoc.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if out != r.HTML {
		t.Error("render 출력이 mdoc.Render 와 다름")
	}
}

// render 조각도 폴더 기본값(_metadata.yml)을 -page·check 와 같게 얹는다. 조각에 -theme 을 주면 stderr 에 알리고 그린다.
func TestRenderFolderMeta(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir) // 폴더 찾기는 홈에서 멈춘다 — 임시 폴더 밖을 읽지 않게
	doc := filepath.Join(dir, "docs", "a.md")
	if err := os.MkdirAll(filepath.Dir(doc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doc, []byte("# 제목\n\n## 절\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plain, err := runCLI(t, "render", doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_metadata.yml"), []byte("number-sections: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src, _ := os.ReadFile(doc)
	r, err := mdoc.Render(src, mdoc.Options{FolderMeta: page.FindFolderMetas(doc)})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := run([]string{"render", "-theme", "mono", doc}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != r.HTML || out.String() == plain {
		t.Errorf("폴더 기본값이 조각에 얹히지 않음\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "조각 출력은 테마를 쓰지 않는다") {
		t.Errorf("조각의 -theme 알림이 없음: %q", errOut.String())
	}
}

// -page -theme 에 없는 테마를 주면 오류(종료 2)이고, 메시지에 쓸 수 있는 테마 목록이 한 줄로 실린다.
func TestRenderPageUnknownTheme(t *testing.T) {
	out, err := runCLI(t, "render", "-page", "-theme", "nope", report)
	if err == nil || errors.Is(err, errWarnings) || out != "" {
		t.Fatalf("모르는 테마를 받음: %v", err)
	}
	if msg := err.Error(); !strings.Contains(msg, `"nope"`) || !strings.Contains(msg, "bold, classic, compact, mono, slate, spine, swiss") || strings.Contains(msg, "\n") {
		t.Errorf("메시지: %s", msg)
	}
}

// -page 는 base.css 와 고른 테마의 CSS(@scope (.page))를 넣고 본문을 .page 상자에 담은 단일 HTML 이다 — JS 는 없다.
func TestRenderPage(t *testing.T) {
	out, err := runCLI(t, "render", "-page", "-theme", "mono", chartsGuide)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<!doctype html>", "<title>차트 안내서 예제</title>", "@scope (.page) {", "/* 테마 mono", `<div class="page"><div class="page-body">`, `<figure class="chart chart-bar`} {
		if !strings.Contains(out, want) {
			t.Errorf("단일 HTML 에 %q 가 없음", want)
		}
	}
	if strings.Contains(out, "<script") {
		t.Error("단일 HTML 에 script 가 있음")
	}
}

// check 는 경고를 "파일:줄: 코드 메시지" 로 내고, 경고가 있으면 errWarnings(종료 코드 1)다. 경고 없는 견본은 nil.
func TestCheck(t *testing.T) {
	out, err := runCLI(t, "check", report)
	if !errors.Is(err, errWarnings) {
		t.Fatalf("경고 있는 문서의 오류: %v", err)
	}
	if !strings.Contains(out, report+":97: remote-image ") {
		t.Errorf("경고 줄 꼴:\n%s", out)
	}
	if out, err := runCLI(t, "check", chartsGuide); err != nil || out != "" {
		t.Errorf("견본에 경고: %v\n%s", err, out)
	}
}

// syntax 는 주제 목록, 주제 하나의 글, 모르는 주제는 오류다.
func TestSyntax(t *testing.T) {
	out, err := runCLI(t, "syntax")
	if err != nil || !strings.Contains(out, "core\n") || !strings.HasSuffix(out, "all\n") {
		t.Errorf("주제 목록: %v\n%s", err, out)
	}
	out, err = runCLI(t, "syntax", "charts")
	if err != nil || !strings.Contains(out, "주제 charts") {
		t.Errorf("주제 글: %v\n%.200s", err, out)
	}
	if _, err := runCLI(t, "syntax", "nope"); err == nil {
		t.Error("모르는 주제를 받음")
	}
}

// themes 는 내장 다섯과 사용자 테마 폴더의 테마를 보인다. 쓸 수 없는 테마는 까닭과 함께.
func TestThemes(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(dir+"/broken", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dir+"/broken/theme.json", []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCLI(t, "themes", "-themes", dir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "경고: ") {
		t.Errorf("경고가 없어야 할 목록에 경고가 있음:\n%s", out)
	}
	for _, n := range []string{"bold", "classic", "compact", "mono", "slate", "spine", "swiss"} {
		if !strings.Contains(out, n+"\t내장\t") {
			t.Errorf("내장 테마 %s 가 없음:\n%s", n, out)
		}
	}
	if !strings.Contains(out, "broken\t사용자\t쓸 수 없음: ") {
		t.Errorf("깨진 테마의 까닭이 없음:\n%s", out)
	}
	for _, n := range []string{"toss", "kakao", "naver", "daangn"} {
		if strings.Contains(out, n) {
			t.Errorf("브랜드 테마 %s 가 목록에 있음", n)
		}
	}
}

// themes 는 어휘 밖 group 을 가진 테마 줄 끝에 경고를 붙인다 — 테마는 쓸 수 있는 채로(itda-doc#24).
func TestThemesGroupWarning(t *testing.T) {
	dir := t.TempDir()
	for name, js := range map[string]string{
		"fun":  `{"title":"재미","extends":"classic","group":"Vivid"}`,
		"good": `{"title":"좋음","extends":"classic","group":"vivid"}`,
	} {
		if err := os.MkdirAll(dir+"/"+name, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dir+"/"+name+"/theme.json", []byte(js), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, err := runCLI(t, "themes", "-themes", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "fun\t사용자\t재미\t경고: group \"Vivid\" 는 모르는 묶음") {
		t.Errorf("fun 줄에 group 경고가 없음:\n%s", out)
	}
	if !strings.Contains(out, "good\t사용자\t좋음\n") {
		t.Errorf("good 줄이 경고 없이 나오지 않음:\n%s", out)
	}
}
