package mdoc

import (
	"errors"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
)

// 코드 블록 src=(itda-hyve#880, codesrc.go): info string 가르기, ReadInclude 로 불러오기(코드 글자로만 — 이스케이프·강조·줄 강조),
// 내려받기 표시(BOM·줄 끝·마지막 줄 끝), 까닭별 오류 칸·경고(여는 펜스 줄), 블록 안 글·그림·수식·차트의 src 경고, 문서 모델(CodeText·CodeSource).

func TestCodeSrcParse(t *testing.T) {
	cases := []struct {
		info, lang, title, src, hl string
		hasSrc                     bool
	}{
		{`powershell title="~/a.ps1" src="./assets/03/a.ps1"`, "powershell", "~/a.ps1", "./assets/03/a.ps1", "", true},
		{`src="./a b.py"`, "", "a b.py", "./a b.py", "", true},
		{`src='x.go'`, "", "x.go", "x.go", "", true},
		{`go src=x.go {2}`, "go", "x.go", "x.go", "2", true},
		{`go src="a{2}.go" {1}`, "go", "a{2}.go", "a{2}.go", "1", true}, // src 값 안의 {…} 는 경로 글자
		{`go mysrc=x.go`, "go", "", "", "", false},
		{`go title="a src=b.go"`, "go", "a src=b.go", "", "", false}, // 다른 속성의 따옴표 값 안은 낱말이 아니다
		{`go src="x.go`, "go", "", "", "", true},                     // 닫히지 않은 따옴표 — 낱말은 있고 값이 없다
		{`go:main.go src=./m.go`, "go", "main.go", "./m.go", "", true},
	}
	for _, c := range cases {
		ci := parseCodeInfo([]byte(c.info))
		if ci.lang != c.lang || ci.title != c.title || ci.src != c.src || ci.hasSrc != c.hasSrc || ci.hl != c.hl {
			t.Errorf("%q: lang=%q title=%q src=%q hasSrc=%v hl=%q", c.info, ci.lang, ci.title, ci.src, ci.hasSrc, ci.hl)
		}
	}
}

// fakeFS 는 rel → 내용 / 오류 이다. 부른 rel 을 센다.
type fakeFS struct {
	files map[string][]byte
	errs  map[string]error
	calls map[string]int
}

func (f *fakeFS) read(rel string) ([]byte, error) {
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[rel]++
	if err, ok := f.errs[rel]; ok {
		return nil, err
	}
	if b, ok := f.files[rel]; ok {
		return b, nil
	}
	return nil, &IncludeError{Kind: IncludeMissing}
}

func codeSrcRender(t *testing.T, src string, fs *fakeFS) Result {
	t.Helper()
	opts := Options{}
	if fs != nil {
		opts.ReadInclude = fs.read
	}
	r, err := Render([]byte(src), opts)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func warnCodesOf(r Result) []string {
	var out []string
	for _, w := range r.Warnings {
		out = append(out, w.Code)
	}
	return out
}

func TestCodeSrcLoad(t *testing.T) {
	fs := &fakeFS{files: map[string][]byte{
		"assets/run.ps1": []byte("# 제목 {{secret:token}}\n<b>x</b> & y\nWrite-Host 1\n"),
	}}
	src := "본문\n\n```src=\"./assets/run.ps1\" {3}\n```\n"
	r := codeSrcRender(t, src, fs)
	if len(r.Warnings) != 0 {
		t.Fatalf("경고: %+v", r.Warnings)
	}
	h := r.HTML
	for _, want := range []string{
		`data-title="run.ps1"`, `data-src="./assets/run.ps1"`, `data-src-name="run.ps1"`, `data-hl="3"`,
		`class="language-powershell"`, `{{secret:token}}`, `&lt;`, `&amp;`,
	} {
		if !strings.Contains(h, want) {
			t.Errorf("HTML 에 %q 가 없음:\n%s", want, h)
		}
	}
	for _, bad := range []string{"<h1", "<b>", "data-src-bom", "data-src-eol", "data-src-nonl", "data-src-href"} {
		if strings.Contains(h, bad) {
			t.Errorf("HTML 에 %q 가 있으면 안 됨:\n%s", bad, h)
		}
	}
	if fs.calls["assets/run.ps1"] != 1 {
		t.Errorf("ReadInclude 는 정리된 경로로 한 번: %v", fs.calls)
	}
}

func TestCodeSrcBytesMarks(t *testing.T) {
	fs := &fakeFS{files: map[string][]byte{
		"a.txt":   []byte("\xef\xbb\xbf한글\r\n둘\r\n셋"), // BOM + CRLF + 마지막 줄 끝 없음
		"mix.sh":  []byte("a\r\nb\nc\r\n"),
		"cr.txt":  []byte("a\rb\r"),
		"same.sh": []byte("x\n"),
	}}
	r := codeSrcRender(t, "```src=a.txt\n```\n\n```src=mix.sh\n```\n\n```src=cr.txt\n```\n\n```src=same.sh\n```\n\n```src=same.sh\n```\n", fs)
	h := r.HTML
	for _, want := range []string{`data-src-bom="1"`, `data-src-eol="crlf"`, `data-src-nonl="1"`, `data-src-eol="cr"`, "한글\n둘\n셋\n"} {
		if !strings.Contains(h, want) {
			t.Errorf("HTML 에 %q 가 없음:\n%s", want, h)
		}
	}
	if strings.Contains(h, "\r") || strings.Contains(h, "\xef\xbb\xbf") {
		t.Error("화면 글자에 CR·BOM 이 남음")
	}
	codes := warnCodesOf(r)
	if len(codes) != 1 || codes[0] != WarnCodeSrcEOL || r.Warnings[0].Line != 4 {
		t.Errorf("줄 끝 섞임 경고 하나(4줄)여야 함: %+v", r.Warnings)
	}
	if fs.calls["same.sh"] != 1 {
		t.Errorf("같은 파일은 한 번만 읽음: %v", fs.calls)
	}
}

func TestCodeSrcRejects(t *testing.T) {
	big := []byte(strings.Repeat("a", MaxIncludeBytes+1))
	cases := []struct {
		src, code, msg string
		err            error
		data           []byte
	}{
		{"../x.ps1", WarnCodeSrcOutside, "문서 폴더 밖", nil, nil},
		{"/etc/x.sh", WarnCodeSrcOutside, "문서 폴더 밖", nil, nil},
		{"~/x.sh", WarnCodeSrcOutside, "문서 폴더 밖", nil, nil},
		{"https://e.com/x.sh", WarnCodeSrcOutside, "문서 폴더 밖", nil, nil},
		{`a\x.sh`, WarnCodeSrcOutside, "/ 로", nil, nil},
		{"a/../../x.sh", WarnCodeSrcOutside, "문서 폴더 밖", nil, nil},
		{".git/config.txt", WarnCodeSrcHidden, "숨긴 이름", nil, nil},
		{"a/.env.txt", WarnCodeSrcHidden, "숨긴 이름", nil, nil},
		{"tool.exe", WarnCodeSrcExt, "`.exe` 는 코드로 불러올 수 없는 파일입니다(텍스트 코드 파일만)", nil, nil},
		{"Makefile", WarnCodeSrcExt, "확장자가 없어", nil, nil},
		{"doc.md", WarnCodeSrcExt, "`.md`", nil, nil},
		{"", WarnCodeSrcMissing, "src 경로가 비어 있습니다", nil, nil},
		{"none.sh", WarnCodeSrcMissing, "`none.sh` 파일이 없습니다", nil, nil},
		{"link.sh", WarnCodeSrcLink, "심볼릭 링크", &IncludeError{Kind: IncludeLink}, nil},
		{"assets/03/a.ps1", WarnCodeSrcNotAllowed, "`assets/03/a.ps1` 을 쓰려면 `lecture.yml` 의 `files:` 에 적어 허용해야 합니다", &IncludeError{Kind: IncludeNotAllowed}, nil},
		{"new.sh", WarnCodeSrcUncommitted, "커밋에 없습니다", &IncludeError{Kind: IncludeUncommitted}, nil},
		{"dir.sh", WarnCodeSrcMissing, "보통 파일이 아니라", &IncludeError{Kind: IncludeIrregular}, nil},
		{"io.sh", WarnCodeSrcMissing, "읽지 못함", errors.New("권한"), nil},
		{"big.txt", WarnCodeSrcTooLarge, "256KB", nil, big},
		{"bad.txt", WarnCodeSrcNotUTF8, "UTF-8 글이 아니라", nil, []byte{0xff, 0xfe, 'a'}},
		{"nul.txt", WarnCodeSrcNotUTF8, "NUL", nil, []byte("a\x00b")},
	}
	for _, c := range cases {
		fs := &fakeFS{files: map[string][]byte{}, errs: map[string]error{}}
		if clean, perr := CheckIncludePath(c.src); perr == nil {
			if c.err != nil {
				fs.errs[clean] = c.err
			}
			if c.data != nil {
				fs.files[clean] = c.data
			}
		}
		src := "앞\n\n```sh src=\"" + c.src + "\"\n```\n"
		r := codeSrcRender(t, src, fs)
		if len(r.Warnings) != 1 || r.Warnings[0].Code != c.code || r.Warnings[0].Line != 3 || !strings.Contains(r.Warnings[0].Message, c.msg) {
			t.Errorf("%q: 경고 %+v — %s·3줄·%q 여야 함", c.src, r.Warnings, c.code, c.msg)
			continue
		}
		if !strings.Contains(r.HTML, `<div class="code-src-error" data-line="3"`) || !strings.Contains(r.HTML, `data-code="`+c.code+`"`) || strings.Contains(r.HTML, "<pre") {
			t.Errorf("%q: 오류 칸이 아님:\n%s", c.src, r.HTML)
		}
		if !CodeSrcErrorCode(c.code) {
			t.Errorf("%s 는 오류 칸 코드여야 함", c.code)
		}
	}
}

// ReadInclude 가 nil 이면(경로 없는 렌더) 오류 칸과 code-src-unchecked 다 — 확장자처럼 글자로 알 수 있는 까닭은 그 까닭이 먼저다.
func TestCodeSrcNoReader(t *testing.T) {
	r := codeSrcRender(t, "```ps1 src=./a.ps1\n```\n\n```src=a.exe\n```\n", nil)
	codes := warnCodesOf(r)
	if len(codes) != 2 || codes[0] != WarnCodeSrcUnchecked || codes[1] != WarnCodeSrcExt {
		t.Fatalf("경고: %+v", r.Warnings)
	}
	if !strings.Contains(r.Warnings[0].Message, "경로 없이 검사해 src 를 확인하지 못함") {
		t.Errorf("문구: %s", r.Warnings[0].Message)
	}
	if strings.Count(r.HTML, `class="code-src-error"`) != 2 || !strings.Contains(r.HTML, "<code>./a.ps1</code>") {
		t.Errorf("오류 칸 둘이어야 함:\n%s", r.HTML)
	}
}

func TestCodeSrcIgnored(t *testing.T) {
	fs := &fakeFS{files: map[string][]byte{"a.py": []byte("print(1)\n"), "g.txt": []byte("flowchart LR\n  A-->B\n")}}
	r := codeSrcRender(t, "```py src=a.py\nold()\n```\n\n```mermaid src=g.txt\nflowchart LR\n  X-->Y\n```\n\n```math src=g.txt\nx^2\n```\n\n:::chart {src=\"a.csv\"}\n| 항목 | 값 |\n|---|---|\n| a | 1 |\n:::\n", fs)
	var lines []int
	for _, w := range r.Warnings {
		switch w.Code {
		case WarnCodeSrcIgnored:
			lines = append(lines, w.Line)
		case WarnChart: // :::chart 의 src 는 모르는 차트 속성 경고(chart.go)가 이미 알린다
			if w.Line != 14 || !strings.Contains(w.Message, "src") {
				t.Errorf("차트 경고: %+v", w)
			}
		default:
			t.Errorf("다른 경고: %+v", w)
		}
	}
	if len(lines) != 3 || lines[0] != 1 || lines[1] != 5 || lines[2] != 10 {
		t.Errorf("ignored 경고 줄: %v (%+v)", lines, r.Warnings)
	}
	if !strings.Contains(r.HTML, "print") || strings.Contains(r.HTML, "old()") {
		t.Errorf("블록 안 글 대신 src 를 써야 함:\n%s", r.HTML)
	}
	if fs.calls["g.txt"] != 0 {
		t.Errorf("그림·수식의 src 는 읽지 않음: %v", fs.calls)
	}
	if !strings.Contains(r.HTML, `class="diagram"`) {
		t.Errorf("mermaid 는 블록 안 글로 그림:\n%s", r.HTML)
	}
}

func TestCodeSrcHref(t *testing.T) {
	fs := &fakeFS{files: map[string][]byte{"a/b.sh": []byte("echo\n")}}
	r, err := Render([]byte("```src=./a/b.sh\n```\n"), Options{ReadInclude: fs.read, IncludeHref: func(rel string) string { return "../../_f/x/" + rel }})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.HTML, `data-src-href="../../_f/x/a/b.sh"`) {
		t.Errorf("href 없음:\n%s", r.HTML)
	}
}

func TestCodeSrcDocModel(t *testing.T) {
	fs := &fakeFS{files: map[string][]byte{"a.go": []byte("package a")}}
	d, err := Parse([]byte("```src=a.go\n```\n\n```src=none.go\n```\n\n```go\nx\n```\n"), Options{ReadInclude: fs.read})
	if err != nil {
		t.Fatal(err)
	}
	var fcs []*ast.FencedCodeBlock
	_ = ast.Walk(d.Root(), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if fc, ok := n.(*ast.FencedCodeBlock); ok && entering {
			fcs = append(fcs, fc)
		}
		return ast.WalkContinue, nil
	})
	if len(fcs) != 3 {
		t.Fatalf("블록 %d", len(fcs))
	}
	if got := d.CodeText(fcs[0]); got != "package a\n" {
		t.Errorf("불러온 글: %q", got)
	}
	if ci := d.Code(fcs[0]); ci.Lang != "go" || ci.Title != "a.go" {
		t.Errorf("언어·파일명: %+v", ci)
	}
	if ci := d.Code(fcs[1]); ci.Lang != "" {
		t.Errorf("오류 블록은 강조하지 않음: %+v", ci)
	}
	cs, ok := d.CodeSource(fcs[1])
	if !ok || cs.Err == "" || cs.Code != WarnCodeSrcMissing || !strings.Contains(d.CodeText(fcs[1]), "불러오지 못함") {
		t.Errorf("오류 블록: %+v %q", cs, d.CodeText(fcs[1]))
	}
	if _, ok := d.CodeSource(fcs[2]); ok {
		t.Error("보통 블록은 src 블록이 아님")
	}
}

// CodeSrcPaths 는 렌더 없이 원문의 src 경로를 뽑는다 — 따옴표 꼴·없는 꼴·노트 안 포함, 그림·수식·잘못된 경로·겹침은 뺀다.
func TestCodeSrcPaths(t *testing.T) {
	md := "---\ntitle: t\n---\n\n" +
		"```ps1 src=\"./assets/run.ps1\"\n```\n\n" +
		"```src=b.txt title=\"보기\"\n```\n\n" +
		":::notes\n\n```src=notes/n.py\n```\n\n:::\n\n" +
		"```mermaid src=m.txt\nflowchart LR\n  a --> b\n```\n\n" +
		"```math src=x.txt\nx\n```\n\n" +
		"```src=../out.txt\n```\n\n" +
		"```src=.hidden/a.txt\n```\n\n" +
		"```src=a.exe\n```\n\n" +
		"```src=b.txt\n```\n\n" +
		"    ```src=indented.txt\n\n" + // 들여쓴 코드 블록 안의 글자 — 펜스가 아니다
		"`` `src=inline.txt` ``\n"
	got := CodeSrcPaths([]byte(md))
	want := []string{"assets/run.ps1", "b.txt", "notes/n.py"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := CodeSrcPaths([]byte("# 제목\n\n본문\n")); got != nil {
		t.Fatalf("src 없음: %q", got)
	}
	// 렌더가 읽으려 하는 경로와 같다(노트 포함 렌더)
	var read []string
	_, err := Render([]byte(md), Options{ReadInclude: func(rel string) ([]byte, error) { read = append(read, rel); return []byte("x\n"), nil }})
	if err != nil {
		t.Fatal(err)
	}
	if len(read) != len(want) {
		t.Fatalf("렌더가 읽은 경로: %q", read)
	}
	for _, r := range read {
		found := false
		for _, g := range got {
			found = found || g == r
		}
		if !found {
			t.Fatalf("렌더가 읽은 %q 가 CodeSrcPaths 에 없음(%q)", r, got)
		}
	}
}
