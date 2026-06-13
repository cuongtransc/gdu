package analyze

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dundee/gdu/v5/internal/testdir"
)

// When the analyzer is stopped before scanning starts, the recursion into
// subdirectories must be skipped, yielding only the partially-scanned top dir.
func TestParallelAnalyzerStoppedBeforeScan(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateAnalyzer()
	analyzer.Stop()

	dir := analyzer.AnalyzeDir(
		"test_dir",
		func(_, _ string) bool { return false },
		func(_ string) bool { return false },
	).(*Dir)
	analyzer.GetDone().Wait()

	assert.True(t, analyzer.IsStopped())
	assert.Equal(t, "test_dir", dir.Name)
	assert.Equal(t, 0, len(dir.Files), "recursion into subdirs should be skipped")
}

func TestSequentialAnalyzerStoppedBeforeScan(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateSeqAnalyzer()
	analyzer.Stop()

	dir := analyzer.AnalyzeDir(
		"test_dir",
		func(_, _ string) bool { return false },
		func(_ string) bool { return false },
	).(*Dir)
	analyzer.GetDone().Wait()

	assert.True(t, analyzer.IsStopped())
	assert.Equal(t, 0, len(dir.Files), "recursion into subdirs should be skipped")
}

func TestStableAnalyzerStoppedBeforeScan(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	analyzer := CreateStableOrderAnalyzer()
	analyzer.Stop()

	dir := analyzer.AnalyzeDir(
		"test_dir",
		func(_, _ string) bool { return false },
		func(_ string) bool { return false },
	).(*Dir)
	analyzer.GetDone().Wait()

	assert.Equal(t, 0, len(dir.Files), "recursion into subdirs should be skipped")
}

// ResetProgress (used by the TUI before every rescan) must clear the stopped flag
// so a subsequent scan runs to completion.
func TestResetProgressClearsStopped(t *testing.T) {
	analyzer := CreateAnalyzer()
	analyzer.Stop()
	assert.True(t, analyzer.IsStopped())

	analyzer.ResetProgress()
	assert.False(t, analyzer.IsStopped())
}
