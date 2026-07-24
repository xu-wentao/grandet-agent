package infrastructure

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type Clock struct{}

func (Clock) Now() time.Time { return time.Now() }

type IDGenerator struct{}

func (IDGenerator) New() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}
