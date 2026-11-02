---
title: 수식 견본 — 재무 지표와 통계
subtitle: $…$ 와 $$…$$ (설계 1.10)
date: 2026-10-13
lang: ko
theme: classic
organization: 잇다워크 주식회사
doc-no: 수식-2026-01
---

# 지표의 정의

매출은 단가와 수량의 곱 $R = p \times q$ 이고, 영업이익률은 $m = \frac{\text{영업이익}}{\text{매출}} \times 100\%$ 다.
전년 대비 성장률은 $g = \frac{R_t - R_{t-1}}{R_{t-1}}$ 로 센다. MATH_A01

금액의 달러 기호는 수식이 아니다: 계약금 $30 와 잔금 $40, 범위 $30–$40, 그리고 이스케이프한 \$5 는 모두 글자다. MATH_A02

$$
\text{순현재가치} = \sum_{t=0}^{n} \frac{CF_t}{(1+r)^t}
$$

MATH_A03 위는 블록 수식이다.

## 근의 공식과 행렬

이차방정식 $ax^2 + bx + c = 0$ 의 근은 다음과 같다. MATH_B01

$$
x = \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}
$$

연립방정식을 행렬로 쓰면 MATH_B02

$$
\begin{pmatrix} 2 & 1 \\ 1 & 3 \end{pmatrix}
\begin{pmatrix} x \\ y \end{pmatrix}
=
\begin{pmatrix} 5 \\ 10 \end{pmatrix},
\qquad
\det A = \begin{vmatrix} 2 & 1 \\ 1 & 3 \end{vmatrix} = 5
$$

## 정렬과 경우 나누기

$$
\begin{aligned}
(a+b)^2 &= (a+b)(a+b) \\
        &= a^2 + 2ab + b^2
\end{aligned}
$$

MATH_C01 절댓값은

$$
|x| = \begin{cases} x & x \geq 0 \\ -x & \text{그 외} \end{cases}
$$

## 표 안의 수식

| 지표 | 식 | 비고 |
|:--|:--|:--|
| 영업이익률 | $\frac{P}{R} \times 100$ | MATH_D01 |
| 증가율 | $\frac{\Delta R}{R}$ | 전년 대비 |
| 표준편차 | $\sigma = \sqrt{\frac{1}{n}\sum_{i=1}^{n}(x_i - \bar{x})^2}$ | 모집단 |
| 노름 | $\lVert x \rVert = \sqrt{x \cdot x}$ | MATH_D02 |

: 지표 식 MATH_D03

## 긴 문단(쪽 경계에서 수식이 나뉘지 않는다)

이 문단은 쪽 경계에 걸리도록 길게 쓴다. 매 문장에 글 안 수식이 들어 있다: 할인율 $d = \frac{r}{1+r}$, 연금 현가 $PV = C \cdot \frac{1-(1+r)^{-n}}{r}$,
복리 $FV = PV(1+r)^n$, 연속 복리 $FV = PV e^{rt}$, 내부수익률은 $\sum_{t=0}^{n} \frac{CF_t}{(1+IRR)^t} = 0$ 을 만족하는 $IRR$ 이다.
듀레이션 $D = \frac{\sum t \cdot PV(CF_t)}{\sum PV(CF_t)}$, 볼록성 $C = \frac{1}{P}\frac{d^2P}{dy^2}$, 샤프 지수 $S = \frac{E[R_p] - R_f}{\sigma_p}$,
베타 $\beta_i = \frac{\operatorname{Cov}(R_i, R_m)}{\operatorname{Var}(R_m)}$, 자본자산가격결정모형 $E[R_i] = R_f + \beta_i (E[R_m] - R_f)$,
옵션의 블랙-숄즈 $C = S_0 N(d_1) - K e^{-rT} N(d_2)$ 와 $d_{1,2} = \frac{\ln(S_0/K) + (r \pm \sigma^2/2)T}{\sigma\sqrt{T}}$,
가중평균자본비용 $WACC = \frac{E}{V} R_e + \frac{D}{V} R_d (1 - T_c)$, 고든 성장 모형 $P_0 = \frac{D_1}{r - g}$, 재고 회전율 $\frac{\text{매출원가}}{\text{평균 재고}}$,
매출채권 회전일수 $\frac{\text{매출채권}}{\text{매출}} \times 365$, 손익분기점 $Q^* = \frac{F}{p - v}$, 경제적 주문량 $EOQ = \sqrt{\frac{2DS}{H}}$. MATH_F01
같은 문단이 더 이어진다: 기하평균 $\left(\prod_{i=1}^{n} x_i\right)^{1/n}$, 조화평균 $\frac{n}{\sum_{i=1}^{n} 1/x_i}$, 이항계수 $\binom{n}{k} = \frac{n!}{k!(n-k)!}$,
포아송 $P(X=k) = \frac{\lambda^k e^{-\lambda}}{k!}$, 지수분포 $f(x) = \lambda e^{-\lambda x}$, 베이즈 정리 $P(A \mid B) = \frac{P(B \mid A)\,P(A)}{P(B)}$,
엔트로피 $H(X) = -\sum_{x} p(x) \log_2 p(x)$, 쿨백-라이블러 발산 $D_{KL}(P \parallel Q) = \sum_x P(x) \log \frac{P(x)}{Q(x)}$. MATH_F02

# 통계

정규분포의 확률밀도함수는 $f(x) = \frac{1}{\sigma\sqrt{2\pi}} e^{-\frac{(x-\mu)^2}{2\sigma^2}}$ 이다. 표본평균 $\bar{x} = \frac{1}{n}\sum_{i=1}^{n} x_i$ 의
기댓값은 $E[\bar{x}] = \mu$ 이고 분산은 $\operatorname{Var}(\bar{x}) = \frac{\sigma^2}{n}$ 이다. 중심극한정리에 따라 $n \to \infty$ 이면
$\frac{\bar{x} - \mu}{\sigma / \sqrt{n}}$ 는 표준정규분포 $N(0, 1)$ 에 가까워진다. 신뢰구간은 $\bar{x} \pm z_{\alpha/2} \frac{\sigma}{\sqrt{n}}$ 이고,
회귀계수는 $\hat{\beta} = (X^\top X)^{-1} X^\top y$ 다. 상관계수 $r = \frac{\sum (x_i - \bar{x})(y_i - \bar{y})}{\sqrt{\sum (x_i - \bar{x})^2 \sum (y_i - \bar{y})^2}}$ 는
$-1 \leq r \leq 1$ 이다. MATH_E01 그리스 문자 $\alpha, \beta, \gamma, \delta, \epsilon, \theta, \lambda, \mu, \pi, \rho, \sigma, \phi, \omega$ 와
$\Gamma, \Delta, \Theta, \Lambda, \Sigma, \Phi, \Omega$, 집합 $\mathbb{R}, \mathbb{N}, \mathbb{Z}, \mathbb{Q}, \mathbb{C}$ 도 쓴다.

극한과 적분: $\lim_{n\to\infty}\left(1 + \frac{1}{n}\right)^n = e$, $\int_0^\infty e^{-x^2}\,dx = \frac{\sqrt{\pi}}{2}$, $\oint_C \vec{F} \cdot d\vec{r}$. MATH_E02

$$
\int_{-\infty}^{\infty} \frac{1}{\sigma\sqrt{2\pi}} e^{-\frac{(x-\mu)^2}{2\sigma^2}}\, dx = 1
$$

## 그리지 못한 수식

모양을 적는 명령은 그리지 않고 원문을 둔다: $\color{red}{x}$ 와 $\Huge y$. 모르는 명령 $\foo{z}$ 도 원문이다. MATH_G01

# 부록: 블록 수식 여러 개

$$
\frac{\partial f}{\partial x} = \lim_{h \to 0} \frac{f(x+h) - f(x)}{h}
$$

$$
\nabla \cdot \vec{E} = \frac{\rho}{\varepsilon_0}, \quad \nabla \times \vec{B} = \mu_0 \vec{J} + \mu_0 \varepsilon_0 \frac{\partial \vec{E}}{\partial t}
$$

$$
\begin{bmatrix} a_{11} & a_{12} & \cdots & a_{1n} \\ a_{21} & a_{22} & \cdots & a_{2n} \\ \vdots & \vdots & \ddots & \vdots \\ a_{m1} & a_{m2} & \cdots & a_{mn} \end{bmatrix}
$$

$$
\begin{array}{|l|c|r|}
\hline
\text{항목} & \text{식} & \text{값} \\
\hline
\text{매출} & p \times q & 1{,}200 \\
\text{비용} & F + vq & 900 \\
\hline
\end{array}
$$

```math
\sum_{k=1}^{n} k^2 = \frac{n(n+1)(2n+1)}{6}
```

MATH_H01 끝.
