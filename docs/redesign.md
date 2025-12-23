Redesign Goals in Go

위 한계들을 해결하기 위해, 본 프로젝트는 Go 언어의 동시성 모델을 활용하여 다음 방향으로 재설계합니다.


	•	상태 변경을 “이벤트 흐름”으로 명시화: 공유 상태를 직접 건드리기보다 메시지(이벤트) 기반으로 상태 변경 경로를 단순화
	•	goroutine 기반 동시성으로 확장성 확보: 연결 처리의 경량화를 통해 높은 동시 접속 환경을 구조적으로 수용
	•	프로토콜/라우팅/상태 관리의 책임 분리: 컴포넌트 경계를 명확히 하여 결합도를 낮추고 테스트 가능성을 높임
	•	단계적 개발 로드맵: (1) Go 서버 → (2) 콘솔 클라이언트 검증 → (3) 구조 이해/정리 → (4) GUI 확장


# System Redesign in Go

## 1. Purpose of the Redesign

This document describes the architectural redesign of the chat system
based on the limitations identified in the legacy Java implementation.

Rather than performing a direct language port,
the system is redesigned with a focus on:

- explicit concurrency boundaries
- message-driven state transitions
- reduced shared mutable state
- improved scalability and testability

Go was chosen as the implementation language
because its concurrency primitives naturally support this design.

---

## 2. High-Level Architectural Shift

### From Shared-State Concurrency to Message Passing

The legacy system relied on shared mutable state protected by locks.
In contrast, the redesigned system adopts a **message-passing concurrency model**.

Key conceptual shifts include:

- Shared memory → Explicit message flow
- Lock-based synchronization → Serialized state ownership
- Implicit concurrency boundaries → Explicit goroutine ownership
- Centralized coordination → Responsibility-based components

This redesign treats concurrency as a first-class design concern,
not an implementation detail.

---

## 3. Core Design Principles

### 3.1 Explicit Ownership of State

Each mutable state in the system has a **single logical owner**.
State is never modified directly by multiple concurrent entities.

All state transitions occur as a result of processing messages,
ensuring predictable and traceable behavior.

---

### 3.2 Message-Driven Communication

Components communicate exclusively through message passing.
No component directly invokes another component’s internal logic.

This reduces coupling and makes concurrency boundaries explicit.

---

### 3.3 Separation of Responsibilities

The system is decomposed into components with clear, narrow responsibilities.
No single component is responsible for connection handling,
state management, and message routing simultaneously.

---

## 4. Proposed Component Architecture

The redesigned system is composed of the following logical components.

---

### 4.1 Connection Acceptor

**Responsibility**
- Accept incoming client connections
- Perform minimal initialization
- Delegate connection handling to independent workers

**Concurrency Model**
- Each connection is handled independently
- No shared state access

---

### 4.2 Client Session Handler

**Responsibility**
- Manage the lifecycle of a single client session
- Parse incoming messages
- Forward messages to the routing layer

**Concurrency Model**
- One concurrent execution context per client
- No direct access to global server state

---

### 4.3 Message Router

**Responsibility**
- Route messages based on type and destination
- Distinguish between broadcast and point-to-point messages
- Apply routing rules without modifying server state

**Concurrency Model**
- Message-driven
- Stateless with respect to client registry

---

### 4.4 Client Registry (State Owner)

**Responsibility**
- Maintain authoritative client metadata
- Handle join/leave events
- Publish user list updates

**Concurrency Model**
- Single owner of client-related state
- All state transitions processed sequentially

**Design Rationale**
This component replaces shared mutable state with
serialized state ownership,
eliminating the need for locks.

---

### 4.5 Outbound Dispatcher

**Responsibility**
- Deliver outgoing messages to clients
- Apply backpressure and failure handling
- Prevent slow clients from affecting the system

**Concurrency Model**
- Asynchronous delivery
- Isolation between client send paths

---

## 5. Message Flow Overview

A typical message flow follows this sequence:

1. Client sends a message
2. Client Session Handler parses input
3. Message is forwarded to the Message Router
4. Router determines routing strategy
5. Client Registry is consulted if necessary
6. Outbound Dispatcher delivers messages

At no point does a component directly mutate
another component’s internal state.

---

## 6. Concurrency and Scalability Considerations

### 6.1 Reduced Lock Contention

By avoiding shared mutable state,
the system eliminates lock contention as a scalability bottleneck.

### 6.2 Backpressure Awareness (Implemented)

Outbound delivery is decoupled from message production using a per-client outbound channel and a dedicated writer loop.

To prevent slow clients from blocking system progress, outbound sends are bounded and non-blocking at the routing layer.
This ensures that the Registry event loop remains responsive even under uneven client consumption rates.

### 6.3 Failure Isolation (Implemented)

Each client session runs independently, and outbound delivery is isolated per client.
A slow or stalled client affects only its own send path and does not stall shared state progression in the Registry.

---

## 7. Testability Improvements

The redesign improves testability by:

- Enabling component-level testing
- Allowing deterministic state transition testing
- Making concurrency behavior observable through message flows

Each component can be tested independently
without requiring a running GUI client.

---
## 8. Implementation Status & Verification

This redesign is not only documented conceptually; key design claims are validated in code.

### 8.1 Registry-Centered State Transition Tests

The system includes unit tests that validate deterministic state transitions in the Registry (single-owner state component):

- **Duplicate username rejection**: Registering the same username twice returns an explicit error.
- **User list consistency (`/users`)**: Join/leave transitions are reflected in the user list output.
- **Direct message routing (`/w`)**:
  - Successful routing delivers only to the intended receiver.
  - Unknown receivers produce `ERR user_not_found`.
  - Self-whisper is rejected with `ERR cannot_whisper_self`.

These tests ensure the “single-writer state ownership + message-driven transitions” principle is enforced in practice.

### 8.2 Concurrency Validation (`go test -race`)

Since concurrency is a first-class concern in this design, the code is validated using the Go race detector:

- `go test -race` is used to detect unintended shared-state access and data races.
- This provides an empirical confidence layer beyond manual testing.

### 8.3 Test-Friendly Lifecycle Control (Stop/Wait)

The Registry run loop supports explicit shutdown semantics (Stop/Wait), enabling reliable testing and preventing goroutine leaks:

- The Registry can be stopped without closing shared channels unsafely.
- Tests can call `Stop()` and `Wait()` to ensure deterministic teardown.
- This avoids failure modes such as `send on closed channel` and improves long-term maintainability.
---
## 9. Summary

This redesign represents an architectural shift rather than a refactor.

By moving from shared mutable state and lock-based synchronization
to a message-driven, ownership-based concurrency model,
the system becomes more scalable, maintainable, and robust.

Go is not chosen for performance alone,
but because its concurrency model enables this design
to be expressed clearly and correctly.

## 10. Planned Work (Non-MVP)

The following items are intentionally out of scope for the current MVP, but are natural extensions of the architecture:

- **Explicit Router component extraction** (currently routing may be co-located with the Registry for simplicity)
- **Server-level graceful shutdown** (listener close + coordinated component teardown)
- **Observability**: structured logging, metrics, profiling (pprof)
- **Rooms**: join/leave room, room broadcast

