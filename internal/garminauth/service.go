package garminauth

import (
	"context"
	"errors"
)

// ErrEncKeyUnconfigured is returned when an operation is attempted but no
// encryption key was configured. In practice the handler short-circuits with
// 503 garmin_disabled before reaching the service, but the service refuses
// defensively so the blob is never stored or read unencrypted.
var ErrEncKeyUnconfigured = errors.New("garmin token encryption key not configured")

// Service seals blobs on store and opens them on read. The blob is opaque:
// the service never parses or interprets it.
type Service struct {
	repo   *Repo
	crypto *crypto
}

// NewService builds the service over a repo and the 32-byte encryption key.
// A nil/empty key yields a service whose operations return
// ErrEncKeyUnconfigured; callers gate on garmin being enabled before wiring a
// real key.
func NewService(repo *Repo, encKey []byte) (*Service, error) {
	s := &Service{repo: repo}
	if len(encKey) == 0 {
		return s, nil
	}
	c, err := newCrypto(encKey)
	if err != nil {
		return nil, err
	}
	s.crypto = c
	return s, nil
}

// Store encrypts and persists the opaque blob, replacing any prior value.
func (s *Service) Store(ctx context.Context, blob []byte) error {
	if s.crypto == nil {
		return ErrEncKeyUnconfigured
	}
	ciphertext, nonce, err := s.crypto.seal(blob)
	if err != nil {
		return err
	}
	return s.repo.Upsert(ctx, record{Ciphertext: ciphertext, Nonce: nonce})
}

// Get returns the decrypted blob byte-identical to what was stored, or
// ErrNotFound when nothing has been stored.
func (s *Service) Get(ctx context.Context) ([]byte, error) {
	if s.crypto == nil {
		return nil, ErrEncKeyUnconfigured
	}
	rec, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	return s.crypto.open(rec.Ciphertext, rec.Nonce)
}

// Delete removes the stored blob, returning ErrNotFound when none existed.
func (s *Service) Delete(ctx context.Context) error {
	if s.crypto == nil {
		return ErrEncKeyUnconfigured
	}
	return s.repo.Delete(ctx)
}
