# Graph Report - Gova  (2026-10-31)

## Corpus Check
- Corpus is ~35,479 words - fits in a single context window. You may not need a graph.

## Summary
- 472 nodes · 1165 edges · 17 communities (13 shown, 4 thin omitted)
- Extraction: 81% EXTRACTED · 19% INFERRED · 0% AMBIGUOUS · INFERRED: 218 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- REST API Example Service
- Application Facade & Lifecycle Hooks
- Dependency Injection Engine
- Injel CLI & Scaffolding Tooling
- Environment Configuration Loader
- Configuration Errors & Validation
- HTTP Server & Middleware Architecture
- Lifecycle State & Concurrency Tests
- Core Architectural Decision Records
- CI/CD GitHub Actions Pipeline
- Basic DI Starter Example
- Author Identity & Branding Metadata
- Product Brief & Framework Roadmap
- Lifecycle & HTTP Module Decisions
- CLI & Code Generation Strategy
- Configuration Module Architecture
- Root Framework Module Identity

## God Nodes (most connected - your core abstractions)
1. `Server` - 35 edges
2. `Container` - 28 edges
3. `New()` - 25 edges
4. `New()` - 22 edges
5. `App` - 22 edges
6. `New()` - 22 edges
7. `Middleware` - 21 edges
8. `router` - 17 edges
9. `WithMap()` - 16 edges
10. `NewHTTPServer()` - 13 edges

## Surprising Connections (you probably didn't know these)
- `main()` --calls--> `Run()`  [EXTRACTED]
  cmd/injel/main.go → internal/cli/cli.go
- `TestProvideConfigFailureFailsStartup()` --calls--> `WithMap()`  [EXTRACTED]
  injel_test.go → config/config.go
- `TestProvideConfigIntegrationWithDI()` --calls--> `WithMap()`  [EXTRACTED]
  injel_test.go → config/config.go
- `TestProvideConfigPointerAndValueSemantics()` --calls--> `WithMap()`  [EXTRACTED]
  injel_test.go → config/config.go
- `TestProvideConfigPointerDirectly()` --calls--> `WithMap()`  [EXTRACTED]
  injel_test.go → config/config.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Injel Core Application Subsystems** — readme_ioc_container, readme_environment_configuration, readme_application_lifecycle, readme_httpserver_engine [EXTRACTED 1.00]
- **Architecture Decision Record Series (ADR 0001-0006)** — docs_adr_0001_dependency_injection_adr_0001, docs_adr_0002_interface_binding_adr_0002, docs_adr_0003_application_lifecycle_adr_0003, docs_adr_0004_configuration_adr_0004, docs_adr_0005_http_module_adr_0005, docs_adr_0006_cli_architecture_adr_0006 [EXTRACTED 1.00]
- **DI Strategy Architectural Consensus (Reflection vs Codegen)** — docs_adr_0001_dependency_injection_startup_reflection_di, docs_research_code_generation_research_codegen_vs_reflection, docs_adr_0006_cli_architecture_adr_0006 [EXTRACTED 1.00]

## Communities (17 total, 4 thin omitted)

### Community 0 - "REST API Example Service"
Cohesion: 0.06
Nodes (25): NewHTTPServer(), NewUserHandler(), NewUserRepository(), NewUserService(), AppConfig, User, UserHandler, UserRepository (+17 more)

### Community 1 - "Application Facade & Lifecycle Hooks"
Cohesion: 0.09
Nodes (44): App, appConfig, appService, appStore, diAppConfig, diServiceWithConfig, failingProviderTarget, funcStopper (+36 more)

### Community 2 - "Dependency Injection Engine"
Cohesion: 0.07
Nodes (54): Bind(), extractInterfaceType(), Get(), New(), ProvideAs(), newCycleA(), newCycleB(), newTestConfig() (+46 more)

### Community 3 - "Injel CLI & Scaffolding Tooling"
Cohesion: 0.07
Nodes (21): main(), RouteError, ServerError, Run(), TestCLIDoctor(), TestCLINewCommand(), TestCLIRunHelpAndVersion(), TestCLIUnknownCommand() (+13 more)

### Community 4 - "Environment Configuration Loader"
Cohesion: 0.10
Nodes (33): Bind(), bindValue(), implementsTextUnmarshaler(), Load(), newOptions(), parseTag(), setFieldValue(), setSliceValue() (+25 more)

### Community 5 - "Configuration Errors & Validation"
Cohesion: 0.09
Nodes (17): ConversionError, InvalidTagError, InvalidTargetError, MissingRequiredError, UnsupportedTypeError, cyclePath(), extendPath(), extractConcreteType() (+9 more)

### Community 6 - "HTTP Server & Middleware Architecture"
Cohesion: 0.14
Nodes (32): componentDep, RequestID(), mockRepo, mockService, orderTracker, New(), newMockRepo(), newMockService() (+24 more)

### Community 7 - "Lifecycle State & Concurrency Tests"
Cohesion: 0.11
Nodes (12): cancellableStartComponent, cleanupErrorComponent, dbComponent, failingStartComponent, lifecycleTracker, onlyStopComponent, recordedEvent, repoComponent (+4 more)

### Community 8 - "Core Architectural Decision Records"
Cohesion: 0.08
Nodes (17): ADR 0001: Dependency Injection for Injel MVP, Fail-Fast Graph Validation (Cycles & Missing Dependencies), ADR 0002: Interface Binding in Injel, Thread-Safe Application State Machine, StartHook and StopHook Lifecycle Contracts, Struct Tag env Parser & Type Converter, Idiomatic Middleware Chain (RequestID, Logging, Recovery), Project Scaffolding Engine (injel new) (+9 more)

### Community 9 - "CI/CD GitHub Actions Pipeline"
Cohesion: 0.11
Nodes (17): jobs, test, name, on, pull_request, push, branches, branches (+9 more)

### Community 10 - "Basic DI Starter Example"
Cohesion: 0.27
Nodes (5): Config, UserRepository, UserService, NewUserRepository(), NewUserService()

### Community 11 - "Author Identity & Branding Metadata"
Cohesion: 0.67
Nodes (3): Injel Framework Project Maintenance, Phạm Trương Thông (Creator & Lead Maintainer), Open Graph Social Preview Guidelines

### Community 12 - "Product Brief & Framework Roadmap"
Cohesion: 0.67
Nodes (3): Injel MVP Scope & Non-Goals, Injel Development Roadmap, Framework Phases 0-5 Implementation Progress

## Knowledge Gaps
- **18 isolated node(s):** `github.com/Injel-Framework/Injel`, `Injel Framework Project Maintenance`, `GoVa and Tertex Historical Predecessor Names`, `Fail-Fast Graph Validation (Cycles & Missing Dependencies)`, `ADR 0002: Interface Binding in Injel` (+13 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 85 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **4 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Server` connect `REST API Example Service` to `Injel CLI & Scaffolding Tooling`, `HTTP Server & Middleware Architecture`, `Lifecycle State & Concurrency Tests`?**
  _High betweenness centrality (0.082) - this node is a cross-community bridge._
- **Are the 23 inferred relationships involving `New()` (e.g. with `TestConcurrentResolveUsesSingleton()` and `TestDependencyCycle()`) actually correct?**
  _`New()` has 23 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/Injel-Framework/Injel`, `Injel Framework Project Maintenance`, `GoVa and Tertex Historical Predecessor Names` to the rest of the system?**
  _18 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `REST API Example Service` be split into smaller, more focused modules?**
  _Cohesion score 0.05755693581780538 - nodes in this community are weakly interconnected._
- **Why does `Container` connect `Configuration Errors & Validation` to `REST API Example Service`, `Application Facade & Lifecycle Hooks`, `Dependency Injection Engine`, `Injel CLI & Scaffolding Tooling`, `Environment Configuration Loader`, `Lifecycle State & Concurrency Tests`?**
  _High betweenness centrality (0.053) - this node is a cross-community bridge._
- **Should `Application Facade & Lifecycle Hooks` be split into smaller, more focused modules?**
  _Cohesion score 0.08764568764568764 - nodes in this community are weakly interconnected._
- **Why does `Middleware` connect `REST API Example Service` to `Injel CLI & Scaffolding Tooling`, `HTTP Server & Middleware Architecture`?**
  _High betweenness centrality (0.044) - this node is a cross-community bridge._