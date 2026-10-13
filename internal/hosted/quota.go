package hosted

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
)

var (
	ErrDeviceLimit = errors.New("device quota exhausted")
	ErrGlobalLimit = errors.New("global quota exhausted")
)

type Usage struct {
	Used        int64 `json:"used"`
	Limit       int64 `json:"limit"`
	Remaining   int64 `json:"remaining"`
	GlobalSpent int64 `json:"globalSpent"`
	GlobalLimit int64 `json:"globalLimit"`
}

// Reservation counts against both quotas before an upstream request starts.
// Its unique token makes rollback idempotent even if an HTTP response is lost.
type Reservation struct {
	ID, Device, Month string
	Used              int64
}

type QuotaStore interface {
	Reserve(context.Context, string, string, int64, int64) (Reservation, error)
	Commit(context.Context, Reservation) error
	Rollback(context.Context, Reservation) error
	RefundDevice(context.Context, Reservation) error
	Usage(context.Context, string, string) (used, globalSpent int64, err error)
	Reset(context.Context, string, string, bool) error
}

func reservationID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
