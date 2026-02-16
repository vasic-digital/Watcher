package filter

import (
	"testing"
	"time"

	"digital.vasic.watcher/pkg/watcher"
	"github.com/stretchr/testify/assert"
)

func makeEvent(path string, eventType watcher.EventType) watcher.Event {
	return watcher.Event{
		Type:      eventType,
		Path:      path,
		Timestamp: time.Now(),
	}
}

func TestExtensionFilterMatch(t *testing.T) {
	f := NewExtensionFilter("mp4", ".avi", "MKV")

	tests := []struct {
		path    string
		matches bool
	}{
		{"/movies/film.mp4", true},
		{"/movies/film.MP4", true},
		{"/movies/film.avi", true},
		{"/movies/film.mkv", true},
		{"/movies/film.txt", false},
		{"/movies/film.go", false},
		{"/movies/film", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ev := makeEvent(tt.path, watcher.Create)
			assert.Equal(t, tt.matches, f.Match(ev))
		})
	}
}

func TestExtensionFilterNormalization(t *testing.T) {
	f := NewExtensionFilter("go", ".Go", "GO")
	// All should be normalized to ".go"
	assert.Len(t, f.Extensions, 3)
	for _, ext := range f.Extensions {
		assert.Equal(t, ".go", ext)
	}
}

func TestTypeFilterMatch(t *testing.T) {
	f := NewTypeFilter(watcher.Create, watcher.Write)

	tests := []struct {
		eventType watcher.EventType
		matches   bool
	}{
		{watcher.Create, true},
		{watcher.Write, true},
		{watcher.Remove, false},
		{watcher.Rename, false},
		{watcher.Chmod, false},
	}

	for _, tt := range tests {
		t.Run(tt.eventType.String(), func(t *testing.T) {
			ev := makeEvent("/test", tt.eventType)
			assert.Equal(t, tt.matches, f.Match(ev))
		})
	}
}

func TestGlobFilterMatch(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		matches bool
	}{
		{"star extension", "*.go", "/src/main.go", true},
		{"star extension no match", "*.go", "/src/main.rs", false},
		{"prefix match", "test_*", "/src/test_main.go", true},
		{"prefix no match", "test_*", "/src/main.go", false},
		{"question mark", "?.txt", "/tmp/a.txt", true},
		{"question mark no match", "?.txt", "/tmp/ab.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewGlobFilter(tt.pattern)
			ev := makeEvent(tt.path, watcher.Create)
			assert.Equal(t, tt.matches, f.Match(ev))
		})
	}
}

func TestAndFilter(t *testing.T) {
	extFilter := NewExtensionFilter("go")
	typeFilter := NewTypeFilter(watcher.Create)

	combined := And(extFilter, typeFilter)

	tests := []struct {
		name    string
		path    string
		evType  watcher.EventType
		matches bool
	}{
		{"both match", "/src/main.go", watcher.Create, true},
		{"ext matches type no", "/src/main.go", watcher.Remove, false},
		{"type matches ext no", "/src/main.txt", watcher.Create, false},
		{"neither matches", "/src/main.txt", watcher.Remove, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := makeEvent(tt.path, tt.evType)
			assert.Equal(t, tt.matches, combined.Match(ev))
		})
	}
}

func TestOrFilter(t *testing.T) {
	extGo := NewExtensionFilter("go")
	extRs := NewExtensionFilter("rs")

	combined := Or(extGo, extRs)

	tests := []struct {
		path    string
		matches bool
	}{
		{"/src/main.go", true},
		{"/src/main.rs", true},
		{"/src/main.py", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ev := makeEvent(tt.path, watcher.Create)
			assert.Equal(t, tt.matches, combined.Match(ev))
		})
	}
}

func TestNotFilter(t *testing.T) {
	extFilter := NewExtensionFilter("tmp")
	notTmp := Not(extFilter)

	tests := []struct {
		path    string
		matches bool
	}{
		{"/test.tmp", false},
		{"/test.go", true},
		{"/test.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			ev := makeEvent(tt.path, watcher.Create)
			assert.Equal(t, tt.matches, notTmp.Match(ev))
		})
	}
}

func TestComplexFilterComposition(t *testing.T) {
	// Match: (Go OR Rust files) AND (Create OR Write) AND NOT tmp files
	filter := And(
		Or(
			NewExtensionFilter("go"),
			NewExtensionFilter("rs"),
		),
		NewTypeFilter(watcher.Create, watcher.Write),
		Not(NewGlobFilter("*_test.*")),
	)

	tests := []struct {
		name    string
		path    string
		evType  watcher.EventType
		matches bool
	}{
		{"go create", "/src/main.go", watcher.Create, true},
		{"rs write", "/src/main.rs", watcher.Write, true},
		{"go remove", "/src/main.go", watcher.Remove, false},
		{"py create", "/src/main.py", watcher.Create, false},
		{"go test", "/src/main_test.go", watcher.Create, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := makeEvent(tt.path, tt.evType)
			assert.Equal(t, tt.matches, filter.Match(ev))
		})
	}
}

func TestAndFilterEmpty(t *testing.T) {
	// Empty AND should match everything (vacuous truth).
	combined := And()
	ev := makeEvent("/anything", watcher.Create)
	assert.True(t, combined.Match(ev))
}

func TestOrFilterEmpty(t *testing.T) {
	// Empty OR should match nothing.
	combined := Or()
	ev := makeEvent("/anything", watcher.Create)
	assert.False(t, combined.Match(ev))
}
