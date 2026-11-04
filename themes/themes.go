// Package themes 는 내장 테마다 — 구조 테마 다섯(classic·bold·compact·mono·spine)과 그 위에 extends 로 얹은 중립 스타일 테마(slate·swiss·contrast). 폴더마다 theme.json·theme.css(새 문서 서식 template.md 는 없다 — 앱의 서식 갤러리가 맡는다).
package themes

import "embed"

// FS 는 내장 테마 폴더를 뿌리에 둔 파일 묶음이다(FS 의 "classic/theme.json" 꼴).
//
//go:embed classic bold compact mono spine slate swiss contrast
var FS embed.FS
