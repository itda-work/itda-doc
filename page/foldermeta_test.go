package page

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// 폴더 기본 frontmatter `_metadata.yml`(itda-hyve#474 ①) — 찾기·읽기(foldermeta.go)와 모든 렌더 입구가
// 같은 합친 값을 쓰는지(renderViewOpts·Check·공유). 시험은 HOME 을 임시 폴더로 바꿔 홈 멈춤을 확정한다
// (사용자 실제 홈·~/.itda 에 파일을 만들지 않는다).

// homeSandbox 는 홈 역할을 할 임시 폴더를 HOME 로 삼고 그 안에 문서 폴더 x/y 를 만든다.
// 위 폴더(x 위 — 홈 밖)에도 폴더 메타를 둘 수 있다(홈 멈춤 시험이 쓴다).
func homeSandbox(t *testing.T) (home, docs string) {
	t.Helper()
	tmp := t.TempDir()
	home = filepath.Join(tmp, "home")
	docs = filepath.Join(home, "x", "y")
	if err := os.MkdirAll(docs, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home) // os.UserHomeDir 이 여기를 홈으로 본다
	return home, docs
}

func writeMeta(t *testing.T, dir, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, FolderMetaName), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeMetaDoc(t *testing.T, p, text string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// 문서 폴더부터 위로 찾아 먼 폴더 → 가까운 폴더 순서로, Rel 은 문서 기준 상대 경로다.
func TestFindFolderMetasOrder(t *testing.T) {
	_, docs := homeSandbox(t)
	x := filepath.Dir(docs)
	writeMeta(t, x, "theme: classic\n")
	writeMeta(t, docs, "theme: bold\n")
	doc := filepath.Join(docs, "c.md")
	writeMetaDoc(t, doc, "# c\n")
	fm := findFolderMetas(doc)
	if len(fm) != 2 || fm[0].Rel != "../_metadata.yml" || fm[1].Rel != "_metadata.yml" {
		t.Fatalf("찾은 순서·경로: %+v", fm)
	}
	if string(fm[0].YAML) != "theme: classic\n" || string(fm[1].YAML) != "theme: bold\n" {
		t.Fatalf("내용: %+v", fm)
	}
}

// 홈 폴더의 파일까지 읽고 그 위(홈 밖)로는 가지 않는다. 경로가 없으면(빈글자) 상속이 없다.
func TestFindFolderMetasStopsAtHome(t *testing.T) {
	home, docs := homeSandbox(t)
	outside := filepath.Dir(home) // 홈의 위 폴더(임시 폴더 루트) — 홈 밖
	writeMeta(t, outside, "theme: classic\n")
	writeMeta(t, home, "theme: mono\n")
	writeMeta(t, docs, "theme: bold\n")
	doc := filepath.Join(docs, "c.md")
	writeMetaDoc(t, doc, "# c\n")
	fm := findFolderMetas(doc)
	if len(fm) != 2 { // 홈(먼) + 문서 폴더(가까운) — 홈 밖은 안 읽는다
		t.Fatalf("홈에서 멈추지 않음: %+v", fm)
	}
	if fm[0].Rel != "../../_metadata.yml" {
		t.Errorf("홈 파일의 상대 경로: %s", fm[0].Rel)
	}
	if fm2 := findFolderMetas(""); fm2 != nil {
		t.Errorf("빈 경로: %+v", fm2)
	}
}

// 문서 폴더가 홈 자체여도 홈에서 멈춘다(홈의 파일만 읽고 그 위로는 가지 않는다 — 리뷰 R1 [4] 반례 이전).
func TestFindFolderMetasHomeItself(t *testing.T) {
	home, _ := homeSandbox(t)
	writeMeta(t, home, "theme: mono\n")
	writeMeta(t, filepath.Dir(home), "theme: classic\n") // 홈 밖 — 읽지 않는다
	doc := filepath.Join(home, "a.md")
	writeMetaDoc(t, doc, "# a\n")
	fm := findFolderMetas(doc)
	if len(fm) != 1 || fm[0].Rel != "_metadata.yml" || !strings.Contains(string(fm[0].YAML), "mono") {
		t.Fatalf("홈 자체 문서: %+v", fm)
	}
}

// 볼륨 경계(경로 함수 단위): macOS 는 /Volumes/<디스크> 폴더에서 멈추고(그 폴더의 파일은 읽는다),
// Windows 는 드라이브 루트, 그 밖은 경로 루트다.
func TestFolderMetaStop(t *testing.T) {
	cases := []struct {
		dir  string
		want bool
	}{
		{"/", true},
		{"/Volumes", false}, // 볼륨을 두는 폴더 자체는 경계가 아니다
		{"/Volumes/Backup", true},
		{"/Volumes/Backup/프로젝트", false},
		{"/Users/a/docs", false},
		{`C:\`, true},
		{`C:\docs`, false},
	}
	for _, c := range cases {
		if got := FolderMetaStop(c.dir); got != c.want {
			t.Errorf("FolderMetaStop(%q)=%v, 기대 %v", c.dir, got, c.want)
		}
	}
}

// 렌더는 폴더 파일을 매번 다시 읽는다(내용 캐시가 없다) — 같은 크기·같은 수정 시각으로 내용을 덮어써도
// 다음 렌더가 새 값을 쓴다. 감시(FolderMetaStamp)만 이 변경을 못 본다(아래 시험).
func TestFindFolderMetasRereadsContent(t *testing.T) {
	_, docs := homeSandbox(t)
	writeMeta(t, docs, "theme: classic\n")
	doc := filepath.Join(docs, "c.md")
	writeMetaDoc(t, doc, "# c\n")
	p := filepath.Join(docs, FolderMetaName)
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("theme: custom\n"), 0o644); err != nil { // 같은 길이
		t.Fatal(err)
	}
	if err := os.Chtimes(p, st.ModTime(), st.ModTime()); err != nil { // 같은 수정 시각으로 되돌림
		t.Fatal(err)
	}
	fm := findFolderMetas(doc)
	if len(fm) != 1 || !strings.Contains(string(fm[0].YAML), "custom") {
		t.Fatalf("같은 크기·시각의 내용 변경을 못 읽음: %+v", fm)
	}
}

// 창 렌더(renderViewOpts)가 폴더 값을 문서에 얹힌다 — 인수 조건 ①②③(itda-hyve#474).
func TestRenderViewFolderMeta(t *testing.T) {
	ts, err := BuiltinThemes()
	if err != nil {
		t.Fatal(err)
	}
	_, docs := homeSandbox(t)
	x := filepath.Dir(docs)
	writeMeta(t, x, "slides:\n  font-scale: 0.8\n")
	srcA := "---\nslides:\n  ratio: \"4:3\"\n---\n\n# a\n"
	docA := filepath.Join(docs, "a.md")
	writeMetaDoc(t, docA, srcA)

	// ①: 폴더 font-scale + 문서 ratio — 슬라이드로 열리고 둘 다 적용.
	v := renderView([]byte(srcA), docA, "", ts, "", false)
	if !v.Slides.Present || v.Slides.Ratio != "4:3" || v.Slides.FontScale != 0.8 {
		t.Errorf("인수 ①: slides %+v", v.Slides)
	}
	if len(v.FolderMeta) != 1 || v.FolderMeta[0] != "../_metadata.yml" {
		t.Errorf("출처 목록: %v", v.FolderMeta)
	}
	// ②: slides 없는 문서는 슬라이드로 열리지 않는다.
	v = renderView([]byte("# b\n"), filepath.Join(docs, "b.md"), "", ts, "", false)
	if v.Slides.Present {
		t.Errorf("인수 ②: 슬라이드로 열림 %+v", v.Slides)
	}
	// ③: 먼 폴더 < 가까운 폴더 < 문서.
	writeMeta(t, docs, "theme: bold\n")
	c := filepath.Join(docs, "c.md")
	writeMetaDoc(t, c, "# c\n")
	if v := renderView([]byte("# c\n"), c, "", ts, "", false); v.Theme != "bold" {
		t.Errorf("인수 ③ 가까운 폴더: %q", v.Theme)
	}
	if v := renderView([]byte("---\ntheme: mono\n---\n\n# c\n"), c, "", ts, "", false); v.Theme != "mono" {
		t.Errorf("인수 ③ 문서: %q", v.Theme)
	}
	// 경로 없는 렌더(붙여넣은 원문)는 상속이 없다 — 지금과 같다.
	v = renderView([]byte(srcA), "", "", ts, "", false)
	if v.FolderMeta != nil {
		t.Errorf("빈 경로에 상속: %v", v.FolderMeta)
	}
	if !v.Slides.Present || v.Slides.FontScale != 0 {
		t.Errorf("빈 경로의 slides: %+v", v.Slides)
	}
}

// md_check(docview.Check)도 경로가 있으면 폴더 값을 함께 검사한다(itda-hyve#474 — 문서 창과 같은 값).
func TestCheckFolderMeta(t *testing.T) {
	_, docs := homeSandbox(t)
	writeMeta(t, docs, "theme: mono\ntoc: 안녕\n")
	src := "---\n---\n\n# c\n"
	doc := filepath.Join(docs, "c.md")
	writeMetaDoc(t, doc, src)
	res, err := Check([]byte(src), doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Meta.Theme != "mono" {
		t.Errorf("폴더 theme 가 검사에 안 얹힘: %+v", res.Meta)
	}
	if len(res.Warnings) != 1 || res.Warnings[0].Line != 0 || !strings.Contains(res.Warnings[0].Message, "_metadata.yml 2줄") {
		t.Errorf("폴더 값 경고: %+v", res.Warnings)
	}
	// 응답에 적용된 폴더 파일 목록이 함께 간다(가까운 것부터 — 같은 원문도 조상 파일에 따라 결과가 달라진다).
	if !slices.Equal(res.FolderMeta, []string{"_metadata.yml"}) {
		t.Errorf("folder_meta: %v", res.FolderMeta)
	}
	// 경로 없이 검사하면 상속이 없다(지금과 같다).
	res, err = Check([]byte(src), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Meta.Theme != "classic" || len(res.Warnings) != 0 || res.FolderMeta != nil {
		t.Errorf("경로 없는 검사가 달라짐: %+v %v %v", res.Meta, res.Warnings, res.FolderMeta)
	}
}

// FolderMetaStamp 는 폴더 파일의 수정 시각·크기·목록 변화를 잡는다(감시·공유의 기준 — 문서 자신의
// 감시 Doc.Changed 와 같은 방식). 같은 크기·같은 수정 시각으로 덮어쓴 내용 변경은 못 본다(렌더는
// 다시 읽어 반영한다 — TestFindFolderMetasRereadsContent).
func TestFolderMetaStamp(t *testing.T) {
	_, docs := homeSandbox(t)
	doc := filepath.Join(docs, "c.md")
	writeMetaDoc(t, doc, "# c\n")
	if st := FolderMetaStamp(doc); st != "" {
		t.Fatalf("파일이 없는데 %q", st)
	}
	writeMeta(t, docs, "theme: classic\n")
	st1 := FolderMetaStamp(doc)
	if st1 == "" {
		t.Fatal("새로 생긴 파일을 못 잡음")
	}
	time.Sleep(2 * time.Millisecond) // 같은 크기로 다시 써도 수정 시각이 오른다
	writeMeta(t, docs, "theme: bold\n")
	if st2 := FolderMetaStamp(doc); st2 == st1 {
		t.Error("수정 변화를 못 잡음")
	}
	writeMeta(t, filepath.Dir(docs), "toc: true\n") // 위 폴더에 새로 생기는 파일도 목록 변화로 잡는다
	if st3 := FolderMetaStamp(doc); !strings.Contains(st3, "/x|") || !strings.Contains(st3, "/x/y|") {
		t.Errorf("두 폴더가 다 기준에 없음: %q", st3)
	}
	// 한계(문서 자신의 감시와 같다): 같은 크기·같은 수정 시각으로 덮어쓴 내용 변경은 stamp 가 잡지
	// 못한다(시각 복원의 ns 정밀도가 파일 시스템마다 달라 여기서 못박지 않는다). 렌더는 다시 읽어
	// 반영한다(TestFindFolderMetasRereadsContent) — 감시만 못 본다.
}

// 읽기 상한: 64KB 는 읽고 64KB+1 은 거부한다. 폴더(디렉터리)도 거부(리뷰 2회차 반례 이전).
func TestFolderMetaReadBound(t *testing.T) {
	_, docs := homeSandbox(t)
	p := filepath.Join(docs, FolderMetaName)
	for _, n := range []int{MaxFolderMetaBytes, MaxFolderMetaBytes + 1} {
		writeMeta(t, docs, strings.Repeat("#", n))
		f := ReadFolderMeta("_metadata.yml", p)
		if (f.Err != "") != (n > MaxFolderMetaBytes) {
			t.Fatalf("size=%d err=%q", n, f.Err)
		}
	}
	if f := ReadFolderMeta("directory", docs); f.Err == "" {
		t.Fatal("폴더를 받아들임")
	}
}

// md_check 응답: source_hash 는 원문 해시 그대로(path 가 있어도 변하지 않고), folder_meta 는
// 경로 있을 때만 나타나 가까운 것부터다(리뷰 2회차 반례 이전).
func TestCheckFolderMetaJSON(t *testing.T) {
	home, docs := homeSandbox(t)
	writeMeta(t, home, "theme: classic\n")
	writeMeta(t, docs, "toc: true\n")
	src := []byte("# a\n")
	a, err := Check(src, filepath.Join(docs, "a.md"), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Check(src, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.SourceHash != b.SourceHash {
		t.Fatal("source_hash 가 path 로 바뀜")
	}
	for _, tc := range []struct {
		v    CheckResult
		want bool
	}{{a, true}, {b, false}} {
		data, err := json.Marshal(tc.v)
		if err != nil {
			t.Fatal(err)
		}
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			t.Fatal(err)
		}
		if _, has := obj["folder_meta"]; has != tc.want {
			t.Fatalf("json=%s", data)
		}
	}
	if len(a.FolderMeta) != 2 || a.FolderMeta[0] != "_metadata.yml" || a.FolderMeta[1] != "../../_metadata.yml" {
		t.Fatalf("순서=%v", a.FolderMeta)
	}
}

// itda-hyve#504 인수 1: 폴더에서 받은 테마는 View.DocThemeFrom 이 그 파일(문서 기준 상대 경로)이고, 문서 자신의 theme: 은 ""
// (화면 「(문서 설정)」)다. 테마 값 고르기(DocTheme·Theme)는 그대로다. 화면 표기는 TestFolderFormUI 가 본다.
func TestRenderViewDocThemeFrom(t *testing.T) {
	ts, err := BuiltinThemes()
	if err != nil {
		t.Fatal(err)
	}
	_, docs := homeSandbox(t)
	x := filepath.Dir(docs)
	writeMeta(t, x, "theme: bold\n")
	sub := filepath.Join(docs, "a.md")
	writeMetaDoc(t, sub, "# a\n")
	v := renderView([]byte("# a\n"), sub, "", ts, "", false)
	if v.DocTheme != "bold" || v.Theme != "bold" || v.DocThemeFrom != "../_metadata.yml" {
		t.Fatalf("위 폴더 테마: DocTheme %q Theme %q From %q", v.DocTheme, v.Theme, v.DocThemeFrom)
	}
	// 가까운 폴더가 덮으면 그 파일
	writeMeta(t, docs, "theme: mono\n")
	if v := renderView([]byte("# a\n"), sub, "", ts, "", false); v.DocTheme != "mono" || v.DocThemeFrom != "_metadata.yml" {
		t.Fatalf("가까운 폴더 테마: %q %q", v.DocTheme, v.DocThemeFrom)
	}
	// 문서 자신의 theme: — 출처 없음(문서 설정)
	if v := renderView([]byte("---\ntheme: classic\n---\n\n# a\n"), sub, "", ts, "", false); v.DocTheme != "classic" || v.DocThemeFrom != "" {
		t.Fatalf("문서 테마: %q %q", v.DocTheme, v.DocThemeFrom)
	}
	// 화면에서 고른 테마(override)는 DocTheme·출처를 바꾸지 않는다
	if v := renderView([]byte("# a\n"), sub, "", ts, "classic", false); v.Theme != "classic" || v.DocTheme != "mono" || v.DocThemeFrom != "_metadata.yml" {
		t.Fatalf("override: %q %q %q", v.Theme, v.DocTheme, v.DocThemeFrom)
	}
	// 폴더 파일이 theme 을 갖지 않으면 출처도 없다(테마를 정하지 않음)
	writeMeta(t, docs, "toc: true\n")
	writeMeta(t, x, "toc: true\n")
	if v := renderView([]byte("# a\n"), sub, "", ts, "", false); v.DocTheme != "" || v.DocThemeFrom != "" {
		t.Fatalf("테마 없음: %q %q", v.DocTheme, v.DocThemeFrom)
	}
	// 경로 없는 렌더(붙여넣은 원문)
	if v := renderView([]byte("---\ntheme: classic\n---\n# a\n"), "", "", ts, "", false); v.DocThemeFrom != "" {
		t.Fatalf("경로 없음: %q", v.DocThemeFrom)
	}
}
