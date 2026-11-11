// Command reqo is a local, account-free API client.
package main

import (
	"log"

	"github.com/runbrick/reqo/internal/gui"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

func main() {
	svc, err := services()
	if err != nil {
		log.Fatal(err)
	}
	a := gui.New(svc)
	var canvas gfx.Canvas
	if err := app.RunCanvas(a.Config(), &canvas); err != nil {
		log.Fatal(err)
	}
}
