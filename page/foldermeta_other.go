//go:build !unix

package page

import "os"

// openMetaFile 는 폴더 메타 파일을 연다. unix 빌드 태그 밖의 플랫폼(지원 대상으로는 Windows, 그 밖에 wasm·plan9 등)은
// 비차단 열기(foldermeta_unix.go)를 쓰지 않고 그대로 연다. Windows 에는 열 때 writer 를 기다리는 FIFO 가 없다 — 교체
// 경합은 연 파일의 정규 파일 검사·읽기 상한(ReadFolderMeta)이 잡는다. Windows 밖의 !unix 플랫폼은 실행·실측하지 않았다(미검증).
func openMetaFile(p string) (*os.File, error) { return os.Open(p) }
