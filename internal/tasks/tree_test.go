package tasks

import "testing"

func TestBuildAndFilterForestPromotesOpenChild(t *testing.T) {
	parent := &Task{ID: "parent", Name: "Parent", Status: Status{Name: "Done", Type: "closed"}}
	child := &Task{ID: "child", ParentID: "parent", Name: "Child", Status: Status{Name: "Doing", Type: "custom"}}
	closedChild := &Task{ID: "closed-child", ParentID: "child", Name: "Closed", Status: Status{Name: "Done", Type: "done"}}

	forest := BuildForest([]*Task{parent, child, closedChild})
	if len(forest) != 1 || len(forest[0].Children) != 1 {
		t.Fatalf("árbol inesperado: %+v", forest)
	}
	filtered := FilterForest(forest, false)
	if len(filtered) != 1 || filtered[0].Task.ID != "child" || !filtered[0].Promoted {
		t.Fatalf("promoción inesperada: %+v", filtered)
	}
	if len(filtered[0].Children) != 0 {
		t.Fatal("subtarea cerrada no fue filtrada")
	}
	all := FilterForest(forest, true)
	if len(all[0].Children[0].Children) != 1 {
		t.Fatal("modo completo perdió subtarea")
	}
}

func TestBuildForestMarksOrphan(t *testing.T) {
	forest := BuildForest([]*Task{{ID: "orphan", ParentID: "missing", Name: "Orphan"}})
	if len(forest) != 1 || !forest[0].Orphan {
		t.Fatalf("huérfano inesperado: %+v", forest)
	}
}

func TestGroupForestCombinesStatusCaseInsensitive(t *testing.T) {
	forest := BuildForest([]*Task{
		{ID: "1", Name: "A", Status: Status{Name: "In Progress", OrderIndex: 2}},
		{ID: "2", Name: "B", Status: Status{Name: "in progress", OrderIndex: 1}},
	})
	groups := GroupForest(forest)
	if len(groups) != 1 || len(groups[0].Roots) != 2 || groups[0].Status.OrderIndex != 1 {
		t.Fatalf("grupos inesperados: %+v", groups)
	}
}

func TestGroupForestUsesRequestedWorkflowOrder(t *testing.T) {
	names := []string{
		"En refinamiento",
		"Por hacer",
		"En curso",
		"Bloqueado",
		"En review",
		"Ready for test",
		"QA testing",
		"Esperando release",
		"Staging",
		"Esperando deploy",
		"Producción",
		"Completado",
	}
	tasks := make([]*Task, 0, len(names))
	for index, name := range names {
		tasks = append(tasks, &Task{ID: name, Name: name, Status: Status{Name: name, OrderIndex: index}})
	}
	groups := GroupForest(BuildForest(tasks))
	want := []string{
		"Producción",
		"Esperando deploy",
		"Staging",
		"Esperando release",
		"QA testing",
		"Ready for test",
		"Bloqueado",
		"En review",
		"En curso",
		"Por hacer",
		"En refinamiento",
		"Completado",
	}
	if len(groups) != len(want) {
		t.Fatalf("grupos=%d", len(groups))
	}
	for index := range want {
		if groups[index].Status.Name != want[index] {
			t.Fatalf("posición %d: got %q want %q", index, groups[index].Status.Name, want[index])
		}
	}
}
