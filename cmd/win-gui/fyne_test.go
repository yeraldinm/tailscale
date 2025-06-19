package main

import (
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/widget"
)

func main() {
    a := app.New()
    w := a.NewWindow("Test")
    w.SetContent(widget.NewLabel("Hello Fyne"))
    w.ShowAndRun()
}