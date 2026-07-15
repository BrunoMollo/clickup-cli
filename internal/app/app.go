package app

import (
	"errors"
	"flag"
	"os"

	"botty/internal/browser"
	"botty/internal/clickup"
	"botty/internal/config"
	"botty/internal/sprint"
	"botty/internal/tasks"
	"botty/internal/tui"
)

func Run() error {
	settings, err := config.Parse(os.Args[1:], os.Getenv, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}

	client := clickup.NewClient(settings.Token)
	resolver := sprint.NewDateResolver(client)
	loader := tasks.NewService(resolver, client, settings.AnchorView)
	model := tui.NewModel(loader, browser.NewSystemOpener(), settings.IncludeClosed)
	return tui.Run(model)
}
