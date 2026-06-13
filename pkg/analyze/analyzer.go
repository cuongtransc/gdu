package analyze

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
)

// dirIdentity uniquely identifies a directory by its device and inode numbers.
// Two paths with the same identity (e.g. a macOS firmlink and its canonical
// path, or a bind mount) point at the same on-disk directory.
type dirIdentity struct {
	dev uint64
	ino uint64
}

// BaseAnalyzer provides common logic for all analyzers
type BaseAnalyzer struct {
	progressOutChan         chan common.CurrentProgress
	progressDoneChan        chan struct{}
	progressItemCount       atomic.Int64
	progressTotalUsage      atomic.Int64
	progressCurrentItemName atomic.Value
	doneChan                common.SignalGroup
	wait                    *WaitGroup
	ignoreDir               common.ShouldDirBeIgnored
	ignoreFileType          common.ShouldFileBeIgnored
	followSymlinks          bool
	gitAnnexedSize          bool
	matchesTimeFilterFn     common.TimeFilter
	archiveBrowsing         bool
	progressTicker          *time.Ticker
	stopped                 atomic.Bool
	dedupDirs               bool
	visitedDirs             *sync.Map
}

// Init initializes the BaseAnalyzer
func (a *BaseAnalyzer) Init() {
	a.progressOutChan = make(chan common.CurrentProgress, 1)
	a.progressDoneChan = make(chan struct{})
	a.doneChan = make(common.SignalGroup)
	a.wait = (&WaitGroup{}).Init()
	a.progressItemCount.Store(0)
	a.progressTotalUsage.Store(0)
	a.progressCurrentItemName.Store("")
	a.progressTicker = time.NewTicker(50 * time.Millisecond)
	a.stopped.Store(false)
	a.visitedDirs = &sync.Map{}
}

// SetDedupDirs enables skipping directories already scanned under a different
// path (same device+inode), preventing double-counting via macOS firmlinks,
// bind mounts or hard-linked directories.
func (a *BaseAnalyzer) SetDedupDirs(v bool) {
	a.dedupDirs = v
}

// shouldSkipDir reports whether a directory should be skipped entirely at scan
// entry: either a stop was requested (so backlogged goroutines exit immediately
// instead of doing expensive readdir/stat work) or the directory was already
// scanned under a different path (dedup). shouldStop is checked first so a
// stopped scan does not even stat the directory.
func (a *BaseAnalyzer) shouldSkipDir(path string) bool {
	return a.shouldStop() || a.shouldSkipVisited(path)
}

// shouldSkipVisited reports whether path should be skipped because dedup is
// enabled and the directory was already scanned under a different path.
func (a *BaseAnalyzer) shouldSkipVisited(path string) bool {
	return a.dedupDirs && !a.firstVisit(path)
}

// firstVisit reports whether path is being scanned for the first time. When
// deduplication is enabled and the directory's device+inode was already seen,
// it returns false so the caller can skip it. If the identity cannot be
// determined (e.g. Windows), it always returns true.
func (a *BaseAnalyzer) firstVisit(path string) bool {
	id, ok := getDirIdentity(path)
	if !ok {
		return true
	}
	_, loaded := a.visitedDirs.LoadOrStore(id, struct{}{})
	return !loaded
}

// Stop signals the analyzer to stop descending into not-yet-scanned directories.
// In-flight directories finish processing their already-listed entries, so the
// scan returns a partial tree of whatever was reached. It is safe to call from
// another goroutine and is idempotent.
func (a *BaseAnalyzer) Stop() {
	a.stopped.Store(true)
}

// IsStopped reports whether the analyzer was stopped early.
func (a *BaseAnalyzer) IsStopped() bool {
	return a.stopped.Load()
}

// shouldStop reports whether scanning should stop descending into new dirs.
func (a *BaseAnalyzer) shouldStop() bool {
	return a.stopped.Load()
}

// SetFollowSymlinks sets whether symlink to files should be followed
func (a *BaseAnalyzer) SetFollowSymlinks(v bool) {
	a.followSymlinks = v
}

// SetShowAnnexedSize sets whether to use annexed size of git-annex files
func (a *BaseAnalyzer) SetShowAnnexedSize(v bool) {
	a.gitAnnexedSize = v
}

// SetTimeFilter sets the time filter function for file inclusion
func (a *BaseAnalyzer) SetTimeFilter(matchesTimeFilterFn common.TimeFilter) {
	a.matchesTimeFilterFn = matchesTimeFilterFn
}

// SetArchiveBrowsing sets whether browsing of zip/jar/tar archives is enabled
func (a *BaseAnalyzer) SetArchiveBrowsing(v bool) {
	a.archiveBrowsing = v
}

// SetFileTypeFilter sets the file type filter function
func (a *BaseAnalyzer) SetFileTypeFilter(filter common.ShouldFileBeIgnored) {
	a.ignoreFileType = filter
}

// GetDone returns channel for checking when analysis is done
func (a *BaseAnalyzer) GetDone() common.SignalGroup {
	return a.doneChan
}

// ResetProgress resets the analyzer state
func (a *BaseAnalyzer) ResetProgress() {
	a.Init()
}

func (a *BaseAnalyzer) GetProgress() common.CurrentProgress {
	return common.CurrentProgress{
		CurrentItemName: a.progressCurrentItemName.Load().(string),
		ItemCount:       a.progressItemCount.Load(),
		TotalUsage:      a.progressTotalUsage.Load(),
	}
}

// UpdateProgress updates progress
func (a *BaseAnalyzer) UpdateProgress() {
	ticker := a.progressTicker
	defer ticker.Stop()
	for {
		select {
		case <-a.progressDoneChan:
			return
		case <-ticker.C:
			select {
			case a.progressOutChan <- a.GetProgress():
			default:
			}
		}
	}
}
