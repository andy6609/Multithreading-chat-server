

## Background

This project started from analyzing an existing Java-based multithreaded chat system
implemented by an acquaintance.
While running and reviewing the system, I identified architectural limitations that
cannot be addressed through simple bug fixes or minor refactoring.

Since the original code was not authored by me, it is not included in this repository.
Instead, the analysis of the legacy system and the identified structural limitations
are documented separately in `docs/legacy-analysis.md`.

The goal of this project is not to partially modify or port the existing implementation,
but to clearly define its architectural constraints and redesign the system from the ground up.
Based on considerations around concurrency, scalability, and maintainability,
the chat server is re-architected and re-implemented using Go.

----------------------------------------------------------------------------------------
이 프로젝트는 지인이 기존에 구현한 Java 기반 멀티스레드 채팅 시스템을 분석하는 과정에서 출발함
해당 코드를 직접 사용·실행·분석해보며, 단순한 버그 수정이나 구현 실수 차원으로는 해결되지 않는 구조적·아키텍처적 한계가 존재한다는 점을 인식하게 되었고, 이를 개선하는 방향으로 프로젝트를 재구성함

기존 코드는 직접 작성한 것이 아니므로 본 레포지토리에는 포함하지 않았으며, 분석 과정과 구조적 한계에 대한 정리는 별도의 문서(docs/legacy-analysis.md)로 기록함. 본 프로젝트의 목적은 기존 코드를 그대로 유지하거나 부분적으로 수정하는 데 있지 않고, 
기존 구조의 한계를 명확히 정의한 뒤 동시성·확장성·유지보수성 관점에서 더 적합한 설계를 고민하여, 이를 Go 언어 기반으로 재설계·재구현하는 데 있음.

## 아키텍처 개요 (WIP)

본 시스템은 동시성 경계를 명확히 하고, 공유 상태 대신 메시지 전달(message passing)을 중심으로 설계함.

[클라이언트]
  -> (TCP 연결)
  -> 연결 수락기(Connection Acceptor)
  -> 세션 핸들러(Client Session Handler, 연결당 1개)
  -> 메시지 라우터(Message Router)
  -> 클라이언트 레지스트리(Client Registry, 상태 단일 소유자)
  -> 송신 디스패처(Outbound Dispatcher, 클라이언트별 송신 경로 분리)
  -> [클라이언트]

모든 상태 변경은 공유 메모리 접근이 아니라 메시지 흐름을 통해 수행됨.
