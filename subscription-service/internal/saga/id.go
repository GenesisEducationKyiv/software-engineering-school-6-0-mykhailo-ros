package saga

import (
	"crypto/rand"
	"encoding/hex"
)

func correlationID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
