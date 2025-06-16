// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause
//go:build windows

package main

import (
	"context"
	"time"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"tailscale.com/client/local"
	"tailscale.com/ipn"
)

func main() {
	gui := app.New()
	win := gui.NewWindow("Tailscale")

	statusLabel := widget.NewLabel("Status: unknown")
	connectBtn := widget.NewButton("Connect", nil)
	disconnectBtn := widget.NewButton("Disconnect", nil)

	win.SetContent(container.NewVBox(statusLabel, connectBtn, disconnectBtn))

	lc := local.Client{}

	connectBtn.OnTapped = func() {
		ctx := context.Background()
		lc.EditPrefs(ctx, &ipn.MaskedPrefs{Prefs: ipn.Prefs{WantRunning: true}, WantRunningSet: true})
	}

	disconnectBtn.OnTapped = func() {
		ctx := context.Background()
		lc.EditPrefs(ctx, &ipn.MaskedPrefs{Prefs: ipn.Prefs{WantRunning: false}, WantRunningSet: true})
	}

	go func() {
		for range time.Tick(5 * time.Second) {
			st, err := lc.Status(context.Background())
			state := "unknown"
			if err != nil {
				state = "error"
			} else {
				state = st.BackendState
			}
			statusLabel.SetText("Status: " + state)
			if state == ipn.Running.String() {
				connectBtn.Disable()
				disconnectBtn.Enable()
			} else {
				connectBtn.Enable()
				disconnectBtn.Disable()
			}
		}
	}()

	win.ShowAndRun()
}
