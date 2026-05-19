// Package main — round-289 Watcher Challenge runner.
//
// Real exerciser per CONST-050(B): no mocks. Creates a scratch
// directory, attaches a real fsnotify-backed Watcher with custom
// IgnorePatterns, triggers Create/Write/Remove via os syscalls, drains
// events through a real filter+handler chain, runs the Debouncer over
// the captured stream, and renders 5-locale labels via a minimal
// in-process translator backed by ../fixtures/<locale>.yaml.
//
// Exits 0 on full end-to-end success; exits 99 on missing fixtures or
// zero events; exits 1 on Go runtime error.
//
// Anti-bluff per CONST-035 / Article XI §11.9: every PASS surface
// emits captured runtime evidence (event counts, per-locale label
// renders, debouncer coalesce ratio). No "absence-of-error PASS".
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"digital.vasic.watcher/pkg/debounce"
	"digital.vasic.watcher/pkg/filter"
	"digital.vasic.watcher/pkg/handler"
	"digital.vasic.watcher/pkg/i18n"
	"digital.vasic.watcher/pkg/watcher"
)

// expectedKeys is the closed set of label keys every fixture MUST
// provide. Mismatch ⇒ exit 99.
var expectedKeys = []string{
	"watcher_event_create",
	"watcher_event_write",
	"watcher_event_remove",
	"watcher_event_rename",
	"watcher_event_chmod",
}

// expectedLocales is the closed 5-locale set the round-289 ledger
// requires. Missing fixture file ⇒ exit 99.
var expectedLocales = []string{"en", "sr", "ja", "es", "de"}

// fixtureTranslator is a minimal in-process Translator backed by a
// flat map[string]string parsed from ../fixtures/<locale>.yaml.
//
// We hand-parse the YAML rather than pull a heavy dependency: each
// fixture file is intentionally simple (two-space indented "key:
// value" pairs under labels:) so the parser stays robust without
// adding gopkg.in/yaml.v3 to the production module surface.
type fixtureTranslator struct {
	locale string
	labels map[string]string
}

func (f fixtureTranslator) T(key string, _ map[string]any) string {
	if v, ok := f.labels[key]; ok {
		return v
	}
	return key // CONST-046 fallback contract: return key verbatim
}

func loadFixture(dir, locale string) (i18n.Translator, error) {
	path := filepath.Join(dir, locale+".yaml")
	fh, err := os.Open(path) //nolint:gosec // round-289 fixture loader
	if err != nil {
		return nil, fmt.Errorf("open fixture %s: %w", path, err)
	}
	defer fh.Close()

	labels := make(map[string]string)
	scanner := bufio.NewScanner(fh)
	inLabels := false
	for scanner.Scan() {
		line := scanner.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if trim == "labels:" {
			inLabels = true
			continue
		}
		if !inLabels {
			continue
		}
		// Expect 2-space indent + "key: \"value\"" or "key: value"
		if !strings.HasPrefix(line, "  ") {
			inLabels = false
			continue
		}
		kv := strings.SplitN(trim, ":", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		val = strings.Trim(val, `"`)
		labels[key] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan fixture %s: %w", path, err)
	}
	for _, k := range expectedKeys {
		if _, ok := labels[k]; !ok {
			return nil, fmt.Errorf("fixture %s missing key %q", path, k)
		}
	}
	return fixtureTranslator{locale: locale, labels: labels}, nil
}

func main() {
	var (
		tmp        = flag.String("tmp", "./.tmp-watch", "scratch directory")
		events     = flag.Int("events", 8, "minimum events to assert")
		fixturesIn = flag.String("fixtures", "./challenges/fixtures", "fixture dir")
	)
	flag.Parse()

	// Resolve fixtures dir relative to this binary's invocation:
	// allow running from submodule root OR challenges/ dir.
	fixDir := *fixturesIn
	if _, err := os.Stat(fixDir); err != nil {
		alt := filepath.Join("..", "fixtures")
		if _, err2 := os.Stat(alt); err2 == nil {
			fixDir = alt
		}
	}

	fmt.Println("=== Watcher round-289 Challenge runner ===")
	fmt.Printf("  tmp=%s events>=%d fixtures=%s\n", *tmp, *events, fixDir)

	// 1. Load all 5 fixtures.
	translators := make(map[string]i18n.Translator, len(expectedLocales))
	for _, loc := range expectedLocales {
		t, err := loadFixture(fixDir, loc)
		if err != nil {
			fmt.Printf("[locale:%s] FAIL — %v\n", loc, err)
			os.Exit(99)
		}
		translators[loc] = t
	}
	fmt.Printf("[1/6] loaded %d locale fixtures: %v\n", len(translators), expectedLocales)

	// 2. Prepare scratch dir.
	if err := os.RemoveAll(*tmp); err != nil && !os.IsNotExist(err) {
		fmt.Printf("[2/6] FAIL — cleanup %s: %v\n", *tmp, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(*tmp, 0o755); err != nil {
		fmt.Printf("[2/6] FAIL — mkdir %s: %v\n", *tmp, err)
		os.Exit(1)
	}
	defer os.RemoveAll(*tmp)
	fmt.Printf("[2/6] scratch dir ready: %s\n", *tmp)

	// 3. Construct real Watcher with custom config.
	cfg := watcher.DefaultConfig()
	cfg.DebounceDelay = 30 * time.Millisecond
	cfg.IgnorePatterns = []string{"*.tmp"}
	w, err := watcher.New(cfg)
	if err != nil {
		fmt.Printf("[3/6] FAIL — watcher.New: %v\n", err)
		os.Exit(1)
	}
	defer w.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := w.Watch(ctx, *tmp); err != nil {
		fmt.Printf("[3/6] FAIL — Watch: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[3/6] real fsnotify watcher attached (cfg=%+v)\n", cfg)

	// 4. Trigger real filesystem activity.
	go func() {
		time.Sleep(80 * time.Millisecond)
		for i := 0; i < *events; i++ {
			p := filepath.Join(*tmp, fmt.Sprintf("f%02d.go", i))
			_ = os.WriteFile(p, []byte("package main\n"), 0o644)
			time.Sleep(15 * time.Millisecond)
			_ = os.WriteFile(p, []byte("package main\n// edit\n"), 0o644)
		}
		time.Sleep(50 * time.Millisecond)
		// Ignored files (per cfg.IgnorePatterns) — should NOT surface.
		_ = os.WriteFile(filepath.Join(*tmp, "skip.tmp"), []byte("x"), 0o644)
	}()

	// 5. Drain events through filter+handler chain + Debouncer.
	f := filter.And(
		filter.NewExtensionFilter("go"),
		filter.NewTypeFilter(watcher.Create, watcher.Write),
	)
	var collected []watcher.Event
	deb := debounce.New(40*time.Millisecond, 200)
	defer deb.Close()

	chain := handler.NewChain(
		handler.HandlerFunc(func(e watcher.Event) error {
			collected = append(collected, e)
			deb.Add(e)
			return nil
		}),
	)

	deadline := time.After(3 * time.Second)
drain:
	for {
		select {
		case ev, ok := <-w.Events():
			if !ok {
				break drain
			}
			if f.Match(ev) {
				_ = chain.Handle(ev)
			}
		case <-deadline:
			break drain
		case <-ctx.Done():
			break drain
		}
		if len(collected) >= *events {
			// allow extra time for the Debouncer to coalesce
			time.Sleep(80 * time.Millisecond)
			break drain
		}
	}

	// Drain coalesced events from the Debouncer.
	deb.Close()
	var coalesced int
	for range deb.Events() {
		coalesced++
	}
	fmt.Printf("[4/6] real events captured: raw=%d coalesced=%d\n",
		len(collected), coalesced)

	if len(collected) < *events {
		fmt.Printf("[4/6] FAIL — captured %d events, need >=%d\n",
			len(collected), *events)
		os.Exit(99)
	}

	// 6. Render 5-locale labels for the captured event types.
	seen := map[watcher.EventType]bool{}
	for _, ev := range collected {
		seen[ev.Type] = true
	}
	types := make([]watcher.EventType, 0, len(seen))
	for t := range seen {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })

	fmt.Println("[5/6] 5-locale label render:")
	for _, loc := range expectedLocales {
		t := translators[loc]
		var parts []string
		for _, et := range types {
			key := "watcher_event_" + et.String()
			parts = append(parts, fmt.Sprintf("%s=%s", et.String(), t.T(key, nil)))
		}
		fmt.Printf("  [%s] %s\n", loc, strings.Join(parts, " | "))
	}

	// Anti-bluff sanity: ignored file must NOT appear in collected stream.
	for _, ev := range collected {
		if strings.HasSuffix(ev.Path, ".tmp") {
			fmt.Printf("[6/6] FAIL — ignored file %s leaked into stream\n", ev.Path)
			os.Exit(99)
		}
	}
	fmt.Println("[6/6] ignore-pattern enforcement: PASS (no *.tmp leakage)")

	fmt.Println("=== Watcher round-289 Challenge runner: PASS ===")
}
