package clickup

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type ID string

func (id *ID) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*id = ""
		return nil
	}
	var text string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		*id = ID(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("ID inválido: %w", err)
	}
	*id = ID(number.String())
	return nil
}

type View struct {
	ID     ID         `json:"id"`
	Name   string     `json:"name"`
	Type   string     `json:"type"`
	Parent ViewParent `json:"parent"`
}

type ViewParent struct {
	ID   ID  `json:"id"`
	Type int `json:"type"`
}

type viewResponse struct {
	View View `json:"view"`
}

type List struct {
	ID         ID       `json:"id"`
	Name       string   `json:"name"`
	OrderIndex int      `json:"orderindex"`
	DueDate    *string  `json:"due_date"`
	StartDate  *string  `json:"start_date"`
	Archived   bool     `json:"archived"`
	Folder     Location `json:"folder"`
}

type Location struct {
	ID   ID     `json:"id"`
	Name string `json:"name"`
}

type listsResponse struct {
	Lists []List `json:"lists"`
}

type Status struct {
	Name       string `json:"status"`
	Color      string `json:"color"`
	OrderIndex int    `json:"orderindex"`
	Type       string `json:"type"`
}

type Priority struct {
	ID       string `json:"id"`
	Name     string `json:"priority"`
	Color    string `json:"color"`
	OrderRaw string `json:"orderindex"`
}

type Assignee struct {
	ID       ID     `json:"id"`
	Username string `json:"username"`
	Initials string `json:"initials"`
}

type Task struct {
	ID        ID         `json:"id"`
	Name      string     `json:"name"`
	URL       string     `json:"url"`
	ParentID  ID         `json:"parent"`
	Status    Status     `json:"status"`
	Priority  *Priority  `json:"priority"`
	Assignees []Assignee `json:"assignees"`
	DueDate   *string    `json:"due_date"`
}

type tasksResponse struct {
	Tasks    []Task `json:"tasks"`
	LastPage bool   `json:"last_page"`
}
