package analyze

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dundee/gdu/v5/internal/testdir"
	"github.com/dundee/gdu/v5/pkg/fs"
)

func noIgnore(_, _ string) bool { return false }
func noFilter(_ string) bool    { return false }

// firstVisit returns true the first time a path's device+inode is seen and
// false afterwards, regardless of the path string used to reach it.
func TestFirstVisit(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	a := CreateAnalyzer()
	assert.True(t, a.firstVisit("test_dir"))
	assert.False(t, a.firstVisit("test_dir"))
}

// With dedup enabled, a directory whose inode was already visited is skipped
// and contributes an empty subtree (simulating a firmlink/bind-mount alias).
func TestDedupSkipsVisitedDir(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	a := CreateAnalyzer()
	a.SetDedupDirs(true)
	assert.True(t, a.firstVisit("test_dir/nested")) // pre-mark as already scanned

	dir := a.AnalyzeDir("test_dir", noIgnore, noFilter).(*Dir)
	a.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	var nested *Dir
	for _, f := range dir.Files {
		if f.GetName() == "nested" {
			nested = f.(*Dir)
		}
	}
	assert.NotNil(t, nested)
	assert.Equal(t, 0, len(nested.Files), "already-visited dir should be skipped (empty)")
}

// Dedup must not change results for a normal tree with no aliased directories.
func TestDedupNormalScanUnchanged(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	a := CreateAnalyzer()
	a.SetDedupDirs(true)
	dir := a.AnalyzeDir("test_dir", noIgnore, noFilter).(*Dir)
	a.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	assert.Equal(t, int64(7), dir.Size)
	assert.Equal(t, int64(5), dir.ItemCount)
}

func TestSequentialDedupNormalScanUnchanged(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	a := CreateSeqAnalyzer()
	a.SetDedupDirs(true)
	dir := a.AnalyzeDir("test_dir", noIgnore, noFilter).(*Dir)
	a.GetDone().Wait()
	dir.UpdateStats(make(fs.HardLinkedItems))

	assert.Equal(t, int64(7), dir.Size)
	assert.Equal(t, int64(5), dir.ItemCount)
}

// ResetProgress must clear the visited set so a rescan is not falsely skipped.
func TestResetProgressClearsVisited(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	a := CreateAnalyzer()
	assert.True(t, a.firstVisit("test_dir"))
	a.ResetProgress()
	assert.True(t, a.firstVisit("test_dir"), "visited set should be cleared on reset")
}
