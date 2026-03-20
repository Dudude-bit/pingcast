package domain

import (
	"time"

	"github.com/google/uuid"
)

type Plan string

const (
	PlanFree Plan = "free"
	PlanPro  Plan = "pro"
)

type User struct {
	ID        uuid.UUID
	Email     string
	Slug      string
	Plan      Plan
	CreatedAt time.Time
}

func (u User) MonitorLimit() int {
	if u.Plan == PlanPro {
		return 50
	}
	return 5
}

func (u User) MinInterval() int {
	if u.Plan == PlanPro {
		return 30
	}
	return 300
}

func (u User) CanUseEmail() bool {
	return u.Plan == PlanPro
}
