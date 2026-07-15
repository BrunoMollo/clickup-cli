package tasks

import (
	"context"
	"errors"
	"testing"
	"time"

	"clickdown/internal/clickup"
	"clickdown/internal/sprint"
)

type fakeResolver struct {
	sprints []sprint.Sprint
	err     error
}

func (f fakeResolver) Resolve(context.Context, string, int) ([]sprint.Sprint, error) {
	return f.sprints, f.err
}

type fakeSource struct {
	byList map[string][]clickup.Task
	errors map[string]error
}

type captureSource struct {
	fakeSource
	begins  int
	commits int
	aborts  int
}

func (s *captureSource) BeginCacheCapture()  { s.begins++ }
func (s *captureSource) CommitCacheCapture() { s.commits++ }
func (s *captureSource) AbortCacheCapture()  { s.aborts++ }

func (f fakeSource) GetListTasks(_ context.Context, listID string) ([]clickup.Task, error) {
	return f.byList[listID], f.errors[listID]
}

func TestServiceDeduplicatesAndKeepsSprintNames(t *testing.T) {
	resolver := fakeResolver{sprints: []sprint.Sprint{{ID: "new", Name: "Sprint 2"}, {ID: "old", Name: "Sprint 1"}}}
	shared := clickup.Task{ID: "task", Name: "Shared", Status: clickup.Status{Name: "Open", Type: "open"}}
	source := fakeSource{byList: map[string][]clickup.Task{"new": {shared}, "old": {shared}}}
	service := NewService(resolver, source, "view")
	service.now = func() time.Time { return time.Unix(1, 0) }

	snapshot, err := service.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Forest) != 1 || len(snapshot.Forest[0].Task.SprintNames) != 2 {
		t.Fatalf("snapshot inesperado: %+v", snapshot)
	}
	if snapshot.Forest[0].Task.URL != "https://app.clickup.com/t/task" {
		t.Fatalf("fallback URL=%q", snapshot.Forest[0].Task.URL)
	}
}

func TestServiceReturnsPartialSnapshot(t *testing.T) {
	resolver := fakeResolver{sprints: []sprint.Sprint{{ID: "ok", Name: "OK"}, {ID: "bad", Name: "Bad"}}}
	source := fakeSource{
		byList: map[string][]clickup.Task{"ok": {{ID: "task", Name: "Task"}}},
		errors: map[string]error{"bad": errors.New("offline")},
	}
	snapshot, err := NewService(resolver, source, "view").Load(context.Background())
	if err != nil || len(snapshot.Forest) != 1 || len(snapshot.Warnings) != 1 {
		t.Fatalf("partial=%+v err=%v", snapshot, err)
	}
}

func TestServiceFailsWhenBothSourcesFail(t *testing.T) {
	resolver := fakeResolver{sprints: []sprint.Sprint{{ID: "a"}, {ID: "b"}}}
	source := fakeSource{errors: map[string]error{"a": errors.New("a"), "b": errors.New("b")}}
	if _, err := NewService(resolver, source, "view").Load(context.Background()); err == nil {
		t.Fatal("se esperaba error total")
	}
}

func TestServiceCommitsDisplayedCacheURLs(t *testing.T) {
	resolver := fakeResolver{sprints: []sprint.Sprint{{ID: "a"}, {ID: "b"}}}
	source := &captureSource{fakeSource: fakeSource{byList: map[string][]clickup.Task{"a": {}, "b": {}}}}
	if _, err := NewService(resolver, source, "view").Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	if source.begins != 1 || source.commits != 1 || source.aborts != 0 {
		t.Fatalf("capture begin=%d commit=%d abort=%d", source.begins, source.commits, source.aborts)
	}
}

func TestServiceAbortsCacheCaptureOnFailure(t *testing.T) {
	resolver := fakeResolver{err: errors.New("resolver failed")}
	source := &captureSource{}
	if _, err := NewService(resolver, source, "view").Load(context.Background()); err == nil {
		t.Fatal("se esperaba error")
	}
	if source.begins != 1 || source.commits != 0 || source.aborts != 1 {
		t.Fatalf("capture begin=%d commit=%d abort=%d", source.begins, source.commits, source.aborts)
	}
}
