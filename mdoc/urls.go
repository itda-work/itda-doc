package mdoc

import (
	"html"
	"regexp"
	"strings"
)

// 링크·이미지 주소 규칙(1.9).
//
// 링크는 허용 목록이다: 상대 경로·#조각, http·https·mailto·tel.
// 설계 문서는 javascript·vbscript·data·file 을 막는다고 적었지만, 목록 밖의 스킴(ms-msdt:·search-ms: 처럼
// Windows 프로토콜 처리기를 부르는 것)도 같은 위험이라 모르는 스킴은 모두 막는다.

type urlKind int

const (
	urlOK      urlKind = iota
	urlRemote          // http(s) 이미지 — 기본으로 불러오지 않는다
	urlBlocked         // 막은 스킴
)

var schemeRe = regexp.MustCompile(`^([a-z][a-z0-9+.\-]*):`)

// schemeOf 는 브라우저가 볼 스킴을 돌려준다. 엔티티를 풀고, 브라우저가 무시하는 공백·제어 문자를 지운 뒤 본다
// (java&#x09;script: · " javascript:" 같은 우회를 같은 것으로 본다). 스킴이 없으면 "" 다.
// 백슬래시 이스케이프도 먼저 푼다 — 출력의 URLEscape(…, true) 가 javascript\: 를 javascript: 로 풀기 때문이다.
func schemeOf(dest string) (scheme string, cleaned string) {
	s := html.UnescapeString(inlineValue([]byte(dest)))
	s = strings.Map(func(r rune) rune {
		if r <= 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	s = strings.ToLower(s)
	if m := schemeRe.FindStringSubmatch(s); m != nil {
		return m[1], s
	}
	return "", s
}

func classifyLink(dest string) urlKind {
	scheme, _ := schemeOf(dest)
	switch scheme {
	case "", "http", "https", "mailto", "tel":
		return urlOK
	}
	return urlBlocked
}

func classifyImage(dest string) urlKind {
	scheme, s := schemeOf(dest)
	switch scheme {
	case "":
		// //host·\\host 는 원격이다(Windows 에서 \\server 는 SMB 로 나가 자격 증명이 샐 수 있다)
		if len(s) >= 2 && (s[0] == '/' || s[0] == '\\') && (s[1] == '/' || s[1] == '\\') {
			return urlRemote
		}
		return urlOK
	case "http", "https":
		return urlRemote
	case "data":
		if strings.HasPrefix(s, "data:image/") {
			return urlOK
		}
	}
	return urlBlocked
}

// isDataURL 은 data: 주소인지다(classifyImage 가 data:image/* 만 통과시킨다).
func isDataURL(dest string) bool {
	scheme, _ := schemeOf(dest)
	return scheme == "data"
}
