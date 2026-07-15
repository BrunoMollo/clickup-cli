package browser

import (
	"reflect"
	"testing"
)

type recordingStarter struct {
	name string
	args []string
}

func (s *recordingStarter) Start(name string, args ...string) error {
	s.name = name
	s.args = args
	return nil
}

func TestSystemOpenerCommands(t *testing.T) {
	tests := []struct {
		goos string
		name string
		args []string
	}{
		{goos: "darwin", name: "open", args: []string{"https://app.clickup.com/t/abc"}},
		{goos: "linux", name: "xdg-open", args: []string{"https://app.clickup.com/t/abc"}},
		{goos: "windows", name: "rundll32", args: []string{"url.dll,FileProtocolHandler", "https://app.clickup.com/t/abc"}},
	}
	for _, test := range tests {
		t.Run(test.goos, func(t *testing.T) {
			starter := &recordingStarter{}
			if err := NewSystemOpenerWith(test.goos, starter).Open("https://app.clickup.com/t/abc"); err != nil {
				t.Fatal(err)
			}
			if starter.name != test.name || !reflect.DeepEqual(starter.args, test.args) {
				t.Fatalf("got %s %v", starter.name, starter.args)
			}
		})
	}
}

func TestSystemOpenerRejectsUnsafeURL(t *testing.T) {
	if err := NewSystemOpenerWith("darwin", &recordingStarter{}).Open("file:///tmp/x"); err == nil {
		t.Fatal("URL insegura aceptada")
	}
}
