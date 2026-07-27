// Package testutil provides deterministic doubles for existing domain ports.
package testutil

import (
	"time"

	"github.com/xu-wentao/grandet-agent/internal/domain"
)

type FixedClock struct {
	Time time.Time
}

func (c FixedClock) Now() time.Time { return c.Time }

type FixedIDGenerator struct {
	ID string
}

func (g FixedIDGenerator) New() string { return g.ID }

var _ domain.Clock = FixedClock{}
var _ domain.IDGenerator = FixedIDGenerator{}
