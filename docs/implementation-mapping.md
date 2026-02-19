# 설계 원칙 → 코드 구현 매핑

이 문서는 `docs/redesign.md`와 `docs/legacy-analysis.md`에 정의된 설계 원칙이 실제 코드에서 어떻게 구현되었는지를 매핑합니다.

## 핵심 설계 원칙 구현

| 설계 원칙 (docs/redesign.md) | 레거시 문제점 (docs/legacy-analysis.md) | 구현 위치 | 코드 증거 |
|---|---|---|---|
| **3.1 Explicit Ownership of State**<br/>단일 논리적 소유자, 상태는 메시지 처리 결과로만 변경 | 3.1 Shared Mutable State<br/>공유 메모리 + 락 기반 동기화 | `internal/chat/registry.go`<br/>`Registry.Run()` | ```go<br/>func (r *Registry) Run() {<br/>  clients := make(map[string]*Client)<br/>  // 이 goroutine 안에서만 clients 수정<br/>  for ev := range r.events { ... }<br/>}``` |
| **3.2 Message-Driven Communication**<br/>컴포넌트 간 통신은 메시지 전달만 | 3.3 Direct Method Invocation<br/>스레드 경계 넘나드는 직접 호출 | `internal/chat/session.go`<br/>`HandleSession()` | ```go<br/>events <- Event{<br/>  Type: EventWhisper,<br/>  Client: c,<br/>  To: "bob",<br/>  Text: "hello"<br/>}<br/>// 상태 직접 접근 없음``` |
| **3.3 Separation of Responsibilities**<br/>명확하고 좁은 책임의 컴포넌트 | 3.5 Centralized Coordinator<br/>단일 컴포넌트에 여러 책임 집중 | `internal/chat/` 패키지 전체 | - `server.go`: 연결 수락만<br/>- `session.go`: 파싱 + 이벤트 발행만<br/>- `registry.go`: 상태 관리 + 라우팅 결정만<br/>- `writer.go`: 송신만 |

## 컴포넌트 아키텍처 구현

| 컴포넌트 (docs/redesign.md Section 4) | 책임 | 구현 위치 | 구현 방식 |
|---|---|---|---|
| **4.1 Connection Acceptor** | 연결 수락, 최소 초기화, 독립 worker에 위임 | `internal/chat/server.go`<br/>`acceptLoop()` | ```go<br/>for {<br/>  conn, _ := ln.Accept()<br/>  c := &Client{Conn: conn, ...}<br/>  go HandleSession(c, ...)<br/>}``` |
| **4.2 Client Session Handler** | 단일 세션 생명주기, 메시지 파싱, 라우팅 레이어로 전달 | `internal/chat/session.go`<br/>`HandleSession()` | ```go<br/>// username 핸드셰이크<br/>// 입력 파싱 (/users, /w, /exit)<br/>events <- Event{Type: ...}<br/>// 전역 상태 직접 접근 없음``` |
| **4.3 Message Router** | 메시지 타입/목적지 기반 라우팅, 상태 수정 없이 규칙 적용 | `internal/chat/registry.go`<br/>`handleWhisper()`, `handleBroadcast()` | 현재는 Registry에 통합되어 있으나, 라우팅 로직은 명확히 분리됨 |
| **4.4 Client Registry (State Owner)** | 클라이언트 메타데이터 유지, join/leave 처리, 단일 소유자 | `internal/chat/registry.go`<br/>`Registry.Run()` | ```go<br/>clients := make(map[string]*Client)<br/>// 단일 goroutine에서만 접근<br/>// 모든 상태 변경은 이벤트 처리 결과``` |
| **4.5 Outbound Dispatcher** | 클라이언트로 메시지 전달, backpressure 처리, 느린 클라 격리 | `internal/chat/writer.go`<br/>`StartOutboundWriter()` | ```go<br/>// 클라이언트별 goroutine<br/>for msg := range out { ... }<br/>// 느린 클라가 Registry를 막지 않음``` |

## 동시성 및 확장성 고려사항

| 고려사항 (docs/redesign.md Section 6) | 레거시 문제 | 구현 증거 |
|---|---|---|
| **6.1 Reduced Lock Contention**<br/>공유 상태 회피로 락 경합 제거 | 3.4 Lock-based Synchronization<br/>락 기반 동기화 의존 | `registry.go`에 `sync.Mutex` 없음<br/>단일 goroutine에서만 상태 수정 |
| **6.2 Backpressure Awareness**<br/>송신과 생산 분리, 느린 클라 처리 | 3.2 Blocking I/O<br/>스레드당 블로킹 I/O | ```go<br/>// registry.go sendLine()<br/>select {<br/>case c.Out <- line:<br/>default: // Drop when slow<br/>}<br/>// writer.go: 별도 goroutine``` |
| **6.3 Failure Isolation**<br/>세션별 격리, 실패 전파 방지 | - | `session.go`: 각 세션은 독립 goroutine<br/>한 세션 오류가 다른 세션에 영향 없음 |

## 테스트 가능성 개선

| 개선 사항 (docs/redesign.md Section 7) | 구현 위치 | 증거 |
|---|---|---|
| **컴포넌트 레벨 테스트** | `internal/chat/registry_test.go` | Registry만 독립적으로 테스트 가능 |
| **결정적 상태 전이 테스트** | `TestRegistry_RegisterRejectsDuplicateUsername`<br/>`TestRegistry_UsersReflectJoinLeave` | 이벤트 기반으로 상태 변경 예측 가능 |
| **동시성 동작 관찰 가능** | `go test -race ./...` | 메시지 흐름을 통해 동시성 검증 |

## 레거시 → Go 변환 요약

| 레거시 Java 패턴 | Go 구현 패턴 | 코드 위치 |
|---|---|---|
| `HashMap<String, ClientHandler> clients`<br/>`synchronized void addClient(...)` | `clients := make(map[string]*Client)`<br/>단일 goroutine에서만 수정 | `registry.go:42` |
| `ClientHandler` → `ConnectionHandler` 직접 호출 | `events <- Event{...}` 메시지 전달 | `session.go:32-37` |
| Thread-per-connection<br/>블로킹 I/O | Goroutine-per-connection<br/>논블로킹 채널 통신 | `server.go:48`<br/>`writer.go:8-20` |
| 중앙집중식 상태 관리 | 단일 소유자 + 이벤트 기반 상태 변경 | `registry.go:39-70` |

## 추가 구현 (docs에 없는 업그레이드)

| 기능 | 구현 위치 | 설명 |
|---|---|---|
| **Stop/Wait 패턴** | `registry.go:29-37` | 테스트 안전 종료, goroutine leak 방지 |
| **Whisper 프로토콜** | `session.go:11, 70-85`<br/>`registry.go:124-154` | `/w <user> <text>` + 에러 처리 |
| **Username UX** | `session.go:23-49`<br/>`registry.go:45-77` | 재입력 루프, 에러 메시지 명시 |
| **Race detector 테스트** | `registry_test.go` | `go test -race` 통과 |

## 메시지 흐름 예시: Whisper

```
[클라이언트] "/w bob hello"
    ↓
[session.go] 파싱 → Event{Type: EventWhisper, To: "bob", Text: "hello"}
    ↓
[registry.go] clients map 확인 (단일 소유자)
    ↓
[registry.go] 대상 존재 여부 확인 → sendLine(receiver, "WHISPER ...")
    ↓
[writer.go] receiver.Out 채널로 전달 (별도 goroutine)
    ↓
[클라이언트] "WHISPER alice: hello"
```

이 흐름에서 **어떤 컴포넌트도 다른 컴포넌트의 내부 상태를 직접 수정하지 않음**.




