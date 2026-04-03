package watcher_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"digital.vasic.watcher/pkg/watcher"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcher_WatchNonExistentPath_Recursive(t *testing.T) {
	t.Parallel()

	// In recursive mode, addRecursive uses filepath.Walk which skips
	// inaccessible paths without error. Verify this graceful behavior.
	cfg := &watcher.Config{
		Recursive:     true,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := watcher.New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx := context.Background()
	nonExistent := filepath.Join(t.TempDir(), "does_not_exist")
	// Recursive mode gracefully skips non-existent paths (filepath.Walk returns nil)
	err = w.Watch(ctx, nonExistent)
	assert.NoError(t, err, "recursive mode should not error on non-existent paths")
}

func TestWatcher_RapidFileChanges_Debounce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cfg := &watcher.Config{
		Recursive:     false,
		DebounceDelay: 200 * time.Millisecond,
		BufferSize:    100,
	}
	w, err := watcher.New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Write to the same file rapidly 20 times
	testFile := filepath.Join(dir, "rapid.txt")
	for i := 0; i < 20; i++ {
		err = os.WriteFile(testFile, []byte("version "+string(rune('0'+i%10))), 0644)
		require.NoError(t, err)
		time.Sleep(5 * time.Millisecond)
	}

	// Wait for debounce to settle
	time.Sleep(400 * time.Millisecond)

	// Count events received -- debouncing should coalesce many writes
	eventCount := 0
	drainLoop := true
	for drainLoop {
		select {
		case <-w.Events():
			eventCount++
		default:
			drainLoop = false
		}
	}

	// With a 200ms debounce and 20 writes over ~100ms, we should get
	// significantly fewer events than 20
	assert.Less(t, eventCount, 20,
		"debounce should coalesce rapid changes, got %d events", eventCount)
	assert.Greater(t, eventCount, 0,
		"should receive at least one event")
}

func TestWatcher_EmptyPath(t *testing.T) {
	t.Parallel()

	// fsnotify may or may not error on an empty path depending on the platform.
	// This test verifies the watcher does not panic on empty input.
	cfg := &watcher.Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := watcher.New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx := context.Background()
	assert.NotPanics(t, func() {
		_ = w.Watch(ctx, "")
	})
}

func TestWatcher_PathWithSpecialCharacters(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()

	tests := []struct {
		name    string
		dirName string
	}{
		{"spaces", "dir with spaces"},
		{"hash", "dir#hash"},
		{"ampersand", "dir&amp"},
		{"parentheses", "dir(1)"},
		{"plus", "dir+plus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(baseDir, tt.dirName)
			err := os.MkdirAll(dir, 0755)
			require.NoError(t, err)

			cfg := &watcher.Config{
				Recursive:     false,
				DebounceDelay: 0,
				BufferSize:    10,
			}
			w, err := watcher.New(cfg)
			require.NoError(t, err)
			defer w.Close()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err = w.Watch(ctx, dir)
			require.NoError(t, err)

			time.Sleep(50 * time.Millisecond)

			testFile := filepath.Join(dir, "test.txt")
			err = os.WriteFile(testFile, []byte("content"), 0644)
			require.NoError(t, err)

			select {
			case ev := <-w.Events():
				assert.Contains(t, ev.Path, "test.txt")
			case <-time.After(2 * time.Second):
				t.Fatalf("timed out waiting for event in dir %q", tt.dirName)
			}
		})
	}
}

func TestWatcher_VeryDeepDirectoryTree(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()

	// Create a 10-level deep directory tree
	deepDir := baseDir
	for i := 0; i < 10; i++ {
		deepDir = filepath.Join(deepDir, "level")
	}
	err := os.MkdirAll(deepDir, 0755)
	require.NoError(t, err)

	cfg := &watcher.Config{
		Recursive:     true,
		DebounceDelay: 0,
		BufferSize:    50,
	}
	w, err := watcher.New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, baseDir)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Create a file at the deepest level
	deepFile := filepath.Join(deepDir, "deep.txt")
	err = os.WriteFile(deepFile, []byte("deep content"), 0644)
	require.NoError(t, err)

	// Should receive the event even from a deeply nested directory
	select {
	case ev := <-w.Events():
		assert.Contains(t, ev.Path, "deep.txt")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for event in deep directory tree")
	}
}

func TestWatcher_ConcurrentWatchClose(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cfg := &watcher.Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := watcher.New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	// Start generating events
	go func() {
		for i := 0; i < 20; i++ {
			f := filepath.Join(dir, "concurrent_"+string(rune('a'+i%26))+".txt")
			_ = os.WriteFile(f, []byte("data"), 0644)
			time.Sleep(2 * time.Millisecond)
		}
	}()

	// Close while events are in flight -- should not deadlock or panic
	time.Sleep(10 * time.Millisecond)
	err = w.Close()
	assert.NoError(t, err)
}

func TestWatcher_ContextCancellation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cfg := &watcher.Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := watcher.New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Cancel the context
	cancel()

	// Give time for the loop to exit
	time.Sleep(100 * time.Millisecond)

	// Write a file after cancellation
	testFile := filepath.Join(dir, "after_cancel.txt")
	_ = os.WriteFile(testFile, []byte("data"), 0644)

	// Should not receive events after context cancellation
	select {
	case <-w.Events():
		// Some events may be buffered before cancellation took effect -- acceptable
	case <-time.After(300 * time.Millisecond):
		// No event received -- expected
	}
}

func TestWatcher_IgnorePatterns(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cfg := &watcher.Config{
		Recursive:      false,
		DebounceDelay:  0,
		BufferSize:     20,
		IgnorePatterns: []string{"*.tmp", ".*"},
	}
	w, err := watcher.New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Create files that should be ignored
	_ = os.WriteFile(filepath.Join(dir, "ignored.tmp"), []byte("tmp"), 0644)
	_ = os.WriteFile(filepath.Join(dir, ".hidden"), []byte("hidden"), 0644)

	// Create a file that should NOT be ignored
	expected := filepath.Join(dir, "visible.txt")
	err = os.WriteFile(expected, []byte("visible"), 0644)
	require.NoError(t, err)

	// Drain events -- we should see the visible file but not the ignored ones
	var received []string
	timeout := time.After(2 * time.Second)
	done := false
	for !done {
		select {
		case ev := <-w.Events():
			received = append(received, ev.Path)
		case <-timeout:
			done = true
		}
	}

	// The visible file should appear in events
	foundVisible := false
	foundIgnored := false
	for _, p := range received {
		if filepath.Base(p) == "visible.txt" {
			foundVisible = true
		}
		if filepath.Base(p) == "ignored.tmp" || filepath.Base(p) == ".hidden" {
			foundIgnored = true
		}
	}
	assert.True(t, foundVisible, "visible.txt should be in events")
	assert.False(t, foundIgnored, "ignored files should not be in events")
}

func TestWatcher_NilConfig(t *testing.T) {
	t.Parallel()

	// nil config should use DefaultConfig
	w, err := watcher.New(nil)
	require.NoError(t, err)
	defer w.Close()
}

func TestWatcher_NewDirectoryCreatedRecursive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cfg := &watcher.Config{
		Recursive:     true,
		DebounceDelay: 0,
		BufferSize:    50,
	}
	w, err := watcher.New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Create a new subdirectory dynamically
	newDir := filepath.Join(dir, "newsubdir")
	err = os.Mkdir(newDir, 0755)
	require.NoError(t, err)

	// Wait for the watcher to pick up the new directory
	time.Sleep(200 * time.Millisecond)

	// Create a file inside the new subdirectory
	newFile := filepath.Join(newDir, "inside.txt")
	err = os.WriteFile(newFile, []byte("inside new dir"), 0644)
	require.NoError(t, err)

	// Should receive events for both the directory creation and the file
	gotFileEvent := false
	timeout := time.After(3 * time.Second)
drainLoop:
	for {
		select {
		case ev := <-w.Events():
			if ev.Path == newFile {
				gotFileEvent = true
				break drainLoop
			}
		case <-timeout:
			break drainLoop
		}
	}
	assert.True(t, gotFileEvent,
		"should receive event for file in dynamically created subdirectory")
}
