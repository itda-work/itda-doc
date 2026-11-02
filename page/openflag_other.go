//go:build !unix

package page

// openFlags 는 Windows 에서 더할 것이 없다(이름 있는 파이프는 파일 경로로 열리지 않는다). js/wasm 에도 더할 것이 없다.
const openFlags = 0
