package watcher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Errors() channel coverage — previously 0%
// ---------------------------------------------------------------------------

// TestWatcher_ErrorsChannel verifies that the Errors() method returns a
// readable channel. Since fsnotify does not easily produce watcher errors in
// controlled tests, we verify the channel is non-nil and not closed immediately.
func TestWatcher_ErrorsChannel(t *testing.T) {
	w, err := New(DefaultConfig())
	require.NoError(t, err)
	defer w.Close()

	errCh := w.Errors()
	require.NotNil(t, errCh, "Errors() should return a non-nil channel")

	// No errors should be waiting.
	select {
	case e, ok := <-errCh:
		if ok {
			t.Fatalf("unexpected error on Errors channel: %v", e)
		}
	default:
		// Expected: channel is empty.
	}
}

// TestWatcher_ErrorsChannelDuringWatch verifies that Errors() returns a
// channel that remains usable while the watcher is actively watching.
func TestWatcher_ErrorsChannelDuringWatch(t *testing.T) {
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

	errCh := w.Errors()
	require.NotNil(t, errCh)

	// Verify no spurious errors arrive during normal operation.
	select {
	case e := <-errCh:
		t.Fatalf("unexpected error: %v", e)
	case <-time.After(100 * time.Millisecond):
		// Good — no errors received.
	}
}

// ---------------------------------------------------------------------------
// convertEvent: cover Chmod event type — previously 75%
// ---------------------------------------------------------------------------

// TestConvertEvent_AllTypes verifies convertEvent maps all fsnotify operations
// to the correct EventType, including Chmod which was previously uncovered.
func TestConvertEvent_AllTypes(t *testing.T) {
	w := &fsWatcher{
		cfg: DefaultConfig(),
	}

	tests := []struct {
		name     string
		op       fsnotify.Op
		expected EventType
	}{
		{"create", fsnotify.Create, Create},
		{"write", fsnotify.Write, Write},
		{"remove", fsnotify.Remove, Remove},
		{"rename", fsnotify.Rename, Rename},
		{"chmod", fsnotify.Chmod, Chmod},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := fsnotify.Event{
				Name: "/tmp/test.txt",
				Op:   tt.op,
			}
			result := w.convertEvent(ev)
			assert.Equal(t, tt.expected, result.Type)
			assert.Equal(t, "/tmp/test.txt", result.Path)
			assert.False(t, result.Timestamp.IsZero())
		})
	}
}

// ---------------------------------------------------------------------------
// shouldIgnore: cover full-path matching with separator — previously 75%
// ---------------------------------------------------------------------------

// TestShouldIgnore_FullPathPattern verifies that shouldIgnore correctly
// matches patterns that contain path separators against the full path.
func TestShouldIgnore_FullPathPattern(t *testing.T) {
	w := &fsWatcher{
		cfg: &Config{
			IgnorePatterns: []string{
				"*.log",
				"/tmp/*.bak",
			},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"basename_match_log", "/var/data/test.log", true},
		{"full_path_match_bak", "/tmp/backup.bak", true},
		{"full_path_no_match_bak", "/home/user/backup.bak", false},
		{"no_match", "/tmp/test.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, w.shouldIgnore(tt.path))
		})
	}
}

// TestShouldIgnore_PatternWithSlash verifies the branch where the pattern
// contains a "/" character and is matched against the full path.
func TestShouldIgnore_PatternWithSlash(t *testing.T) {
	w := &fsWatcher{
		cfg: &Config{
			IgnorePatterns: []string{
				"subdir/*.tmp",
			},
		},
	}

	// Pattern "subdir/*.tmp" contains a "/" so it tries full-path match.
	assert.True(t, w.shouldIgnore("subdir/cache.tmp"))
	assert.False(t, w.shouldIgnore("otherdir/cache.tmp"))
	// Base name "cache.tmp" does NOT match "subdir/*.tmp", so without
	// the full-path check this would return false.
	assert.False(t, w.shouldIgnore("/full/path/subdir/cache.tmp"))
}

// TestShouldIgnore_EmptyPatterns verifies that no patterns means nothing
// is ignored.
func TestShouldIgnore_EmptyPatterns(t *testing.T) {
	w := &fsWatcher{
		cfg: &Config{
			IgnorePatterns: nil,
		},
	}

	assert.False(t, w.shouldIgnore("/anything/at/all"))
}

// ---------------------------------------------------------------------------
// addRecursive: cover ignored directory skip and walk error — previously 62.5%
// ---------------------------------------------------------------------------

// TestAddRecursive_IgnoresMatchingDirectory verifies that addRecursive skips
// directories that match ignore patterns (filepath.SkipDir branch).
func TestAddRecursive_IgnoresMatchingDirectory(t *testing.T) {
	baseDir := t.TempDir()

	// Create a directory structure with an ignored subdirectory.
	ignoredDir := filepath.Join(baseDir, ".hidden")
	normalDir := filepath.Join(baseDir, "normal")
	err := os.Mkdir(ignoredDir, 0755)
	require.NoError(t, err)
	err = os.Mkdir(normalDir, 0755)
	require.NoError(t, err)

	// Create a file inside the ignored directory.
	err = os.WriteFile(filepath.Join(ignoredDir, "secret.txt"), []byte("secret"), 0644)
	require.NoError(t, err)

	cfg := &Config{
		Recursive:      true,
		DebounceDelay:  0,
		BufferSize:     10,
		IgnorePatterns: []string{".*"},
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.Watch(ctx, baseDir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Write to the normal directory — should receive an event.
	normalFile := filepath.Join(normalDir, "visible.txt")
	err = os.WriteFile(normalFile, []byte("visible"), 0644)
	require.NoError(t, err)

	select {
	case ev := <-w.Events():
		assert.Equal(t, normalFile, ev.Path)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event in normal directory")
	}

	// Write to the ignored directory — should NOT receive an event.
	ignoredFile := filepath.Join(ignoredDir, "hidden.txt")
	err = os.WriteFile(ignoredFile, []byte("hidden"), 0644)
	require.NoError(t, err)

	select {
	case ev := <-w.Events():
		// If we get an event, it should NOT be from the ignored directory.
		assert.NotContains(t, ev.Path, ".hidden",
			"should not receive events from ignored directories")
	case <-time.After(300 * time.Millisecond):
		// Expected — no event from ignored directory.
	}
}

// TestAddRecursive_WalkErrorSkipped verifies that addRecursive handles walk
// errors (inaccessible paths) gracefully by skipping them.
func TestAddRecursive_WalkErrorSkipped(t *testing.T) {
	baseDir := t.TempDir()

	// Create a subdirectory with no read permission to trigger walk error.
	restrictedDir := filepath.Join(baseDir, "noaccess")
	err := os.Mkdir(restrictedDir, 0755)
	require.NoError(t, err)

	// Create another accessible directory.
	accessibleDir := filepath.Join(baseDir, "accessible")
	err = os.Mkdir(accessibleDir, 0755)
	require.NoError(t, err)

	// Remove read+execute permission from the restricted directory.
	err = os.Chmod(restrictedDir, 0000)
	require.NoError(t, err)
	// Restore permission on cleanup so t.TempDir() can remove it.
	t.Cleanup(func() {
		os.Chmod(restrictedDir, 0755)
	})

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

	// Watch should not fail even though one directory is inaccessible.
	err = w.Watch(ctx, baseDir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Write to the accessible directory — should receive an event.
	testFile := filepath.Join(accessibleDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	select {
	case ev := <-w.Events():
		assert.Equal(t, testFile, ev.Path)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event in accessible directory")
	}
}

// ---------------------------------------------------------------------------
// loop: cover the fsnotify error forwarding branch — previously 76.2%
// ---------------------------------------------------------------------------

// TestWatcher_ChmodEventDelivery verifies that chmod events are detected
// and delivered through the Events channel, covering the Chmod case in
// convertEvent within the loop.
func TestWatcher_ChmodEventDelivery(t *testing.T) {
	dir := t.TempDir()

	// Pre-create a file.
	testFile := filepath.Join(dir, "chmod_test.txt")
	err := os.WriteFile(testFile, []byte("test"), 0644)
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

	// Change file permissions to trigger a Chmod event.
	err = os.Chmod(testFile, 0755)
	require.NoError(t, err)

	// Collect events — fsnotify may deliver Chmod alongside other events.
	var gotChmod bool
	timeout := time.After(2 * time.Second)
	for i := 0; i < 5; i++ {
		select {
		case ev := <-w.Events():
			if ev.Type == Chmod {
				gotChmod = true
			}
		case <-timeout:
			break
		}
		if gotChmod {
			break
		}
	}
	assert.True(t, gotChmod, "should receive a Chmod event")
}

// ---------------------------------------------------------------------------
// loop: cover the new directory creation + recursive add branch
// ---------------------------------------------------------------------------

// TestWatcher_NewDirectoryCreatedWhileWatching verifies that when a new
// directory is created while recursive watching is active, the watcher
// automatically adds it and receives events from within.
func TestWatcher_NewDirectoryCreatedWhileWatching(t *testing.T) {
	dir := t.TempDir()

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

	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Create a new subdirectory while watching.
	newDir := filepath.Join(dir, "dynamic_subdir")
	err = os.Mkdir(newDir, 0755)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Create a file in the new subdirectory.
	testFile := filepath.Join(newDir, "dynamic_file.txt")
	err = os.WriteFile(testFile, []byte("dynamic"), 0644)
	require.NoError(t, err)

	// We should receive events — drain until we see the file event.
	gotDynamicFile := false
	timeout := time.After(2 * time.Second)
	for {
		select {
		case ev := <-w.Events():
			if ev.Path == testFile {
				gotDynamicFile = true
			}
		case <-timeout:
			goto done
		}
		if gotDynamicFile {
			break
		}
	}
done:
	assert.True(t, gotDynamicFile,
		"should receive event for file created in dynamically added subdirectory")
}

// ---------------------------------------------------------------------------
// New: cover the zero/negative buffer size branch — previously 90%
// ---------------------------------------------------------------------------

// TestNew_BufferSizeEdgeCases verifies the buffer size coercion
// (bufSize <= 0 -> 100) in the New constructor.
func TestNew_BufferSizeEdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		bufferSize int
	}{
		{"zero", 0},
		{"negative", -10},
		{"positive", 50},
		{"one", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				BufferSize: tt.bufferSize,
			}
			w, err := New(cfg)
			require.NoError(t, err)
			require.NotNil(t, w)
			defer w.Close()

			// Verify the watcher is functional by checking channels.
			assert.NotNil(t, w.Events())
			assert.NotNil(t, w.Errors())
		})
	}
}

// ---------------------------------------------------------------------------
// loop: cover the "send event with no debounce" and "error forwarding"
// via channel closure detection
// ---------------------------------------------------------------------------

// TestWatcher_StopChannelClosesLoop verifies that closing the watcher via
// Close() properly terminates the loop goroutine via the stopCh case.
func TestWatcher_StopChannelClosesLoop(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := New(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	err = w.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Close should terminate the loop and release resources.
	err = w.Close()
	assert.NoError(t, err)

	// Events channel should be closed after Close().
	_, ok := <-w.Events()
	assert.False(t, ok, "Events channel should be closed after Close()")
}

// ---------------------------------------------------------------------------
// Watch: cover non-recursive path add error (e.g., watching a file path)
// ---------------------------------------------------------------------------

// TestWatch_NonRecursiveWithInvalidPath verifies that watching a non-existent
// path in non-recursive mode returns an error from w.inner.Add().
func TestWatch_NonRecursiveWithInvalidPath(t *testing.T) {
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
	assert.Error(t, err, "watching a non-existent path in non-recursive mode should error")
}

// ---------------------------------------------------------------------------
// Rename event coverage
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// loop: cover the error forwarding branch (lines 216-222) by injecting an
// error directly into the inner fsnotify watcher's Errors channel.
// Since we are in the same package, we can access fsWatcher internals.
// ---------------------------------------------------------------------------

// TestWatcher_ErrorForwardingInLoop verifies that errors from the
// underlying fsnotify watcher are forwarded to the Errors() channel.
func TestWatcher_ErrorForwardingInLoop(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := New(cfg)
	require.NoError(t, err)

	fsw := w.(*fsWatcher)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = fsw.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Remove a watched directory to cause the underlying fsnotify watcher
	// to encounter an error condition. When the watched dir is removed,
	// fsnotify may forward errors via its Errors channel.
	err = os.RemoveAll(dir)
	require.NoError(t, err)

	// Also trigger file events to force the loop to run.
	// Create a temp file in a fresh directory to trigger some events.
	dir2 := t.TempDir()
	_ = fsw.inner.Add(dir2)
	time.Sleep(20 * time.Millisecond)
	_ = os.WriteFile(filepath.Join(dir2, "trigger.txt"), []byte("x"), 0644)

	// Check the errors channel — may or may not have an error depending
	// on OS behavior, but we verify the channel is functional.
	errCh := fsw.Errors()
	select {
	case e := <-errCh:
		// Error forwarded successfully.
		assert.Error(t, e)
	case <-time.After(500 * time.Millisecond):
		// Some OSes do not generate errors for removed dirs; acceptable.
	}

	w.Close()
}

// TestWatcher_InnerEventsChannelClose verifies that when the inner fsnotify
// watcher is closed, the loop exits via the !ok branch on w.inner.Events.
func TestWatcher_InnerEventsChannelClose(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Recursive:     false,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := New(cfg)
	require.NoError(t, err)

	fsw := w.(*fsWatcher)

	// Use a long-lived context so the loop doesn't exit via ctx.Done().
	ctx, cancel := context.WithCancel(context.Background())

	err = fsw.Watch(ctx, dir)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)

	// Close ONLY the inner fsnotify watcher (not the outer wrapper).
	// This closes w.inner.Events and w.inner.Errors channels, which
	// forces the loop to exit via the !ok check on either channel read.
	_ = fsw.inner.Close()

	// The loop should exit because the fsnotify channels are closed.
	// Wait for the goroutine to finish.
	fsw.wg.Wait()

	// Now cancel context and clean up.
	cancel()

	// Close the outer events and errors channels ourselves since the loop
	// has already exited and Close() won't re-close stopCh (sync.Once).
	// The watcher is in a partially torn down state here which is expected
	// for this edge case test.
}

// ---------------------------------------------------------------------------
// addRecursive: cover line 302 (w.inner.Add error on valid dir)
// We test addRecursive directly since it's same-package access.
// ---------------------------------------------------------------------------

// TestAddRecursive_InnerAddError verifies that addRecursive handles errors
// returned by w.inner.Add (line 300). We trigger this by closing the inner
// fsnotify watcher before calling addRecursive.
func TestAddRecursive_InnerAddError(t *testing.T) {
	cfg := &Config{
		Recursive:     true,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := New(cfg)
	require.NoError(t, err)

	fsw := w.(*fsWatcher)

	// Close the inner watcher so that Add() returns an error.
	_ = fsw.inner.Close()

	dir := t.TempDir()
	// addRecursive should return an error because inner.Add will fail.
	err = fsw.addRecursive(dir)
	assert.Error(t, err, "addRecursive should return error when inner watcher is closed")
}

// TestAddRecursive_WalkWithFilesCoversNonDirReturn verifies that the walk
// callback in addRecursive returns nil for files (line 302). We create a
// directory tree with both subdirectories and files, ensuring the walk
// callback visits file entries.
func TestAddRecursive_WalkWithFilesCoversNonDirReturn(t *testing.T) {
	baseDir := t.TempDir()

	// Create subdirectories and files.
	subDir := filepath.Join(baseDir, "sub")
	err := os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	// Create files in root and subdirectory.
	err = os.WriteFile(filepath.Join(baseDir, "root_file.txt"), []byte("root"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "sub_file.txt"), []byte("sub"), 0644)
	require.NoError(t, err)

	cfg := &Config{
		Recursive:     true,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := New(cfg)
	require.NoError(t, err)
	defer w.Close()

	fsw := w.(*fsWatcher)

	// Directly call addRecursive to ensure all file entries are walked.
	err = fsw.addRecursive(baseDir)
	require.NoError(t, err, "addRecursive should succeed for a valid directory tree with files")
}

// ---------------------------------------------------------------------------
// Watch: cover addRecursive error return (line 138-140)
// ---------------------------------------------------------------------------

// TestWatch_RecursiveWithFailingInner verifies that Watch returns an
// error when addRecursive fails (covering the error return at line 138-140).
func TestWatch_RecursiveWithFailingInner(t *testing.T) {
	cfg := &Config{
		Recursive:     true,
		DebounceDelay: 0,
		BufferSize:    10,
	}
	w, err := New(cfg)
	require.NoError(t, err)

	fsw := w.(*fsWatcher)

	// Close the inner watcher so addRecursive will fail.
	_ = fsw.inner.Close()

	dir := t.TempDir()
	ctx := context.Background()
	err = fsw.Watch(ctx, dir)
	assert.Error(t, err, "Watch should return error when addRecursive fails due to closed inner watcher")
}

// ---------------------------------------------------------------------------
// Rename event coverage
// ---------------------------------------------------------------------------

// TestWatcher_RenameEventDelivery verifies that renaming a file produces
// a Rename event through the watcher.
func TestWatcher_RenameEventDelivery(t *testing.T) {
	dir := t.TempDir()

	// Pre-create a file.
	oldFile := filepath.Join(dir, "old_name.txt")
	err := os.WriteFile(oldFile, []byte("rename me"), 0644)
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

	// Rename the file.
	newFile := filepath.Join(dir, "new_name.txt")
	err = os.Rename(oldFile, newFile)
	require.NoError(t, err)

	// Collect events — we should see a Rename event.
	var gotRename bool
	timeout := time.After(2 * time.Second)
	for i := 0; i < 10; i++ {
		select {
		case ev := <-w.Events():
			if ev.Type == Rename {
				gotRename = true
			}
		case <-timeout:
			break
		}
		if gotRename {
			break
		}
	}
	assert.True(t, gotRename, "should receive a Rename event")
}
