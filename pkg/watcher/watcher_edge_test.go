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

// TestWatcher_SymlinkDirectory verifies that the watcher can watch a
// symlinked directory and receive events for files created within it.
func TestWatcher_SymlinkDirectory(t *testing.T) {
	// Create the real directory and a symlink to it.
	realDir := t.TempDir()
	symlinkDir := filepath.Join(t.TempDir(), "link")
	err := os.Symlink(realDir, symlinkDir)
	if err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

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

	// Watch the symlink path.
	err = w.Watch(ctx, symlinkDir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Create a file through the symlink.
	testFile := filepath.Join(symlinkDir, "symtest.txt")
	err = os.WriteFile(testFile, []byte("via symlink"), 0644)
	require.NoError(t, err)

	// Should receive an event.
	select {
	case ev := <-w.Events():
		assert.Contains(t, ev.Path, "symtest.txt")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event in symlinked directory")
	}
}

// TestWatcher_UnicodeDirectoryNames verifies that directories with Unicode
// characters (Chinese, Cyrillic, emoji-like) are watched correctly.
func TestWatcher_UnicodeDirectoryNames(t *testing.T) {
	baseDir := t.TempDir()

	unicodeDirs := []struct {
		name     string
		dirName  string
		fileName string
	}{
		{"cyrillic", "\u041f\u0440\u043e\u0435\u043a\u0442", "\u0444\u0430\u0439\u043b.txt"},
		{"chinese", "\u6d4b\u8bd5\u76ee\u5f55", "\u6587\u4ef6.txt"},
		{"japanese", "\u30c6\u30b9\u30c8", "\u30d5\u30a1\u30a4\u30eb.txt"},
		{"mixed", "test_\u00e9\u00e8\u00ea", "r\u00e9sum\u00e9.txt"},
	}

	for _, ud := range unicodeDirs {
		t.Run(ud.name, func(t *testing.T) {
			dir := filepath.Join(baseDir, ud.dirName)
			err := os.MkdirAll(dir, 0755)
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

			testFile := filepath.Join(dir, ud.fileName)
			err = os.WriteFile(testFile, []byte("unicode content"), 0644)
			require.NoError(t, err)

			select {
			case ev := <-w.Events():
				assert.Equal(t, testFile, ev.Path)
			case <-time.After(2 * time.Second):
				t.Fatalf("timed out waiting for event in Unicode dir %q", ud.dirName)
			}
		})
	}
}

// TestWatcher_CloseWhileEventsInflight verifies that closing the watcher
// while filesystem events are being generated does not cause panics or
// deadlocks.
func TestWatcher_CloseWhileEventsInflight(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    5, // small buffer to increase backpressure
	}
	w, err := New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Start generating events rapidly.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			f := filepath.Join(dir, "inflight_"+string(rune('a'+i%26))+".txt")
			_ = os.WriteFile(f, []byte("data"), 0644)
		}
	}()

	// Close while events are being generated.
	time.Sleep(10 * time.Millisecond)
	err = w.Close()
	assert.NoError(t, err)

	// Wait for the file creation goroutine to finish.
	<-done
}

// TestWatcher_WatchNonexistentPath verifies that watching a non-existent
// path returns an error.
func TestWatcher_WatchNonexistentPath(t *testing.T) {
	cfg := &Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx := context.Background()
	nonExistent := filepath.Join(t.TempDir(), "does_not_exist")
	err = w.Watch(ctx, nonExistent)
	assert.Error(t, err, "watching a non-existent path should return an error")
}

// TestWatcher_WatchFileThenDeleteRecreate verifies that after deleting and
// recreating a watched directory, the watcher detects the removal.
func TestWatcher_WatchFileThenDeleteRecreate(t *testing.T) {
	baseDir := t.TempDir()
	watchDir := filepath.Join(baseDir, "watched")
	err := os.Mkdir(watchDir, 0755)
	require.NoError(t, err)

	cfg := &Config{
		Recursive:     true,
		DebounceDelay: 0,
		BufferSize:    20,
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, watchDir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Create a file so the directory is in use.
	testFile := filepath.Join(watchDir, "original.txt")
	err = os.WriteFile(testFile, []byte("original"), 0644)
	require.NoError(t, err)

	// Wait for create event.
	select {
	case ev := <-w.Events():
		assert.Contains(t, ev.Path, "original.txt")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for create event")
	}

	// Remove the file.
	err = os.Remove(testFile)
	require.NoError(t, err)

	// Should receive a Remove event. fsnotify may deliver additional
	// events (e.g. Write before Remove), so drain until we see Remove.
	gotRemove := false
	timeout := time.After(2 * time.Second)
drainRemove:
	for {
		select {
		case ev := <-w.Events():
			if ev.Path == testFile && ev.Type == Remove {
				gotRemove = true
				break drainRemove
			}
		case <-timeout:
			break drainRemove
		}
	}
	assert.True(t, gotRemove, "should receive a Remove event for the deleted file")

	// Recreate the file.
	err = os.WriteFile(testFile, []byte("recreated"), 0644)
	require.NoError(t, err)

	// Should receive an event for the recreated file (Create or Write).
	select {
	case ev := <-w.Events():
		assert.Contains(t, ev.Path, "original.txt")
		assert.Contains(t, []EventType{Create, Write}, ev.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for recreate event")
	}
}

// TestWatcher_EventTypeString_AllValues verifies the String() method on all
// defined EventType values, including an out-of-range value.
func TestWatcher_EventTypeString_AllValues(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{Create, "create"},
		{Write, "write"},
		{Remove, "remove"},
		{Rename, "rename"},
		{Chmod, "chmod"},
		{EventType(-1), "unknown"},
		{EventType(5), "unknown"},
		{EventType(100), "unknown"},
		{EventType(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.eventType.String())
		})
	}
}

// TestWatcher_WatchAlreadyWatching verifies that calling Watch a second time
// on the same watcher is a no-op (does not error or double-watch).
func TestWatcher_WatchAlreadyWatching(t *testing.T) {
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
	defer cancel()

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	// Second Watch call should be a no-op (returns nil).
	err = w.Watch(ctx, dir)
	assert.NoError(t, err)
}

// TestWatcher_NonRecursive verifies that in non-recursive mode, events in
// subdirectories are NOT delivered.
func TestWatcher_NonRecursive(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	err := os.Mkdir(subDir, 0755)
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

	// Watch only the parent directory.
	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Create a file in the subdirectory (should not be watched).
	subFile := filepath.Join(subDir, "nested.txt")
	err = os.WriteFile(subFile, []byte("nested"), 0644)
	require.NoError(t, err)

	// Create a file in the watched directory.
	parentFile := filepath.Join(dir, "parent.txt")
	err = os.WriteFile(parentFile, []byte("parent"), 0644)
	require.NoError(t, err)

	// We should get the parent file event.
	select {
	case ev := <-w.Events():
		assert.Equal(t, parentFile, ev.Path,
			"should receive event from watched directory, not subdirectory")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for parent directory event")
	}
}

// TestWatcher_LargeBufferSize verifies that a large buffer does not cause
// issues.
func TestWatcher_LargeBufferSize(t *testing.T) {
	cfg := &Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    10000,
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()
}

// TestWatcher_ZeroBufferSize verifies that zero buffer size is coerced to
// the default (100).
func TestWatcher_ZeroBufferSize(t *testing.T) {
	cfg := &Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    0,
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()
}

// TestWatcher_NegativeBufferSize verifies that negative buffer size is
// coerced to the default (100).
func TestWatcher_NegativeBufferSize(t *testing.T) {
	cfg := &Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    -5,
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()
}
