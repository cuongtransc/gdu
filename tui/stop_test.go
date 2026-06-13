package tui

import (
	"bytes"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"

	"github.com/dundee/gdu/v5/internal/testapp"
	"github.com/dundee/gdu/v5/internal/testdir"
)

func TestStopScanOnEscape(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)

	// Simulate an in-progress scan.
	ui.pages.AddPage("progress", tview.NewBox(), true, true)

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyEsc, 0, 0))

	assert.Nil(t, key, "Esc during scan should be consumed")
	assert.True(t, ui.Analyzer.IsStopped())
}

func TestStopScanOnCtrlC(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)

	ui.pages.AddPage("progress", tview.NewBox(), true, true)

	key := ui.keyPressed(tcell.NewEventKey(tcell.KeyCtrlC, 0, 0))

	assert.Nil(t, key, "Ctrl-C during scan should be consumed")
	assert.True(t, ui.Analyzer.IsStopped())
}

// Esc while browsing (no scan in progress) must not stop the analyzer.
func TestEscapeWhileBrowsingDoesNotStop(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)

	ui.keyPressed(tcell.NewEventKey(tcell.KeyEsc, 0, 0))

	assert.False(t, ui.Analyzer.IsStopped())
}

func TestSetScanTimeout(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)

	ui.SetScanTimeout(30 * 1e9) // 30s
	assert.Equal(t, int64(30*1e9), int64(ui.scanTimeout))
}

// End-to-end: a scan that is already stopped renders partial results and the
// summary modal through the normal AnalyzePath flow.
func TestAnalyzePathShowsPartialWhenStopped(t *testing.T) {
	fin := testdir.CreateTestDir()
	defer fin()

	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, false, true, false, false)
	ui.done = make(chan struct{})

	ui.Analyzer.Stop() // simulate a scan stopped before it descends

	err := ui.AnalyzePath("test_dir", nil)
	assert.Nil(t, err)

	<-ui.done
	for _, f := range ui.app.(*testapp.MockedApp).GetUpdateDraws() {
		f()
	}

	assert.True(t, ui.scanStopped)
	assert.True(t, ui.pages.HasPage("scanstopped"))
	assert.Contains(t, ui.currentDirLabel.GetText(true), "partial scan")
}

func TestShowScanStoppedModal(t *testing.T) {
	simScreen := testapp.CreateSimScreen()
	defer simScreen.Fini()

	app := testapp.CreateMockedApp(false)
	ui := CreateUI(app, simScreen, &bytes.Buffer{}, true, true, false, false)

	ui.showScanStopped("timeout", ui.Analyzer.GetProgress(), 5*1e9)

	assert.True(t, ui.pages.HasPage("scanstopped"))
}
