package garminauth

import "time"

// record mirrors a garmin_tokens row: the encrypted blob, its nonce, and the
// last-write timestamp. The single-row table holds at most one record.
type record struct {
	Ciphertext []byte
	Nonce      []byte
	UpdatedAt  time.Time
}
