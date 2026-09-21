package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"tidy/config"
	"tidy/db"
	"tidy/engine"
	"tidy/fs"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
)

// Watcher monitors watch directories in real-time and organizes files as they settle.
type Watcher struct {
	cfg      *config.Config
	ledger   *db.Ledger
	debounce time.Duration
	fsw      *fsnotify.Watcher
	mu       sync.Mutex
	timers   map[string]*time.Timer
}

// NewWatcher initializes a file watcher for all configured watch directories.
func NewWatcher(cfg *config.Config, ledger *db.Ledger, debounce time.Duration) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize fsnotify watcher: %w", err)
	}

	w := &Watcher{
		cfg:      cfg,
		ledger:   ledger,
		debounce: debounce,
		fsw:      fsw,
		timers:   make(map[string]*time.Timer),
	}

	// Register all watch directories with the kernel watcher
	for _, dir := range cfg.WatchDirs {
		if err := fsw.Add(dir); err != nil {
			fsw.Close()
			return nil, fmt.Errorf("failed to watch directory %s: %w", dir, err)
		}
	}

	return w, nil
}

// Start begins listening for kernel events until ctx is canceled (e.g. Ctrl+C or SIGTERM).
func (w *Watcher) Start(ctx context.Context) error {
	defer w.fsw.Close()

	// Each daemon session gets a parent run ID in the ledger
	runID := uuid.New().String()
	if err := w.ledger.StartRun(runID, "watch"); err != nil {
		return fmt.Errorf("failed to initialize watch session in ledger: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown: Stop all active debounce timers
			w.mu.Lock()
			for _, t := range w.timers {
				t.Stop()
			}
			w.mu.Unlock()
			return nil

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return nil
			}
			fmt.Printf("\033[31m[WATCHER ERROR]\033[0m %v\n", err)

		case event, ok := <-w.fsw.Events:
			if !ok {
				return nil
			}
			w.handleEvent(event, runID)
		}
	}
}

// handleEvent receives raw filesystem events and manages the debouncing window.
func (w *Watcher) handleEvent(event fsnotify.Event, runID string) {
	// We only care about file creation and writes
	if !event.Has(fsnotify.Create) && !event.Has(fsnotify.Write) {
		return
	}

	filePath := event.Name

	// Strictly ignore directories (top-level only guarantee)
	if info, err := os.Stat(filePath); err == nil && info.IsDir() {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// If an existing debounce timer is running for this file, reset it!
	if timer, exists := w.timers[filePath]; exists {
		timer.Stop()
	}

	// Start a new debounce timer. Only fires when file writes stop for debounce duration.
	w.timers[filePath] = time.AfterFunc(w.debounce, func() {
		w.mu.Lock()
		delete(w.timers, filePath)
		w.mu.Unlock()

		w.processFile(filePath, runID)
	})
}

// processFile checks stability, evaluates rules, moves the file, and logs to ledger.
func (w *Watcher) processFile(filePath, runID string) {
	// 1. Verify file exists and is readable
	info, err := os.Stat(filePath)
	if err != nil {
		return // File deleted or transient temp file
	}

	if info.IsDir() || !info.Mode().IsRegular() {
		return
	}

	// 2. Extra stability check: ensure size isn't actively changing
	initialSize := info.Size()
	time.Sleep(100 * time.Millisecond)
	newInfo, err := os.Stat(filePath)
	if err != nil || newInfo.Size() != initialSize {
		// File is still being written to; let next event catch it
		return
	}

	fileName := filepath.Base(filePath)

	// 3. Evaluate matching rules
	match := engine.MatchFile(fileName, w.cfg)

	// Fallback content sniffing if enabled
	if !match.Matched && !match.Ignored && w.cfg.SniffContent {
		sniffedExt, err := engine.SniffFileExtension(filePath)
		if err == nil && sniffedExt != "" {
			match = engine.MatchFile(fileName+sniffedExt, w.cfg)
		}
	}

	if match.Ignored || !match.Matched {
		return
	}

	// 4. Resolve destination and collision
	targetDest := filepath.Join(match.DestDir, fileName)
	resolvedDest, shouldSkip, err := fs.ResolveCollision(targetDest, fs.StrategyRename)
	if err != nil || shouldSkip {
		return
	}

	// 5. Move the file atomically
	if err := fs.MoveFile(filePath, resolvedDest); err != nil {
		fmt.Printf("\033[31m[ERROR]\033[0m Failed to move %s: %v\n", fileName, err)
		return
	}

	// 6. Record move in SQLite ledger
	if _, err := w.ledger.RecordMove(runID, filePath, resolvedDest, newInfo.Size()); err != nil {
		fmt.Printf("\033[33m[WARNING]\033[0m Moved %s but failed to record in ledger: %v\n", fileName, err)
	}

	fmt.Printf("\033[32m✓ [WATCH]\033[0m %s ➔ \033[1m%s\033[0m (%s)\n",
		fileName, filepath.Base(resolvedDest), match.RuleName)
}
