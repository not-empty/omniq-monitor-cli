package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"omniq-monitoring/internal/config"
	"omniq-monitoring/internal/omniq"
	"omniq-monitoring/internal/storage"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type App struct {
	tviewApp     *tview.Application
	config       *config.Config
	monitor      *omniq.Monitor
	client       *omniq.Client
	currentQueue string

	headerView   *tview.TextView
	statsView    *tview.TextView
	commandsView *tview.TextView
	jobsView     *tview.TextView
	logView      *tview.TextView
	pages        *tview.Pages

	currentStatus string
	currentJobs   []omniq.Job

	cancelLoop context.CancelFunc
}

func NewApp(cfg *config.Config, client *omniq.Client, monitor *omniq.Monitor) *App {
	app := &App{
		tviewApp:      tview.NewApplication(),
		config:        cfg,
		client:        client,
		monitor:       monitor,
		currentQueue:  cfg.Queue,
		currentStatus: "failed",
	}

	app.setupUI()
	return app
}

func (a *App) setupUI() {
	a.headerView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)

	a.statsView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	a.statsView.SetBorder(true).SetTitle(" Queue Stats ")

	a.commandsView = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	a.commandsView.SetBorder(true).SetTitle(" Commands ")

	a.jobsView = tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	a.jobsView.SetBorder(true).SetTitle(" Jobs ")

	a.logView = tview.NewTextView().
		SetDynamicColors(true)
	a.logView.SetBorder(true).SetTitle(" Logs ")

	leftFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.statsView, 12, 1, false).
		AddItem(a.commandsView, 0, 1, false)

	middleFlex := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(leftFlex, 30, 0, false).
		AddItem(a.jobsView, 0, 1, true)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.headerView, 3, 1, false).
		AddItem(middleFlex, 0, 1, true).
		AddItem(a.logView, 5, 1, false)

	a.pages = tview.NewPages()
	a.pages.AddPage("main", flex, true, true)

	a.tviewApp.SetRoot(a.pages, true)

	a.tviewApp.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if _, ok := a.tviewApp.GetFocus().(*tview.InputField); ok {
			return event
		}

		if event.Key() == tcell.KeyCtrlC {
			a.Stop()
			return nil
		}
		if event.Rune() == 's' || event.Rune() == 'S' {
			a.showScanModal()
			return nil
		}
		if event.Rune() == 'a' || event.Rune() == 'A' {
			a.showManualEntryModal()
			return nil
		}
		if event.Rune() == 'l' || event.Rune() == 'L' {
			a.showSavedQueues()
			return nil
		}
		if event.Rune() == 'w' || event.Rune() == 'W' {
			a.currentStatus = "waiting"
			a.updateData(context.Background())
			return nil
		}
		if event.Rune() == 'c' || event.Rune() == 'C' {
			a.currentStatus = "completed"
			a.updateData(context.Background())
			return nil
		}
		if event.Rune() == 'f' || event.Rune() == 'F' {
			a.currentStatus = "failed"
			a.updateData(context.Background())
			return nil
		}
		if event.Rune() == 'r' || event.Rune() == 'R' {
			if a.currentStatus == "failed" {
				a.showRetryJobModal()
			} else {
				a.logView.SetText("[yellow]Retry only available for FAILED status")
			}
			return nil
		}
		if event.Rune() == 'b' || event.Rune() == 'B' {
			if a.currentStatus == "failed" {
				a.performBatchRetry()
			} else {
				a.logView.SetText("[yellow]Batch retry only available for FAILED status")
			}
			return nil
		}
		if event.Rune() == 'x' || event.Rune() == 'X' {
			a.showRemoveJobModal()
			return nil
		}
		if event.Rune() == 'd' || event.Rune() == 'D' {
			a.performBatchRemove()
			return nil
		}
		return event
	})
}

func (a *App) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelLoop = cancel
	go a.refreshLoop(ctx)
	return a.tviewApp.Run()
}

func (a *App) Stop() {
	if a.cancelLoop != nil {
		a.cancelLoop()
	}
	a.tviewApp.Stop()
}

func (a *App) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(a.config.Interval * float64(time.Second)))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.updateData(ctx)
			a.tviewApp.Draw()
		}
	}
}

func (a *App) updateData(ctx context.Context) {
	a.headerView.SetText(fmt.Sprintf("[yellow]Omniq Monitor[white] | Queue: [green]%s[white] | Status: [cyan]%s[white] | Interval: %.2fs",
		a.currentQueue, strings.ToUpper(a.currentStatus), a.config.Interval))

	counts, _ := a.monitor.GetCounts(ctx, a.currentQueue)
	a.logView.SetText(fmt.Sprintf("[green]Last update: %s", time.Now().Format("15:04:05")))

	pausedText := "[green]NO"
	if counts.Paused {
		pausedText = "[red]YES"
	}

	statsText := fmt.Sprintf(`
 Paused:    %s
 Waiting:   [cyan]%d[white]
 Active:    [green]%d[white]
 Delayed:   [yellow]%d[white]
 Completed: [blue]%d[white]
 Failed:    [red]%d[white]
`, pausedText, counts.Waiting, counts.Active, counts.Delayed, counts.Completed, counts.Failed)

	a.statsView.SetText(statsText)

	a.commandsView.SetText(`
[blue]s[white]: Scan Queues
[blue]a[white]: Add Job
[blue]l[white]: List Queues

[yellow]Jobs View:[white]
[blue]w[white]: Waiting
[blue]c[white]: Completed
[blue]f[white]: Failed

[yellow]Actions:[white]
[blue]r[white]: Retry Single
[blue]b[white]: Batch Retry (Fail)
[blue]x[white]: Remove Single
[blue]d[white]: Remove All (Status)
`)

	jobs, _ := a.monitor.GetJobs(ctx, a.currentQueue, a.currentStatus, 100)
	a.currentJobs = jobs
	a.jobsView.SetTitle(fmt.Sprintf(" Jobs (%s - showing last %d) ", strings.ToUpper(a.currentStatus), len(jobs)))

	var sb strings.Builder
	for _, j := range jobs {
		sb.WriteString(fmt.Sprintf("[yellow]ID:[white] %s  [yellow]State:[white] %s\n", j.ID, j.State))

		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, []byte(j.Payload), "", "  "); err == nil {
			sb.WriteString(fmt.Sprintf("[cyan]Payload:[white]\n%s\n", prettyJSON.String()))
		} else {
			sb.WriteString(fmt.Sprintf("[cyan]Payload:[white]\n%s\n", j.Payload))
		}

		if j.LastError != "" {
			sb.WriteString(fmt.Sprintf("[red]Error:[white]\n%s\n", j.LastError))
		}
		sb.WriteString("[gray]--------------------------------------------------[white]\n")
	}
	a.jobsView.SetText(sb.String())
}

func (a *App) showRetryJobModal() {
	if len(a.currentJobs) == 0 {
		a.logView.SetText("[yellow]No jobs available to retry.")
		return
	}

	list := tview.NewList()
	list.SetTitle(" Select FAILED Job to Retry ").SetBorder(true)

	for _, j := range a.currentJobs {
		jobID := j.ID
		list.AddItem(jobID, j.Payload, 0, func() {
			go func() {
				err := a.monitor.RetryJob(context.Background(), a.currentQueue, jobID)
				a.tviewApp.QueueUpdateDraw(func() {
					if err != nil {
						a.logView.SetText(fmt.Sprintf("[red]Retry failed: %v", err))
					} else {
						a.logView.SetText(fmt.Sprintf("[green]Retried job: %s", jobID))
						a.updateData(context.Background())
					}
				})
			}()
			a.pages.RemovePage("retry_job_selection")
		})
	}

	list.AddItem("Cancel", "", 'c', func() {
		a.pages.RemovePage("retry_job_selection")
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(list, 20, 1, true).
			AddItem(nil, 0, 1, false), 60, 1, true).
		AddItem(nil, 0, 1, false)

	a.pages.AddPage("retry_job_selection", modal, true, true)
}

func (a *App) performBatchRetry() {
	if len(a.currentJobs) == 0 {
		a.logView.SetText("[yellow]No listed jobs to retry.")
		return
	}

	ids := make([]string, len(a.currentJobs))
	for i, j := range a.currentJobs {
		ids[i] = j.ID
	}

	a.logView.SetText(fmt.Sprintf("[yellow]Retrying %d listed jobs...", len(ids)))
	go func() {
		ctx := context.Background()
		n, err := a.monitor.RetryFailedBatch(ctx, a.currentQueue, ids)

		a.tviewApp.QueueUpdateDraw(func() {
			if err != nil {
				a.logView.SetText(fmt.Sprintf("[red]Batch retry failed: %v", err))
			} else {
				a.logView.SetText(fmt.Sprintf("[green]Retried %d listed jobs", n))
				a.updateData(context.Background())
			}
		})
	}()
}

func (a *App) showRemoveJobModal() {
	if len(a.currentJobs) == 0 {
		a.logView.SetText(fmt.Sprintf("[yellow]No %s jobs available to remove.", a.currentStatus))
		return
	}

	list := tview.NewList()
	list.SetTitle(fmt.Sprintf(" Select %s Job to Remove ", strings.ToUpper(a.currentStatus))).SetBorder(true)

	for _, j := range a.currentJobs {
		jobID := j.ID
		list.AddItem(jobID, j.Payload, 0, func() {
			lane := a.currentStatus
			if lane == "waiting" {
				lane = "wait"
			}
			go func() {
				err := a.monitor.RemoveJob(context.Background(), a.currentQueue, jobID, lane)
				a.tviewApp.QueueUpdateDraw(func() {
					if err != nil {
						a.logView.SetText(fmt.Sprintf("[red]Remove failed: %v", err))
					} else {
						a.logView.SetText(fmt.Sprintf("[green]Removed job: %s from %s", jobID, a.currentStatus))
						a.updateData(context.Background())
					}
				})
			}()
			a.pages.RemovePage("remove_job_selection")
		})
	}

	list.AddItem("Cancel", "", 'c', func() {
		a.pages.RemovePage("remove_job_selection")
	})

	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(list, 20, 1, true).
			AddItem(nil, 0, 1, false), 60, 1, true).
		AddItem(nil, 0, 1, false)

	a.pages.AddPage("remove_job_selection", modal, true, true)
}

func (a *App) performBatchRemove() {
	if len(a.currentJobs) == 0 {
		a.logView.SetText(fmt.Sprintf("[yellow]No listed %s jobs to remove.", a.currentStatus))
		return
	}

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Are you sure you want to remove the %d LISTED %s jobs?", len(a.currentJobs), a.currentStatus)).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Yes" {
				a.pages.RemovePage("confirm_remove_batch")
				a.logView.SetText(fmt.Sprintf("[yellow]Removing %d listed jobs...", len(a.currentJobs)))

				lane := a.currentStatus
				if lane == "waiting" {
					lane = "wait"
				}

				ids := make([]string, len(a.currentJobs))
				for i, j := range a.currentJobs {
					ids[i] = j.ID
				}

				go func() {
					ctx := context.Background()
					n, err := a.monitor.RemoveJobsBatch(ctx, a.currentQueue, a.currentStatus, ids)

					a.tviewApp.QueueUpdateDraw(func() {
						if err != nil {
							a.logView.SetText(fmt.Sprintf("[red]Batch remove failed: %v", err))
						} else {
							a.logView.SetText(fmt.Sprintf("[green]Removed %d listed jobs from %s", n, a.currentStatus))
							a.updateData(context.Background())
						}
					})
				}()
			} else {
				a.pages.RemovePage("confirm_remove_batch")
			}
		})

	a.pages.AddPage("confirm_remove_batch", modal, false, true)
}

func (a *App) showScanModal() {
	modal := tview.NewModal().
		SetText("Scanning for queues uses the SCAN command and may cause high load on Redis.\n\nDo you want to proceed?").
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Yes" {
				a.pages.RemovePage("scan_warning")
				a.performScan()
			} else {
				a.pages.RemovePage("scan_warning")
			}
		})

	a.pages.AddPage("scan_warning", modal, false, true)
}

func (a *App) performScan() {
	a.logView.SetText("[yellow]Scanning for queues...")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		queues, err := a.monitor.ScanQueues(ctx, "")
		if err != nil {
			a.tviewApp.QueueUpdateDraw(func() {
				a.logView.SetText(fmt.Sprintf("[red]Scan failed: %v", err))
			})
			return
		}

		if err := storage.SaveQueues(queues); err != nil {
			a.tviewApp.QueueUpdateDraw(func() {
				a.logView.SetText(fmt.Sprintf("[red]Failed to save queues: %v", err))
			})
			return
		}

		a.tviewApp.QueueUpdateDraw(func() {
			a.logView.SetText(fmt.Sprintf("[green]Scan complete. Found %d queues. Press 'l' to list.", len(queues)))
		})
	}()
}

func (a *App) showQueueList(queues []string) {
	list := tview.NewList()
	list.SetTitle(" Select Queue ").SetBorder(true)

	for _, q := range queues {
		list.AddItem(q, "", 0, func() {
			selected := q
			qName := selected
			a.currentQueue = qName
			a.pages.RemovePage("queue_list")
			a.logView.SetText(fmt.Sprintf("[green]Switched to queue: %s", qName))
		})
	}

	if len(queues) == 0 {
		list.AddItem("No queues found", "", 0, nil)
	}

	list.AddItem("Cancel", "", 'c', func() {
		a.pages.RemovePage("queue_list")
	})

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(list, 20, 1, true).
			AddItem(nil, 0, 1, false), 40, 1, true).
		AddItem(nil, 0, 1, false)

	a.pages.AddPage("queue_list", flex, true, true)
}

func (a *App) showSavedQueues() {
	queues, err := storage.LoadQueues()
	if err != nil {
		a.logView.SetText(fmt.Sprintf("[red]Failed to load queues: %v", err))
		return
	}
	a.showQueueList(queues)
}
