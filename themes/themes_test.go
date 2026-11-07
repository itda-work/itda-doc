package themes

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// js 판 묶음(themes_js.go)은 네이티브 묶음에서 assets/ 만 뺀 것과 파일이 같다 — 글꼴을 다시 넣거나 새 테마를 빠뜨리면 실패한다.
// 빌드하지 않고 go list 의 EmbedFiles 를 본다. 뮤테이션 시험은 go test -overlay <json> 과 함께 ITDA_DOC_TEST_OVERLAY=<json> 을 준다.
// wasm 산출물 안의 서명은 TestViewerLoaderAndSource(cmd/itda-doc-wasm)가, 테마 CSS 의 동일성은 TestWasmMatchesNative 가 본다.
func TestJSBundleHasNoFonts(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Fatal("go 명령을 찾지 못함")
	}
	args := []string{"list", "-json"}
	if ov := os.Getenv("ITDA_DOC_TEST_OVERLAY"); ov != "" {
		args = append(args, "-overlay", ov)
	}
	cmd := exec.Command(gobin, append(args, ".")...)
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	var pkg struct{ EmbedFiles []string }
	if err := json.Unmarshal(out, &pkg); err != nil {
		t.Fatal(err)
	}

	var want []string
	fonts, themes := 0, 0
	err = fs.WalkDir(FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if strings.Contains(p, "/assets/") {
			if strings.HasSuffix(strings.ToLower(p), ".woff2") {
				fonts++
			}
			return nil
		}
		if strings.HasSuffix(p, "/theme.json") {
			themes++
		}
		want = append(want, p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if fonts == 0 || themes < 10 { // 네이티브 묶음을 못 읽어 빈 목록끼리 같아지지 않게, 네이티브의 글꼴이 그대로인지도 함께
		t.Fatalf("네이티브 묶음: 글꼴 %d 개·테마 %d 개 — 글꼴이 있고 테마가 10 개 이상이어야 함", fonts, themes)
	}
	slices.Sort(want)
	got := slices.Sorted(slices.Values(pkg.EmbedFiles))
	if !slices.Equal(got, want) {
		t.Errorf("js 묶음 파일이 네이티브에서 assets/ 를 뺀 것과 다름\n받음 %q\n기대 %q", got, want)
	}
	for _, p := range got {
		switch strings.ToLower(p[strings.LastIndex(p, ".")+1:]) {
		case "woff2", "woff", "ttf", "otf":
			t.Errorf("js 묶음에 글꼴 파일 %s", p)
		}
	}
}

// 공개 FS 의 ReadFile·ReadDir·Open 을 직접 부르는 소비자가 네이티브와 js 양쪽에서 컴파일되는지 본다. js 판의 FS 를 fs.FS 로 좁히면 실패한다.
// 소비자 소스는 저장소에 두지 않고 overlay 로 없는 폴더에 얹는다. ITDA_DOC_TEST_OVERLAY 가 있으면 그 내용(절대 경로)과 합쳐 뮤테이션도 받는다.
func TestFSPublicMethods(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Fatal("go 명령을 찾지 못함")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	probe := filepath.Join(dir, "probe.go")
	const src = `package apiprobe

import "github.com/itda-work/itda-doc/themes"

func Use() error {
	if _, err := themes.FS.ReadFile("classic/theme.json"); err != nil {
		return err
	}
	if _, err := themes.FS.ReadDir("."); err != nil {
		return err
	}
	f, err := themes.FS.Open("classic/theme.css")
	if err != nil {
		return err
	}
	return f.Close()
}
`
	if err := os.WriteFile(probe, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay := struct{ Replace map[string]string }{}
	if ov := os.Getenv("ITDA_DOC_TEST_OVERLAY"); ov != "" {
		b, err := os.ReadFile(ov)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &overlay); err != nil {
			t.Fatal(err)
		}
	}
	if overlay.Replace == nil {
		overlay.Replace = map[string]string{}
	}
	overlay.Replace[filepath.Join(cwd, "apiprobe", "probe.go")] = probe
	b, err := json.Marshal(overlay)
	if err != nil {
		t.Fatal(err)
	}
	ovPath := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(ovPath, b, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name string
		env  []string
	}{
		{"네이티브", nil},
		{"js", []string{"GOOS=js", "GOARCH=wasm"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			cmd := exec.Command(gobin, "build", "-overlay", ovPath, "./apiprobe")
			cmd.Env = append(os.Environ(), c.env...)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("공개 FS 호출 컴파일 실패: %v\n%s", err, out)
			}
		})
	}
}

// 내장 테마 woff2 의 머리 표식(flavor)이 TrueType 인지 본다. CFF(OTF 기반)는 macOS PDF 에서 글자층(ToUnicode)이 비어
// 검색·복사가 깨진다(itda-hyve#666). 머리의 4~8 바이트가 원래 글꼴의 sfnt 판이다 — TrueType 은 0x00010000, CFF 는 "OTTO".
// 머리만 본다 — 압축 본문의 유효성, glyf·cmap·GSUB, PDF 글자층은 이 시험이 묶지 않는다.
func TestBuiltinWoff2TrueType(t *testing.T) {
	seen := 0
	err := fs.WalkDir(FS, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(p), ".woff2") {
			return err
		}
		seen++
		b, err := fs.ReadFile(FS, p)
		if err != nil {
			return err
		}
		if len(b) < 8 || string(b[:4]) != "wOF2" {
			t.Errorf("%s: woff2 가 아님", p)
			return nil
		}
		if flavor := b[4:8]; !bytes.Equal(flavor, []byte{0, 1, 0, 0}) {
			t.Errorf("%s: flavor %q — TrueType(0x00010000) 윤곽이어야 함", p, flavor)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen == 0 { // 경로가 어긋나 아무것도 안 보고 통과하지 않게
		t.Error("내장 테마에서 woff2 를 하나도 찾지 못함")
	}
}
