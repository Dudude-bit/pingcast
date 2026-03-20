package domain

import (
	"time"

	"github.com/google/uuid"
)

type MonitorStatus string

const (
	StatusUp      MonitorStatus = "up"
	StatusDown    MonitorStatus = "down"
	StatusUnknown MonitorStatus = "unknown"
)

type HTTPMethod string

const (
	MethodGET  HTTPMethod = "GET"
	MethodPOST HTTPMethod = "POST"
)

type Monitor struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	Name               string
	URL                string
	Method             HTTPMethod
	IntervalSeconds    int
	ExpectedStatus     int
	Keyword            *string
	AlertAfterFailures int
	IsPaused           bool
	IsPublic           bool
	CurrentStatus      MonitorStatus
	CreatedAt          time.Time
}

type CheckResult struct {
	ID             int64
	MonitorID      uuid.UUID
	Status         MonitorStatus
	StatusCode     *int
	ResponseTimeMs int
	ErrorMessage   *string
	CheckedAt      time.Time
}
