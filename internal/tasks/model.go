package tasks

import (
	"strconv"
	"strings"
	"time"

	"botty/internal/sprint"
)

type Status struct {
	Name       string
	Type       string
	Color      string
	OrderIndex int
}

func (s Status) Closed() bool {
	kind := strings.ToLower(strings.TrimSpace(s.Type))
	return kind == "done" || kind == "closed"
}

type Priority struct {
	ID    string
	Name  string
	Color string
}

func (p *Priority) Rank() int {
	if p == nil {
		return 5
	}
	rank, err := strconv.Atoi(p.ID)
	if err != nil || rank < 1 || rank > 4 {
		return 5
	}
	return rank
}

type Task struct {
	ID          string
	Name        string
	URL         string
	ParentID    string
	Status      Status
	Priority    *Priority
	Assignees   []string
	DueDate     *time.Time
	SprintNames []string
}

type Node struct {
	Task     *Task
	Children []*Node
	Orphan   bool
	Promoted bool
}

type Snapshot struct {
	Sprints  []sprint.Sprint
	Forest   []*Node
	Warnings []error
	LoadedAt time.Time
}

type StatusGroup struct {
	Status Status
	Roots  []*Node
}
