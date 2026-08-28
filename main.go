package main

import (
	"auto-wuwa-discord-signin/pkg/tray"
)

func main() {
	app := tray.NewTrayApp()
	app.Run()
}
