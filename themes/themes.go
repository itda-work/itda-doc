// Package themes 는 내장 중립 테마 다섯(classic·bold·compact·mono·spine)이다. 폴더마다 theme.json·theme.css(새 문서 서식 template.md 는 없다 — 앱의 서식 갤러리가 맡는다).
package themes

import "embed"

// FS 는 내장 테마 폴더를 뿌리에 둔 파일 묶음이다(FS 의 "classic/theme.json" 꼴).
//
//go:embed classic bold compact mono spine
var FS embed.FS
