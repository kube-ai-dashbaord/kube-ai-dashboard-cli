# Skills - Reference Repository Patterns

> kube-ai-dashboard-cli 개발에 활용할 수 있는 참조 리포지토리 패턴 모음

## Overview

이 디렉토리는 k9s, kubectl-ai, Headlamp, Kubernetes Dashboard에서 배울 수 있는
핵심 아키텍처 패턴과 베스트 프랙티스를 정리한 Skill 문서들을 포함합니다.

---

## Skills 목록

### 1. [k9s Patterns](./k9s-patterns.md)

**학습 포인트**: TUI 아키텍처, 사용자 경험

| 패턴 | 설명 | 활용 |
|------|------|------|
| MVC 3계층 | Model-Render-View 분리 | 리소스 뷰 구조화 |
| Observer 패턴 | Listener 기반 이벤트 | 컴포넌트 간 통신 |
| Stack 네비게이션 | Pages + Stack | 뷰 히스토리 관리 |
| Action 시스템 | KeyActions | 키바인딩 관리 |
| Plugin/HotKey | Scope 기반 플러그인 | 확장성 |
| Skin 시스템 | 계층적 스타일 | 테마 커스터마이징 |
| XDG 설정 | 멀티레벨 설정 | 설정 파일 관리 |
| FishBuff | 자동완성 버퍼 | 명령어 제안 |

**우선순위**: TUI 개발 시 가장 먼저 참조

---

### 2. [kubectl-ai Patterns](./kubectl-ai-patterns.md)

**학습 포인트**: AI 에이전트 설계, 안전한 명령 실행

| 패턴 | 설명 | 활용 |
|------|------|------|
| Agent Loop | State Machine | AI 상태 관리 |
| Tool System | Plugin Architecture | 도구 등록/실행 |
| LLM Provider | Provider-Agnostic | 다중 LLM 지원 |
| MCP 통합 | Adapter Pattern | 외부 도구 연동 |
| Safety Layers | Defense in Depth | 명령 실행 안전성 |
| Session 관리 | Pluggable Store | 대화 영속성 |
| Prompt Engineering | Dual-Mode | 프롬프트 템플릿 |

**우선순위**: AI 기능 개발 시 필수 참조

---

### 3. [Headlamp Patterns](./headlamp-patterns.md)

**학습 포인트**: Plugin 시스템, 엔터프라이즈 기능

| 패턴 | 설명 | 활용 |
|------|------|------|
| Plugin Registry | Registration API | 플러그인 확장점 |
| Multi-Cluster | Multiplexer | 클러스터 전환 |
| Response Cache | Authorization-Aware | API 캐싱 |
| OIDC 인증 | Token Flow | 엔터프라이즈 인증 |
| i18n 시스템 | Dual-System | 다국어 지원 |
| Processor 패턴 | Pipeline | 데이터 변환 |
| WS Multiplexing | Single Connection | 실시간 업데이트 |

**우선순위**: 확장성/엔터프라이즈 기능 개발 시 참조

---

### 4. [Kubernetes Dashboard Patterns](./kubernetes-dashboard-patterns.md)

**학습 포인트**: API 설계, 데이터 처리

| 패턴 | 설명 | 활용 |
|------|------|------|
| Multi-Module | Microservices | 모듈 분리 |
| DataSelector | Generic Processing | 필터/정렬/페이지네이션 |
| Init Registration | Decentralized Routes | 라우트 분산 등록 |
| Request-Scoped | Per-Request Client | 권한 관리 |
| Metrics Integration | Sidecar Scraper | 메트릭 수집 |
| Cache Pattern | Cache-and-Network | 성능 최적화 |
| CSRF 보호 | Dual Framework | 보안 |

**우선순위**: 백엔드/API 설계 시 참조

---

## 패턴 적용 가이드

### 기능별 참조 맵

```
┌─────────────────────────────────────────────────────────────┐
│                    kube-ai-dashboard-cli                     │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │     TUI      │    │      AI      │    │   Backend    │  │
│  │              │    │   Assistant  │    │              │  │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘  │
│         │                   │                   │          │
│         ▼                   ▼                   ▼          │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │  k9s         │    │  kubectl-ai  │    │  Dashboard   │  │
│  │  Patterns    │    │  Patterns    │    │  Patterns    │  │
│  └──────────────┘    └──────────────┘    └──────────────┘  │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐│
│  │                   Headlamp Patterns                     ││
│  │             (Plugin, i18n, Multi-Cluster)              ││
│  └────────────────────────────────────────────────────────┘│
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### 개발 단계별 참조

#### Phase 1: 기본 TUI
```
참조: k9s-patterns.md
- MVC 3계층 아키텍처 적용
- Action 시스템으로 키바인딩 관리
- Stack 네비게이션 구현
```

#### Phase 2: AI 통합
```
참조: kubectl-ai-patterns.md
- Tool Interface 정의
- Agent Loop 구현
- Safety Layers 적용
```

#### Phase 3: 데이터 처리
```
참조: kubernetes-dashboard-patterns.md
- DataSelector 패턴 적용
- Request-Scoped Client 구현
- 메트릭 통합
```

#### Phase 4: 확장성
```
참조: headlamp-patterns.md
- Plugin Registry 구현
- i18n 시스템 통합
- Multi-Cluster 지원
```

---

## Quick Reference 테이블

### 패턴별 파일 위치

| 패턴 | k9s | kubectl-ai | Headlamp | Dashboard |
|------|-----|------------|----------|-----------|
| Model/View | `internal/model/` | - | - | - |
| Render | `internal/render/` | - | - | - |
| Action | `internal/ui/action.go` | - | - | - |
| Tool | - | `pkg/tools/` | - | - |
| Agent | - | `pkg/agent/` | - | - |
| Plugin | `internal/config/plugin.go` | - | `frontend/src/plugin/` | - |
| Cache | - | - | `backend/pkg/k8cache/` | `modules/common/` |
| i18n | - | - | `frontend/src/i18n/` | - |
| DataSelect | - | - | - | `modules/api/pkg/resource/dataselect/` |

### 핵심 인터페이스

```go
// k9s: Listener Pattern
type TableListener interface {
    TableDataChanged(*TableData)
}

// kubectl-ai: Tool Interface
type Tool interface {
    Run(ctx context.Context, args map[string]any) (any, error)
    CheckModifiesResource(args map[string]any) string
}

// Headlamp: Plugin Registry
type Registry interface {
    RegisterSidebarEntry(entry SidebarEntry)
    RegisterRoute(route Route)
}

// Dashboard: DataCell Interface
type DataCell interface {
    GetProperty(PropertyName) ComparableValue
}
```

---

## 사용 방법

### AI Agent에서 참조

```
사용자: "새로운 리소스 뷰를 추가하고 싶어"

에이전트 응답:
k9s-patterns.md를 참조하여 MVC 패턴 적용:
1. Model Layer에 데이터 구조 정의
2. Render Layer에 렌더러 구현
3. View Layer에 UI 컴포넌트 생성
```

### 코드 리뷰에서 활용

```
PR 리뷰 시:
- TUI 코드: k9s-patterns.md의 Observer 패턴 준수 여부
- AI 코드: kubectl-ai-patterns.md의 Safety Layers 적용 여부
- API 코드: kubernetes-dashboard-patterns.md의 DataSelector 패턴 준수 여부
```

---

## 업데이트 가이드

새로운 패턴 발견 시:

1. 해당 Skill 파일에 섹션 추가
2. README.md 테이블 업데이트
3. CLAUDE.md 참조 섹션 업데이트

---

## 참조 리포지토리

| 프로젝트 | URL | 버전 |
|----------|-----|------|
| k9s | https://github.com/derailed/k9s | latest |
| kubectl-ai | https://github.com/GoogleCloudPlatform/kubectl-ai | v0.0.19 |
| Headlamp | https://github.com/headlamp-k8s/headlamp | latest |
| Kubernetes Dashboard | https://github.com/kubernetes/dashboard | latest |
