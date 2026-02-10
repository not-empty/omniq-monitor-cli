package ui

import (
	"fmt"
	"omniq-monitoring/internal/storage"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (a *App) showManualEntryModal() {
	input := tview.NewInputField().
		SetLabel("Enter Queue Name: ").
		SetFieldWidth(30).
		SetAcceptanceFunc(nil)

	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			text := input.GetText()
			if text != "" {
				if err := storage.AddQueue(text); err != nil {
					a.logView.SetText(fmt.Sprintf("[red]Failed to save queue: %v", err))
				} else {
					a.currentQueue = text
					a.logView.SetText(fmt.Sprintf("[green]Switched to queue: %s", text))
				}
			}
			a.pages.RemovePage("manual_entry")
		} else if key == tcell.KeyEsc {
			a.pages.RemovePage("manual_entry")
		}
	})

	frame := tview.NewFrame(input).
		SetBorders(0, 0, 0, 0, 0, 0).
		AddText("Manual Add Queue", true, tview.AlignCenter, tcell.ColorYellow).
		AddText("(Enter to confirm, Esc to cancel)", false, tview.AlignCenter, tcell.ColorWhite)
	frame.SetBorder(true)

	a.pages.AddPage("manual_entry", tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, 7, 1, true).
			AddItem(nil, 0, 1, false), 50, 1, true).
		AddItem(nil, 0, 1, false), true, true)
}
