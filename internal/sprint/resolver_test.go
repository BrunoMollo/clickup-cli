package sprint

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"clickdown/internal/clickup"
)

type fakeHierarchy struct {
	view  clickup.View
	list  clickup.List
	lists []clickup.List
}

func (f fakeHierarchy) GetView(context.Context, string) (clickup.View, error) { return f.view, nil }
func (f fakeHierarchy) GetList(context.Context, string) (clickup.List, error) { return f.list, nil }
func (f fakeHierarchy) GetFolderLists(context.Context, string) ([]clickup.List, error) {
	return f.lists, nil
}

func millis(date string) *string {
	parsed, _ := time.Parse("2006-01-02", date)
	value := strconv.FormatInt(parsed.UnixMilli(), 10)
	return &value
}

func TestParseViewID(t *testing.T) {
	for _, value := range []string{
		"6-901417703320-1",
		"https://app.clickup.com/31037287/v/l/6-901417703320-1",
		"https://app.clickup.com/31037287/v/l/6-901416907235-1",
	} {
		id, err := ParseViewID(value)
		if err != nil || !strings.HasPrefix(id, "6-") {
			t.Fatalf("ParseViewID(%q)=%q, %v", value, id, err)
		}
	}
	if _, err := ParseViewID("https://evil.example/v/l/6-1"); err == nil {
		t.Fatal("host inválido aceptado")
	}
}

func TestDateResolverSelectsTwoMaximumDates(t *testing.T) {
	api := fakeHierarchy{
		view: clickup.View{ID: "view", Parent: clickup.ViewParent{ID: "list", Type: parentList}},
		list: clickup.List{ID: "list", Folder: clickup.Location{ID: "folder"}},
		lists: []clickup.List{
			{ID: "old", Name: "Viejo", DueDate: millis("2026-01-01")},
			{ID: "future", Name: "Futuro", DueDate: millis("2027-01-01")},
			{ID: "current", Name: "Actual", StartDate: millis("2026-07-01")},
			{ID: "archived", Name: "Archivado", DueDate: millis("2028-01-01"), Archived: true},
			{ID: "undated", Name: "Sin fecha"},
		},
	}
	got, err := NewDateResolver(api).Resolve(context.Background(), "view-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "future" || got[1].ID != "current" {
		t.Fatalf("sprints=%+v", got)
	}
}

func TestDateResolverRequiresEnoughDatedLists(t *testing.T) {
	api := fakeHierarchy{
		view:  clickup.View{ID: "view", Parent: clickup.ViewParent{ID: "folder", Type: parentFolder}},
		lists: []clickup.List{{ID: "one", Name: "Uno", DueDate: millis("2026-01-01")}},
	}
	_, err := NewDateResolver(api).Resolve(context.Background(), "view-1", 2)
	if err == nil || !strings.Contains(err.Error(), "encontradas 1") {
		t.Fatalf("error inesperado: %v", err)
	}
}
