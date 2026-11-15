package mdoc

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/util"
)

// 코드 블록 src= — 문서 폴더의 소스 파일을 코드 블록으로 불러온다(itda-hyve#880).
//
//	```powershell title="~/assure-identity.ps1" src="./assets/03/assure-identity.ps1"
//	```
//
// 렌더러는 경로를 모르는 순수 함수 그대로다 — 파일 읽기는 호출자가 Options.ReadInclude 로 넘긴다(FolderMeta 와 같은 원칙). 이 파일은
// 경로의 글자 검사(CheckIncludePath — 폴더 밖·숨김·확장자), 읽은 바이트의 검사(크기·UTF-8)·정규화(BOM·줄 끝)와 오류 칸·경고를 맡는다.
// 불러온 글자는 코드 내용으로만 쓴다 — 마크다운으로 해석하지 않고(HTML 이스케이프, 구문 강조만) {{secret:}} 같은 자리표시자도 펼치지 않는다.
// 원문을 미리 펼쳐 넣지 않는다 — 원문 줄 번호(경고·편집기 동기화)는 그대로다.
//
// 내려받기(화면 — 머리 띠 단추)는 pre 의 data-src-* 표시로 화면 글자에서 원본 바이트를 되살린다: data-src-bom(UTF-8 BOM 이 있었다)·
// data-src-eol(crlf·cr — 없으면 lf)·data-src-nonl(마지막 줄 끝이 없었다). 줄 끝이 섞인 파일은 경고하고 많은 쪽으로 맞춘다.

const (
	// MaxIncludeBytes 는 src 파일 하나의 상한이다(바이트). 넘으면 오류 칸이다.
	MaxIncludeBytes = 256 << 10
	// maxIncludeDocBytes 는 한 문서가 불러오는 src 파일 바이트의 합 상한이다(같은 파일은 한 번만 센다).
	maxIncludeDocBytes = 4 << 20
)

// codeSrcExts 는 코드로 불러올 수 있는 확장자다(소문자, 점 포함 — itda-hyve#880 에서 정한 텍스트 코드 파일 목록).
var codeSrcExts = map[string]bool{}

// CodeSrcExts 는 허용 확장자 목록이다(정렬하지 않은 원래 차례 — 안내서·호출자의 안내 문구에 쓴다).
var CodeSrcExts = []string{".ps1", ".psm1", ".bat", ".cmd", ".sh", ".bash", ".zsh", ".py", ".js", ".mjs", ".ts", ".go", ".rs", ".java",
	".c", ".h", ".cpp", ".cs", ".rb", ".php", ".sql", ".yml", ".yaml", ".json", ".toml", ".ini", ".xml", ".html", ".css", ".txt", ".csv"}

func init() {
	for _, e := range CodeSrcExts {
		codeSrcExts[e] = true
	}
}

// IncludeKind 는 src 파일을 쓰지 못한 까닭이다. 까닭마다 경고 코드(includeCodes)와 오류 칸 문구(includeMessage)가 하나씩이다.
type IncludeKind int

const (
	IncludeMissing     IncludeKind = iota + 1 // 파일이 없음(또는 경로가 비었음·읽지 못함)
	IncludeOutside                            // 문서 폴더 밖(절대 경로·..·주소·역슬래시)
	IncludeHidden                             // 숨긴 이름(. 으로 시작하는 마디)
	IncludeLink                               // 심볼릭 링크를 거침
	IncludeExt                                // 허용 확장자가 아님
	IncludeTooLarge                           // 크기 상한을 넘음
	IncludeNotUTF8                            // UTF-8 글이 아님
	IncludeNotAllowed                         // 문서 위 lecture.yml 의 files: 에 없음(호출자가 판정)
	IncludeUncommitted                        // 커밋에 없음(묶기 — 호출자가 판정)
	IncludeIrregular                          // 보통 파일이 아님(폴더·장치 등)
)

// IncludeError 는 ReadInclude 가 돌려주는 까닭 있는 오류다. Detail 은 문구 뒤에 붙는 덧말이다(비어도 된다).
// ReadInclude 가 다른 오류를 돌려주면 「읽지 못함」(IncludeMissing 의 코드)으로 다룬다.
type IncludeError struct {
	Kind   IncludeKind
	Detail string
}

func (e *IncludeError) Error() string { return fmt.Sprintf("src: %s", includeMessage("", e)) }

// includeCodes 는 까닭 → 경고 코드다.
var includeCodes = map[IncludeKind]string{
	IncludeMissing:     WarnCodeSrcMissing,
	IncludeOutside:     WarnCodeSrcOutside,
	IncludeHidden:      WarnCodeSrcHidden,
	IncludeLink:        WarnCodeSrcLink,
	IncludeExt:         WarnCodeSrcExt,
	IncludeTooLarge:    WarnCodeSrcTooLarge,
	IncludeNotUTF8:     WarnCodeSrcNotUTF8,
	IncludeNotAllowed:  WarnCodeSrcNotAllowed,
	IncludeUncommitted: WarnCodeSrcUncommitted,
	IncludeIrregular:   WarnCodeSrcMissing,
}

// CodeSrcErrorCode 는 경고 코드가 src 블록을 오류 칸으로 그린 것인지다(불러오지 못함 — 경로 없는 검사 포함). 게시 묶음이 이 경고가
// 하나라도 있으면 거부할 때 쓴다(itda-hyve#880). code-src-ignored·code-src-eol 은 블록을 그렸으므로 아니다.
func CodeSrcErrorCode(code string) bool {
	switch code {
	case WarnCodeSrcUnchecked, WarnCodeSrcMissing, WarnCodeSrcOutside, WarnCodeSrcHidden, WarnCodeSrcLink, WarnCodeSrcExt,
		WarnCodeSrcTooLarge, WarnCodeSrcNotUTF8, WarnCodeSrcNotAllowed, WarnCodeSrcUncommitted:
		return true
	}
	return false
}

// includeMessage 는 까닭별 한 문장이다(오류 칸·경고 같은 글). src 는 원문에 적힌 값이다. 경로는 `…` 로 감싼다(오류 칸은 code 로 그린다).
func includeMessage(src string, e *IncludeError) string {
	q := "`" + src + "`"
	var m string
	switch e.Kind {
	case IncludeOutside:
		m = q + " 는 문서 폴더 밖을 가리켜 불러올 수 없습니다(문서 폴더 아래 상대 경로만)"
	case IncludeHidden:
		m = q + " 에 숨긴 이름(. 으로 시작)이 있어 불러오지 않습니다"
	case IncludeLink:
		m = q + " 는 심볼릭 링크를 거쳐 불러오지 않습니다(실제 파일 경로를 적습니다)"
	case IncludeExt:
		if ext := path.Ext(src); ext != "" {
			m = "`" + strings.ToLower(ext) + "` 는 코드로 불러올 수 없는 파일입니다(텍스트 코드 파일만)"
		} else {
			m = q + " 는 확장자가 없어 코드로 불러올 수 없는 파일입니다(텍스트 코드 파일만)"
		}
	case IncludeTooLarge:
		m = fmt.Sprintf("%s 가 너무 커서(%dKB 넘음) 불러오지 않습니다", q, MaxIncludeBytes>>10)
	case IncludeNotUTF8:
		m = q + " 는 UTF-8 글이 아니라 불러오지 않습니다(UTF-8 로 저장합니다)"
	case IncludeNotAllowed:
		m = q + " 을 쓰려면 `lecture.yml` 의 `files:` 에 적어 허용해야 합니다"
	case IncludeUncommitted:
		m = q + " 가 커밋에 없습니다 — 묶기는 커밋한 판만 읽습니다"
	case IncludeIrregular:
		m = q + " 는 보통 파일이 아니라 불러오지 않습니다"
	default:
		m = q + " 파일이 없습니다"
		if src == "" {
			m = "src 경로가 비어 있습니다"
		}
	}
	if e.Detail != "" {
		m += " — " + e.Detail
	}
	return m
}

// includeSchemeRe 는 주소·드라이브 꼴(http:·file:·C:)의 머리다.
var includeSchemeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// CheckIncludePath 는 src 값의 글자만 보고(디스크를 보지 않는다) 문서 폴더 기준의 정리된 상대 경로(슬래시)를 돌려준다. 빈 값·주소·
// 절대 경로·~·역슬래시·폴더 밖(..)·숨긴 마디·허용 밖 확장자는 *IncludeError 다. 렌더러가 ReadInclude 를 부르기 전에 쓰고, 디스크를
// 읽는 호출자도 같은 판정으로 다시 본다.
func CheckIncludePath(src string) (string, *IncludeError) {
	switch {
	case src == "" || strings.TrimSpace(src) == "":
		return "", &IncludeError{Kind: IncludeMissing}
	case strings.ContainsAny(src, "\\\x00"):
		return "", &IncludeError{Kind: IncludeOutside, Detail: "경로 구분은 / 로 적습니다"}
	case strings.HasPrefix(src, "/") || strings.HasPrefix(src, "~") || includeSchemeRe.MatchString(src):
		return "", &IncludeError{Kind: IncludeOutside}
	}
	clean := path.Clean(src)
	if clean == "." {
		return "", &IncludeError{Kind: IncludeMissing}
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", &IncludeError{Kind: IncludeOutside}
	}
	for seg := range strings.SplitSeq(clean, "/") {
		if strings.HasPrefix(seg, ".") {
			return "", &IncludeError{Kind: IncludeHidden}
		}
	}
	if !codeSrcExts[strings.ToLower(path.Ext(clean))] {
		return "", &IncludeError{Kind: IncludeExt}
	}
	return clean, nil
}

// codeSrc 는 src 블록 하나를 불러온 결과다. err 가 있으면 오류 칸(msg·code)이다.
type codeSrc struct {
	src  string // 원문에 적힌 값
	rel  string // 정리된 상대 경로(CheckIncludePath)
	text string // 불러온 글자 — 줄 끝은 \n, 비지 않으면 \n 으로 끝난다
	n    int    // 내용 줄 수
	bom  bool   // 원본에 UTF-8 BOM 이 있었다
	eol  string // 원본 줄 끝: "" (lf 또는 줄 끝 없음)·"crlf"·"cr"
	nonl bool   // 원본 마지막 줄에 줄 끝이 없었다
	href string // 원본 바이트 주소(Options.IncludeHref — 묶음의 첨부 주소)
	mix  bool   // 줄 끝이 섞여 있었다(경고)
	err  *IncludeError
	msg  string // 오류 칸·경고 문구
	code string // 경고 코드
}

// includeData 는 한 렌더에서 읽은 파일 하나다(같은 rel 을 여러 블록이 불러도 한 번만 읽고 한 번만 센다).
type includeData struct {
	b   []byte
	err *IncludeError
}

// codeSource 는 src 블록 fc 를 불러온다. 경고(까닭별 코드, 블록 안 글과 함께 적은 것, 줄 끝 섞임)는 여기서 낸다(블록마다 한 번 — codeBlock 의
// 들어가는 자리에서만 부른다). 결과는 문서 모델(Doc.CodeText·CodeSource)이 다시 쓴다.
func (st *state) codeSource(fc *ast.FencedCodeBlock, info codeInfo) *codeSrc {
	if cs, ok := st.srcOut[fc]; ok {
		return cs
	}
	line := st.lines.line(fc.Pos())
	cs := &codeSrc{src: info.src}
	st.srcOut[fc] = cs
	if hasCodeBody(fc, st.src) {
		st.warn(line, WarnCodeSrcIgnored, fmt.Sprintf("코드 블록 안 글과 src 가 함께 있음 — src 파일(%s) 내용을 쓰고 블록 안 글은 버림", info.src))
	}
	fail := func(e *IncludeError) *codeSrc {
		cs.err, cs.msg, cs.code = e, includeMessage(info.src, e), includeCodes[e.Kind]
		if cs.code == "" {
			cs.code = WarnCodeSrcMissing
		}
		st.warn(line, cs.code, cs.msg)
		return cs
	}
	rel, perr := CheckIncludePath(info.src)
	if perr != nil {
		return fail(perr)
	}
	cs.rel = rel
	if st.opts.ReadInclude == nil {
		cs.msg, cs.code = fmt.Sprintf("`%s` — 문서 파일 경로 없이 검사해 src 를 확인하지 못함(문서 파일로 열거나 경로로 검사합니다)", info.src), WarnCodeSrcUnchecked
		cs.err = &IncludeError{Kind: IncludeMissing}
		st.warn(line, cs.code, cs.msg)
		return cs
	}
	d, ok := st.srcFiles[rel]
	if !ok {
		d = st.readInclude(rel)
		st.srcFiles[rel] = d
	}
	if d.err != nil {
		return fail(d.err)
	}
	b := d.b
	if bytes.HasPrefix(b, []byte("\xef\xbb\xbf")) {
		cs.bom, b = true, b[3:]
	}
	crlf, lf, cr := countEOL(b)
	switch {
	case crlf >= lf && crlf >= cr && crlf > 0:
		cs.eol = "crlf"
	case cr > lf && cr > crlf:
		cs.eol = "cr"
	}
	if kinds := btoi(crlf > 0) + btoi(lf > 0) + btoi(cr > 0); kinds > 1 {
		cs.mix = true
		want := map[string]string{"": "LF", "crlf": "CRLF", "cr": "CR"}[cs.eol]
		st.warn(line, WarnCodeSrcEOL, fmt.Sprintf("`%s` 의 줄 끝이 섞여 있음(CRLF %d·LF %d·CR %d) — 화면은 같고, 내려받으면 줄 끝이 모두 %s 로 맞춰짐", info.src, crlf, lf, cr, want))
	}
	t := strings.ReplaceAll(string(b), "\r\n", "\n")
	t = strings.ReplaceAll(t, "\r", "\n")
	if t != "" && !strings.HasSuffix(t, "\n") {
		cs.nonl, t = true, t+"\n"
	}
	cs.text, cs.n = t, strings.Count(t, "\n")
	if st.opts.IncludeHref != nil {
		cs.href = st.opts.IncludeHref(rel)
	}
	return cs
}

// readInclude 는 ReadInclude 로 rel 을 읽고 크기·UTF-8·문서 합 상한을 본다.
func (st *state) readInclude(rel string) includeData {
	b, err := st.opts.ReadInclude(rel)
	if err != nil {
		var ie *IncludeError
		if errors.As(err, &ie) {
			return includeData{err: ie}
		}
		return includeData{err: &IncludeError{Kind: IncludeMissing, Detail: "읽지 못함"}}
	}
	switch {
	case len(b) > MaxIncludeBytes:
		return includeData{err: &IncludeError{Kind: IncludeTooLarge}}
	case st.srcBytes+len(b) > maxIncludeDocBytes:
		return includeData{err: &IncludeError{Kind: IncludeTooLarge, Detail: fmt.Sprintf("한 문서가 불러오는 파일의 합은 %dMB 까지", maxIncludeDocBytes>>20)}}
	case !utf8.Valid(b):
		return includeData{err: &IncludeError{Kind: IncludeNotUTF8}}
	case bytes.IndexByte(b, 0) >= 0:
		return includeData{err: &IncludeError{Kind: IncludeNotUTF8, Detail: "글자가 아닌 바이트(NUL)가 있음"}}
	}
	st.srcBytes += len(b)
	return includeData{b: b}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// countEOL 은 줄 끝 종류별 수다(CRLF·홀로 LF·홀로 CR).
func countEOL(b []byte) (crlf, lf, cr int) {
	for i := 0; i < len(b); i++ {
		switch b[i] {
		case '\r':
			if i+1 < len(b) && b[i+1] == '\n' {
				crlf++
				i++
			} else {
				cr++
			}
		case '\n':
			lf++
		}
	}
	return
}

// hasCodeBody 는 펜스 안에 공백 아닌 글이 있는지다.
func hasCodeBody(fc *ast.FencedCodeBlock, src []byte) bool {
	lines := fc.Lines()
	for i := 0; i < lines.Len(); i++ {
		if len(bytes.TrimSpace(segValue(lines.At(i), src))) > 0 {
			return true
		}
	}
	return false
}

// srcErrorBox 는 불러오지 못한 src 블록 자리의 오류 칸이다 — 문구의 `…` 는 code 로 그린다(이스케이프한 뒤).
func (r *htmlRenderer) srcErrorBox(w util.BufWriter, fc *ast.FencedCodeBlock, cs *codeSrc) {
	_, _ = w.WriteString(`<div class="code-src-error"`)
	dataAttrs(w, fc)
	attr(w, "data-src", cs.src)
	attr(w, "data-code", cs.code)
	_, _ = w.WriteString(` role="note"><strong>코드 파일을 불러오지 못함</strong> `)
	_, _ = w.WriteString(codeSpans(cs.msg))
	_, _ = w.WriteString("</div>\n")
}

// codeSpans 는 글을 이스케이프하고 짝이 맞는 `…` 를 <code> 로 바꾼다(짝이 안 맞으면 글자 그대로).
func codeSpans(s string) string {
	parts := strings.Split(s, "`")
	if len(parts)%2 == 0 {
		return string(util.EscapeHTML([]byte(s)))
	}
	var b strings.Builder
	for i, p := range parts {
		e := string(util.EscapeHTML([]byte(p)))
		if i%2 == 1 {
			b.WriteString("<code>" + e + "</code>")
		} else {
			b.WriteString(e)
		}
	}
	return b.String()
}

// srcAttrs 는 불러온 src 블록의 pre 표시다 — 내려받기(화면)가 원본 바이트를 되살리는 데 쓴다.
func srcAttrs(w util.BufWriter, cs *codeSrc) {
	attr(w, "data-src", cs.src)
	attr(w, "data-src-name", path.Base(cs.rel))
	if cs.bom {
		attr(w, "data-src-bom", "1")
	}
	if cs.eol != "" {
		attr(w, "data-src-eol", cs.eol)
	}
	if cs.nonl {
		attr(w, "data-src-nonl", "1")
	}
	if cs.href != "" {
		attr(w, "data-src-href", cs.href)
	}
}

// CodeSource 는 src 블록의 불러온 결과다(문서 모델 — 내보내기가 쓴다). Err 가 비지 않으면 불러오지 못한 블록이고 Text 는 비어 있다.
type CodeSource struct {
	Src  string // 원문에 적힌 src 값
	Name string // 파일 이름(src 의 마지막 이름)
	Text string // 불러온 글자(줄 끝 \n, \n 으로 끝난다)
	Err  string // 오류 칸 문구(경고와 같은 글)
	Code string // 오류의 경고 코드
}

// CodeSource 는 펜스 코드 블록이 src 로 불러온 블록이면 그 결과다(아니면 ok 가 거짓 — mermaid·수식의 src 는 쓰지 않으므로 아니다).
func (d *Doc) CodeSource(n ast.Node) (CodeSource, bool) {
	fc, ok := n.(*ast.FencedCodeBlock)
	if !ok || d.st == nil {
		return CodeSource{}, false
	}
	cs, ok := d.st.srcOut[fc]
	if !ok {
		return CodeSource{}, false
	}
	out := CodeSource{Src: cs.src, Name: path.Base(cs.rel), Text: cs.text}
	if cs.err != nil {
		out = CodeSource{Src: cs.src, Err: cs.msg, Code: cs.code}
	}
	return out, true
}

// CodeSrcPaths 는 원문에서 코드 블록 src= 가 불러올 경로(문서 폴더 기준, CheckIncludePath 로 정리한 슬래시 경로)를 원문 차례로, 겹침 없이
// 돌려준다(itda-hyve#880 — 호스트가 렌더 전에 불러올 파일의 상태로 다시 그리기 도장을 만든다). 렌더와 같은 파서·펜스 판정·info 해석
// (parseCodeInfo)을 쓰되 렌더는 하지 않는다. 노트(:::notes) 안 블록도 넣는다 — 노트를 빼는 렌더든 넣는 렌더든 같은 목록이다. 그림·수식
// (mermaid·math)의 src 는 쓰지 않으므로 빼고, CheckIncludePath 를 통과하지 못한 값(폴더 밖·숨김·확장자 …)도 뺀다(읽지 않는다).
// 블록 중첩 상한을 넘어 원문 글자로 보이는 문서는 코드 블록이 없으므로 nil 이다.
func CodeSrcPaths(src []byte) []string {
	src = normalize(src)
	meta, body, _, _ := parseFrontmatter(src, nil)
	st := newState(body, meta, Options{})
	st.nestingLimit = MaxNestingDepth
	doc, over := st.parseGuarded(body)
	if over > 0 || doc == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		fc, ok := n.(*ast.FencedCodeBlock)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		info := parseCodeInfo(fenceInfo(fc, body))
		if !info.hasSrc || info.lang == "math" || info.lang == "mermaid" {
			return ast.WalkContinue, nil
		}
		if clean, err := CheckIncludePath(info.src); err == nil && !seen[clean] {
			seen[clean] = true
			out = append(out, clean)
		}
		return ast.WalkContinue, nil
	})
	return out
}
