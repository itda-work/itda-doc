//go:build unix

package page

import "syscall"

func mkfifo(p string) { _ = syscall.Mkfifo(p, 0o644) }
