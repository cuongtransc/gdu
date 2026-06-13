package tui

import (
	"fmt"
	"os"
	"time"

	"github.com/dundee/gdu/v5/internal/common"
	"github.com/dundee/gdu/v5/pkg/path"
)

func (ui *UI) updateProgress(analyzer common.Analyzer, doneChan common.SignalGroup) {
	color := "[white:black:b]"
	if ui.UseColors {
		color = "[red:black:b]"
	}

	deviceSize := ui.currentDeviceSize
	showBar := ui.showDiskProgressBar

	start := time.Now()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-doneChan:
			if deviceSize > 0 && showBar {
				clearTerminalProgress()
				ui.currentDeviceSize = 0
			}
			stopped := analyzer.IsStopped()
			sinceStop := ui.timeSinceStop().Round(time.Second)
			ui.app.QueueUpdateDraw(func() {
				ui.progress.SetTitle(" Finalizing... ")
				if stopped {
					ui.progress.SetText(fmt.Sprintf(
						"Scan stopped after %s — calculating disk usage for partial results...",
						sinceStop))
				} else {
					ui.progress.SetText("Calculating disk usage...")
				}
			})
			return
		case <-ticker.C:
		}

		// Once a stop was requested, the scan still needs to drain in-flight
		// directories and finalize stats, which can take a moment. Switch the
		// modal to a clear "stopping" state (with live elapsed time) so the
		// interrupt is acknowledged immediately instead of looking frozen.
		if analyzer.IsStopped() {
			ui.markStopTime()
			ui.app.QueueUpdateDraw(ui.showScanStopping)
			continue
		}

		progress := analyzer.GetProgress()

		func(itemCount int64, totalUsage int64, currentItem string) {
			delta := time.Since(start).Round(time.Second)

			if deviceSize > 0 && showBar {
				percent := int(totalUsage * 100 / deviceSize)
				writeTerminalProgress(percent)
				if ui.progressBar != nil {
					ui.progressBar.SetProgress(percent)
				}
			}

			ui.app.QueueUpdateDraw(func() {
				ui.progress.SetText("Total items: " +
					color +
					common.FormatNumber(int64(itemCount)) +
					"[white:black:-], size: " +
					color +
					ui.formatSize(totalUsage, false, false) +
					"[white:black:-], elapsed time: " +
					color +
					delta.String() +
					"[white:black:-]\nCurrent item: [white:black:b]" +
					path.ShortenPath(currentItem, ui.currentItemNameMaxLen) +
					"[white:black:-]\n\nPress Esc or Ctrl-C to stop scanning and show partial results")
			})
		}(progress.ItemCount, progress.TotalUsage, progress.CurrentItemName)
	}
}

// showScanStopping switches the progress modal to a "stopping" state so a
// requested interrupt (Esc/Ctrl-C/timeout) is acknowledged immediately, even
// while the scan drains and partial stats are finalized. Must run on the UI
// goroutine.
func (ui *UI) showScanStopping() {
	if ui.progress == nil {
		return
	}
	ui.progress.SetTitle(" Stopping... ")
	ui.progress.SetText(fmt.Sprintf(
		"Scan interrupted %s ago — stopping scan and preparing partial results...\n"+
			"(draining in-flight directories; this is bounded and should be quick)",
		ui.timeSinceStop().Round(time.Second)))
}

// writeTerminalProgress emits an OSC 9;4 sequence to update the terminal
// tab/taskbar progress indicator. percent must be in the range [0, 100].
// This sequence is supported by Windows Terminal, ConEmu, and compatible
// terminals.  Writing to stderr ensures it reaches the terminal even when
// the TUI has taken over stdout/stdin via tcell.
func writeTerminalProgress(percent int) {
	fmt.Fprintf(os.Stderr, "\x1b]9;4;1;%d\x1b\\", percent)
}

// clearTerminalProgress removes the terminal tab/taskbar progress indicator.
func clearTerminalProgress() {
	fmt.Fprintf(os.Stderr, "\x1b]9;4;0;0\x1b\\")
}
