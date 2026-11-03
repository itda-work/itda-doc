package state

import "github.com/itda-work/itda-doc/diagram"

// 엔티티 정책 B(itda-hyve#409 ④-2 접근 방식 리뷰, 2026-10-31 사용자 결정)는 여섯 종류가 함께 쓰는 공통 구현으로 옮겼다
// (diagram/policy.go — RestoreEntities·EntityNumOK·EntityUnicodeVersion, 규칙은 그 머리 주석). 여기 이름은 state 의
// 파서·시험이 쓰는 별칭이다(동작은 옮기기 전과 같다).

// unicodeVersion 은 숫자 정책이 기대는 Go unicode 표의 판본이다(Go 를 올려 판본이 바뀌면 TestEntityPolicyTable 이 알린다).
const unicodeVersion = diagram.EntityUnicodeVersion

// numOK 는 숫자 엔티티의 값 n 을 받는지다(정책 B).
func numOK(n int) bool { return diagram.EntityNumOK(n) }

// restore 는 글의 엔티티 표식을 정책 B 로 한 번 되돌린다. 받지 않는 표식이 있으면 bad 에 그 엔티티(`#…;`)를 돌려준다.
func restore(s string) (v, bad string) { return diagram.RestoreEntities(s) }
