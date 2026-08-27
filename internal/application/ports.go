package application

import (
	"context"
	"encoding/json"
	"time"

	"trapreview/internal/domain"
)

type Mutation func(*domain.Aggregate) (json.RawMessage, error)

type Repository interface {
	Create(context.Context, *domain.Aggregate, string, json.RawMessage) (json.RawMessage, bool, error)
	Transact(context.Context, string, int64, string, Mutation) (json.RawMessage, bool, error)
	Load(context.Context, string) (*domain.Aggregate, error)
	LookupIdempotency(context.Context, string, string) (json.RawMessage, bool, error)
	FindRelease(context.Context, string) (*domain.DatasetRelease, error)
}

type Clock interface{ Now() time.Time }

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }
