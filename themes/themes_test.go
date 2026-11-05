package themes

import (
	"bytes"
	"io/fs"
	"strings"
	"testing"
)

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
