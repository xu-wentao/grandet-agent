package domain

import "time"

// Clock keeps time-dependent application behavior deterministic in tests.
type Clock interface {
	Now() time.Time
}

// IDGenerator creates identifiers without coupling domain code to a UUID implementation.
type IDGenerator interface {
	NewID() string
}
