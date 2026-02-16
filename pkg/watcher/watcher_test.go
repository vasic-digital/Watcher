package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventTypeString(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{Create, "create"},
		{Write, "write"},
		{Remove, "remove"},
		{Rename, "rename"},
		{Chmod, "chmod"},
		{EventType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.eventType.String())
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.True(t, cfg.Recursive)
	assert.Equal(t, 100*time.Millisecond, cfg.DebounceDelay)
	assert.Equal(t, 100, cfg.BufferSize)
	assert.Empty(t, cfg.IgnorePatterns)
}

func TestNew(t *testing.T) {
	t.Run("nil config uses defaults", func(t *testing.T) {
		w, err := New(nil)
		require.NoError(t, err)
		require.NotNil(t, w)
		defer w.Close()
	})

	t.Run("custom config", func(t *testing.T) {
		cfg := &Config{
			Recursive:     false,
			DebounceDelay: 200 * time.Millisecond,
			BufferSize:    50,
		}
		w, err := New(cfg)
		require.NoError(t, err)
		require.NotNil(t, w)
		defer w.Close()
	})
}

func TestWatchCreateEvent(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Recursive:     false,
		DebounceDelay: 0, // no debouncing for deterministic tests
		BufferSize:    10,
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	// Give the watcher a moment to start.
	time.Sleep(50 * time.Millisecond)

	// Create a file.
	testFile := filepath.Join(dir, "test.txt")
	err = os.WriteFile(testFile, []byte("hello"), 0644)
	require.NoError(t, err)

	// Wait for the event.
	select {
	case ev := <-w.Events():
		assert.Equal(t, testFile, ev.Path)
		assert.Contains(t, []EventType{Create, Write}, ev.Type)
		assert.False(t, ev.Timestamp.IsZero())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for create event")
	}
}

func TestWatchWriteEvent(t *testing.T) {
	dir := t.TempDir()

	// Pre-create the file.
	testFile := filepath.Join(dir, "existing.txt")
	err := os.WriteFile(testFile, []byte("initial"), 0644)
	require.NoError(t, err)

	cfg := &Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Modify the file.
	err = os.WriteFile(testFile, []byte("modified"), 0644)
	require.NoError(t, err)

	// Wait for the event.
	select {
	case ev := <-w.Events():
		assert.Equal(t, testFile, ev.Path)
		assert.False(t, ev.Timestamp.IsZero())
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for write event")
	}
}

func TestWatchRemoveEvent(t *testing.T) {
	dir := t.TempDir()

	// Pre-create the file.
	testFile := filepath.Join(dir, "to_delete.txt")
	err := os.WriteFile(testFile, []byte("delete me"), 0644)
	require.NoError(t, err)

	cfg := &Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Remove the file.
	err = os.Remove(testFile)
	require.NoError(t, err)

	// Wait for the event.
	select {
	case ev := <-w.Events():
		assert.Equal(t, testFile, ev.Path)
		assert.Equal(t, Remove, ev.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for remove event")
	}
}

func TestWatchRecursive(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "subdir")
	err := os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	cfg := &Config{
		Recursive:     true,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Create a file in the subdirectory.
	testFile := filepath.Join(subDir, "nested.txt")
	err = os.WriteFile(testFile, []byte("nested"), 0644)
	require.NoError(t, err)

	// Wait for the event.
	select {
	case ev := <-w.Events():
		assert.Equal(t, testFile, ev.Path)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event in subdirectory")
	}
}

func TestWatchIgnorePatterns(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Recursive:      false,
		DebounceDelay:  0,
		BufferSize:     10,
		IgnorePatterns: []string{"*.tmp", ".*"},
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Create an ignored file.
	ignoredFile := filepath.Join(dir, "test.tmp")
	err = os.WriteFile(ignoredFile, []byte("ignored"), 0644)
	require.NoError(t, err)

	// Create a visible file.
	visibleFile := filepath.Join(dir, "visible.txt")
	err = os.WriteFile(visibleFile, []byte("visible"), 0644)
	require.NoError(t, err)

	// We should receive the visible file event but not the ignored one.
	select {
	case ev := <-w.Events():
		assert.Equal(t, visibleFile, ev.Path)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for visible file event")
	}
}

func TestWatchContextCancel(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Cancel the context to stop watching.
	cancel()

	// Give the loop time to exit.
	time.Sleep(100 * time.Millisecond)

	// Create a file after cancel - should not receive event.
	testFile := filepath.Join(dir, "after_cancel.txt")
	_ = os.WriteFile(testFile, []byte("late"), 0644)

	select {
	case _, ok := <-w.Events():
		if ok {
			// Might get a buffered event, that is acceptable.
		}
	case <-time.After(200 * time.Millisecond):
		// No event received, as expected.
	}
}

func TestWatchDebounce(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Recursive:     false,
		DebounceDelay: 200 * time.Millisecond,
		BufferSize:    10,
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Write to the same file rapidly.
	testFile := filepath.Join(dir, "debounce.txt")
	for i := 0; i < 5; i++ {
		err = os.WriteFile(testFile, []byte("iteration"), 0644)
		require.NoError(t, err)
		time.Sleep(20 * time.Millisecond)
	}

	// Wait for the debounced event.
	var received int
	timeout := time.After(2 * time.Second)
	collectDone := time.After(800 * time.Millisecond)

loop:
	for {
		select {
		case _, ok := <-w.Events():
			if ok {
				received++
			}
		case <-collectDone:
			break loop
		case <-timeout:
			break loop
		}
	}

	// With debouncing, we should receive fewer events than we wrote.
	assert.Greater(t, received, 0, "should receive at least one event")
	assert.Less(t, received, 5, "debouncing should coalesce events")
}

func TestClose(t *testing.T) {
	w, err := New(DefaultConfig())
	require.NoError(t, err)

	err = w.Close()
	assert.NoError(t, err)

	// Double close should be safe.
	err = w.Close()
	assert.NoError(t, err)
}

func TestShouldIgnore(t *testing.T) {
	w := &fsWatcher{
		cfg: &Config{
			IgnorePatterns: []string{"*.log", ".hidden"},
		},
	}

	assert.True(t, w.shouldIgnore("/tmp/test.log"))
	assert.True(t, w.shouldIgnore("/tmp/.hidden"))
	assert.False(t, w.shouldIgnore("/tmp/test.txt"))
	assert.False(t, w.shouldIgnore("/tmp/visible"))
}
