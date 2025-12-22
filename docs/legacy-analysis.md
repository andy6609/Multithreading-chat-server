
# Legacy System Analysis

## 1. Background

본 문서는 지인이 기존에 구현한 Java 기반 멀티스레드 채팅 시스템을 분석하는 과정에서 출발하였다.
해당 시스템을 직접 실행하고 코드 구조를 검토하며,
단순한 버그 수정이나 구현 실수 차원이 아닌
구조적·아키텍처적 한계가 존재한다는 점을 인식하게 되었다.

해당 코드는 직접 작성한 코드가 아니기 때문에 본 레포지토리에는 포함하지 않았으며,
분석 결과와 설계적 시사점을 문서 형태로 정리하였다.
이 분석은 이후 Go 언어 기반으로 채팅 서버를 재설계·재구현하는 출발점이 되었다.

---

## 2. Analysis Criteria

본 분석은 다음 기준을 중심으로 진행되었다.

- 동시 접속 환경에서의 확장성
- 서버 상태 관리 방식의 명확성
- 컴포넌트 간 결합도
- 동시성 모델의 안전성
- 테스트 및 유지보수 가능성

단순한 리팩토링이나 일부 코드 수정으로 해결 가능한 문제는
분석 대상에서 제외하였다.

---

## 3. Structural Limitations Identified
### 3.1 Shared Mutable State for Server State Management

**Observation**  
서버는 접속 중인 클라이언트 목록과 사용자 정보 등을
공유 자료구조에 저장하고,
락 기반 동기화를 통해 이를 보호하는 방식으로 상태를 관리하고 있었다.

**Structural Limitation**  
공유 메모리 기반 상태 관리는 락을 통해 동기화될 수밖에 없으며,
이로 인해 lock contention, critical section 확대,
그리고 상황에 따라 deadlock 가능성이 구조적으로 내재된다.
또한 상태 변경이 여러 코드 경로에 분산되어 있어
state transition의 추적이 어렵다는 문제가 있다.


**Why It Matters**  
공유 상태 중심 설계는 동시 접속자가 증가할수록
확장성과 안정성을 동시에 저해한다.
또한 동시성 오류가 발생했을 때
원인을 추적하고 수정하는 비용이 크게 증가한다.


---

### 3.2 Blocking I/O with Thread-per-Connection Model

**Observation**  
각 클라이언트 연결은 thread-per-connection 모델로 처리되며,
입력은 blocking I/O 방식으로 수행되고 있었다.


**Structural Limitation**  
클라이언트 입력(블로킹 I/O)는 입력 대기 동안 스레드를 점유하게 하며,
이는 곧 idle thread 증가와 리소스 낭비로 이어진다.
동시 접속 수가 증가할수록 OS 스레드 수와 메모리 사용량이 선형적으로 증가한다.
스레드 수와 메모리 사용량이 선형적으로 증가한다.

**Why It Matters**  
이 모델은 소규모 환경에서는 단순하지만,
동시 접속 규모가 커질수록 구조적으로 확장에 한계가 있다.
이는 단순한 성능 문제가 아니라,
시스템이 감당할 수 있는 사용자 수 자체를 제한한다.

---

### 3.3 Direct Method Invocation Across Concurrency Boundaries

**Observation**  
서버 내부에서 메시지 전달이나 브로드캐스트 과정이
다른 컴포넌트의 메서드를 직접 호출하는 방식으로 이루어져 있었다.

**Structural Limitation**  
이 방식은 스레드 경계(concurrency boundary)를 명확히 분리하지 못하고,
동기화 책임이 호출자와 피호출자 사이에 분산된다.
결과적으로 tight coupling과 implicit synchronization dependency가 발생하고 
시스템 전체의 동작 흐름을 이해하기 어려워진다.

**Why It Matters**  
컴포넌트 간 직접 호출이 많아질수록
결합도가 증가하고,
동시성 관련 버그가 발생할 가능성이 높아진다.
이는 기능 확장과 유지보수를 어렵게 만든다.
메시지 전달을 message passing 모델로 분리할 필요성이 있다.

---

### 3.4 Lock-based Synchronization as Primary Concurrency Control

**Observation**  
서버의 핵심 로직은 락 기반 동기화에 의존하여
동시 접근을 제어하고 있었다.

**Structural Limitation**  
Lock-based synchronization(락 기반 구조)에서는 동시 접근이 많아질수록
경합과 대기 시간이 증가한다.
또한 락 획득 순서나 범위를 잘못 설계할 경우
데드락과 같은 문제가 발생할 가능성이 있다.

**Why It Matters**  
락 중심 설계는 규모가 커질수록
성능과 안정성 모두에 부담이 된다.
특히 동시성 구조를 확장하거나 변경할 때
위험 요소가 빠르게 증가한다.

---

### 3.5 Centralized Coordinator with Multiple Responsibilities

**Observation**  
연결 관리, 사용자 목록 관리, 메시지 라우팅 등
여러 책임이 하나의 중심 컴포넌트에 집중되어 있었다.

**Structural Limitation**  
단일 컴포넌트가 많은 책임을 가지게 되면
병목 지점이 생기기 쉽고,
변경 하나가 시스템 전체에 영향을 미치게 된다.

**Why It Matters**  
책임이 분리되지 않은 구조는
확장성과 테스트 가능성을 동시에 저해한다.
장기적으로 유지보수 비용이 크게 증가할 수 있다.

---

## 4. Design Implications

위에서 정리한 한계점들은
일부 로직 수정이나 리팩토링으로 해결하기 어렵다고 판단했다.
특히 동시성 처리와 서버 상태 관리 측면에서는
아키텍처 수준의 재설계가 필요하다고 결론지었다.

---

## 5. Conclusion

본 분석은 기존 Java 기반 채팅 시스템의 구조적 한계를 정리한 것이다.
이 문서는 이후 Go 언어 기반 서버를 설계하는 과정에서
기술 선택과 아키텍처 결정을 위한 근거로 활용되었다.

## Summary of Structural Differences

The legacy system is fundamentally based on
shared mutable state, blocking I/O, and lock-based synchronization.

In contrast, the Go re-design adopts
message passing, goroutine-based concurrency,
and explicit separation of concurrency boundaries.

These differences represent architectural shifts,
not issues solvable by minor refactoring.

