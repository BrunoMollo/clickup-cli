package sprint

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"botty/internal/clickup"
)

type DateResolver struct {
	api HierarchyAPI
}

func NewDateResolver(api HierarchyAPI) *DateResolver {
	return &DateResolver{api: api}
}

func (r *DateResolver) Resolve(ctx context.Context, anchor string, limit int) ([]Sprint, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("límite de sprints debe ser positivo")
	}
	viewID, err := ParseViewID(anchor)
	if err != nil {
		return nil, err
	}
	view, err := r.api.GetView(ctx, viewID)
	if err != nil {
		return nil, fmt.Errorf("resolver vista %s: %w", viewID, err)
	}

	folderID, err := r.folderID(ctx, view)
	if err != nil {
		return nil, err
	}
	lists, err := r.api.GetFolderLists(ctx, folderID)
	if err != nil {
		return nil, fmt.Errorf("listar sprints de carpeta %s: %w", folderID, err)
	}

	candidates := make([]Sprint, 0, len(lists))
	for _, list := range lists {
		if list.Archived {
			continue
		}
		startDate, startErr := parseMillis(list.StartDate)
		if startErr != nil {
			return nil, fmt.Errorf("start_date inválida en lista %q: %w", list.Name, startErr)
		}
		dueDate, dueErr := parseMillis(list.DueDate)
		if dueErr != nil {
			return nil, fmt.Errorf("due_date inválida en lista %q: %w", list.Name, dueErr)
		}
		if startDate == nil && dueDate == nil {
			continue
		}
		candidates = append(candidates, Sprint{
			ID:         string(list.ID),
			Name:       list.Name,
			StartDate:  startDate,
			DueDate:    dueDate,
			OrderIndex: list.OrderIndex,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if !left.EffectiveDate().Equal(right.EffectiveDate()) {
			return left.EffectiveDate().After(right.EffectiveDate())
		}
		leftStart, rightStart := optionalTime(left.StartDate), optionalTime(right.StartDate)
		if !leftStart.Equal(rightStart) {
			return leftStart.After(rightStart)
		}
		if left.OrderIndex != right.OrderIndex {
			return left.OrderIndex > right.OrderIndex
		}
		return left.ID > right.ID
	})

	if len(candidates) < limit {
		names := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			names = append(names, candidate.Name)
		}
		return nil, fmt.Errorf("se necesitan %d listas con fecha; encontradas %d (%s)", limit, len(candidates), strings.Join(names, ", "))
	}
	return candidates[:limit], nil
}

func (r *DateResolver) folderID(ctx context.Context, view clickup.View) (string, error) {
	switch view.Parent.Type {
	case parentFolder:
		if view.Parent.ID == "" {
			return "", fmt.Errorf("vista %s no tiene carpeta padre", view.ID)
		}
		return string(view.Parent.ID), nil
	case parentList:
		list, err := r.api.GetList(ctx, string(view.Parent.ID))
		if err != nil {
			return "", fmt.Errorf("resolver lista padre %s: %w", view.Parent.ID, err)
		}
		if list.Folder.ID == "" {
			return "", fmt.Errorf("lista %s no pertenece a una carpeta", list.ID)
		}
		return string(list.Folder.ID), nil
	default:
		return "", fmt.Errorf("vista %s tiene parent.type=%d; se requiere carpeta (5) o lista (6)", view.ID, view.Parent.Type)
	}
}

func parseMillis(raw *string) (*time.Time, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	millis, err := strconv.ParseInt(*raw, 10, 64)
	if err != nil {
		return nil, err
	}
	parsed := time.UnixMilli(millis)
	return &parsed, nil
}

func optionalTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
