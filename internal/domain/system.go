package domain

import "time"

// Clock makes time-dependent application work deterministic in tests.
type Clock interface {
	Now() time.Time
}

// IDGenerator provides stable identifiers without binding domain code to a generator.
type IDGenerator interface {
	New() string
}
