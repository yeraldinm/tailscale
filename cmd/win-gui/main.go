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
	a := app.New()
	w := a.NewWindow("Tailscale")

	statusLabel := widget.NewLabel("Status: unknown")
	client := local.Client{}

	refresh := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		st, err := client.Status(ctx)
		if err != nil {
			statusLabel.SetText("Status: " + err.Error())
			return
		}
		statusLabel.SetText("Status: " + st.BackendState)
	}

	loginBtn := widget.NewButton("Login", func() {
		go client.StartLoginInteractive(context.Background())
	})
	connectBtn := widget.NewButton("Connect", func() {
		mp := &ipn.MaskedPrefs{Prefs: ipn.Prefs{WantRunning: true}, WantRunningSet: true}
		if _, err := client.EditPrefs(context.Background(), mp); err != nil {
			statusLabel.SetText("Connect error: " + err.Error())
		} else {
			refresh()
		}
	})
	disconnectBtn := widget.NewButton("Disconnect", func() {
		mp := &ipn.MaskedPrefs{Prefs: ipn.Prefs{WantRunning: false}, WantRunningSet: true}
		if _, err := client.EditPrefs(context.Background(), mp); err != nil {
			statusLabel.SetText("Disconnect error: " + err.Error())
		} else {
			refresh()
		}
	})
	refreshBtn := widget.NewButton("Refresh", func() { refresh() })

	w.SetContent(container.NewVBox(statusLabel, refreshBtn, loginBtn, connectBtn, disconnectBtn))

	refresh()
	w.ShowAndRun()
}
