//go:build unix

package page

import (
	"os"
	"syscall"
)

// openMetaFile 는 폴더 메타 파일을 연다. 비차단(O_NONBLOCK)으로 열고 연 파일로 정규 파일인지 본다 —
// 탐색(folderMetaDirs)이 정규 파일로 확인한 뒤 누군가 FIFO 로 바꿔 놓아도 열기에서 붙잡히지 않고
// "정규 파일이 아님"으로 건너뛴다(읽는 쪽의 writer 연결 대기가 없다). 정규 파일에서 O_NONBLOCK 은
// 읽기에 영향이 없다(#474 리뷰 2회차 [4]).
func openMetaFile(p string) (*os.File, error) {
	fd, err := syscall.Open(p, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), p), nil
}
