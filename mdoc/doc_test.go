package mdoc

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
)

// Render 는 parseLimit 를 거쳐 Doc.Result 를 돌려주므로(mdoc.go) golden 시험(golden_test.go)이 이미 Parse 의 길을 지난다.
// 여기서는 공개 입구 Parse 가 같은 Result 를 내는지만 견본 전부에서 본다 — Parse 가 다른 상한·선택지로 갈라지면 걸린다.
func TestParseResultMatchesRender(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("견본 없음: %v", err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.Base(f), func(t *testing.T) {
			want, err := Render(src, Options{})
			if err != nil {
				t.Fatal(err)
			}
			d, err := Parse(src, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(d.Result, want) {
				t.Fatal("Parse 의 Result 가 Render 와 다르다")
			}
			if d.Root() == nil {
				t.Fatal("Root 가 nil")
			}
		})
	}
}

func parseDoc(t *testing.T, src string, opts ...Options) *Doc {
	t.Helper()
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	d, err := Parse([]byte(src), o)
	if err != nil {
		t.Fatal(err)
	}
	if d.Root() == nil {
		t.Fatal("Root 가 nil")
	}
	return d
}

// nodes 는 문서 순서대로 T 노드를 모은다.
func nodes[T ast.Node](d *Doc) []T {
	var out []T
	_ = ast.Walk(d.Root(), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if v, ok := n.(T); ok && entering {
			out = append(out, v)
		}
		return ast.WalkContinue, nil
	})
	return out
}

func TestDocHeading(t *testing.T) {
	d := parseDoc(t, "---\nnumber-sections: true\n---\n\n# 하나\n\n## 둘 {#two}\n\n## 목록 밖 {.unlisted}\n")
	hs := nodes[*ast.Heading](d)
	if len(hs) != 3 {
		t.Fatalf("제목 %d 개", len(hs))
	}
	if got := d.Heading(hs[0]); got.Number != "1" || got.ID == "" || got.Unlisted {
		t.Errorf("첫 제목 %+v", got)
	}
	if got := d.Heading(hs[1]); got.ID != "two" || got.Number != "1.1" {
		t.Errorf("둘째 제목 %+v", got)
	}
	if got := d.Heading(hs[2]); !got.Unlisted {
		t.Errorf("셋째 제목 %+v", got)
	}
	if got := d.Line(hs[1]); got != 7 {
		t.Errorf("둘째 제목 줄 %d, 7 이어야(frontmatter 포함)", got)
	}
	if got := d.PlainText(hs[0]); got != "하나" {
		t.Errorf("PlainText %q", got)
	}
}

func TestDocImage(t *testing.T) {
	const png = "data:image/png;base64,iVBORw0KGgo="
	src := "![지역](img/a.png \"캡션\")\n\n![자료](" + png + ")\n\n![원격](https://example.com/a.png)\n\n![막음](javascript:alert(1))\n"
	cases := []struct {
		kind ImageKind
		alt  string
	}{{ImageLocal, "지역"}, {ImageData, "자료"}, {ImageRemote, "원격"}, {ImageBlocked, ""}}
	for _, allow := range []bool{false, true} {
		d := parseDoc(t, src, Options{AllowRemoteImages: allow})
		imgs := nodes[*ast.Image](d)
		if len(imgs) != len(cases) {
			t.Fatalf("그림 %d 개", len(imgs))
		}
		for i, c := range cases {
			got := d.Image(imgs[i])
			if got.Kind != c.kind {
				t.Errorf("allow=%v %d번째 종류 %v, %v 이어야", allow, i, got.Kind, c.kind)
			}
			if c.alt != "" && got.Alt != c.alt {
				t.Errorf("%d번째 대체 글 %q", i, got.Alt)
			}
		}
		if got := d.Image(imgs[0]); got.Dest != "img/a.png" || got.Title != "캡션" {
			t.Errorf("지역 그림 %+v", got)
		}
		if got := d.Line(imgs[2]); got != 5 {
			t.Errorf("원격 그림 줄 %d", got)
		}
	}
}

func TestDocLink(t *testing.T) {
	d := parseDoc(t, "[가](https://example.com/x) [나](javascript:alert(1)) <https://example.org>\n")
	links := nodes[*ast.Link](d)
	if len(links) != 2 {
		t.Fatalf("링크 %d 개", len(links))
	}
	if href, ok := d.Link(links[0]); !ok || href != "https://example.com/x" {
		t.Errorf("링크 %q %v", href, ok)
	}
	if _, ok := d.Link(links[1]); ok {
		t.Error("막은 주소 링크가 ok")
	}
	autos := nodes[*ast.AutoLink](d)
	if len(autos) != 1 {
		t.Fatalf("자동 링크 %d 개", len(autos))
	}
	if href, ok := d.Link(autos[0]); !ok || href != "https://example.org" {
		t.Errorf("자동 링크 %q %v", href, ok)
	}
	if got := d.AutoLinkLabel(autos[0]); got != "https://example.org" {
		t.Errorf("자동 링크 글자 %q", got)
	}
}

func TestDocMath(t *testing.T) {
	d := parseDoc(t, "가 $x^2$ 나 $\\foo{x}$\n\n$$\na+b\n$$\n")
	inl := nodes[*MathInline](d)
	if len(inl) != 2 {
		t.Fatalf("인라인 수식 %d 개", len(inl))
	}
	if m, ok := d.Math(inl[0]); !ok || !strings.HasPrefix(m, "<math") || !strings.Contains(m, "<msup>") {
		t.Errorf("그린 수식 %q %v", m, ok)
	}
	if got := d.MathSource(inl[0]); got != "$x^2$" {
		t.Errorf("수식 원문 %q", got)
	}
	if _, ok := d.Math(inl[1]); ok {
		t.Error("미지원 명령 수식이 ok")
	}
	if !hasWarn(d.Result, WarnMath) {
		t.Error("미지원 명령 경고 없음")
	}
	blk := nodes[*MathBlock](d)
	if len(blk) != 1 {
		t.Fatalf("수식 블록 %d 개", len(blk))
	}
	if m, ok := d.Math(blk[0]); !ok || !strings.Contains(m, "display=\"block\"") {
		t.Errorf("수식 블록 %q %v", m, ok)
	}
	if got := d.Line(blk[0]); got != 3 {
		t.Errorf("수식 블록 줄 %d", got)
	}
}

func TestDocDiagram(t *testing.T) {
	d := parseDoc(t, "```mermaid\n---\ntitle: 흐름 캡션\n---\nflowchart LR\n  a[가] --> b[나]\n```\n\n```mermaid\n이건 그림이 아님\n```\n")
	fcs := nodes[*ast.FencedCodeBlock](d)
	if len(fcs) != 2 {
		t.Fatalf("코드 블록 %d 개", len(fcs))
	}
	p, ok := d.Diagram(fcs[0])
	if !ok || !strings.HasPrefix(p.SVG, "<svg") || p.Caption != "흐름 캡션" {
		t.Errorf("그림 ok=%v 캡션 %q svg 앞 %.20q", ok, p.Caption, p.SVG)
	}
	if _, ok := d.Diagram(fcs[1]); ok {
		t.Error("읽지 못한 mermaid 가 그림으로 ok")
	}
}

func TestDocCode(t *testing.T) {
	d := parseDoc(t, "```go title=\"main.go\"\nfmt.Println(1)\n```\n\n``` title=\"run.py\"\nprint(1)\n```\n\n    들여 쓴 코드\n")
	fcs := nodes[*ast.FencedCodeBlock](d)
	if len(fcs) != 2 {
		t.Fatalf("펜스 코드 %d 개", len(fcs))
	}
	if got := d.Code(fcs[0]); got.Lang != "go" || got.Title != "main.go" {
		t.Errorf("첫 코드 %+v", got)
	}
	if got := d.Code(fcs[1]); got.Lang != "python" || got.Title != "run.py" {
		t.Errorf("파일명에서 짐작 %+v", got)
	}
	if got := d.CodeText(fcs[0]); got != "fmt.Println(1)\n" {
		t.Errorf("코드 글 %q", got)
	}
	ind := nodes[*ast.CodeBlock](d)
	if len(ind) != 1 {
		t.Fatalf("들여 쓴 코드 %d 개", len(ind))
	}
	if got := d.Code(ind[0]); got != (CodeInfo{}) {
		t.Errorf("들여 쓴 코드 %+v, 빈 값이어야", got)
	}
	if got := d.CodeText(ind[0]); got != "들여 쓴 코드\n" {
		t.Errorf("들여 쓴 코드 글 %q", got)
	}
}

func TestDocAlertTitle(t *testing.T) {
	if got := parseDoc(t, "> [!NOTE]\n> 가\n").AlertTitle("note"); got != "참고" {
		t.Errorf("lang 없음 %q", got)
	}
	if got := parseDoc(t, "---\nlang: en\n---\n\n> [!TIP]\n> a\n").AlertTitle("tip"); got != "Tip" {
		t.Errorf("lang en %q", got)
	}
}

// 블록 중첩 상한을 넘은 문서는 AST 없이 원문 줄을 내준다.
func TestDocRawLines(t *testing.T) {
	src := strings.Repeat(">", MaxNestingDepth+5) + " 깊다\n"
	d, err := Parse([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Root() != nil {
		t.Skip("이 입력은 상한을 넘지 않는다")
	}
	if got := d.RawLines(); len(got) != 1 || !strings.Contains(got[0], "깊다") {
		t.Errorf("RawLines %q", got)
	}
}
