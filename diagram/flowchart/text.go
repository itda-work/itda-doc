package flowchart

import (
	"math"

	"github.com/itda-work/itda-doc/diagram"
	"github.com/itda-work/itda-doc/diagram/layered"
)

// 글자 폭 어림·줄바꿈은 공통 패키지(diagram)다 — 간트 차트가 함께 쓴다(itda-hyve#409). 여기는 흐름도 배치가
// 쓰는 짧은 위임과 모양 크기 계산이다.

func wrapLabel(s string, width float64) []string { return diagram.WrapLabel(s, width) }

func linesW(lines []string) float64 { return diagram.LinesW(lines) }

// labelSize 는 간선 라벨 상자(바탕 포함)의 크기다.
func labelSize(lines []string) (float64, float64) { return layered.LabelSize(lines) }

func hexSlant(h float64) float64  { return h * 0.3 }
func leanSlant(h float64) float64 { return h * 0.35 }
func cylRy(w float64) float64     { return math.Min(10, float64(w*0.06)+2) }

// shapeSize 는 라벨 줄을 담는 모양의 너비·높이다.
func shapeSize(sh Shape, lines []string) [2]float64 {
	tw := linesW(lines)
	th := float64(len(lines)) * lineH
	w, h := tw+2*padX, th+2*padY
	switch sh {
	case ShapeSubroutine:
		w += 14
	case ShapeStadium:
		w += h / 2
	case ShapeCircle:
		d := math.Max(hyp(tw, th)+14, 44)
		w, h = d, d
	case ShapeDiamond:
		// 글 상자(bw×bh)를 담는 마름모 가운데 둘레가 가장 짧은 것: 반대각선 a = bw/2·s, b = bh/2·t, 1/s + 1/t = 1
		bw, bh := tw+12, th+6
		s := 1 + math.Sqrt(bh/bw)
		t := 1 + math.Sqrt(bw/bh)
		w, h = math.Max(bw*s, 44), math.Max(bh*t, 36)
	case ShapeHexagon:
		w += float64(2 * hexSlant(h))
	case ShapeLean, ShapeLeanAlt, ShapeTrap, ShapeTrapAlt:
		w += float64(2 * leanSlant(h))
	case ShapeCylinder:
		h += float64(2 * cylRy(w))
	}
	return [2]float64{math.Ceil(w), math.Ceil(h)}
}

// shapePoly 는 다각형 모양의 꼭짓점이다(최종 좌표).
func shapePoly(n nodeBox, sh Shape) []pt {
	x0, y0, x1, y1 := n.c.X-n.w/2, n.c.Y-n.h/2, n.c.X+n.w/2, n.c.Y+n.h/2
	switch sh {
	case ShapeDiamond:
		return []pt{{n.c.X, y0}, {x1, n.c.Y}, {n.c.X, y1}, {x0, n.c.Y}}
	case ShapeHexagon:
		k := hexSlant(n.h)
		return []pt{{x0, n.c.Y}, {x0 + k, y0}, {x1 - k, y0}, {x1, n.c.Y}, {x1 - k, y1}, {x0 + k, y1}}
	case ShapeLean: // /글/
		k := leanSlant(n.h)
		return []pt{{x0 + k, y0}, {x1, y0}, {x1 - k, y1}, {x0, y1}}
	case ShapeLeanAlt: // \글\
		k := leanSlant(n.h)
		return []pt{{x0, y0}, {x1 - k, y0}, {x1, y1}, {x0 + k, y1}}
	case ShapeTrap: // /글\ — 위가 좁다
		k := leanSlant(n.h)
		return []pt{{x0 + k, y0}, {x1 - k, y0}, {x1, y1}, {x0, y1}}
	case ShapeTrapAlt: // \글/ — 아래가 좁다
		k := leanSlant(n.h)
		return []pt{{x0, y0}, {x1, y0}, {x1 - k, y1}, {x0 + k, y1}}
	}
	return []pt{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
}

// hyp 는 √(x²+y²) 다. math.Hypot 와 곱셈-덧셈을 쓰지 않는다 — arm64 는 x*y+z 를 한 번에 반올림하는 명령(FMA)으로 합칠 수 있어
// 아키텍처마다 끝자리가 달라진다. 명시적 float64(…) 변환은 합치기를 막는다(Go 명세). 이 패키지의 곱셈-덧셈은 모두 그렇게 쓴다 —
// 같은 원문이 macOS(arm64)·Windows(amd64·arm64)에서 같은 SVG 다(TestCorpusHash 를 GOARCH=amd64 로도 돌린다).
func hyp(x, y float64) float64 { return math.Sqrt(float64(x*x) + float64(y*y)) }
