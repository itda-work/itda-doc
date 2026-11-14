//go:build !js

package themes

import "embed"

// FS 는 내장 테마 폴더를 뿌리에 둔 파일 묶음이다(FS 의 "classic/theme.json" 꼴).
//
//go:embed classic bold compact mono spine slate swiss contrast verdant editorial hyrule
var FS embed.FS
