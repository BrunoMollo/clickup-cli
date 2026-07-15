package app

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"os"
	"path/filepath"

	"clickdown/internal/browser"
	"clickdown/internal/clickup"
	"clickdown/internal/config"
	"clickdown/internal/sprint"
	"clickdown/internal/tasks"
	"clickdown/internal/tui"
	"clickdown/internal/urlcache"
)

func Run() error {
	settings, err := config.Parse(os.Args[1:], os.Getenv, os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}

	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	fingerprint := sha256.Sum256([]byte(settings.Token))
	cacheDir := filepath.Join(cacheRoot, "clickdown", "clickup", hex.EncodeToString(fingerprint[:8]))
	responseCache, err := urlcache.New(cacheDir)
	if err != nil {
		return err
	}

	client := clickup.NewCachedClient(settings.Token, responseCache)
	resolver := sprint.NewDateResolver(client)
	loader := tasks.NewService(resolver, client, settings.AnchorView)
	model := tui.NewModel(loader, browser.NewSystemOpener(), settings.IncludeClosed, client)
	return tui.Run(model)
}
