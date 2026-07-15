package browser

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
)

type Opener interface {
	Open(rawURL string) error
}

type CommandStarter interface {
	Start(name string, args ...string) error
}

type commandStarter struct{}

func (commandStarter) Start(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}

type SystemOpener struct {
	goos    string
	starter CommandStarter
}

func NewSystemOpener() *SystemOpener {
	return &SystemOpener{goos: runtime.GOOS, starter: commandStarter{}}
}

func NewSystemOpenerWith(goos string, starter CommandStarter) *SystemOpener {
	return &SystemOpener{goos: goos, starter: starter}
}

func (o *SystemOpener) Open(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" {
		return fmt.Errorf("URL de tarea inválida")
	}

	var name string
	var args []string
	switch o.goos {
	case "darwin":
		name, args = "open", []string{parsed.String()}
	case "linux":
		name, args = "xdg-open", []string{parsed.String()}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", parsed.String()}
	default:
		return fmt.Errorf("apertura de navegador no soportada en %s", o.goos)
	}
	if err := o.starter.Start(name, args...); err != nil {
		return fmt.Errorf("abrir navegador: %w", err)
	}
	return nil
}
