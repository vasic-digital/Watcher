package filter

import (
	"testing"
	"time"

	"digital.vasic.watcher/pkg/watcher"
	"github.com/stretchr/testify/assert"
)

func makeEdgeEvent(path string, eventType watcher.EventType) watcher.Event {
	return watcher.Event{
		Type:      eventType,
		Path:      path,
		Timestamp: time.Now(),
	}
}

// TestExtensionFilter_CaseInsensitive verifies that extension matching is
// case-insensitive. A filter configured with ".jpg" should match .JPG, .Jpg,
// .jPg, etc.
func TestExtensionFilter_CaseInsensitive(t *testing.T) {
	f := NewExtensionFilter("jpg")

	tests := []struct {
		name    string
		path    string
		matches bool
	}{
		{"lowercase .jpg", "/photos/image.jpg", true},
		{"uppercase .JPG", "/photos/image.JPG", true},
		{"mixed case .Jpg", "/photos/image.Jpg", true},
		{"mixed case .jPG", "/photos/image.jPG", true},
		{"mixed case .jpG", "/photos/image.jpG", true},
		{"different ext .png", "/photos/image.png", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := makeEdgeEvent(tt.path, watcher.Create)
			assert.Equal(t, tt.matches, f.Match(ev))
		})
	}
}

// TestExtensionFilter_EmptyExtension verifies that a file with no extension
// does not match any extension filter.
func TestExtensionFilter_EmptyExtension(t *testing.T) {
	f := NewExtensionFilter("txt", "go", "rs")

	tests := []struct {
		name    string
		path    string
		matches bool
	}{
		{"no extension", "/tmp/Makefile", false},
		{"dotfile no ext", "/tmp/.gitignore", false},
		{"trailing dot", "/tmp/file.", false},
		{"path with dot in dir", "/tmp/my.dir/noext", false},
		{"has extension", "/tmp/file.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := makeEdgeEvent(tt.path, watcher.Create)
			assert.Equal(t, tt.matches, f.Match(ev))
		})
	}
}

// TestPatternFilter_ComplexGlobs tests glob patterns with various special
// characters and wildcards. Note: filepath.Match does not support "**"
// (double star) — it treats that as a bad pattern. This test verifies the
// actual behavior of GlobFilter with such patterns.
func TestPatternFilter_ComplexGlobs(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		matches bool
	}{
		// temp_* prefix pattern
		{"temp prefix match", "temp_*", "/tmp/temp_data.csv", true},
		{"temp prefix no match", "temp_*", "/tmp/my_temp.csv", false},
		{"temp prefix exact", "temp_*", "/tmp/temp_", true},

		// Hidden file pattern [.]*
		{"hidden file dot star", ".*", "/home/.bashrc", true},
		{"hidden file .gitignore", ".*", "/repo/.gitignore", true},
		{"not hidden", ".*", "/repo/visible", false},

		// Character class: Go's filepath.Match uses ^ for negation, not !.
		// [^.]* means "starts with a non-dot character followed by anything".
		{"caret negation match", "[^.]*", "/tmp/visible.txt", true},
		{"caret negation dot", "[^.]*", "/tmp/.hidden", false},

		// filepath.Match does not support "**" — it returns an error
		// and GlobFilter.Match returns false for bad patterns.
		{"double star unsupported", "**/*.log", "/var/log/app.log", false},

		// Question mark patterns
		{"question mark single char", "?.txt", "/tmp/a.txt", true},
		{"question mark two chars", "?.txt", "/tmp/ab.txt", false},
		{"question mark any char", "?.txt", "/tmp/Z.txt", true},

		// Bracket patterns
		{"bracket range match", "[a-z].txt", "/tmp/m.txt", true},
		{"bracket range no match", "[a-z].txt", "/tmp/M.txt", false},

		// Complex combined pattern
		{"star middle", "test_*.go", "/src/test_main.go", true},
		{"star middle no match", "test_*.go", "/src/main_test.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewGlobFilter(tt.pattern)
			ev := makeEdgeEvent(tt.path, watcher.Create)
			assert.Equal(t, tt.matches, f.Match(ev))
		})
	}
}

// TestCompositeFilter_DeepNesting tests deeply nested filter compositions
// such as And(Or(f1, f2), Not(f3)).
func TestCompositeFilter_DeepNesting(t *testing.T) {
	// Filter: And(Or(ext=.go, ext=.rs), Not(glob=*_test.*))
	// Matches Go or Rust files that are NOT test files.
	f := And(
		Or(
			NewExtensionFilter("go"),
			NewExtensionFilter("rs"),
		),
		Not(NewGlobFilter("*_test.*")),
	)

	tests := []struct {
		name    string
		path    string
		matches bool
	}{
		{"go source", "/src/main.go", true},
		{"rust source", "/src/lib.rs", true},
		{"go test", "/src/main_test.go", false},
		{"rust test", "/src/lib_test.rs", false},
		{"python file", "/src/main.py", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := makeEdgeEvent(tt.path, watcher.Create)
			assert.Equal(t, tt.matches, f.Match(ev))
		})
	}

	// Triple-nested: Not(And(Or(ext=.tmp, ext=.log), type=Create))
	// Matches everything EXCEPT newly created .tmp or .log files.
	f2 := Not(
		And(
			Or(
				NewExtensionFilter("tmp"),
				NewExtensionFilter("log"),
			),
			NewTypeFilter(watcher.Create),
		),
	)

	tests2 := []struct {
		name    string
		path    string
		evType  watcher.EventType
		matches bool
	}{
		{"create tmp rejected", "/tmp/a.tmp", watcher.Create, false},
		{"create log rejected", "/tmp/a.log", watcher.Create, false},
		{"write tmp allowed", "/tmp/a.tmp", watcher.Write, true},
		{"create go allowed", "/tmp/a.go", watcher.Create, true},
		{"remove log allowed", "/tmp/a.log", watcher.Remove, true},
	}

	for _, tt := range tests2 {
		t.Run(tt.name, func(t *testing.T) {
			ev := makeEdgeEvent(tt.path, tt.evType)
			assert.Equal(t, tt.matches, f2.Match(ev))
		})
	}
}

// TestCompositeFilter_EmptyFilters verifies the behavior of And() and Or()
// when given zero sub-filters.
func TestCompositeFilter_EmptyFilters(t *testing.T) {
	ev := makeEdgeEvent("/any/path.txt", watcher.Create)

	t.Run("empty And matches everything (vacuous truth)", func(t *testing.T) {
		f := And()
		assert.True(t, f.Match(ev))
	})

	t.Run("empty Or matches nothing", func(t *testing.T) {
		f := Or()
		assert.False(t, f.Match(ev))
	})

	t.Run("And of empty And and empty Or", func(t *testing.T) {
		// And(true, false) = false
		f := And(And(), Or())
		assert.False(t, f.Match(ev))
	})

	t.Run("Or of empty And and empty Or", func(t *testing.T) {
		// Or(true, false) = true
		f := Or(And(), Or())
		assert.True(t, f.Match(ev))
	})

	t.Run("Not of empty And", func(t *testing.T) {
		// Not(true) = false
		f := Not(And())
		assert.False(t, f.Match(ev))
	})

	t.Run("Not of empty Or", func(t *testing.T) {
		// Not(false) = true
		f := Not(Or())
		assert.True(t, f.Match(ev))
	})
}

// TestFilter_NilEvent tests filter behavior when passed an event with
// zero-value fields (the closest to "nil" for a value type).
func TestFilter_NilEvent(t *testing.T) {
	zeroEvent := watcher.Event{}

	t.Run("extension filter on zero event", func(t *testing.T) {
		f := NewExtensionFilter("go")
		// Zero event has empty path, no extension.
		assert.False(t, f.Match(zeroEvent))
	})

	t.Run("type filter matches zero event type (Create=0)", func(t *testing.T) {
		// watcher.Create is 0 (iota), which is the zero value of EventType.
		f := NewTypeFilter(watcher.Create)
		assert.True(t, f.Match(zeroEvent))
	})

	t.Run("type filter for Write does not match zero event", func(t *testing.T) {
		f := NewTypeFilter(watcher.Write)
		assert.False(t, f.Match(zeroEvent))
	})

	t.Run("glob filter on zero event", func(t *testing.T) {
		f := NewGlobFilter("*.go")
		// Empty path, base name is ".", should not match *.go.
		assert.False(t, f.Match(zeroEvent))
	})

	t.Run("Not filter on zero event", func(t *testing.T) {
		f := Not(NewExtensionFilter("go"))
		// Extension filter returns false, Not inverts to true.
		assert.True(t, f.Match(zeroEvent))
	})

	t.Run("And with single filter on zero event", func(t *testing.T) {
		f := And(NewExtensionFilter("go"))
		assert.False(t, f.Match(zeroEvent))
	})

	t.Run("Or with single filter on zero event", func(t *testing.T) {
		f := Or(NewExtensionFilter("go"))
		assert.False(t, f.Match(zeroEvent))
	})
}

// TestExtensionFilter_SpecialExtensions tests edge cases with unusual file
// extensions (double extensions, numeric, very long).
func TestExtensionFilter_SpecialExtensions(t *testing.T) {
	tests := []struct {
		name       string
		filterExts []string
		path       string
		matches    bool
	}{
		{"double extension tar.gz", []string{"gz"}, "/archive/file.tar.gz", true},
		{"double extension only tar matches", []string{"tar"}, "/archive/file.tar.gz", false},
		{"numeric extension", []string{"7z"}, "/archive/data.7z", true},
		{"single char extension", []string{"c"}, "/src/main.c", true},
		{"long extension", []string{"typescript"}, "/src/file.typescript", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewExtensionFilter(tt.filterExts...)
			ev := makeEdgeEvent(tt.path, watcher.Create)
			assert.Equal(t, tt.matches, f.Match(ev))
		})
	}
}

// TestGlobFilter_EmptyPattern tests GlobFilter with an empty pattern string.
func TestGlobFilter_EmptyPattern(t *testing.T) {
	f := NewGlobFilter("")

	t.Run("non-empty path does not match empty pattern", func(t *testing.T) {
		ev := makeEdgeEvent("/tmp/file.txt", watcher.Create)
		assert.False(t, f.Match(ev))
	})

	t.Run("empty path matches empty pattern on full path comparison", func(t *testing.T) {
		// filepath.Base("") returns ".", Match("", ".") = false.
		// But Match("", "") on the full path returns true.
		ev := makeEdgeEvent("", watcher.Create)
		assert.True(t, f.Match(ev))
	})
}

// TestTypeFilter_EmptyTypes verifies that a TypeFilter with no types matches
// nothing.
func TestTypeFilter_EmptyTypes(t *testing.T) {
	f := NewTypeFilter()

	allTypes := []watcher.EventType{watcher.Create, watcher.Write, watcher.Remove, watcher.Rename, watcher.Chmod}
	for _, et := range allTypes {
		t.Run(et.String(), func(t *testing.T) {
			ev := makeEdgeEvent("/tmp/file.txt", et)
			assert.False(t, f.Match(ev))
		})
	}
}
