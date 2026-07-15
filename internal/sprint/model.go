package sprint

import "time"

type Sprint struct {
	ID         string
	Name       string
	StartDate  *time.Time
	DueDate    *time.Time
	OrderIndex int
}

func (s Sprint) EffectiveDate() time.Time {
	if s.DueDate != nil {
		return *s.DueDate
	}
	if s.StartDate != nil {
		return *s.StartDate
	}
	return time.Time{}
}
