package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"omniq-monitoring/internal/storage"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func parsePayloadStrict(raw string) (any, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("payload is required")
	}

	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil, fmt.Errorf("invalid JSON payload")
	}

	switch v.(type) {
	case map[string]any, []any:
		return v, nil
	default:
		return nil, fmt.Errorf("payload must be a JSON object")
	}
}

func (a *App) showAddJobModal() {
	if strings.TrimSpace(a.currentQueue) == "" {
		a.logView.SetText("[yellow]No queue selected. Use 'l' to select a queue or 'q' to add one.")
		return
	}

	queueField := tview.NewInputField().
		SetLabel("Queue: ").
		SetText(a.currentQueue).
		SetFieldWidth(40)
	queueField.SetDisabled(true)

	payloadField := tview.NewTextArea().
		SetLabel("Payload: ").
		SetPlaceholder("JSON object/array required (no auto-wrap)")
	payloadField.SetSize(8, 0)

	errorField := tview.NewInputField().
		SetLabel("").
		SetFieldWidth(0).
		SetFieldTextColor(tcell.ColorRed).
		SetFieldBackgroundColor(tcell.ColorDefault)
	errorField.SetDisabled(true)

	validatePayload := func() {
		raw := strings.TrimSpace(payloadField.GetText())
		if raw == "" {
			errorField.SetText("")
			return
		}
		if _, err := parsePayloadStrict(raw); err != nil {
			errorField.SetText(err.Error())
		} else {
			errorField.SetText("")
		}
	}
	payloadField.SetChangedFunc(func() {
		validatePayload()
	})

	form := tview.NewForm().
		SetButtonsAlign(tview.AlignRight)
	form.SetBorder(true).SetTitle(" Add Job ")

	form.AddFormItem(queueField)
	form.AddFormItem(payloadField)
	form.AddFormItem(errorField)

	form.AddButton("Publish", func() {
		queue := strings.TrimSpace(a.currentQueue)
		if queue == "" {
			errorField.SetText("queue is required")
			return
		}

		payloadAny, err := parsePayloadStrict(payloadField.GetText())
		if err != nil {
			errorField.SetText(err.Error())
			a.tviewApp.SetFocus(payloadField)
			return
		}
		errorField.SetText("")

		a.pages.RemovePage("add_job")
		a.logView.SetText(fmt.Sprintf("[yellow]Publishing job to queue: %s ...", queue))

		go func() {
			jobID, err := a.monitor.PublishJob(context.Background(), queue, payloadAny)
			a.tviewApp.QueueUpdateDraw(func() {
				if err != nil {
					a.logView.SetText(fmt.Sprintf("[red]Publish failed: %v", err))
					return
				}

				a.currentQueue = queue
				_ = storage.AddQueue(queue)

				a.logView.SetText(fmt.Sprintf("[green]Published job %s to queue: %s", jobID, queue))
				a.updateData(context.Background())
			})
		}()
	})

	form.AddButton("Cancel", func() {
		a.pages.RemovePage("add_job")
	})

	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			a.pages.RemovePage("add_job")
			return nil
		}
		return event
	})

	frame := tview.NewFrame(form).
		SetBorders(0, 0, 0, 0, 0, 0).
		AddText("Tab/Shift+Tab: navigate fields | Enter: activate button | Esc: cancel", true, tview.AlignCenter, tcell.ColorGray)

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, 22, 0, true).
			AddItem(nil, 0, 1, false), 80, 1, true).
		AddItem(nil, 0, 1, false)

	a.pages.AddPage("add_job", modal, true, true)
	validatePayload()
	a.tviewApp.SetFocus(payloadField)
}
