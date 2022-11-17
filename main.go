package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var app = tview.NewApplication()

var view = tview.NewTextView().SetTextColor(tcell.ColorGreen).SetText("Press q to quit")

func main() {
    app.SetInputCapture(
        func(event *tcell.EventKey) *tcell.EventKey {
            if event.Rune() == 113 {
                app.Stop()
            }
            return event
        })

    if err := app.SetRoot(view, true).EnableMouse(true).Run(); err != nil {
        panic(err)
    }

}
