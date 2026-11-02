//go:build unix

package page

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/itda-work/itda-doc/mdoc"
)

// FIFO 로 바뀐 폴더 메타 파일을 열 때 기다리지 않는다(Unix 의 비차단 열기 — 리뷰 2회차 [4] 반례 이전).
// writer 가 붙지 않은 FIFO 를 readFolderMeta 에 주면 열기가 200ms 안에 "정규 파일이 아님"으로 돌아온다.
func TestFolderMetaFIFOOpen(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan mdoc.FolderMeta, 1)
	go func() { done <- readFolderMeta("_metadata.yml", fifo) }()
	select {
	case f := <-done:
		if f.Err == "" {
			t.Fatal("FIFO 를 받아들임")
		}
		return
	case <-time.After(200 * time.Millisecond):
	}
	// 이 경로에 오면 열기가 writer 연결을 기다렸다는 뜻이다 — writer 를 붙여 끝내고 실패로 처리한다.
	fd, err := syscall.Open(fifo, syscall.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	syscall.Close(fd)
	select {
	case f := <-done:
		t.Fatalf("열기가 writer 연결까지 막혔음(최종 err=%q)", f.Err)
	case <-time.After(time.Second):
		t.Fatal("reader 가 아직 막혀 있음")
	}
}
