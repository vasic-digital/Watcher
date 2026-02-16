// Package filter provides filesystem event filtering by type, pattern, and extension.
package filter

import (
	"path/filepath"
	"strings"

	"digital.vasic.watcher/pkg/watcher"
)

// Filter determines if an event should be processed.
type Filter interface {
	Match(event watcher.Event) bool
}

// ExtensionFilter matches events by file extension.
type ExtensionFilter struct {
	Extensions []string
}

// NewExtensionFilter creates a filter that matches files with any of the
// given extensions. Extensions are normalized to lower-case with a leading dot.
func NewExtensionFilter(exts ...string) *ExtensionFilter {
	normalized := make([]string, len(exts))
	for i, ext := range exts {
		ext = strings.ToLower(ext)
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		normalized[i] = ext
	}
	return &ExtensionFilter{Extensions: normalized}
}

// Match returns true if the event path has one of the configured extensions.
func (f *ExtensionFilter) Match(e watcher.Event) bool {
	ext := strings.ToLower(filepath.Ext(e.Path))
	for _, allowed := range f.Extensions {
		if ext == allowed {
			return true
		}
	}
	return false
}

// TypeFilter matches events by event type.
type TypeFilter struct {
	Types []watcher.EventType
}

// NewTypeFilter creates a filter that matches events of the given types.
func NewTypeFilter(types ...watcher.EventType) *TypeFilter {
	return &TypeFilter{Types: types}
}

// Match returns true if the event type is one of the configured types.
func (f *TypeFilter) Match(e watcher.Event) bool {
	for _, t := range f.Types {
		if e.Type == t {
			return true
		}
	}
	return false
}

// GlobFilter matches events by glob pattern against the file path.
type GlobFilter struct {
	Pattern string
}

// NewGlobFilter creates a filter that matches event paths against the given
// glob pattern. The pattern is matched against the base name of the path.
func NewGlobFilter(pattern string) *GlobFilter {
	return &GlobFilter{Pattern: pattern}
}

// Match returns true if the event path matches the glob pattern.
// The match is attempted against both the full path and the base name.
func (f *GlobFilter) Match(e watcher.Event) bool {
	// Try matching against the base name first.
	base := filepath.Base(e.Path)
	if matched, err := filepath.Match(f.Pattern, base); err == nil && matched {
		return true
	}
	// Also try against the full path.
	if matched, err := filepath.Match(f.Pattern, e.Path); err == nil && matched {
		return true
	}
	return false
}

// andFilter combines multiple filters with AND logic.
type andFilter struct {
	filters []Filter
}

// And combines filters with AND logic. All filters must match for the
// composite filter to match.
func And(filters ...Filter) Filter {
	return &andFilter{filters: filters}
}

// Match returns true only if all sub-filters match.
func (f *andFilter) Match(e watcher.Event) bool {
	for _, sub := range f.filters {
		if !sub.Match(e) {
			return false
		}
	}
	return true
}

// orFilter combines multiple filters with OR logic.
type orFilter struct {
	filters []Filter
}

// Or combines filters with OR logic. At least one filter must match
// for the composite filter to match.
func Or(filters ...Filter) Filter {
	return &orFilter{filters: filters}
}

// Match returns true if at least one sub-filter matches.
func (f *orFilter) Match(e watcher.Event) bool {
	for _, sub := range f.filters {
		if sub.Match(e) {
			return true
		}
	}
	return false
}

// notFilter negates a filter.
type notFilter struct {
	inner Filter
}

// Not negates a filter. The result matches when the inner filter does not.
func Not(filter Filter) Filter {
	return &notFilter{inner: filter}
}

// Match returns true if the inner filter does not match.
func (f *notFilter) Match(e watcher.Event) bool {
	return !f.inner.Match(e)
}
