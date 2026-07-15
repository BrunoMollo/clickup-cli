package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

const DefaultAnchorView = "https://app.clickup.com/31037287/v/l/6-901417703320-1"

type Config struct {
	Token         string
	AnchorView    string
	IncludeClosed bool
}

func Parse(args []string, getenv func(string) string, stderr io.Writer) (Config, error) {
	fs := flag.NewFlagSet("botty", flag.ContinueOnError)
	fs.SetOutput(stderr)

	anchorDefault := strings.TrimSpace(getenv("CLICKUP_ANCHOR_VIEW"))
	if anchorDefault == "" {
		anchorDefault = DefaultAnchorView
	}

	var cfg Config
	fs.StringVar(&cfg.AnchorView, "anchor-view", anchorDefault, "URL o ID de vista usado para localizar el proyecto")
	fs.BoolVar(&cfg.IncludeClosed, "include-closed", false, "mostrar tareas abiertas y cerradas al iniciar")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if fs.NArg() != 0 {
		return Config{}, fmt.Errorf("argumentos inesperados: %s", strings.Join(fs.Args(), " "))
	}

	cfg.Token = strings.TrimSpace(getenv("CLICKUP_API_TOKEN"))
	if cfg.Token == "" {
		return Config{}, errors.New("falta CLICKUP_API_TOKEN; exportá tu token personal de ClickUp antes de ejecutar botty")
	}
	cfg.AnchorView = strings.TrimSpace(cfg.AnchorView)
	if cfg.AnchorView == "" {
		return Config{}, errors.New("anchor-view no puede estar vacío")
	}
	return cfg, nil
}
