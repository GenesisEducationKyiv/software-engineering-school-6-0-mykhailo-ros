package saga

import (
	"crypto/rand"
	"encoding/hex"
)

func correlationID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("saga: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
