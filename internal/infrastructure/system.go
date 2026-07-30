package infrastructure

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/xu-wentao/grandet-agent/internal/domain"
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

var (
	_ domain.Clock       = Clock{}
	_ domain.IDGenerator = IDGenerator{}
)
