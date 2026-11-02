package page

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing/fstest"
	"unicode"
	"unicode/utf8"
)

// 사용자 테마 폴더(설계 3장 "회사"). 기본은 ~/.itda/hyve/themes/<이름>/ 이고, 경로는 문서 창을 띄우는 쪽(cmd_md.go)이
// 정해 Options.ThemesDir 로 넘긴다 — 문서 창은 internal/config 를 쥐지 않는다(설정·시크릿에 닿지 않는다는 불변식).
//
// 테마 파일은 남이 만든 파일이다. 읽을 때:
//   - 테마 폴더 안의 링크는 따라가지 않는다(테마 폴더 자체가 링크여도 그 테마는 쓰지 않는다). 테마 폴더 묶음(themes/)이
//     링크인 것은 받는다 — 팀이 공유 폴더를 걸어 두는 쓰임이 있고, 그것은 사용자가 고른 것이다.
//   - 보통 파일만, 허용 확장자·크기 안에서(checkThemeFiles), os.Root 안에서 연다(.. 로 벗어나지 못한다).
//   - 다 읽어 메모리에 둔다. 화면이 그림·글꼴을 달라고 할 때 디스크를 다시 보지 않는다 — 고른 뒤 파일을 링크로 바꿔
//     폴더 밖 파일을 내주게 하는 길이 없다. 폴더가 바뀌면 감시(App.watchThemes)가 통째로 다시 읽는다.

// readUserThemes 는 dir 의 테마 폴더들을 메모리 FS 로 읽는다. 폴더 하나의 문제는 broken(폴더 이름 → 이유)에 남긴다.
// dir 이 없으면 빈 묶음이다(처음 쓰는 사람).
func readUserThemes(dir string) (fs.FS, map[string]string, error) {
	mfs, broken := fstest.MapFS{}, map[string]string{}
	root, err := os.OpenRoot(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return mfs, broken, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("테마 폴더 %s: %w", dir, err)
	}
	defer root.Close()
	ents, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, nil, fmt.Errorf("테마 폴더 %s: %w", dir, err)
	}
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.Type()&fs.ModeSymlink != 0 {
			broken[name] = "링크인 테마 폴더는 쓰지 않는다 — 테마 폴더 안에 실제 폴더로 둬라"
			continue
		}
		if !e.IsDir() {
			continue // 맨 위의 낱 파일(메모 등)은 테마가 아니다
		}
		files, err := readThemeTree(root, name)
		if err != nil {
			broken[name] = err.Error()
			continue
		}
		mfs[name] = &fstest.MapFile{Mode: fs.ModeDir | 0o555} // 빈 폴더도 "theme.json 없음" 으로 보이게
		for p, b := range files {
			mfs[p] = &fstest.MapFile{Data: b, Mode: 0o444}
		}
	}
	return mfs, broken, nil
}

// readThemeTree 는 테마 폴더 하나의 파일을 읽는다(슬래시 경로 → 내용).
func readThemeTree(root *os.Root, name string) (map[string][]byte, error) {
	out := map[string][]byte{}
	var total int64
	err := fs.WalkDir(root.FS(), name, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, name), "/")
		if rel != "" && hiddenPath(rel) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s: 링크는 따라가지 않는다 — 실제 파일을 둬라", rel)
		}
		if d.IsDir() {
			if strings.Count(rel, "/") > 3 {
				return fmt.Errorf("%s: 폴더가 너무 깊음", rel)
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("%s: 보통 파일이 아님", rel)
		}
		if _, ok := ThemeFileTypes[strings.ToLower(path.Ext(p))]; !ok {
			return fmt.Errorf("%s: 테마에 둘 수 없는 파일(css·json·md·svg·png·jpg·webp·woff2 만)", rel)
		}
		if len(out) >= maxThemeFiles {
			return fmt.Errorf("파일이 너무 많음(%d개까지)", maxThemeFiles)
		}
		b, err := readRootFile(root, p, maxThemeFile)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		total += int64(len(b))
		if total > maxThemeTotal {
			return fmt.Errorf("테마가 너무 큼(%dMB 까지)", maxThemeTotal>>20)
		}
		out[p] = b
		return nil
	})
	return out, err
}

// readRootFile 은 root 안의 보통 파일 하나를 limit 바이트까지 읽는다. FIFO 로 막히지 않게 O_NONBLOCK 으로 연다.
func readRootFile(root *os.Root, p string, limit int64) ([]byte, error) {
	f, err := root.OpenFile(filepath.FromSlash(p), os.O_RDONLY|OpenFlags, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("보통 파일이 아님")
	}
	if st.Size() > limit {
		return nil, fmt.Errorf("파일이 너무 큼(%dMB 넘음)", limit>>20)
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("파일이 너무 큼(%dMB 넘음)", limit>>20)
	}
	return b, nil
}

// UserThemesStamp 는 테마 폴더의 지문이다(이름·크기·수정 시각·종류). 바뀌면 다시 읽는다. fsnotify 없이(새 의존성
// 없음) 1초마다 본다 — 테마 폴더는 작아서(파일 수십 개) 훑는 값이 싸다. 너무 크면 앞부분만 본다.
func UserThemesStamp(dir string) string {
	h := fnv.New64a()
	n := 0
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(h, "err:%s|", p)
			return nil
		}
		if n++; n > 5000 {
			return filepath.SkipAll
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		fmt.Fprintf(h, "%s|%d|%d|%v\n", p, info.Size(), info.ModTime().UnixNano(), info.Mode())
		return nil
	})
	return fmt.Sprintf("%d:%x", n, h.Sum64())
}

// ---- 회사 테마 만들기(설계 3장 "회사", 화면의 "회사 테마 만들기")

// NewTheme 은 화면의 "회사 테마 만들기" 에서 받은 값이다. 경로는 없다 — 폴더 이름은 Go 가 Name 에서 만들고,
// 로고는 Go 가 띄운 대화상자에서 고른 파일(App.ChooseThemeLogo)만 쓴다(설계 결정 2: JS 는 경로를 정하지 못한다).
type NewTheme struct {
	Name         string `json:"name"`         // 테마 이름(화면에 보이는 이름. 폴더·theme: 이름은 여기서 만든다)
	Base         string `json:"base"`         // 바탕 테마(내장 중 하나)
	Accent       string `json:"accent"`       // 강조색 #rrggbb
	Organization string `json:"organization"` // 조직 이름(선택) — 문서에 organization 이 없을 때 표지·머리글에 쓴다
	Logo         bool   `json:"logo"`         // 고른 로고를 넣을지
}

var accentRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// themeIDFromName 은 화면에 적은 이름에서 폴더 이름(= 문서의 theme: 값)을 만든다: 앞뒤 공백을 떼고, 영문 대문자는
// 소문자로, 공백은 - 로. 그 밖의 글자(/ . : 등)는 고치지 않고 거부한다 — 몰래 지우면 다른 이름이 된다.
func themeIDFromName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("테마 이름을 적어라")
	}
	var b strings.Builder
	dash := false
	for _, r := range name {
		switch {
		case unicode.IsSpace(r):
			if !dash {
				b.WriteByte('-')
			}
			dash = true
			continue
		case r >= 'A' && r <= 'Z':
			r += 'a' - 'A'
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r >= '가' && r <= '힣':
		default:
			return "", fmt.Errorf("테마 이름에 %q 는 쓸 수 없다 — 글자(영문·한글)·숫자·-·_ 만", string(r))
		}
		b.WriteRune(r)
		dash = false
	}
	id := strings.Trim(b.String(), "-_")
	if !ValidThemeName(id) {
		return "", fmt.Errorf("테마 이름 %q 는 글자 또는 숫자로 시작하고 40자를 넘지 않아야 한다", name)
	}
	return id, nil
}

// createdTheme 은 만든 theme.json 이다(필드 순서가 곧 파일의 키 순서).
type createdTheme struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Extends     string            `json:"extends"`
	Tokens      map[string]string `json:"tokens"`
	Logo        string            `json:"logo,omitempty"`
	Defaults    map[string]string `json:"defaults,omitempty"`
}

const maxLogo = 2 << 20

// CreateUserTheme 은 dir(사용자 테마 폴더) 안에 새 테마 폴더를 만든다. 있는 폴더는 덮어쓰지 않는다(이름이 같으면
// 실패). 쓰는 곳은 dir 안뿐이다 — os.Root 로 열어 링크·.. 로 벗어나지 못한다. 만든 테마의 이름을 돌려준다.
func CreateUserTheme(dir string, ts *Themes, req NewTheme, logoPath string) (string, error) {
	if dir == "" {
		return "", errors.New("사용자 테마 폴더가 정해지지 않았다")
	}
	id, err := themeIDFromName(req.Name)
	if err != nil {
		return "", err
	}
	if t, ok := ts.Get(id); ok {
		return "", fmt.Errorf("이름 %q 는 이미 테마 %s 가 쓴다 — 다른 이름을 지어라", id, t.Title)
	}
	if _, ok := ts.Broken(id); ok {
		return "", fmt.Errorf("테마 폴더에 %q 폴더가 이미 있다(지금은 쓸 수 없는 테마) — 다른 이름을 짓거나 그 폴더를 고쳐라", id)
	}
	base, ok := ts.Get(req.Base)
	if !ok || base.User {
		return "", fmt.Errorf("바탕 테마 %q 는 내장 테마여야 한다", req.Base)
	}
	if !accentRe.MatchString(req.Accent) {
		return "", fmt.Errorf("강조색 %q 는 #rrggbb 꼴이어야 한다", req.Accent)
	}
	org := strings.TrimSpace(req.Organization)
	if org != "" {
		if utf8.RuneCountInString(org) > 60 {
			return "", errors.New("조직 이름은 60자까지")
		}
		if err := checkDefault("organization", org); err != nil {
			return "", fmt.Errorf("조직 이름: 한 줄 글자로, { } 없이")
		}
	}
	title := strings.Join(strings.Fields(req.Name), " ")
	tf := createdTheme{
		Title:       title,
		Description: "회사 테마 — 바탕 " + base.Title, // 조사가 바탕 이름에 따라 바뀌지 않게
		Extends:     base.Name,
		Tokens:      map[string]string{"--accent": strings.ToLower(req.Accent)},
	}
	if org != "" {
		tf.Defaults = map[string]string{"organization": org}
	}
	// 바탕 테마의 파생 색(옅은 면·제목 번호 등)을 새 강조색으로 미리 계산해 정적 값으로 적는다. 상대 색을 모르는 옛 WebKit
	// 에서도 바탕의 남색 등이 섞이지 않고, 지원하는 엔진에서는 같은 값이다(정적 값을 준 토큰은 물려받은 식이 덮지 않는다).
	vals := map[string]string{}
	for _, kv := range base.Tokens {
		vals[kv[0]] = kv[1]
	}
	vals["--accent"] = tf.Tokens["--accent"]
	for _, kv := range base.Derived {
		if !strings.Contains(kv[1], "var(--accent)") {
			continue // 다른 색(매뉴얼의 --highlight)에서 나온 것은 바탕 그대로
		}
		if v, err := evalRelColor(kv[1], vals); err == nil {
			tf.Tokens[kv[0]] = v
		}
	}
	var logo []byte
	logoName := ""
	if req.Logo {
		if logoPath == "" {
			return "", errors.New("로고 파일을 먼저 골라라")
		}
		b, ext, err := ReadLogo(logoPath)
		if err != nil {
			return "", err
		}
		logo, logoName = b, "assets/logo"+ext
		tf.Logo = logoName
	}
	js, err := json.MarshalIndent(tf, "", "  ")
	if err != nil {
		return "", err
	}
	js = append(js, '\n')

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err := root.Mkdir(id, 0o755); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("테마 폴더에 %q 가 이미 있다 — 덮어쓰지 않는다", id)
		}
		return "", err
	}
	var made []string
	undo := func() {
		for _, p := range slices.Backward(made) {
			_ = root.Remove(p)
		}
		_ = root.Remove(id)
	}
	write := func(rel string, b []byte) error {
		p := filepath.Join(id, filepath.FromSlash(rel))
		f, err := root.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return err
		}
		made = append(made, p)
		if _, err := f.Write(b); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}
	if logo != nil {
		if err := root.Mkdir(filepath.Join(id, "assets"), 0o755); err != nil {
			undo()
			return "", err
		}
		if err := write(logoName, logo); err != nil {
			undo()
			_ = root.Remove(filepath.Join(id, "assets"))
			return "", err
		}
	}
	if err := write("theme.json", js); err != nil {
		undo()
		return "", err
	}
	return id, nil
}

// ReadLogo 는 대화상자에서 고른 로고를 읽는다: 그림 확장자, 링크가 아닌 보통 파일, 2MB 까지.
func ReadLogo(p string) ([]byte, string, error) {
	ext := strings.ToLower(filepath.Ext(p))
	if ext == ".jpeg" {
		ext = ".jpg"
	}
	if !isImageExt("x" + ext) {
		return nil, "", errors.New("로고는 svg·png·jpg·webp 파일이어야 한다")
	}
	st, err := os.Lstat(p)
	if err != nil {
		return nil, "", err
	}
	if !st.Mode().IsRegular() {
		return nil, "", errors.New("로고가 보통 파일이 아니다(링크 등)")
	}
	if st.Size() > maxLogo {
		return nil, "", fmt.Errorf("로고가 너무 크다(%dMB 까지)", maxLogo>>20)
	}
	f, err := os.OpenFile(p, os.O_RDONLY|OpenFlags, 0)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxLogo+1))
	if err != nil {
		return nil, "", err
	}
	if len(b) > maxLogo {
		return nil, "", fmt.Errorf("로고가 너무 크다(%dMB 까지)", maxLogo>>20)
	}
	return b, ext, nil
}
