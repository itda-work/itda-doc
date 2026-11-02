//go:build unix

package timingtest

import (
	"syscall"
	"time"
)

// cpuTime 은 이 프로세스가 지금까지 쓴 CPU 시간(사용자 + 시스템)이다.
func cpuTime() (time.Duration, bool) {
	var ru syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &ru) != nil {
		return 0, false
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano()), true
}
