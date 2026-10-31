# Graph Report - Gova  (2026-11-01)

## Corpus Check
- Corpus is ~43,618 words - fits in a single context window. You may not need a graph.

## Summary
- 638 nodes · 1563 edges · 26 communities (14 shown, 12 thin omitted)
- Extraction: 84% EXTRACTED · 16% INFERRED · 0% AMBIGUOUS · INFERRED: 248 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- HTTP API Example & Handlers
- DI Container Core & Resolution
- Application Facade & Lifecycle Hooks
- Environment Configuration Binding
- Database Mock Driver & Connections
- Configuration Test Suite
- Architecture Concepts & Database ADR
- CLI Entrypoint & Commands
- HTTP Server & User Components
- Database Module & Connection Pool
- HTTP Middleware & Server Tests
- CI GitHub Actions Workflow
- Database SQL Stmt & Rows
- Core Architecture Decisions (DI & Design)
- Project Maintenance & Author Branding
- Lifecycle & HTTP ADRs
- CLI Architecture & Codegen Research
- Dependency Graph Validation
- Configuration Module ADR
- Environment Tag Parsing
- HTTP Middleware Specifications
- Framework Scope & Non-Goals
- Module Root Identity

## God Nodes (most connected - your core abstractions)
1. `Server` - 35 edges
2. `New()` - 32 edges
3. `Container` - 28 edges
4. `New()` - 25 edges
5. `App` - 23 edges
6. `New()` - 22 edges
7. `Middleware` - 21 edges
8. `fakeConfig()` - 19 edges
9. `registerFakeDriver()` - 17 edges
10. `router` - 17 edges

## Surprising Connections (you probably didn't know these)
- `Zero External Dependencies Principle` --semantically_similar_to--> `User-Managed Driver Policy`  [INFERRED] [semantically similar]
  README.md → docs/adr/0007-database-integration.md
- `Database Startup Health Check (PingContext)` --semantically_similar_to--> `Fail-Fast Design Principle`  [INFERRED] [semantically similar]
  docs/adr/0007-database-integration.md → README.vi.md
- `Phase 1 — DI Core & Interface Binding` --implements--> `IoC & DI Engine (di.Container)`  [INFERRED]
  docs/roadmap.md → README.md
- `Phase 2 — Application Lifecycle` --implements--> `Application Lifecycle Management`  [INFERRED]
  docs/roadmap.md → README.md
- `main()` --calls--> `Run()`  [EXTRACTED]
  cmd/injel/main.go → internal/cli/cli.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Architecture Decision Record Series (ADR 0001-0006)** — docs_adr_0001_dependency_injection_adr_0001, docs_adr_0002_interface_binding_adr_0002, docs_adr_0003_application_lifecycle_adr_0003, docs_adr_0004_configuration_adr_0004, docs_adr_0005_http_module_adr_0005, docs_adr_0006_cli_architecture_adr_0006 [EXTRACTED 1.00]
- **DI Strategy Architectural Consensus (Reflection vs Codegen)** — docs_adr_0001_dependency_injection_startup_reflection_di, docs_research_code_generation_research_codegen_vs_reflection, docs_adr_0006_cli_architecture_adr_0006 [EXTRACTED 1.00]
- **Database Integration with DI Container and Application Lifecycle** — docs_adr_0007_database_integration_database_provide, readme_ioc_di_engine, docs_adr_0007_database_integration_startup_health_check, docs_adr_0007_database_integration_graceful_shutdown, readme_application_lifecycle [EXTRACTED 1.00]
- **Four Core Module Pillars of Injel Framework** — readme_ioc_di_engine, readme_env_configuration, readme_application_lifecycle, readme_http_server [EXTRACTED 1.00]
- **Phased Roadmap Delivery: DI Core → Lifecycle → Config → HTTP → CLI → Integrations** — docs_roadmap_phase1_di_core, docs_roadmap_phase2_lifecycle, docs_roadmap_phase3_configuration, docs_roadmap_phase4_http, docs_roadmap_phase5_cli_scaffolding, docs_roadmap_phase6_optional_integrations [EXTRACTED 1.00]

## Communities (26 total, 12 thin omitted)

### Community 0 - "HTTP API Example & Handlers"
Cohesion: 0.06
Nodes (24): NewHTTPServer(), NewUserHandler(), NewUserRepository(), NewUserService(), AppConfig, User, UserHandler, UserRepository (+16 more)

### Community 1 - "DI Container Core & Resolution"
Cohesion: 0.07
Nodes (52): Get(), New(), ProvideAs(), newCycleA(), newCycleB(), newTestConfig(), newTestDB(), newTestGreeterConsumer() (+44 more)

### Community 2 - "Application Facade & Lifecycle Hooks"
Cohesion: 0.07
Nodes (42): App, appConfig, appService, appStore, diAppConfig, diServiceWithConfig, failingProviderTarget, funcStopper (+34 more)

### Community 3 - "Environment Configuration Binding"
Cohesion: 0.06
Nodes (36): Bind(), bindValue(), implementsTextUnmarshaler(), Load(), newOptions(), parseTag(), setFieldValue(), setSliceValue() (+28 more)

### Community 4 - "Database Mock Driver & Connections"
Cohesion: 0.06
Nodes (21): fakeCloseErrDriver, fakeConn, fakeDriver, handle, UserRepository, getenv(), main(), NewUserRepository() (+13 more)

### Community 5 - "Configuration Test Suite"
Cohesion: 0.08
Nodes (45): TestConfigDefaultsAndOverrides(), TestConversionErrors(), TestDuplicateTagOptions(), TestExistingEmptyVariableNonRequired(), TestInvalidTags(), TestMissingRequiredVariable(), TestNestedStructs(), TestPointerFields() (+37 more)

### Community 6 - "Architecture Concepts & Database ADR"
Cohesion: 0.05
Nodes (37): Thread-Safe Application State Machine, StartHook and StopHook Lifecycle Contracts, Project Scaffolding Engine (injel new), ADR 0007 — Database Integration (database/sql), Connection Pool Configuration, database.Provide() — DI Registration API, Database Graceful Shutdown (db.Close), Database Startup Health Check (PingContext) (+29 more)

### Community 7 - "CLI Entrypoint & Commands"
Cohesion: 0.08
Nodes (21): main(), Run(), TestCLIDoctor(), TestCLINewCommand(), TestCLIRunHelpAndVersion(), TestCLIUnknownCommand(), RunDoctor(), PrintNewUsage() (+13 more)

### Community 8 - "HTTP Server & User Components"
Cohesion: 0.07
Nodes (15): Config, UserRepository, UserService, serverConfig, CloseError, Config, ConfigError, OpenError (+7 more)

### Community 9 - "Database Module & Connection Pool"
Cohesion: 0.15
Nodes (31): newHandle(), Provide(), fakeConfig(), registerCloseErrDriver(), registerFakeDriver(), setPingErr(), TestConcurrentOnStop_Idempotent(), TestConcurrentResolve_NoPanic() (+23 more)

### Community 10 - "HTTP Middleware & Server Tests"
Cohesion: 0.14
Nodes (32): componentDep, RequestID(), mockRepo, mockService, orderTracker, New(), newMockRepo(), newMockService() (+24 more)

### Community 11 - "CI GitHub Actions Workflow"
Cohesion: 0.11
Nodes (17): jobs, test, name, on, pull_request, push, branches, branches (+9 more)

### Community 13 - "Core Architecture Decisions (DI & Design)"
Cohesion: 0.40
Nodes (3): ADR 0001: Dependency Injection for Injel MVP, ADR 0002: Interface Binding in Injel, Injel Product Brief & Philosophy

### Community 14 - "Project Maintenance & Author Branding"
Cohesion: 0.67
Nodes (3): Injel Framework Project Maintenance, Phạm Trương Thông (Creator & Lead Maintainer), Open Graph Social Preview Guidelines

## Knowledge Gaps
- **26 isolated node(s):** `github.com/Injel-Framework/Injel`, `Injel Framework Project Maintenance`, `Fail-Fast Graph Validation (Cycles & Missing Dependencies)`, `ADR 0002: Interface Binding in Injel`, `ADR 0003: Application Lifecycle Management` (+21 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 131 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **12 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Server` connect `HTTP API Example & Handlers` to `Database Mock Driver & Connections`, `Configuration Test Suite`, `CLI Entrypoint & Commands`, `HTTP Server & User Components`, `HTTP Middleware & Server Tests`?**
  _High betweenness centrality (0.061) - this node is a cross-community bridge._
- **Are the 19 inferred relationships involving `New()` (e.g. with `TestAppProvideAndGet()` and `TestAppProvideAsAndBind()`) actually correct?**
  _`New()` has 19 INFERRED edges - model-reasoned connections that need verification._
- **What connects `github.com/Injel-Framework/Injel`, `Injel Framework Project Maintenance`, `Fail-Fast Graph Validation (Cycles & Missing Dependencies)` to the rest of the system?**
  _26 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `HTTP API Example & Handlers` be split into smaller, more focused modules?**
  _Cohesion score 0.058823529411764705 - nodes in this community are weakly interconnected._
- **Why does `Container` connect `Environment Configuration Binding` to `HTTP API Example & Handlers`, `DI Container Core & Resolution`, `Application Facade & Lifecycle Hooks`, `Database Mock Driver & Connections`, `HTTP Server & User Components`?**
  _High betweenness centrality (0.038) - this node is a cross-community bridge._
- **Should `DI Container Core & Resolution` be split into smaller, more focused modules?**
  _Cohesion score 0.07093253968253968 - nodes in this community are weakly interconnected._
- **Why does `Middleware` connect `HTTP API Example & Handlers` to `HTTP Server & User Components`, `HTTP Middleware & Server Tests`?**
  _High betweenness centrality (0.032) - this node is a cross-community bridge._