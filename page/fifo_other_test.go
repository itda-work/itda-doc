//go:build !unix

package page

func mkfifo(string) {}
