//go:build !js

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// 웹 뷰어(web/viewer, itda-work/itda-hyve#577 2b)의 node 로 볼 수 있는 것: 로더(itda-doc.js)가 옵션 오류를 throw 하는지, 뷰어 원천
// (viewer.js viewerSource)을 share.js pickSource 가 고르고 그 원천이 wasm 으로 그린 View 를 화면 api 로 넘기는지(상대 경로 그림·경고 목록·
// 테마 CSS·원격 그림 스위치 포함), 빌드(scripts/build-web.sh)가 싣는 견본이 오류 없이 그려지는지. 브라우저 동작(화면·인쇄)은 보지 않는다.
func TestViewerLoaderAndSource(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("ITDA_DOC_STRICT") == "1" {
			t.Fatal("node 가 없어 뷰어 검사를 할 수 없다 — ITDA_DOC_STRICT=1(just test·CI)은 node 가 필요하다")
		}
		t.Skip("node 가 없어 뷰어 검사를 건너뜀")
	}
	if testing.Short() {
		t.Skip("-short: wasm 빌드를 건너뜀")
	}
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Fatal("go 명령을 찾지 못함")
	}
	dir := t.TempDir()
	wasm := filepath.Join(dir, "itda-doc.wasm")
	args := []string{"build", "-o", wasm}
	if ov := os.Getenv("ITDA_DOC_TEST_OVERLAY"); ov != "" {
		args = append(args, "-overlay", ov)
	}
	build := exec.Command(gobin, append(args, ".")...)
	build.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("wasm 빌드 실패: %v\n%s", err, out)
	}
	// 글꼴은 wasm 에 들어가지 않는다(themes/themes_js.go, itda-doc#48) — 웹 뷰어의 글꼴은 정적 파일(web/fonts)이다
	if n := bytes.Count([]byte(readFile(t, wasm)), []byte("wOF2")); n != 0 {
		t.Errorf("wasm 안에 woff2 서명(wOF2)이 %d 개", n)
	}

	// 견본 목록은 빌드 스크립트의 SAMPLES 한 곳에서 읽는다(「<저장소 경로>|<번들 이름>|<고르기 글>」)
	script := readFile(t, filepath.Join("..", "..", "scripts", "build-web.sh"))
	type sample struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	var samples []sample
	for _, m := range regexp.MustCompile(`(?m)^  "([^"|]+)\|([^"|]+)\|([^"|]+)"$`).FindAllStringSubmatch(script, -1) {
		p, _ := filepath.Abs(filepath.Join("..", "..", m[1]))
		samples = append(samples, sample{m[2], p})
	}
	if len(samples) != 3 {
		t.Fatalf("build-web.sh 의 견본이 %d 개(3 이어야 함)", len(samples))
	}
	sj, _ := json.Marshal(samples)
	samplesPath, outPath := filepath.Join(dir, "samples.json"), filepath.Join(dir, "out.json")
	if err := os.WriteFile(samplesPath, sj, 0o600); err != nil {
		t.Fatal(err)
	}
	wasmExec := filepath.Join(goEnv(t, gobin, "GOROOT"), "lib", "wasm", "wasm_exec.js")
	temmlJS, _ := filepath.Abs(filepath.Join("..", "..", "texmath", "temml", "temml.min.js"))
	webDir, _ := filepath.Abs(filepath.Join("..", "..", "web"))
	runner, _ := filepath.Abs(filepath.Join("testdata", "viewer.cjs"))
	if out, err := exec.Command(node, runner, wasmExec, temmlJS, wasm, webDir, samplesPath, outPath).CombinedOutput(); err != nil {
		t.Fatalf("node 실행 실패: %v\n%s", err, out)
	}
	var got struct {
		Checks []struct {
			Name string `json:"name"`
			OK   bool   `json:"ok"`
			Got  string `json:"got"`
		} `json:"checks"`
		Samples []struct {
			Name string         `json:"name"`
			View map[string]any `json:"view"`
		} `json:"samples"`
	}
	if err := json.Unmarshal([]byte(readFile(t, outPath)), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Checks) < 30 {
		t.Errorf("검사가 %d 개뿐 — viewer.cjs 가 중간에 멈췄다", len(got.Checks))
	}
	for _, c := range got.Checks {
		if !c.OK {
			t.Errorf("%s: %s", c.Name, c.Got)
		}
	}
	if len(got.Samples) != len(samples) {
		t.Fatalf("견본 결과 %d/%d", len(got.Samples), len(samples))
	}
	for _, s := range got.Samples {
		html, _ := s.View["html"].(string)
		if e, _ := s.View["error"].(string); e != "" || len(html) < 500 {
			t.Errorf("견본 %s: error %q, html %d 바이트", s.Name, e, len(html))
		}
		if s.Name == "math.md" && !strings.Contains(html, "<math") {
			t.Errorf("견본 math.md 에 <math 가 없음")
		}
	}
}
