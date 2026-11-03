package page

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/itda-work/itda-doc/mdoc"
	"github.com/itda-work/itda-doc/mdsyntax"
)

// MaxCheckBytes 는 Check 가 받는 원문 상한이다(itda-hyve#135). 문서 창의 편집기·렌더 상한을 재 본 문서는 수백 KB 이고(64겹 중첩 시험의
// 8만 줄 문서가 약 1MB), MCP 응답·CLI 가 한 번에 다룰 크기로 4MB 를 둔다 — 넘으면 검사하지 않고 알린다.
const MaxCheckBytes = 4 << 20

// CheckWarning 은 경고 하나다(code·line·message — 문서 창의 경고 단추와 같은 목록).
type CheckWarning struct {
	Code    string `json:"code"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// CheckMeta 는 검사한 문서의 정보 몇 가지다. 값은 글자 그대로다(문서 내용은 데이터).
type CheckMeta struct {
	Title    string `json:"title,omitempty"`
	Theme    string `json:"theme"`               // 실제로 보일 테마(모르면 classic)
	DocTheme string `json:"doc_theme,omitempty"` // 문서가 적은 theme:(쓸 수 있을 때만)
	Slides   bool   `json:"slides"`              // frontmatter slides: 가 있다
	Headings int    `json:"headings"`            // 목차에 드는 제목 수
}

// CheckResult 는 Check 의 결과다. 같은 원문·같은 path(문서 위 폴더의 `_metadata.yml` 까지 같으면)이면 같은 결과다.
type CheckResult struct {
	// SourceHash 는 검사한 원문 바이트의 sha256(앞 16자리)이다. path 가 있어도 원문만 해시한다 —
	// 조상 폴더 파일이 달라지면 결과가 달라질 수 있으니 folder_meta 를 함께 본다.
	SourceHash string `json:"source_hash"`
	SyntaxHash string `json:"syntax_hash"` // 이 판의 문법 단일본 해시(mdsyntax.Hash)
	// FolderMeta 는 검사에 얹힌 폴더 `_metadata.yml` 의 문서 기준 상대 경로다(itda-hyve#474 — 가까운 것부터).
	// 같은 원문도 path 의 조상 파일에 따라 결과가 달라지므로 출처를 함께 돌려준다. 경로 없는 검사는 비어 있다.
	FolderMeta []string       `json:"folder_meta,omitempty"`
	Warnings   []CheckWarning `json:"warnings"`
	Meta       CheckMeta      `json:"meta"`
	// LayoutChecked 는 쪽 나눔·넘침까지 봤는지다. 늘 false — 렌더 경고만 본다(쪽수·잘림은 문서 창·PDF 에서 본다).
	LayoutChecked bool `json:"layout_checked"`
}

// Check 는 문서 창과 같은 렌더러·테마 고르기로 원문을 검사한다(itda-hyve#135 — 앱의 검사 도구와 `itda-doc check` 가 부른다). 창을 띄우지 않고,
// 테마 폴더와 path 가 있으면 그 위 폴더의 `_metadata.yml` 만 읽는다 — 원격 그림은 문서 창의 기본처럼
// 불러오지 않은 것으로 센다. 폴더 파일은 frontmatter 기본값으로 함께 검사에 얹힌다(itda-hyve#474 — 문서 창과 같은 값).
// path 가 비면 상속이 없다(붙여넣은 원문 검사는 지금과 같다). ts 는 쓸 테마 묶음이다(nil 이면 내장 테마만).
func Check(src []byte, path string, ts *Themes) (CheckResult, error) {
	if len(src) > MaxCheckBytes {
		return CheckResult{}, fmt.Errorf("문서가 너무 큼(%d바이트 — 상한 %d)", len(src), MaxCheckBytes)
	}
	if ts == nil {
		var err error
		if ts, err = BuiltinThemes(); err != nil {
			return CheckResult{}, err
		}
	}
	sum := sha256.Sum256(src)
	v := renderView(src, path, "", ts, "", false)
	out := CheckResult{SourceHash: hex.EncodeToString(sum[:])[:16], SyntaxHash: mdsyntax.Hash(), FolderMeta: v.FolderMeta, Warnings: []CheckWarning{}}
	if v.Error != "" {
		return CheckResult{}, errors.New(v.Error) // 렌더러 내부 실패뿐이다 — 문법 문제는 경고로 온다
	}
	for _, w := range v.Warnings {
		out.Warnings = append(out.Warnings, CheckWarning{Code: w.Code, Line: w.Line, Message: w.Message})
	}
	out.Meta = CheckMeta{Title: v.Title, Theme: v.Theme, DocTheme: v.DocTheme, Slides: v.Slides.Present, Headings: countHeadings(v.TOC)}
	return out, nil
}

// LoadCheckThemes 는 Check 가 쓸 테마 묶음이다 — 내장 테마와 사용자 테마 폴더(dir, 비면 내장만).
func LoadCheckThemes(dir string) (*Themes, error) {
	if dir == "" {
		return BuiltinThemes()
	}
	return loadThemeSet(dir)
}

// countHeadings 는 목차 트리의 제목 수다(toc-depth·.unlisted 를 따른 목차 기준).
func countHeadings(items []mdoc.TOCItem) int {
	n := len(items)
	for _, it := range items {
		n += countHeadings(it.Children)
	}
	return n
}
