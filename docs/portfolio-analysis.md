# 포트폴리오 기술 분석 문서 — Go Chat Server

---

## 1. 프로젝트 5줄 요약

Java로 작성된 멀티스레드 채팅 서버의 구조적 한계(공유 상태 + 락 기반 동기화)를 분석하고, 단순 리팩토링이 아닌 **아키텍처 수준의 재설계**를 선택했다.
Go의 동시성 모델(goroutine + channel)을 활용해 "공유 메모리 → 메시지 패싱, 락 → 단일 소유권"으로 전환하는 TCP 채팅 서버를 처음부터 새로 구현했다.
클라이언트당 2개의 goroutine(읽기/쓰기 분리), 상태 변경을 단일 goroutine(Registry)이 독점적으로 처리하는 구조로 데이터 레이스를 락 없이 원천 차단했다.
Prometheus + Grafana 기반 실시간 모니터링, Docker Compose 기반 풀 스택 실행 환경, GitHub Actions CI 파이프라인을 갖추고 있다.
`go test -race`로 레이스 컨디션 검증, 상태 전이 단위 테스트로 설계 원칙이 코드 수준에서도 유효함을 입증했다.

---

## 2. 핵심 기술 스택

| 분류 | 기술 |
|------|------|
| **언어** | Go 1.24 |
| **동시성 모델** | goroutine, channel, select |
| **네트워크** | TCP (`net` 표준 라이브러리) |
| **모니터링** | Prometheus (`client_golang`), Grafana |
| **인프라** | Docker, Docker Compose |
| **CI/CD** | GitHub Actions (단위 테스트, 레이스 감지, 이미지 빌드) |
| **로깅** | Go 표준 `log/slog` (구조화 로그) |
| **테스트** | Go 표준 `testing` + `-race` 플래그 |

---

## 3. 내가 담당한 주요 역할 / 구현한 기능

이 프로젝트는 1인 개발로, 설계·구현·인프라·문서화 전체를 담당했다.

### 아키텍처 설계
- Java 레거시 시스템 구조 분석 → 5가지 구조적 한계 식별 및 문서화 (`docs/legacy-analysis.md`)
- 재설계 원칙 도출 및 컴포넌트 아키텍처 설계 (`docs/redesign.md`)

### 핵심 동시성 구현
- **Registry (단일 소유권 상태 관리)**: 모든 클라이언트 상태를 하나의 goroutine이 이벤트 채널을 통해서만 변경. 락 없이 동시성 안전성 보장
- **HandleSession (클라이언트 세션 처리)**: 클라이언트 1개당 읽기 goroutine 1개. 입력 파싱 후 이벤트 채널로 전달
- **OutboundWriter (비동기 쓰기)**: 클라이언트 1개당 쓰기 goroutine 1개. 버퍼 채널로 Registry와 분리
- **Backpressure**: 느린 클라이언트에 대해 non-blocking send → 메시지 드롭으로 서버 전체 블로킹 방지

### 채팅 기능
- 사용자 이름 등록 (중복 거부, 길이 제한)
- 브로드캐스트 메시지
- 귓속말 (`/w <username> <message>`) — 자기 자신 발송 거부, 미존재 사용자 에러 처리
- 접속자 목록 (`/users`)
- `/exit` 명령 및 연결 끊김 처리
- SIGTERM/SIGINT Graceful Shutdown

### 모니터링 및 인프라
- Prometheus 메트릭 3종 정의: 현재 접속자 수(gauge), 메시지 유형별 카운터(counter), 이벤트 처리 지연(histogram)
- Grafana 대시보드 자동 프로비저닝 (`docker-compose up` 한 명령으로 전체 스택 실행)
- 멀티스테이지 Dockerfile, GitHub Actions CI 파이프라인 구성

### 테스트
- Registry 상태 전이 단위 테스트: 중복 사용자명 거부, 접속자 목록 일관성, 귓속말 라우팅(성공/미존재 사용자/자기 자신)
- `go test -race` 전 테스트 통과 (레이스 컨디션 없음 검증)
- `Stop()`/`Wait()` 기반 goroutine 누수 없는 테스트 생명주기 설계

---

## 4. 기술적으로 어려웠던 부분과 해결 방법

### 문제 1 — 락 없이 공유 상태를 어떻게 안전하게 관리할 것인가

**어려웠던 점**: 여러 클라이언트 goroutine이 동시에 접속자 목록(map)을 읽고 쓰면 데이터 레이스가 발생한다. 일반적인 해법은 `sync.RWMutex`지만, 이는 레거시 Java 시스템이 가진 구조적 문제(lock contention, 상태 변경 경로 분산)를 그대로 답습하는 것이다.

**해결 방법**: Registry 패턴 도입. 클라이언트 map을 오직 `Registry.Run()` goroutine만 접근할 수 있게 제한하고, 모든 상태 변경 요청은 이벤트 채널(`chan Event`)을 통해서만 전달하도록 설계했다. 어떤 goroutine도 map에 직접 접근하지 않으므로 락이 필요 없다.

```
클라이언트 goroutine → events 채널 → Registry.Run() 단독 처리
```

### 문제 2 — 느린 클라이언트가 서버 전체를 멈추는 문제 (Backpressure)

**어려웠던 점**: Registry가 메시지를 브로드캐스트할 때, 느린 클라이언트의 채널이 꽉 차면 `c.Out <- line` 이 블로킹되어 Registry 이벤트 루프 전체가 멈춘다.

**해결 방법**: `sendLine()` 함수에서 `select`의 `default` 케이스를 활용해 non-blocking send로 구현했다. 채널이 꽉 찬 클라이언트에 대해서는 해당 메시지를 드롭하고 Registry 루프는 즉시 다음 이벤트로 진행한다.

```go
select {
case c.Out <- line:
default:
    // slow client: drop message, do not block registry
}
```

### 문제 3 — 테스트에서 goroutine 누수 방지

**어려웠던 점**: `Registry.Run()`은 무한 루프를 도는 goroutine이다. 테스트에서 이를 제대로 종료하지 않으면 goroutine 누수가 발생하고, 채널이 이미 닫힌 상태에서 전송 시 패닉이 발생할 수 있다.

**해결 방법**: `stopCh`(종료 신호)와 `doneCh`(종료 완료 확인)를 분리해 `Stop()` / `Wait()` API를 설계했다. 테스트에서는 `defer reg.Stop(); reg.Wait()`로 goroutine의 완전한 종료를 보장한 뒤 다음 단계로 진행한다.

---

## 5. 이 프로젝트에서 드러나는 나의 관심사나 강점

**시스템 사고**: 버그를 고치는 것보다, 버그가 반복될 수밖에 없는 구조적 원인을 먼저 분석한다. 레거시 Java 코드를 보고 "코드 수정"이 아닌 "아키텍처 재설계"가 필요하다고 판단하고 그 근거를 문서화한 것이 이를 보여준다.

**동시성에 대한 진지한 관심**: 단순히 "동작하는 서버"를 만드는 것이 아니라, 데이터 레이스를 구조적으로 불가능하게 만드는 설계를 추구한다. `go test -race`를 CI에 포함시킨 것은 동시성 정확성을 검증 가능한 단계까지 끌어올리려는 태도를 보여준다.

**설계와 구현의 일관성**: `docs/redesign.md`에서 세운 설계 원칙("단일 소유권", "메시지 패싱", "명시적 동시성 경계")이 실제 코드(`registry.go`, `session.go`)에 그대로 반영되어 있다. 문서와 코드가 괴리되지 않는다.

**관찰 가능성(Observability) 중시**: 채팅 기능 외에도 Prometheus 메트릭(접속자, 메시지 처리량, p50/p95/p99 레이턴시), Grafana 대시보드를 직접 구성한 것은 운영 관점까지 고려하는 시야를 보여준다.

**문서화 능력**: 기술적 의사결정 과정을 `legacy-analysis.md`, `redesign.md` 형태로 남겼다. 코드만이 아닌 "왜 이렇게 만들었는가"를 설명할 수 있다.

---

## 6. 프로젝트 규모

| 항목 | 내용 |
|------|------|
| **팀 규모** | 1인 (설계 · 구현 · 인프라 · 문서화 전담) |
| **개발 언어** | Go 1.24 |
| **핵심 소스 파일** | 7개 (`server.go`, `session.go`, `registry.go`, `writer.go`, `metrics.go`, `types.go`, `registry_test.go`) |
| **외부 의존성** | Prometheus client_golang 1개 (모니터링용), 나머지 전부 Go 표준 라이브러리 |
| **인프라 구성** | 3개 컨테이너 (chat-server, Prometheus, Grafana) |
| **CI 파이프라인** | 3단계 (단위 테스트 → 레이스 감지 → Docker 빌드) |
| **사용자 수** | 포트폴리오/학습 프로젝트 (실서비스 미배포) |
| **출발점** | Java 레거시 시스템 구조 분석 → Go 재설계·재구현 |
