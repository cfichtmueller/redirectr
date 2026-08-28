package health

import (
	"context"
	"fmt"
)

const (
	StatusCodeUp      = "UP"
	StatusCodeDown    = "DOWN"
	StatusCodeUnknown = "UNKNOWN"
)

type Status struct {
	name       string
	Status     string             `json:"status"`
	Components map[string]*Status `json:"components,omitempty"`
	Details    map[string]string  `json:"details,omitempty"`
}

func newStatus(name string, status string) *Status {
	return &Status{
		name:       name,
		Status:     status,
		Components: make(map[string]*Status),
		Details:    make(map[string]string),
	}
}

func StatusUnknown(name string) *Status {
	return newStatus(name, StatusCodeUnknown)
}

func StatusUp(name string) *Status {
	return newStatus(name, StatusCodeUp)
}

func StatusDown(name string) *Status {
	return newStatus(name, StatusCodeDown)
}

func (s *Status) AddComponent(c *Status) *Status {
	s.Components[c.name] = c
	s.Status = aggregate(s.Status, c.Status)
	return s
}

func (s *Status) AddDetail(name string, value any) *Status {
	s.Details[name] = fmt.Sprintf("%v", value)
	return s
}

// Down transitions the status to DOWN
func (s *Status) Down() *Status {
	s.Status = StatusCodeDown
	return s
}

type Indicator interface {
	GetHealth(c context.Context) (*Status, error)
}

type Endpoint struct {
	indicators []Indicator
}

func NewEndpoint() *Endpoint {
	return &Endpoint{indicators: make([]Indicator, 0)}
}

func (e *Endpoint) AddIndicator(i Indicator) {
	e.indicators = append(e.indicators, i)
}

func (e *Endpoint) GetHealth(c context.Context) (*Status, error) {
	if len(e.indicators) == 0 {
		return StatusUnknown(""), nil
	}
	status := StatusUp("")
	for _, i := range e.indicators {
		s, err := i.GetHealth(c)
		if err != nil {
			return nil, err
		}
		status.AddComponent(s)
	}
	return status, nil
}

func aggregate(statusCode1 string, statusCode2 string) string {
	if statusCode1 == StatusCodeDown || statusCode2 == StatusCodeDown {
		return StatusCodeDown
	} else if statusCode1 == StatusCodeUnknown || statusCode2 == StatusCodeUnknown {
		return StatusCodeUnknown
	}
	return StatusCodeUp
}
