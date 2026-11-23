// Command reva is a local, account-free API client.
package main

import (
	"log"
	"os"
	"runtime"

	"github.com/runbrick/reva/internal/gui"
	"github.com/runbrick/reva/internal/update"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func main() {
	var updateFailed bool
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case update.ApplyCommand:
			if err := update.RunHelper(os.Args[2:]); err != nil {
				log.Fatal(err)
			}
			return
		case update.RestartCommand:
			updateFailed = update.FinishRestart(os.Args[2:])
		}
	}
	svc, err := services()
	if err != nil {
		log.Fatal(err)
	}
	newApp := func() *gui.App {
		a := gui.New(svc)
		if err := a.InitializeCollectionWorkspace(a.S.Collections.WorkspaceDefaultName); err != nil {
			a.Toast(a.S.Common.Error, err.Error(), ui.ToastError)
		}
		return a
	}
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		if err := app.RunCanvas(newApp().Config(), &gfx.Canvas{}); err != nil {
			log.Fatal(err)
		}
		return
	}
	if err := app.RunApplication(func(application *app.Application) error {
		a := newApp()
		a.ConfigureUpdates(application.Quit, updateFailed)
		_, err := application.OpenWindow(a.Config(), &gfx.Canvas{})
		return err
	}); err != nil {
		log.Fatal(err)
	}
}
