# Test Coverage Ledger — Watcher

> **Round 289 deep-doc audit** (template mirror of rounds 220 / 242-285).
> Symbol→test→Challenge mapping for every exported symbol in
> `pkg/watcher`, `pkg/handler`, `pkg/filter`, `pkg/debounce`, `pkg/i18n`.
> Anti-bluff invariant per CONST-035 / Article XI §11.9: every PASS row
> carries either a unit test that exercises the real implementation
> (NOT a mock) or a Challenge with captured runtime evidence.

## How to read this ledger

- **Symbol** — exported identifier as it appears in source.
- **Unit test** — test function that exercises the symbol directly.
- **Edge test** — additional table or property test covering boundary cases.
- **Challenge** — runtime exerciser script that captures positive
  evidence (process exits 0 when the feature works, exits non-zero when
  the planted-mutation runs).
- **Evidence kind** — what the Challenge captures (events, exit codes,
  locale labels, etc.).

## `pkg/watcher` — fsnotify-backed Watcher

| Symbol                       | Unit test                                          | Edge test                                                    | Challenge                                  | Evidence kind                   |
|------------------------------|----------------------------------------------------|--------------------------------------------------------------|--------------------------------------------|---------------------------------|
| `EventType`                  | `watcher_test.TestEventTypeString`                 | `watcher_edge_test.TestEventTypeStringUnknown`               | `runner` enumerates all 5 + unknown        | enumeration stdout              |
| `Event`                      | `watcher_test.TestWatcher_*`                       | `watcher_edge2_test.TestEvent_RenameOldPath`                 | `runner` emits real Create/Write/Remove    | real fsnotify events            |
| `Watcher` (interface)        | `watcher_test.TestWatcher_Watch`                   | `watcher_coverage_test.TestWatcher_CloseIdempotent`          | `runner` Watch+Events+Close round-trip     | event channel drain             |
| `Config`                     | `watcher_test.TestDefaultConfig`                   | `watcher_edge_test.TestConfig_ZeroDebounce`                  | `runner` honors custom IgnorePatterns      | filter-by-config stdout         |
| `DefaultConfig`              | `watcher_test.TestDefaultConfig`                   | n/a (constant)                                               | `runner` start-up path                     | non-nil pointer                 |
| `New`                        | `watcher_test.TestNew`                             | `watcher_edge_test.TestNew_NilConfig`                        | `runner` constructs real instance          | fsnotify FD open                |
| `Watch`                      | `watcher_test.TestWatcher_Watch`                   | `watcher_edge_test.TestWatch_NonExistentPath`                | `runner` Watch tmp dir                     | real path registration          |
| `Events` / `Errors` / `Close`| `watcher_test.TestWatcher_*Channel*`               | `watcher_coverage_test.TestWatcher_ClosedChannelSafety`      | `runner` graceful shutdown                 | channel close detection         |

## `pkg/handler` — Event handler interface + chain

| Symbol            | Unit test                                       | Edge test                                              | Challenge                       | Evidence kind            |
|-------------------|-------------------------------------------------|--------------------------------------------------------|---------------------------------|--------------------------|
| `Handler`         | `handler_test.TestHandler_HandleSuccess`        | `handler_edge_test.TestHandler_NilSafety`              | `runner` dispatches real events | observed side-effects    |
| `HandlerFunc`     | `handler_test.TestHandlerFunc_Adapter`          | `handler_edge_test.TestHandlerFunc_ErrorPropagation`   | `runner` adapter exercise       | error bubble-up          |
| `Chain`           | `handler_test.TestChain_Sequential`             | `handler_edge_test.TestChain_FirstErrorStops`          | `runner` 2-stage chain          | sequential output        |
| `NewChain` / `Len`| `handler_test.TestNewChain_Variadic`            | `handler_edge_test.TestChain_LenEmpty`                 | `runner` reports chain depth    | depth integer            |

## `pkg/filter` — Event filtering

| Symbol                | Unit test                                    | Edge test                                                 | Challenge                                 | Evidence kind            |
|-----------------------|----------------------------------------------|-----------------------------------------------------------|-------------------------------------------|--------------------------|
| `Filter`              | `filter_test.TestFilter_Interface`           | `filter_edge_test.TestFilter_NilSafe`                     | `runner` And/Or/Not composition           | match/miss counts        |
| `ExtensionFilter`     | `filter_test.TestExtensionFilter_Match`      | `filter_edge_test.TestExtensionFilter_NoDot`              | `runner` filters by .go/.rs               | filtered stdout          |
| `NewExtensionFilter`  | `filter_test.TestExtensionFilter_Match`      | `filter_edge_test.TestExtensionFilter_EmptyList`          | `runner` constructs real filter           | constructor return       |
| `TypeFilter`          | `filter_test.TestTypeFilter_Match`           | `filter_edge_test.TestTypeFilter_AllTypes`                | `runner` Create+Write only                | type-gated emission      |
| `NewTypeFilter`       | `filter_test.TestTypeFilter_Match`           | `filter_edge_test.TestTypeFilter_EmptyList`               | `runner` constructs real filter           | constructor return       |
| `GlobFilter`          | `filter_test.TestGlobFilter_Match`           | `filter_edge_test.TestGlobFilter_InvalidPattern`          | `runner` excludes `*_test.*`              | glob exclusion           |
| `NewGlobFilter`       | `filter_test.TestGlobFilter_Match`           | `filter_edge_test.TestGlobFilter_EmptyPattern`            | `runner` constructs real filter           | constructor return       |
| `And` / `Or` / `Not`  | `filter_test.TestComposite_*`                | `filter_edge_test.TestComposite_NilOperands`              | `runner` 3-way composition                | logical truth table      |

## `pkg/debounce` — Standalone debouncer

| Symbol             | Unit test                                  | Edge test                                              | Challenge                          | Evidence kind            |
|--------------------|--------------------------------------------|--------------------------------------------------------|------------------------------------|--------------------------|
| `Debouncer`        | `debounce_test.TestDebouncer_Coalesce`     | `debounce_edge_test.TestDebouncer_ConcurrentAdd`       | `runner` floods 100 events         | coalesced count          |
| `New`              | `debounce_test.TestNew`                    | `debounce_edge_test.TestNew_ZeroBuffer`                | `runner` constructs real debouncer | constructor return       |
| `Add`              | `debounce_test.TestDebouncer_Add`          | `debounce_edge_test.TestDebouncer_AddAfterClose`       | `runner` real Add invocation       | enqueue side-effect      |
| `Events`           | `debounce_test.TestDebouncer_EventsDrain`  | `debounce_edge_test.TestDebouncer_BufferFull`          | `runner` drains channel            | event throughput         |
| `Close`            | `debounce_test.TestDebouncer_Close`        | `debounce_edge_test.TestDebouncer_DoubleClose`         | `runner` graceful shutdown         | channel-closed detection |

## `pkg/i18n` — Locale-aware translation seam

| Symbol              | Unit test                                  | Edge test                                  | Challenge                                | Evidence kind             |
|---------------------|--------------------------------------------|--------------------------------------------|------------------------------------------|---------------------------|
| `Translator`        | `translator_test.TestNoopTranslator_T`     | n/a (interface)                            | `runner` 5-locale label render           | locale-tagged stdout      |
| `NoopTranslator`    | `translator_test.TestNoopTranslator_T`     | `translator_test.TestNoopTranslator_Empty` | `runner` fallback when no bundle present | verbatim key echo         |
| `T`                 | `translator_test.TestNoopTranslator_T`     | `translator_test.TestT_NilParams`          | `runner` 5 fixture invocations           | translated string slice   |

## Round-289 Challenge harness

- **`challenges/runner/main.go`** — real Watcher exerciser. Creates a
  scratch directory, attaches a Watcher with custom IgnorePatterns,
  triggers Create/Write/Remove via `os` package, drains events through
  a filter+handler chain, runs the Debouncer over the captured stream,
  and renders 5-locale labels via the `pkg/i18n` translator + bilingual
  fixtures under `challenges/fixtures/<locale>.yaml`. Exits 0 on full
  end-to-end success; exits non-zero on missing fixtures or zero events.
- **`challenges/watcher_describe_challenge.sh`** — paired-mutation gate.
  Invokes the runner; verifies stdout matches the 5-locale ledger and
  the event count exceeds the configured floor. With
  `WATCHER_DESCRIBE_MUTATE=1`, it plants a forbidden token in the
  expected output and asserts the runner FAILS the gate (exit 99). This
  proves the gate is not a tautology — the mutation deliberately breaks
  the invariant and the harness catches the break.

## Anti-bluff posture

- No mock is imported from production code (`pkg/*` files not ending
  `_test.go`); mocks are confined to unit-test sources per CONST-050(A).
- No hardcoded user-facing strings in production code; the only
  inevitable identifiers are protocol-level `EventType.String()` returns
  ("create", "write", "remove", "rename", "chmod", "unknown") kept
  verbatim for round-trip stability per the `pkg/i18n` scope note.
- The Challenge runner uses real filesystem syscalls (no `os.File`
  in-memory shim, no `httptest`-style stand-in) — fsnotify watches real
  inotify/kqueue/ReadDirectoryChangesW handles.
- Paired-mutation step proves the gate's positive evidence — see
  `challenges/watcher_describe_challenge.sh`.

## Verbatim 2026-05-19 operator mandate (CONST-049 §11.4.17 archival)

> "all existing tests and Challenges do work in anti-bluff manner - they
> MUST confirm that all tested codebase really works as expected! We had
> been in position that all tests do execute with success and all
> Challenges as well, but in reality the most of the features does not
> work and can't be used! This MUST NOT be the case and execution of
> tests and Challenges MUST guarantee the quality, the completition and
> full usability by end users of the product!"
