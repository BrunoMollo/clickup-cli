package tasks

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"botty/internal/clickup"
	"botty/internal/sprint"
)

type SprintResolver interface {
	Resolve(ctx context.Context, anchor string, limit int) ([]sprint.Sprint, error)
}

type TaskSource interface {
	GetListTasks(ctx context.Context, listID string) ([]clickup.Task, error)
}

type Loader interface {
	Load(ctx context.Context) (Snapshot, error)
}

type Service struct {
	resolver SprintResolver
	source   TaskSource
	anchor   string
	now      func() time.Time
}

func NewService(resolver SprintResolver, source TaskSource, anchor string) *Service {
	return &Service{resolver: resolver, source: source, anchor: anchor, now: time.Now}
}

func (s *Service) Load(ctx context.Context) (Snapshot, error) {
	sprints, err := s.resolver.Resolve(ctx, s.anchor, 2)
	if err != nil {
		return Snapshot{}, err
	}
	if len(sprints) != 2 {
		return Snapshot{}, fmt.Errorf("resolver devolvió %d sprints; se esperaban 2", len(sprints))
	}

	type result struct {
		index int
		tasks []clickup.Task
		err   error
	}
	results := make(chan result, len(sprints))
	var wait sync.WaitGroup
	for index, current := range sprints {
		wait.Add(1)
		go func(index int, current sprint.Sprint) {
			defer wait.Done()
			loaded, loadErr := s.source.GetListTasks(ctx, current.ID)
			results <- result{index: index, tasks: loaded, err: loadErr}
		}(index, current)
	}
	wait.Wait()
	close(results)

	ordered := make([]result, len(sprints))
	failed := 0
	for loaded := range results {
		ordered[loaded.index] = loaded
		if loaded.err != nil {
			failed++
		}
	}
	if failed == len(sprints) {
		return Snapshot{}, fmt.Errorf("no se pudieron cargar tareas: %w", ordered[0].err)
	}

	byID := make(map[string]*Task)
	order := make([]string, 0)
	warnings := make([]error, 0, failed)
	for index, loaded := range ordered {
		current := sprints[index]
		if loaded.err != nil {
			warnings = append(warnings, fmt.Errorf("sprint %q: %w", current.Name, loaded.err))
			continue
		}
		for _, raw := range loaded.tasks {
			id := string(raw.ID)
			if id == "" {
				continue
			}
			if existing, ok := byID[id]; ok {
				existing.SprintNames = appendUnique(existing.SprintNames, current.Name)
				continue
			}
			mapped := mapTask(raw, current.Name)
			byID[id] = &mapped
			order = append(order, id)
		}
	}

	mapped := make([]*Task, 0, len(order))
	for _, id := range order {
		mapped = append(mapped, byID[id])
	}
	return Snapshot{
		Sprints:  sprints,
		Forest:   BuildForest(mapped),
		Warnings: warnings,
		LoadedAt: s.now(),
	}, nil
}

func mapTask(raw clickup.Task, sprintName string) Task {
	assignees := make([]string, 0, len(raw.Assignees))
	for _, assignee := range raw.Assignees {
		name := strings.TrimSpace(assignee.Username)
		if name == "" {
			name = strings.TrimSpace(assignee.Initials)
		}
		if name != "" {
			assignees = append(assignees, name)
		}
	}

	var priority *Priority
	if raw.Priority != nil {
		priority = &Priority{ID: raw.Priority.ID, Name: raw.Priority.Name, Color: raw.Priority.Color}
	}
	url := strings.TrimSpace(raw.URL)
	if url == "" {
		url = "https://app.clickup.com/t/" + string(raw.ID)
	}
	return Task{
		ID:       string(raw.ID),
		Name:     raw.Name,
		URL:      url,
		ParentID: string(raw.ParentID),
		Status: Status{
			Name:       raw.Status.Name,
			Type:       raw.Status.Type,
			Color:      raw.Status.Color,
			OrderIndex: raw.Status.OrderIndex,
		},
		Priority:    priority,
		Assignees:   assignees,
		DueDate:     parseTaskMillis(raw.DueDate),
		SprintNames: []string{sprintName},
	}
}

func parseTaskMillis(raw *string) *time.Time {
	if raw == nil || *raw == "" {
		return nil
	}
	millis, err := strconv.ParseInt(*raw, 10, 64)
	if err != nil {
		return nil
	}
	parsed := time.UnixMilli(millis)
	return &parsed
}

func appendUnique(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func sortNodes(nodes []*Node) {
	sort.SliceStable(nodes, func(i, j int) bool {
		left, right := nodes[i].Task, nodes[j].Task
		if left.Priority.Rank() != right.Priority.Rank() {
			return left.Priority.Rank() < right.Priority.Rank()
		}
		if left.DueDate != nil && right.DueDate == nil {
			return true
		}
		if left.DueDate == nil && right.DueDate != nil {
			return false
		}
		if left.DueDate != nil && right.DueDate != nil && !left.DueDate.Equal(*right.DueDate) {
			return left.DueDate.Before(*right.DueDate)
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})
	for _, node := range nodes {
		sortNodes(node.Children)
	}
}
