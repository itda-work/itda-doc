// Package web 은 쪽 모델 자원이다 — 문서 기본 모양(base.css)·쪽 나눔(pager.js)·슬라이드(slides.js)·보기 전용 공유 화면(share.*)·글꼴(fonts/).
package web

import "embed"

// FS 는 자원을 뿌리에 둔 파일 묶음이다(FS 의 "base.css"·"fonts/pretendard-regular.woff2" 꼴).
//
//go:embed base.css pager.js slides.js share.css share.html share.js fonts LICENSE-Lucide.txt
var FS embed.FS

// BaseCSS 는 base.css 하나다. FS 를 쓰지 않는 쪽(page 의 예약 글꼴 모으기)이 글꼴 묶음까지 바이너리에 끌어오지 않게 따로 둔다.
//
//go:embed base.css
var BaseCSS []byte
