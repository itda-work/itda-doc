// itda-doc 은 Itda Doc 문서를 HTML 로 그리고(render), 경고를 보고(check), 문법 안내(syntax)와 테마 목록(themes)을 보이는 명령이다.
//
//	itda-doc render [-theme NAME] [-themes DIR] [-page] FILE
//	itda-doc check [-themes DIR] FILE...
//	itda-doc syntax [TOPIC]
//	itda-doc themes [-themes DIR]
package main

import (
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/itda-work/itda-doc/mdoc"
	"github.com/itda-work/itda-doc/mdsyntax"
	"github.com/itda-work/itda-doc/page"
	"github.com/itda-work/itda-doc/web"
)

// maxFileBytes 는 읽을 문서의 상한이다(page.MaxCheckBytes 와 같은 4MB).
const maxFileBytes = page.MaxCheckBytes

// errWarnings 는 check 가 경고를 하나라도 찾았다는 뜻이다(종료 코드 1, 따로 알리지 않는다).
var errWarnings = errors.New("경고 있음")

const usage = `itda-doc — Itda Doc 마크다운 문서를 그리고 검사한다.

쓰는 법:
  itda-doc render [-theme NAME] [-themes DIR] [-page] FILE   HTML 을 stdout 에(-page 면 표지·차례·테마 CSS 까지 든 단일 HTML)
  itda-doc check [-themes DIR] FILE...                       경고를 "파일:줄: 코드 메시지" 로, 하나라도 있으면 종료 코드 1
  itda-doc syntax [TOPIC]                                    문법 안내 주제 목록, 또는 한 주제의 글(all 이면 모두)
  itda-doc themes [-themes DIR]                              내장·사용자 테마 목록(쓸 수 없는 테마는 까닭과 함께)
`

func main() {
	err := run(os.Args[1:], os.Stdout, os.Stderr)
	switch {
	case err == nil:
	case errors.Is(err, errWarnings):
		os.Exit(1)
	case errors.Is(err, flag.ErrHelp):
	default:
		fmt.Fprintln(os.Stderr, "itda-doc:", err)
		os.Exit(2)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return errors.New("하위 명령이 없음")
	}
	switch args[0] {
	case "-h", "-help", "--help", "help":
		fmt.Fprint(stdout, usage)
		return nil
	case "render":
		return cmdRender(args[1:], stdout, stderr)
	case "check":
		return cmdCheck(args[1:], stdout, stderr)
	case "syntax":
		return cmdSyntax(args[1:], stdout, stderr)
	case "themes":
		return cmdThemes(args[1:], stdout, stderr)
	}
	fmt.Fprint(stderr, usage)
	return fmt.Errorf("모르는 하위 명령 %q", args[0])
}

func newFlags(name, line string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "itda-doc %s — %s\n", name, line)
		fs.PrintDefaults()
	}
	return fs
}

func readDoc(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxFileBytes {
		return nil, fmt.Errorf("%s: 문서가 너무 큼(상한 %d MB)", p, maxFileBytes>>20)
	}
	return b, nil
}

func cmdRender(args []string, stdout, stderr io.Writer) error {
	fs := newFlags("render", "문서를 HTML 로 그린다(기본은 본문 조각, -page 면 단일 HTML 문서)", stderr)
	theme := fs.String("theme", "", "-page 에서 쓸 테마(비면 문서의 theme:, 그다음 classic)")
	themesDir := fs.String("themes", "", "-page 에서 함께 읽을 사용자 테마 폴더")
	full := fs.Bool("page", false, "표지·차례를 조립하고 base.css·테마 CSS 를 넣은 단일 HTML 로 낸다(JS 없음 — 쪽 나눔은 뷰어 몫)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("render 는 파일 하나를 받는다")
	}
	src, err := readDoc(fs.Arg(0))
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		return err
	}
	if !*full {
		themeSet := false
		fs.Visit(func(f *flag.Flag) { themeSet = themeSet || f.Name == "theme" || f.Name == "themes" })
		if themeSet {
			fmt.Fprintln(stderr, "itda-doc: 조각 출력은 테마를 쓰지 않는다 — -page 와 함께 쓴다")
		}
		// 폴더 기본값(_metadata.yml)은 -page·check 와 같게 읽는다
		r, err := mdoc.Render(src, mdoc.Options{FolderMeta: page.FindFolderMetas(abs)})
		if err != nil {
			return err
		}
		_, err = io.WriteString(stdout, r.HTML)
		return err
	}
	ts, err := page.LoadCheckThemes(*themesDir)
	if err != nil {
		return err
	}
	if *theme != "" {
		if _, ok := ts.Get(*theme); !ok {
			var names []string
			for _, i := range ts.List() {
				if i.Error == "" {
					names = append(names, i.Name)
				}
			}
			return fmt.Errorf("모르는 테마 %q — 쓸 수 있는 테마: %s", *theme, strings.Join(names, ", "))
		}
	}
	v := page.RenderView(src, abs, "", ts, *theme, false)
	if v.Error != "" {
		return errors.New(v.Error)
	}
	t, _ := ts.Get(v.Theme)
	base, err := web.FS.ReadFile("base.css")
	if err != nil {
		return err
	}
	_, err = io.WriteString(stdout, pageHTML(v, string(base), t))
	return err
}

// pageHTML 은 View 를 단일 HTML 로 감싼다. 테마 CSS 는 @scope (.page) 로 갇히므로 본문을 .page 상자 안에 둔다.
func pageHTML(v page.View, base string, t *page.Theme) string {
	var b strings.Builder
	lang := v.Lang
	if lang == "" {
		lang = "ko"
	}
	b.WriteString("<!doctype html>\n<html lang=\"" + html.EscapeString(lang) + "\">\n<head>\n<meta charset=\"utf-8\">\n")
	b.WriteString("<title>" + html.EscapeString(v.Title) + "</title>\n<style>\n")
	b.WriteString(base)
	b.WriteString("\n</style>\n")
	if t != nil {
		b.WriteString("<style>\n" + t.ThemeStyleSheet() + "\n</style>\n")
	}
	body := "paper-" + v.Paper
	for c := range strings.FieldsSeq(v.ListStyle) {
		body += " list-" + c
	}
	b.WriteString("</head>\n<body class=\"" + html.EscapeString(body) + "\">\n<div class=\"page\"><div class=\"page-body\">\n")
	b.WriteString(v.HTML)
	b.WriteString("\n</div></div>\n</body>\n</html>\n")
	return b.String()
}

func cmdCheck(args []string, stdout, stderr io.Writer) error {
	fs := newFlags("check", "문서의 경고를 \"파일:줄: 코드 메시지\" 로 보인다(경고가 있으면 종료 코드 1)", stderr)
	themesDir := fs.String("themes", "", "함께 읽을 사용자 테마 폴더")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return errors.New("check 는 파일을 하나 이상 받는다")
	}
	ts, err := page.LoadCheckThemes(*themesDir)
	if err != nil {
		return err
	}
	warned := false
	for _, p := range fs.Args() {
		src, err := readDoc(p)
		if err != nil {
			return err
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		res, err := page.Check(src, abs, ts)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		for _, w := range res.Warnings {
			warned = true
			fmt.Fprintf(stdout, "%s:%d: %s %s\n", p, w.Line, w.Code, w.Message)
		}
	}
	if warned {
		return errWarnings
	}
	return nil
}

func cmdSyntax(args []string, stdout, stderr io.Writer) error {
	fs := newFlags("syntax", "문법 안내 — 주제 이름이 없으면 주제 목록", stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	switch fs.NArg() {
	case 0:
		for _, t := range mdsyntax.Topics {
			fmt.Fprintln(stdout, t)
		}
		fmt.Fprintln(stdout, mdsyntax.TopicAll)
		return nil
	case 1:
		s, err := mdsyntax.Topic(fs.Arg(0))
		if err != nil {
			return err
		}
		_, err = io.WriteString(stdout, s)
		return err
	}
	fs.Usage()
	return errors.New("syntax 는 주제를 하나까지 받는다")
}

func cmdThemes(args []string, stdout, stderr io.Writer) error {
	fs := newFlags("themes", "내장·사용자 테마 목록", stderr)
	themesDir := fs.String("themes", "", "함께 읽을 사용자 테마 폴더")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return errors.New("themes 는 인자를 받지 않는다")
	}
	ts, err := page.LoadCheckThemes(*themesDir)
	if err != nil {
		return err
	}
	for _, i := range ts.List() {
		kind := "내장"
		if i.User {
			kind = "사용자"
		}
		switch {
		case i.Error != "":
			fmt.Fprintf(stdout, "%s\t%s\t쓸 수 없음: %s\n", i.Name, kind, i.Error)
		case i.Description != "":
			fmt.Fprintf(stdout, "%s\t%s\t%s — %s\n", i.Name, kind, i.Title, i.Description)
		default:
			fmt.Fprintf(stdout, "%s\t%s\t%s\n", i.Name, kind, i.Title)
		}
	}
	return nil
}
