package page

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/itda-work/itda-doc/mdoc"
)

// 테마(설계 3장). 테마는 폴더 하나다 — theme.json(이름·토큰·여백·머리글/바닥글·표지 메타 키) + 선택적 theme.css + assets/.
// 테마는 스크립트도 HTML 틀도 담지 못한다. 표지·제목 블록·목차·머리글의 구조는 앱이 정하고, 테마는 CSS 로 모양만 바꾼다.
//
// 테마는 두 곳에서 온다: 바이너리에 든 내장 테마(themes/)와 사용자 테마 폴더(자리는 앱이 정한다, usertheme.go).
// 둘은 같은 로더·같은 검사를 지난다 — 내장 테마도 남이 쓴 테마처럼 토큰 이름·값, 파일 확장자·크기, CSS 의 @import·
// 원격 url()·창 틀을 노리는 선택자를 거부하고, CSS 는 쪽 상자 안에 가둔다(themecss.go).
// 다른 점은 하나다: 내장 테마가 틀리면 창이 뜨지 않고(테스트로 고정된다), 사용자 테마가 틀리면 그 테마만 이유와 함께
// "쓸 수 없음" 으로 목록에 남는다 — 테마를 만드는 사람이 고칠 곳을 보아야 하기 때문이다.

// themeFile 은 theme.json 이다. 키는 frontmatter 처럼 kebab-case.
type themeFile struct {
	Name        string            `json:"name"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Extends     string            `json:"extends"`
	Aliases     []string          `json:"aliases"`
	Tokens      map[string]string `json:"tokens"`
	// Derived 는 지원하는 엔진에서만 tokens 를 덮는 식이다(상대 색 oklch(from var(--accent) …)). 같은 이름의 정적 값이
	// tokens 에(또는 물려받은 테마에) 꼭 있어야 한다 — 옛 WebKit(macOS 12~14 의 Safari 17 이하)은 상대 색을 몰라
	// 사용자 정의 속성이 계산 시점에 무효가 되고, CSS 의 "정적 값 다음 줄에 새 문법" 식 대체가 통하지 않는다.
	Derived map[string]string `json:"derived"`
	Margin  *marginFile       `json:"margin"`
	Header  *slotsFile        `json:"header"`
	Footer  *slotsFile        `json:"footer"`
	// Cover 는 "auto"(문서의 cover: true 를 따름) 또는 "never"(공문처럼 표지를 쓰지 않고 제목 블록).
	Cover string `json:"cover"`
	// Meta 는 표지·제목 블록의 <dl> 에 보일 frontmatter 키다(순서대로). 값이 없는 키는 건너뛴다.
	Meta   []string          `json:"meta"`
	Labels map[string]string `json:"labels"`
	Logo   string            `json:"logo"`
	// Defaults 는 문서에 없는 frontmatter 값을 테마가 채운다(회사 테마의 organization 등). 글자로만 쓰인다.
	Defaults map[string]string `json:"defaults"`
	// H1Break 는 h1 앞에서 새 쪽을 연다(문법이 아니라 테마 설정 — 설계 1.4). true 면 쪽이 1/3 도 안 찼을 때는
	// 나누지 않고(거의 빈 쪽이 생기지 않게), "always" 면 늘 나눈다(매뉴얼처럼 장이 늘 새 쪽에서 시작하는 문서).
	H1Break *breakSetting `json:"h1-break"`
	// TOCBreak 는 목차 뒤에서 새 쪽을 연다.
	TOCBreak *bool `json:"toc-break"`
	// ListStyle 은 문서가 list-style 을 적지 않았을 때의 목록 기호 사다리다(itda-hyve#141, mdoc/liststyle.go).
	// frontmatter 와 같은 값: "gaejosik"·"gongmun"·"plain" 또는 ["gaejosik", "gongmun"].
	ListStyle *listStyleSetting `json:"list-style"`
}

// listStyleSetting 은 글자 하나 또는 글자 목록을 받아 정규형으로 둔다.
type listStyleSetting string

func (l *listStyleSetting) UnmarshalJSON(data []byte) error {
	var vals []string
	var one string
	if err := json.Unmarshal(data, &one); err == nil {
		vals = []string{one}
	} else if err := json.Unmarshal(data, &vals); err != nil {
		return fmt.Errorf(`list-style 는 "gaejosik"·"gongmun"·"plain" 또는 ["gaejosik", "gongmun"] (받은 값 %s)`, data)
	}
	s, err := mdoc.NormalizeListStyle(vals)
	if err != nil {
		return fmt.Errorf("list-style: %w", err)
	}
	*l = listStyleSetting(s)
	return nil
}

// breakSetting 은 true·false·"always" 를 받는다.
type breakSetting struct{ on, always bool }

func (b *breakSetting) UnmarshalJSON(data []byte) error {
	switch strings.TrimSpace(string(data)) {
	case "true":
		*b = breakSetting{on: true}
	case "false":
		*b = breakSetting{}
	case `"always"`:
		*b = breakSetting{on: true, always: true}
	default:
		return fmt.Errorf(`h1-break 는 true·false·"always" (받은 값 %s)`, data)
	}
	return nil
}

type marginFile struct {
	Top    string `json:"top"`
	Right  string `json:"right"`
	Bottom string `json:"bottom"`
	Left   string `json:"left"`
}

// slotsFile 은 머리글·바닥글의 세 자리다. 포인터인 것은 extends 에서 "적지 않음" 과 "비움" 을 가르기 위해서다.
type slotsFile struct {
	Left   *string `json:"left"`
	Center *string `json:"center"`
	Right  *string `json:"right"`
}

// Theme 은 extends 를 풀고 검사를 마친 테마다.
type Theme struct {
	Name        string
	Title       string
	Description string
	Aliases     []string
	Extends     string
	User        bool // 사용자 테마 폴더에서 왔다
	// Tokens 는 CSS 변수(이름 순). 여백도 --page-margin-* 토큰으로 들어 있다.
	Tokens [][2]string
	// Derived 는 상대 색을 아는 엔진에서만 Tokens 를 덮는 값이다(이름 순, @supports 안에 낸다).
	Derived  [][2]string
	Header   [3]string // 왼쪽·가운데·오른쪽 틀(자리표시자 포함)
	Footer   [3]string
	NoCover  bool
	Meta     []string
	Labels   map[string]string
	Defaults map[string]string
	LogoURL  string // 앱 주소(/_theme/…). 없으면 ""
	H1Break  bool
	// H1Always 는 h1-break: "always" 다(쪽이 얼마나 찼든 나눈다).
	H1Always bool
	TOCBreak bool
	// ListStyle 은 목록 기호 사다리의 테마 기본이다(정규형, "" 이면 보통 목록).
	ListStyle string
	// CSS 는 물려받은 테마부터 차례로 이은 theme.css 의 규칙이다(쪽 상자 안에 가둘 것). url() 은 /_theme/<테마>/ 주소다.
	CSS string
	// Fonts 는 theme.css 에서 꺼낸 @font-face 규칙이다(@scope 안에 둘 수 없어 따로 낸다).
	Fonts string
	// Template 은 "새 문서" 의 시작 글(template.md)이다. 없으면 물려받은 테마의 것.
	Template string
	// files 는 이 테마(와 조상)의 assets 를 내줄 때 쓴다: 앱 주소 → (fs, 경로).
	files map[string]themeAsset
}

type themeAsset struct {
	fsys fs.FS
	path string
}

// Themes 는 이름으로 찾는 테마 묶음이다.
type Themes struct {
	byName map[string]*Theme
	alias  map[string]string
	order  []string
	// broken 은 쓸 수 없는 사용자 테마다(이름 → 이유). 목록에는 보이지만 고를 수 없다.
	broken map[string]string
	// Rev 는 불러올 때마다 오르는 번호다. 화면이 테마 CSS 를 다시 받을지 이것으로 안다.
	Rev int64
}

// ThemeInfo 는 화면의 테마 고르기에 보일 것이다.
type ThemeInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	HasTemplate bool   `json:"has_template"` // "새 문서" 메뉴에 보일지
	User        bool   `json:"user"`         // 사용자 테마 폴더의 테마
	Extends     string `json:"extends,omitempty"`
	Error       string `json:"error,omitempty"` // 쓸 수 없는 까닭(있으면 고를 수 없다)
}

const (
	maxExtendsDepth = 4
	maxThemeJSON    = 64 << 10
	maxTemplate     = 64 << 10
)

var (
	// 테마 이름 = 폴더 이름. 소문자 영문·숫자·한글 음절·-·_ (40자). 대문자를 받지 않는 것은 대소문자를 가리지 않는
	// 파일 시스템(macOS·Windows)에서 "Acme" 와 "acme" 가 한 폴더가 되기 때문이다. 한글은 완성형(가-힣)만 —
	// 풀어쓴 꼴(NFD)은 보기에 같아도 다른 이름이 된다.
	themeNameRe = regexp.MustCompile(`^[a-z0-9가-힣][a-z0-9가-힣_-]*$`)
	tokenNameRe = regexp.MustCompile(`^--[a-z0-9][a-z0-9-]{0,63}$`)
	// 길이 형식(mdoc 의 cssLength 와 같은 단위)
	lengthRe = regexp.MustCompile(`^(?:0|\d+(?:\.\d+)?(?:mm|cm|in|pt|pc|px|em|rem))$`)
	// defaults 의 키는 frontmatter 키 꼴(자리표시자와 같다)
	metaKeyRe = regexp.MustCompile(`^[\p{L}\p{N}_-]{1,40}$`)
)

// ValidThemeName 은 테마 이름으로 쓸 수 있는 글자인지다(40자까지, themeNameRe).
func ValidThemeName(n string) bool {
	return utf8.RuneCountInString(n) <= 40 && themeNameRe.MatchString(n)
}

var themeRev atomic.Int64

// themeEntry 는 읽기만 하고 아직 extends 를 풀지 않은 테마다. fsys 의 맨 위에 <이름>/ 폴더가 있다.
type themeEntry struct {
	tf   *themeFile
	fsys fs.FS
	user bool
}

// LoadThemes 는 fsys 의 맨 위 폴더마다 테마 하나를 읽는다. 하나라도 틀리면 그 이름과 함께 실패한다 —
// 내장 테마와 테스트가 쓴다. 사용자 테마 폴더는 테마마다 따로 실패하는 LoadThemeSet 을 쓴다.
func LoadThemes(fsys fs.FS) (*Themes, error) {
	entries, broken, err := readThemeDir(fsys, false)
	if err != nil {
		return nil, err
	}
	if n := firstKey(broken); n != "" {
		return nil, fmt.Errorf("테마 %s: %s", n, broken[n])
	}
	ts := newThemes()
	if err := ts.resolveAll(entries, nil); err != nil {
		return nil, err
	}
	return ts, nil
}

// LoadThemeSet 은 내장 테마(builtin)와 사용자 테마 폴더(userDir, 비면 없음)를 함께 불러온다.
// 내장 테마가 틀리면 실패하고, 사용자 테마는 틀린 것만 broken 에 이유와 함께 남긴다. 사용자 테마의 이름이나
// 별칭이 내장 테마와 겹치면 내장이 이기고 사용자 테마는 "이름이 겹침" 이 된다.
func LoadThemeSet(builtin fs.FS, userDir string) (*Themes, error) {
	entries, broken, err := readThemeDir(builtin, false)
	if err != nil {
		return nil, err
	}
	if n := firstKey(broken); n != "" {
		return nil, fmt.Errorf("내장 테마 %s: %s", n, broken[n])
	}
	ts := newThemes()
	if userDir != "" {
		ufs, ubroken, err := readUserThemes(userDir)
		if err != nil {
			return nil, err
		}
		maps.Copy(ts.broken, ubroken)
		taken := map[string]string{} // 내장 테마의 이름·별칭 → 내장 테마 이름
		for n, e := range entries {
			taken[n] = n
			for _, a := range e.tf.Aliases {
				taken[a] = n
			}
		}
		uentries, ub2, err := readThemeDir(ufs, true)
		if err != nil {
			return nil, err
		}
		maps.Copy(ts.broken, ub2)
		for n, e := range uentries {
			if b, ok := taken[n]; ok {
				ts.broken[n] = fmt.Sprintf("이름이 겹침 — 내장 테마 %s 가 이 이름을 쓴다. 폴더 이름을 바꿔라", b)
				continue
			}
			bad := ""
			for _, a := range e.tf.Aliases {
				if b, ok := taken[a]; ok {
					bad = fmt.Sprintf("별칭 %q 가 내장 테마 %s 와 겹침", a, b)
					break
				}
			}
			if bad != "" {
				ts.broken[n] = bad
				continue
			}
			entries[n] = e
		}
		// 사용자 테마끼리 별칭이 겹치면 둘 다 고칠 곳이므로 둘 다 뺀다
		seen := map[string]string{}
		for _, n := range sortedKeys(uentries) {
			if e, ok := entries[n]; !ok || !e.user {
				continue
			}
			for _, a := range entries[n].tf.Aliases {
				if other, ok := seen[a]; ok {
					ts.broken[n] = fmt.Sprintf("별칭 %q 가 사용자 테마 %s 와 겹침", a, other)
					ts.broken[other] = fmt.Sprintf("별칭 %q 가 사용자 테마 %s 와 겹침", a, n)
				} else if _, isName := entries[a]; isName && a != n {
					ts.broken[n] = fmt.Sprintf("별칭 %q 가 테마 이름과 겹침", a)
				}
				seen[a] = n
			}
		}
		for n := range ts.broken {
			if e, ok := entries[n]; ok && e.user {
				delete(entries, n)
			}
		}
	}
	if err := ts.resolveAll(entries, ts.broken); err != nil {
		return nil, err
	}
	return ts, nil
}

func newThemes() *Themes {
	return &Themes{byName: map[string]*Theme{}, alias: map[string]string{}, broken: map[string]string{}, Rev: themeRev.Add(1)}
}

// resolveAll 은 extends 를 풀어 테마를 만든다. 내장 테마의 오류는 돌려주고, 사용자 테마의 오류는 broken 에 남긴다
// (broken 이 nil 이면 모두 오류다). 사용자 테마가 쓸 수 없는 테마를 물려받으면 그것도 쓸 수 없다.
func (ts *Themes) resolveAll(entries map[string]*themeEntry, broken map[string]string) error {
	var builtin, user []string
	for _, n := range sortedKeys(entries) {
		if entries[n].user {
			user = append(user, n)
		} else {
			builtin = append(builtin, n)
		}
	}
	for _, n := range append(builtin, user...) {
		t, err := resolveTheme(entries, broken, n, ts.Rev, 0)
		if err != nil {
			if broken == nil || !entries[n].user {
				return fmt.Errorf("테마 %s: %w", n, err)
			}
			broken[n] = err.Error()
			continue
		}
		ts.byName[n] = t
		ts.order = append(ts.order, n)
		for _, a := range t.Aliases {
			ts.alias[a] = n
		}
	}
	return nil
}

// readThemeDir 는 fsys 의 맨 위 폴더마다 theme.json 을 읽는다. 폴더 하나의 문제는 broken 에 남긴다.
func readThemeDir(fsys fs.FS, user bool) (map[string]*themeEntry, map[string]string, error) {
	dirents, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, nil, err
	}
	entries, broken := map[string]*themeEntry{}, map[string]string{}
	for _, e := range dirents {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		tf, err := readThemeFile(fsys, e.Name())
		if err != nil {
			broken[e.Name()] = err.Error()
			continue
		}
		entries[e.Name()] = &themeEntry{tf: tf, fsys: fsys, user: user}
	}
	return entries, broken, nil
}

func readThemeFile(fsys fs.FS, dir string) (*themeFile, error) {
	if !ValidThemeName(dir) {
		return nil, fmt.Errorf("폴더 이름 %q 는 소문자 영문·숫자·한글·-·_ 로 40자까지만 쓴다", dir)
	}
	if err := checkThemeFiles(fsys, dir); err != nil {
		return nil, err
	}
	b, err := fs.ReadFile(fsys, dir+"/theme.json")
	if err != nil {
		return nil, fmt.Errorf("theme.json 을 읽지 못함: %w", err)
	}
	if len(b) > maxThemeJSON {
		return nil, errors.New("theme.json 이 너무 큼(64KB 넘음)")
	}
	var tf themeFile
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields() // 오타(예: "foter")를 조용히 무시하지 않는다
	if err := dec.Decode(&tf); err != nil {
		return nil, fmt.Errorf("theme.json: %w", err)
	}
	if tf.Name != "" && tf.Name != dir {
		return nil, fmt.Errorf("theme.json 의 name(%q)이 폴더 이름과 다름", tf.Name)
	}
	tf.Name = dir
	switch tf.Cover {
	case "", "auto", "never":
	default:
		return nil, fmt.Errorf(`cover 는 "auto" 또는 "never" (받은 값 %q)`, tf.Cover)
	}
	for _, k := range sortedKeys(tf.Tokens) {
		if err := checkToken(k, tf.Tokens[k]); err != nil {
			return nil, err
		}
	}
	for _, k := range sortedKeys(tf.Derived) {
		if err := checkToken(k, tf.Derived[k]); err != nil {
			return nil, fmt.Errorf("derived: %w", err)
		}
	}
	if m := tf.Margin; m != nil {
		for _, v := range []string{m.Top, m.Right, m.Bottom, m.Left} {
			if v != "" && !lengthRe.MatchString(v) {
				return nil, fmt.Errorf("margin 값 %q 는 길이(예: 20mm)여야 함", v)
			}
		}
	}
	for _, k := range sortedKeys(tf.Defaults) {
		if err := checkDefault(k, tf.Defaults[k]); err != nil {
			return nil, err
		}
	}
	for _, s := range []string{tf.Title, tf.Description} {
		if len(s) > 400 || strings.ContainsAny(s, "\x00\r\n") {
			return nil, errors.New("title·description 은 한 줄 글자(400바이트까지)")
		}
	}
	return &tf, nil
}

// 테마 폴더에 둘 수 있는 파일. 스크립트·HTML·실행 파일은 담지 못한다. 크기 상한은 테마를 고르는 순간 창이
// 굳지 않게(사용자 테마는 메모리에 읽어 둔다 — usertheme.go) 한다. 한글 글꼴 woff2 한 굵기가 1~3MB 다.
var themeFileTypes = map[string]string{
	".css": "text/css; charset=utf-8", ".json": "application/json", ".md": "text/markdown; charset=utf-8",
	".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp",
	".woff2": "font/woff2",
}

// ThemeFileType 은 테마 폴더에 둘 수 있는 확장자 ext(점 포함, 소문자 — 부르는 쪽이 strings.ToLower(path.Ext(p)) 로 맞춘다)의
// content-type 이다. 허용 목록(themeFileTypes)에 없으면 ok 가 거짓이다 — 목록 자체는 밖에서 바꿀 수 없다(리뷰 1회차 [4]).
func ThemeFileType(ext string) (ctype string, ok bool) {
	ctype, ok = themeFileTypes[ext]
	return ctype, ok
}

const (
	maxThemeFile  = 8 << 20
	maxThemeTotal = 32 << 20
	maxThemeFiles = 200
)

// checkThemeFiles 는 테마 폴더 안의 파일을 본다: 허용 확장자, 파일 하나·전체 크기, 파일 수. 숨김 이름은 쓰지
// 않는다(건너뛴다 — Finder 가 폴더를 열면 .DS_Store 를 만든다). 링크·폴더 탈출은 사용자 폴더를 읽을 때 막는다.
func checkThemeFiles(fsys fs.FS, dir string) error {
	var total int64
	n := 0
	return fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, dir), "/")
		if rel != "" && hiddenPath(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if strings.Count(rel, "/") > 3 {
				return fmt.Errorf("%s: 폴더가 너무 깊음", rel)
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s: 보통 파일이 아님(링크 등)", rel)
		}
		if _, ok := themeFileTypes[strings.ToLower(path.Ext(p))]; !ok {
			return fmt.Errorf("%s: 테마에 둘 수 없는 파일(css·json·md·svg·png·jpg·webp·woff2 만)", rel)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxThemeFile {
			return fmt.Errorf("%s: 파일이 너무 큼(%dMB 넘음)", rel, maxThemeFile>>20)
		}
		total += info.Size()
		n++
		if total > maxThemeTotal || n > maxThemeFiles {
			return fmt.Errorf("테마가 너무 큼(파일 %d개·%dMB 까지)", maxThemeFiles, maxThemeTotal>>20)
		}
		return nil
	})
}

// 토큰 이름 중 창 틀이 쓰는 것과 용지 크기는 테마가 줄 수 없다. 토큰은 :root 변수라 창 틀(app.css 의 --ui-*,
// Wails 가 읽는 --wails-draggable 등)에도 닿는다 — 그 이름을 막아 창 틀 색·끌기 영역을 바꾸지 못하게 한다.
// app.css 가 쓰는 이름 전부(reservedTokens, themecss.go 가 내장 app.css 에서 모은다)도 막는다.
var reservedTokenPrefixes = []string{"--ui-", "--wails-"}

var reservedTokenNames = map[string]bool{"--page-w": true, "--page-h": true, "--img-max-h": true, "--body-h": true}

// checkToken 은 토큰 이름과 값을 본다. 값은 CSS 변수에 그대로 들어가므로 선언을 끝내거나(; } {) 바깥을 부르는
// 것(url(·@·\ 이스케이프)을 막는다. 그림은 logo·theme.css 로만 쓴다.
func checkToken(name, value string) error {
	if !tokenNameRe.MatchString(name) {
		return fmt.Errorf("토큰 이름 %q 는 --소문자-이름 꼴이어야 함", name)
	}
	for _, p := range reservedTokenPrefixes {
		if strings.HasPrefix(name, p) {
			return fmt.Errorf("토큰 %s: %s… 는 창 틀이 쓰는 이름이라 테마가 줄 수 없음", name, p)
		}
	}
	if reservedTokenNames[name] || reservedTokens()[name] {
		return fmt.Errorf("토큰 %s 는 앱이 정하는 값이라 테마가 줄 수 없음", name)
	}
	if len(value) > 200 {
		return fmt.Errorf("토큰 %s 의 값이 너무 김", name)
	}
	low := strings.ToLower(value)
	if strings.ContainsAny(value, ";{}<>\\\n\r\x00") || strings.Contains(low, "url(") || strings.Contains(low, "@") ||
		strings.Contains(low, "expression(") || strings.Contains(low, "/*") || urlLikeFuncRe.MatchString(low) {
		return fmt.Errorf("토큰 %s 의 값 %q 에 쓸 수 없는 글자가 있음", name, value)
	}
	return nil
}

func checkDefault(key, value string) error {
	if !metaKeyRe.MatchString(key) {
		return fmt.Errorf("defaults 의 키 %q 는 frontmatter 키 꼴이어야 함", key)
	}
	if len(value) > 200 || strings.ContainsAny(value, "\x00\r\n{}") {
		return fmt.Errorf("defaults.%s 의 값은 한 줄 글자(200바이트까지, { } 없이)", key)
	}
	return nil
}

func resolveTheme(entries map[string]*themeEntry, broken map[string]string, name string, rev int64, depth int) (*Theme, error) {
	e, ok := entries[name]
	if !ok {
		if why, ok := broken[name]; ok {
			return nil, fmt.Errorf("물려받을 테마 %q 를 쓸 수 없음(%s)", name, why)
		}
		return nil, fmt.Errorf("물려받을 테마 %q 가 없음", name)
	}
	if depth > maxExtendsDepth {
		return nil, errors.New("extends 가 너무 깊거나 돌고 있음")
	}
	tf, fsys := e.tf, e.fsys
	t := &Theme{Labels: map[string]string{}, Defaults: map[string]string{}, files: map[string]themeAsset{}}
	tokens, derived := map[string]string{}, map[string]string{}
	if tf.Extends != "" {
		if tf.Extends == name {
			return nil, errors.New("자기 자신을 extends 함")
		}
		if !e.user && entries[tf.Extends] != nil && entries[tf.Extends].user {
			return nil, errors.New("내장 테마는 사용자 테마를 물려받지 못함")
		}
		parent, err := resolveTheme(entries, broken, tf.Extends, rev, depth+1)
		if err != nil {
			return nil, err
		}
		*t = *parent
		t.Aliases = nil // 별칭은 물려받지 않는다(같은 별칭이 두 테마를 가리키게 된다)
		t.Labels = copyMap(parent.Labels)
		t.Defaults = copyMap(parent.Defaults)
		t.files = map[string]themeAsset{}
		maps.Copy(t.files, parent.files)
		for _, kv := range parent.Tokens {
			tokens[kv[0]] = kv[1]
		}
		for _, kv := range parent.Derived {
			derived[kv[0]] = kv[1]
		}
	} else {
		// 뿌리 테마의 기본값
		t.Header = [3]string{"{organization}", "", "{title}"}
		t.Footer = [3]string{"", "{page} / {pages}", ""}
		t.Meta = []string{"doc-no", "version", "date", "author", "classification"}
	}
	t.Name, t.Title, t.Description, t.Aliases, t.Extends, t.User = name, tf.Title, tf.Description, tf.Aliases, tf.Extends, e.user
	if t.Title == "" {
		t.Title = name
	}
	for k, v := range tf.Tokens {
		tokens[k] = v
		delete(derived, k) // 정적 값을 직접 준 토큰은 물려받은 식이 덮지 않는다
	}
	for k, v := range tf.Derived {
		if _, ok := tokens[k]; !ok {
			return nil, fmt.Errorf("derived %s 에 정적 값이 없음 — 옛 WebKit 에서 쓸 값을 tokens 에 함께 적어라", k)
		}
		derived[k] = v
	}
	if m := tf.Margin; m != nil {
		for side, v := range map[string]string{"top": m.Top, "right": m.Right, "bottom": m.Bottom, "left": m.Left} {
			if v != "" {
				tokens["--page-margin-"+side] = v
			}
		}
	}
	t.Tokens = nil
	for _, k := range sortedKeys(tokens) {
		t.Tokens = append(t.Tokens, [2]string{k, tokens[k]})
	}
	t.Derived = nil
	for _, k := range sortedKeys(derived) {
		t.Derived = append(t.Derived, [2]string{k, derived[k]})
	}
	applySlots(&t.Header, tf.Header)
	applySlots(&t.Footer, tf.Footer)
	switch tf.Cover {
	case "never":
		t.NoCover = true
	case "auto":
		t.NoCover = false
	}
	if tf.Meta != nil {
		t.Meta = append([]string(nil), tf.Meta...)
	}
	maps.Copy(t.Labels, tf.Labels)
	maps.Copy(t.Defaults, tf.Defaults)
	if tf.H1Break != nil {
		t.H1Break, t.H1Always = tf.H1Break.on, tf.H1Break.always
	}
	if tf.TOCBreak != nil {
		t.TOCBreak = *tf.TOCBreak
	}
	if tf.ListStyle != nil {
		t.ListStyle = string(*tf.ListStyle)
	}

	// assets/ 는 /_theme/<이 테마>/assets/… 로 낸다. 물려받은 테마의 것은 그 테마 이름 아래에 이미 있다.
	// 사용자 테마는 고치며 보므로 주소에 판(?v=)을 붙여 창이 옛 그림·글꼴을 붙들지 않게 한다.
	if !e.user {
		rev = 0
	}
	if sub, err := fs.Sub(fsys, name); err == nil {
		_ = fs.WalkDir(sub, "assets", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || hiddenPath(p) {
				return nil
			}
			t.files[ThemeURL(name, p)] = themeAsset{fsys: sub, path: p}
			return nil
		})
	}
	if tf.Logo != "" {
		p, err := themeRelPath(tf.Logo)
		if err != nil {
			return nil, fmt.Errorf("logo: %w", err)
		}
		u := ThemeURL(name, p)
		if _, ok := t.files[u]; !ok {
			return nil, fmt.Errorf("logo %q 가 assets 에 없음", tf.Logo)
		}
		if !isImageExt(p) {
			return nil, fmt.Errorf("logo %q 는 그림(svg·png·jpg·webp)이어야 함", tf.Logo)
		}
		t.LogoURL = withRev(u, rev)
	}

	if b, err := fs.ReadFile(fsys, name+"/template.md"); err == nil {
		if len(b) > maxTemplate {
			return nil, errors.New("template.md 가 너무 큼(64KB 넘음)")
		}
		t.Template = string(b)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	if b, err := fs.ReadFile(fsys, name+"/theme.css"); err == nil {
		css, fonts, err := checkThemeCSS(string(b), name, t.files, rev)
		if err != nil {
			return nil, fmt.Errorf("theme.css: %w", err)
		}
		t.CSS += "/* ---- " + name + " ---- */\n" + css + "\n"
		if fonts != "" {
			t.Fonts += "/* ---- " + name + " ---- */\n" + fonts
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return t, nil
}

func applySlots(dst *[3]string, s *slotsFile) {
	if s == nil {
		return
	}
	for i, v := range []*string{s.Left, s.Center, s.Right} {
		if v != nil {
			dst[i] = *v
		}
	}
}

// ThemeURL 은 테마 파일(theme 의 p)을 내주는 앱 주소다.
func ThemeURL(theme, p string) string { return "/_theme/" + theme + "/" + p }

func withRev(u string, rev int64) string {
	if rev == 0 {
		return u
	}
	return fmt.Sprintf("%s?v=%d", u, rev)
}

func isImageExt(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".svg", ".png", ".jpg", ".jpeg", ".webp":
		return true
	}
	return false
}

// themeRelPath 는 테마 폴더 안의 상대 경로만 받는다(assets/ 아래, 숨김·탈출·스킴 없음).
func themeRelPath(p string) (string, error) {
	if p == "" || strings.Contains(p, "\\") || strings.Contains(p, ":") || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "?#") {
		return "", fmt.Errorf("%q 는 테마 폴더 기준 상대 경로여야 함", p)
	}
	c := path.Clean(p)
	if c == ".." || strings.HasPrefix(c, "../") || !strings.HasPrefix(c, "assets/") || hiddenPath(c) {
		return "", fmt.Errorf("%q 는 assets/ 아래 파일이어야 함", p)
	}
	return c, nil
}

// hiddenPath 는 쓰지 않는(건너뛰는) 이름이다: 숨김 이름(.DS_Store·.git), 편집기 백업(theme.css~·#theme.css#).
// 테마를 고치며 보는 동안 편집기가 만드는 파일 때문에 테마가 "쓸 수 없음" 이 되지 않게 한다.
func hiddenPath(p string) bool {
	for part := range strings.SplitSeq(p, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, "~") || (len(part) > 1 && strings.HasPrefix(part, "#") && strings.HasSuffix(part, "#")) {
			return true
		}
	}
	return false
}

// Get 은 이름(또는 별칭)으로 테마를 찾는다. 쓸 수 없는 테마는 찾지 못한다(Broken 으로 까닭을 본다).
func (ts *Themes) Get(name string) (*Theme, bool) {
	if t, ok := ts.byName[name]; ok {
		return t, true
	}
	if n, ok := ts.alias[name]; ok {
		return ts.byName[n], true
	}
	return nil, false
}

// Broken 은 쓸 수 없는 사용자 테마의 까닭이다.
func (ts *Themes) Broken(name string) (string, bool) {
	why, ok := ts.broken[name]
	return why, ok
}

// List 는 테마 고르기에 보일 목록이다: 내장(이름 순) → 사용자(이름 순) → 쓸 수 없는 사용자 테마(이름 순).
func (ts *Themes) List() []ThemeInfo {
	out := make([]ThemeInfo, 0, len(ts.order)+len(ts.broken))
	for _, n := range ts.order {
		t := ts.byName[n]
		out = append(out, ThemeInfo{Name: t.Name, Title: t.Title, Description: t.Description, HasTemplate: t.Template != "",
			User: t.User, Extends: t.Extends})
	}
	for _, n := range sortedKeys(ts.broken) {
		out = append(out, ThemeInfo{Name: n, Title: n, User: true, Error: ts.broken[n]})
	}
	return out
}

// StyleSheet 는 /_theme/<이름>/theme.css 로 낼 CSS 다: 토큰(:root 변수) + 파생 토큰(@supports) + 글꼴 + 테마 CSS.
//
// 테마 CSS 는 @scope (.page) 안에 둔다 — 선택자가 쪽 상자 안의 요소에만 맞는다(WKWebView 실측: body·.bar·.page 자신을
// 고르는 규칙이 아무것도 바꾸지 못했다). 쪽 상자는 contain: paint 라 position: fixed 로도 쪽 밖을 덮지 못하고,
// 보기 영역은 isolation 으로 확인창·메뉴보다 위로 올라오지 못한다(base.css·app.css). @scope 는 선택자의 우선순위를
// 바꾸지 않아 가두기 전과 화면·PDF 가 같다(내장 3종 PDF 픽셀 비교 0).
//
// css 는 엔진에 맞춘 꼴이다(compat.go): @scope 를 모르는 엔진(옛 WebKit)이면 가두지 않고 낸다. 그 꼴은 내장 테마만
// 받는다 — 사용자 테마는 그 엔진에서 아예 쓰지 않는다(App.themeSet). 상대 색 판정은 CSS 의 @supports 가 한다.
func (t *Theme) StyleSheet(css CSSMode) string {
	var b strings.Builder
	b.WriteString("/* 테마 ")
	b.WriteString(t.Name)
	b.WriteString(" — itda-doc 이 theme.json·theme.css 에서 만든다 */\n:root {\n")
	writeDecls(&b, t.Tokens)
	b.WriteString("}\n")
	if len(t.Derived) > 0 {
		// 상대 색을 채널 calc() 까지 아는 엔진에서만 덮는다(Safari 18+·Chromium 119+). 모르는 엔진은 위의 정적 값을 쓴다.
		cond := relColorSupports
		if css.FakeNoRelColor {
			cond = fakeFalseSupports
		}
		b.WriteString("@supports ")
		b.WriteString(cond)
		b.WriteString(" {\n:root {\n")
		writeDecls(&b, t.Derived)
		b.WriteString("}\n}\n")
	}
	b.WriteString(t.Fonts)
	if css.Unscoped && !t.User {
		b.WriteString(t.CSS)
	} else {
		b.WriteString("@scope (.page) {\n")
		b.WriteString(t.CSS)
		b.WriteString("}\n")
	}
	return b.String()
}

// writeDecls 는 변수 선언 줄들("  이름: 값;")이다.
func writeDecls(b *strings.Builder, kvs [][2]string) {
	for _, kv := range kvs {
		b.WriteString("  ")
		b.WriteString(kv[0])
		b.WriteString(": ")
		b.WriteString(kv[1])
		b.WriteString(";\n")
	}
}

// HasFile 은 u(/_theme/<이름>/… 주소)가 이 테마의 파일인지다 — 다른 테마의 파일은 거짓이다.
func (t *Theme) HasFile(u string) bool {
	_, ok := t.files[u]
	return ok
}

// Asset 은 테마 파일 하나를 찾는다. u 는 /_theme/<이름>/assets/… 주소다(물음표 뒤는 뺀 것).
func (ts *Themes) Asset(u string) (fs.FS, string, bool) {
	for _, t := range ts.byName {
		if a, ok := t.files[u]; ok {
			return a.fsys, a.path, true
		}
	}
	return nil, "", false
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func firstKey[V any](m map[string]V) string {
	if k := sortedKeys(m); len(k) > 0 {
		return k[0]
	}
	return ""
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	maps.Copy(out, m)
	return out
}
