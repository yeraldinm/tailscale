// Copyright (c) Tailscale Inc & AUTHORS
// SPDX-License-Identifier: BSD-3-Clause
//go:build windows

package main

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"tailscale.com/client/local"
	"tailscale.com/ipn"
	"tailscale.com/tailcfg"
)

func main() {
	// Set up logging
	logFile, err := os.Create("tailscale_gui.log")
	if err != nil {
		fmt.Printf("Error creating log file: %v\n", err)
		return
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	log.Println("Starting Tailscale GUI...")

	a := app.New()
	w := a.NewWindow("Tailscale")

	statusLabel := widget.NewLabel("Status: unknown")
	ipLabel := widget.NewLabel("IP: Not connected")
	netInfo := widget.NewLabel("Network: Not connected")
	rxLabel := widget.NewLabel("Received: 0 B")
	txLabel := widget.NewLabel("Sent: 0 B")
	connectBtn := widget.NewButton("Connect", nil)
	disconnectBtn := widget.NewButton("Disconnect", nil)

	// Add login/logout button
	loginBtn := widget.NewButton("Login", nil)
	logoutBtn := widget.NewButton("Logout", nil)
	loginBtn.Hide()
	logoutBtn.Hide()

	// Shared routes
	routesBox := container.NewVBox()
	addRouteEntry := widget.NewEntry()
	addRouteEntry.SetPlaceHolder("e.g. 192.168.1.0/24")
	addRouteBtn := widget.NewButtonWithIcon("Add Route", theme.ContentAddIcon(), nil)
	addRouteRow := container.NewHBox(addRouteEntry, addRouteBtn)

	// Peers
	peersBox := container.NewVBox(
		widget.NewLabelWithStyle("Connected Peers", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	lc := local.Client{}

	var routes []string
	// var peers []string

	var updateStatus func()
	updateRoutesUI := func(routes []string) {
		routesBox.Objects = nil
		if len(routes) == 0 {
			routesBox.Add(widget.NewLabel("No routes advertised."))
		} else {
			for i, r := range routes {
				idx := i
				row := container.NewHBox(
					widget.NewLabel(r),
					layout.NewSpacer(),
					widget.NewButtonWithIcon("Remove", theme.DeleteIcon(), func() {
						prefs, err := lc.GetPrefs(context.Background())
						if err != nil {
							dialog.ShowError(fmt.Errorf("Failed to get preferences: %v", err), w)
							return
						}
						if idx < 0 || idx >= len(prefs.AdvertiseRoutes) {
							dialog.ShowError(fmt.Errorf("Invalid route index"), w)
							return
						}
						prefs.AdvertiseRoutes = append(prefs.AdvertiseRoutes[:idx], prefs.AdvertiseRoutes[idx+1:]...)
						_, err = lc.EditPrefs(context.Background(), &ipn.MaskedPrefs{
							Prefs:              *prefs,
							AdvertiseRoutesSet: true,
						})
						if err != nil {
							dialog.ShowError(fmt.Errorf("Failed to remove route: %v", err), w)
							return
						}
						updateStatus()
					}),
				)
				routesBox.Add(row)
			}
		}
		routesBox.Refresh()
	}

	statusDetails := widget.NewLabel("")
	statusDetails.Hide()
	showDetails := false
	insideDetailsbtn := widget.NewButton("", func() {})
	detailsBtn := widget.NewButton("Show Details", func() {
		showDetails = !showDetails
		if showDetails {
			statusDetails.Show()
			insideDetailsbtn.SetText("Hide Details")
		} else {
			statusDetails.Hide()
			insideDetailsbtn.SetText("Show Details")
		}
	})

	exitNodeNames := []string{"None"}
	exitNodeMap := map[string]string{}
	exitNodeSelect := widget.NewSelect(exitNodeNames, nil)
	exitNodeSelect.SetSelected("None")
	clearExitBtn := widget.NewButton("Clear Exit Node", nil)
	clearExitBtn.Hide()

	// Settings dialog controls
	magicDNSCheck := widget.NewCheck("Use Tailscale DNS Settings", nil)
	acceptRoutesCheck := widget.NewCheck("Use Tailscale Subnets", nil)
	allowLocalNetworksCheck := widget.NewCheck("Allow Local Networks", nil)
	runExitNodeCheck := widget.NewCheck("Run Exit Node", nil)
	runSSHCheck := widget.NewCheck("Enable SSH", nil)

	tagsEntry := widget.NewEntry()
	tagsEntry.SetPlaceHolder("tag:example,tag:server")
	saveTagsBtn := widget.NewButton("Save Tags", nil)
	tagsRow := container.NewHBox(widget.NewLabel("Node Tags"), tagsEntry, saveTagsBtn)

	keyExpiryLabel := widget.NewLabel("")

	settingsForm := container.NewVBox(
		magicDNSCheck,
		acceptRoutesCheck,
		allowLocalNetworksCheck,
		runExitNodeCheck,
		runSSHCheck,
		tagsRow,
		keyExpiryLabel,
	)

	settingsDialog := dialog.NewCustom("Settings", "Close", settingsForm, w)

	// Menu bar
	fileMenu := fyne.NewMenu("File",
		fyne.NewMenuItem("Exit", func() { a.Quit() }),
	)
	settingsMenu := fyne.NewMenu("Settings",
		fyne.NewMenuItem("Preferences...", func() { settingsDialog.Show() }),
	)
	mainMenu := fyne.NewMainMenu(fileMenu, settingsMenu)
	w.SetMainMenu(mainMenu)

	updateStatus = func() {
		st, err := lc.Status(context.Background())
		if err != nil {
			log.Printf("Error getting status: %v", err)
			statusLabel.SetText("Status: error")
			ipLabel.SetText("IP: Error")
			netInfo.SetText("Network: Error")
			peersBox.Objects = nil
			updateRoutesUI(nil)
			keyExpiryLabel.SetText("Key Expiry: N/A")
			statusDetails.SetText("")
			return
		}
		statusLabel.SetText("Status: " + st.BackendState)
		if st.Self != nil && len(st.Self.TailscaleIPs) > 0 {
			ipLabel.SetText("IP: " + st.Self.TailscaleIPs[0].String())
		} else {
			ipLabel.SetText("IP: Not connected")
		}
		if st.Self != nil {
			netInfo.SetText(fmt.Sprintf("Network: %s", st.Self.DNSName))
			rxLabel.SetText(fmt.Sprintf("Received: %d B", st.Self.RxBytes))
			txLabel.SetText(fmt.Sprintf("Sent: %d B", st.Self.TxBytes))
		} else {
			netInfo.SetText("Network: Not connected")
			rxLabel.SetText("Received: 0 B")
			txLabel.SetText("Sent: 0 B")
		}

		// peers = make([]string, 0)
		for _, peer := range st.Peer {
			peerLabel := widget.NewLabel(fmt.Sprintf("%s (%s)", peer.DNSName, peer.TailscaleIPs[0]))
			stats := fmt.Sprintf("Rx: %d B  Tx: %d B", peer.RxBytes, peer.TxBytes)
			if !peer.LastHandshake.IsZero() {
				stats += fmt.Sprintf("  Last: %s", peer.LastHandshake.Format(time.RFC3339))
			}
			statsLabel := widget.NewLabel(stats)
			addr, err := netip.ParseAddr(peer.TailscaleIPs[0].String())
			if err != nil {
				fyne.CurrentApp().SendNotification(&fyne.Notification{
					Title:   "Ping Error",
					Content: fmt.Sprintf("Invalid IP: %v", err),
				})
				continue
			}
			pingBtn := widget.NewButton("Ping", func() {
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					res, err := lc.Ping(ctx, addr, tailcfg.PingICMP)
					fyne.CurrentApp().SendNotification(&fyne.Notification{
						Title:   "Ping Result",
						Content: fmt.Sprintf("Ping %s: %v (err: %v)", peer.TailscaleIPs[0], res, err),
					})
					// Or use dialog.ShowInformation if you prefer a dialog:
					// dialog.ShowInformation("Ping Result", fmt.Sprintf("Ping %s: %v (err: %v)", p, res, err), w)
				}()
			})
			row := container.NewVBox(container.NewHBox(peerLabel, pingBtn), statsLabel)
			peersBox.Add(row)
		}
		prefs, err := lc.GetPrefs(context.Background())
		if err == nil && prefs != nil {
			routes = make([]string, 0, len(prefs.AdvertiseRoutes))
			for _, p := range prefs.AdvertiseRoutes {
				routes = append(routes, p.String())
			}
			updateRoutesUI(routes)
			magicDNSCheck.SetChecked(prefs.CorpDNS)
			acceptRoutesCheck.SetChecked(prefs.RouteAll)
			allowLocalNetworksCheck.SetChecked(prefs.ExitNodeAllowLANAccess)
			runSSHCheck.SetChecked(prefs.RunSSH)
			tagsEntry.SetText(strings.Join(prefs.AdvertiseTags, ","))
			if st.Self != nil && st.Self.KeyExpiry != nil {
				keyExpiryLabel.SetText("Key Expiry: " + st.Self.KeyExpiry.Format(time.RFC1123))
			} else {
				keyExpiryLabel.SetText("Key Expiry: N/A")
			}
			// This host is an exit node if it advertises both 0.0.0.0/0 and ::/0
			isExitNode := false
			if prefs != nil {
				has4, has6 := false, false
				for _, r := range prefs.AdvertiseRoutes {
					if r.String() == "0.0.0.0/0" {
						has4 = true
					}
					if r.String() == "::/0" {
						has6 = true
					}
				}
				isExitNode = has4 && has6
			}
			runExitNodeCheck.SetChecked(isExitNode)

			// println(prefs.ExitNodeAllowLANAccess)
		} else {
			updateRoutesUI(nil)
			keyExpiryLabel.SetText("Key Expiry: N/A")
		}
		if st.BackendState == ipn.Running.String() {
			connectBtn.Disable()
			disconnectBtn.Enable()
		} else {
			connectBtn.Enable()
			disconnectBtn.Disable()
		}

		// Show login/logout button based on user status
		var userEmail string
		if len(st.User) > 0 {
			logoutBtn.Show()
			loginBtn.Hide()
			// Get the first user profile (there is usually only one)
			for _, profile := range st.User {
				userEmail = profile.LoginName
				break
			}
		} else {
			loginBtn.Show()
			logoutBtn.Hide()
		}

		// Exit node selection
		var currentExitNode string
		if err == nil && prefs != nil && prefs.ExitNodeID != "" {
			currentExitNode = string(prefs.ExitNodeID)
		} else {
			currentExitNode = "None"
		}
		exitNodeNames = []string{"None"}
		exitNodeMap = map[string]string{}
		for _, peer := range st.Peer {
			if peer.ExitNode {
				name := peer.HostName
				if name == "" {
					name = fmt.Sprintf("Peer %s", peer.ID)
				}
				exitNodeNames = append(exitNodeNames, name)
				exitNodeMap[name] = string(peer.ID)
				if currentExitNode == string(peer.ID) {
					exitNodeSelect.SetSelected(name)
				}
			}
		}
		if currentExitNode == "None" {
			exitNodeSelect.SetSelected("None")
		}
		if currentExitNode != "None" {
			clearExitBtn.Show()
		} else {
			clearExitBtn.Hide()
		}

		// Device details
		var details []string
		if st.Self != nil {
			details = append(details, fmt.Sprintf("Hostname: %s", st.Self.HostName))
			details = append(details, fmt.Sprintf("OS: %s", st.Self.OS))
			details = append(details, fmt.Sprintf("Last seen: %s", st.Self.LastSeen))
			if st.Self.ExitNode {
				details = append(details, "This device is an Exit Node")
			}
			if st.Self.Relay != "" {
				details = append(details, fmt.Sprintf("Relay: %s", st.Self.Relay))
			}
		}
		if userEmail != "" {
			details = append(details, fmt.Sprintf("User: %s", userEmail))
		}
		if currentExitNode != "None" {
			details = append(details, fmt.Sprintf("Current Exit Node: %s", exitNodeSelect.Selected))
		}
		statusDetails.SetText(strings.Join(details, "\n"))
	}

	connectBtn.OnTapped = func() {
		log.Println("Connect button tapped")
		ctx := context.Background()
		_, err := lc.EditPrefs(ctx, &ipn.MaskedPrefs{Prefs: ipn.Prefs{WantRunning: true}, WantRunningSet: true})
		if err != nil {
			log.Printf("Error connecting: %v", err)
			statusLabel.SetText("Error: " + err.Error())
		}
	}
	disconnectBtn.OnTapped = func() {
		log.Println("Disconnect button tapped")
		ctx := context.Background()
		_, err := lc.EditPrefs(ctx, &ipn.MaskedPrefs{Prefs: ipn.Prefs{WantRunning: false}, WantRunningSet: true})
		if err != nil {
			log.Printf("Error disconnecting: %v", err)
			statusLabel.SetText("Error: " + err.Error())
		}
	}

	loginBtn.OnTapped = func() {
		log.Println("Login button tapped")
		ctx := context.Background()
		err := lc.StartLoginInteractive(ctx)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Login failed: %v", err), w)
			return
		}
		updateStatus()
	}
	logoutBtn.OnTapped = func() {
		log.Println("Logout button tapped")
		ctx := context.Background()
		err := lc.Logout(ctx)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Logout failed: %v", err), w)
			return
		}
		updateStatus()
	}

	exitNodeSelect.OnChanged = func(selected string) {
		if selected == "None" {
			return
		}
		id, ok := exitNodeMap[selected]
		if !ok {
			return
		}
		prefs, err := lc.GetPrefs(context.Background())
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to get preferences: %v", err), w)
			return
		}
		prefs.ExitNodeID = tailcfg.StableNodeID(id)
		_, err = lc.EditPrefs(context.Background(), &ipn.MaskedPrefs{
			Prefs:         *prefs,
			ExitNodeIDSet: true,
		})
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to set exit node: %v", err), w)
			return
		}
		updateStatus()
	}

	clearExitBtn.OnTapped = func() {
		prefs, err := lc.GetPrefs(context.Background())
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to get preferences: %v", err), w)
			return
		}
		prefs.ExitNodeID = ""
		_, err = lc.EditPrefs(context.Background(), &ipn.MaskedPrefs{
			Prefs:         *prefs,
			ExitNodeIDSet: true,
		})
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to clear exit node: %v", err), w)
			return
		}
		updateStatus()
	}

	onAddRoute := func() {
		cidr := strings.TrimSpace(addRouteEntry.Text)
		if cidr == "" {
			dialog.ShowError(fmt.Errorf("Please enter a subnet in CIDR format."), w)
			return
		}
		_, err := netip.ParsePrefix(cidr)
		if err != nil {
			dialog.ShowError(fmt.Errorf("Invalid CIDR: %v", err), w)
			return
		}
		prefs, err := lc.GetPrefs(context.Background())
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to get preferences: %v", err), w)
			return
		}
		for _, r := range prefs.AdvertiseRoutes {
			if r.String() == cidr {
				dialog.ShowInformation("Route Exists", "This route is already being advertised.", w)
				return
			}
		}
		pfx, _ := netip.ParsePrefix(cidr)
		prefs.AdvertiseRoutes = append(prefs.AdvertiseRoutes, pfx)
		_, err = lc.EditPrefs(context.Background(), &ipn.MaskedPrefs{
			Prefs:              *prefs,
			AdvertiseRoutesSet: true,
		})
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to add route: %v", err), w)
			return
		}
		addRouteEntry.SetText("")
		updateStatus()
	}
	addRouteBtn.OnTapped = onAddRoute

	acceptRoutesCheck.OnChanged = func(val bool) {
		prefs, err := lc.GetPrefs(context.Background())
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to get preferences: %v", err), w)
			return
		}
		prefs.RouteAll = val
		_, err = lc.EditPrefs(context.Background(), &ipn.MaskedPrefs{
			Prefs:       *prefs,
			RouteAllSet: true,
		})
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to set Route All: %v", err), w)
			return
		}
		updateStatus()
	}

	magicDNSCheck.OnChanged = func(val bool) {
		prefs, err := lc.GetPrefs(context.Background())
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to get preferences: %v", err), w)
			return
		}
		prefs.CorpDNS = val
		_, err = lc.EditPrefs(context.Background(), &ipn.MaskedPrefs{
			Prefs:      *prefs,
			CorpDNSSet: true,
		})
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to set Corp DNS: %v", err), w)
			return
		}
		updateStatus()
	}

	runExitNodeCheck.OnChanged = func(val bool) {
		prefs, err := lc.GetPrefs(context.Background())
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to get preferences: %v", err), w)
			return
		}
		if val {
			// Advertise all default routes (0.0.0.0/0 and ::/0)
			pfx4, _ := netip.ParsePrefix("0.0.0.0/0")
			pfx6, _ := netip.ParsePrefix("::/0")
			found4, found6 := false, false
			for _, r := range prefs.AdvertiseRoutes {
				if r == pfx4 {
					found4 = true
				}
				if r == pfx6 {
					found6 = true
				}
			}
			if !found4 {
				prefs.AdvertiseRoutes = append(prefs.AdvertiseRoutes, pfx4)
			}
			if !found6 {
				prefs.AdvertiseRoutes = append(prefs.AdvertiseRoutes, pfx6)
			}
		} else {
			// Remove default routes
			newRoutes := prefs.AdvertiseRoutes[:0]
			for _, r := range prefs.AdvertiseRoutes {
				if r.String() != "0.0.0.0/0" && r.String() != "::/0" {
					newRoutes = append(newRoutes, r)
				}
			}
			prefs.AdvertiseRoutes = newRoutes
		}
		_, err = lc.EditPrefs(context.Background(), &ipn.MaskedPrefs{
			Prefs:              *prefs,
			AdvertiseRoutesSet: true,
		})
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to set Run Exit Node: %v", err), w)
			return
		}
		updateStatus()
	}

	allowLocalNetworksCheck.OnChanged = func(val bool) {
		prefs, err := lc.GetPrefs(context.Background())
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to get preferences: %v", err), w)
			return
		}
		prefs.ExitNodeAllowLANAccess = val
		_, err = lc.EditPrefs(context.Background(), &ipn.MaskedPrefs{
			Prefs:                     *prefs,
			ExitNodeAllowLANAccessSet: true,
		})
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to set Allow Local Networks: %v", err), w)
			return
		}
		updateStatus()
	}

	runSSHCheck.OnChanged = func(val bool) {
		prefs, err := lc.GetPrefs(context.Background())
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to get preferences: %v", err), w)
			return
		}
		prefs.RunSSH = val
		_, err = lc.EditPrefs(context.Background(), &ipn.MaskedPrefs{
			Prefs:     *prefs,
			RunSSHSet: true,
		})
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to set SSH preference: %v", err), w)
			return
		}
		updateStatus()
	}

	saveTagsBtn.OnTapped = func() {
		raw := strings.TrimSpace(tagsEntry.Text)
		var tags []string
		if raw != "" {
			for _, t := range strings.Split(raw, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}
		prefs, err := lc.GetPrefs(context.Background())
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to get preferences: %v", err), w)
			return
		}
		prefs.AdvertiseTags = tags
		_, err = lc.EditPrefs(context.Background(), &ipn.MaskedPrefs{
			Prefs:            *prefs,
			AdvertiseTagsSet: true,
		})
		if err != nil {
			dialog.ShowError(fmt.Errorf("Failed to set tags: %v", err), w)
			return
		}
		updateStatus()
	}

	go func() {
		for range time.Tick(5 * time.Second) {
			updateStatus()
		}
	}()
	updateStatus()

	// Layout
	statusBox := container.NewVBox(
		widget.NewLabelWithStyle("Connection Status", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		statusLabel,
		ipLabel,
		netInfo,
		rxLabel,
		txLabel,
		container.NewHBox(connectBtn, disconnectBtn, loginBtn, logoutBtn),
		exitNodeSelect,
		clearExitBtn,
		detailsBtn,
		statusDetails,
	)
	routesBoxOuter := container.NewVBox(
		widget.NewLabelWithStyle("Shared Routes (Advertised)", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		routesBox,
		addRouteRow,
	)
	peersBox = container.NewVBox(
		widget.NewLabelWithStyle("Connected Peers", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	content := container.NewVBox(
		statusBox,
		widget.NewSeparator(),
		routesBoxOuter,
		widget.NewSeparator(),
		peersBox,
	)
	w.SetContent(container.NewPadded(content))
	w.Resize(fyne.NewSize(520, 700))
	w.ShowAndRun()
}
