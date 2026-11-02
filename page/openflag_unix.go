//go:build unix

package page

import "syscall"

// OpenFlags 는 그림을 열 때 더하는 플래그다. FIFO 를 그림 이름으로 두면 쓰는 쪽이 없을 때 open 이 막힌다 —
// 막지 않고 열어 정규 파일인지 본 뒤 거부한다(docfile_test 가 잡은 결함).
const OpenFlags = syscall.O_NONBLOCK
