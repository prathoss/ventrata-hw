package pkg

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type correlationIDKeyType string

const correlationIDKey correlationIDKeyType = "correlation-id"

func GetCorrelationIDCtx(ctx context.Context) uuid.UUID {
	if correlationID, ok := ctx.Value(correlationIDKey).(uuid.UUID); ok {
		return correlationID
	}
	return uuid.UUID{}
}

func GetCorrelationIDReq(r *http.Request) uuid.UUID {
	correlationIdString := r.Header.Get("x-correlation-id")
	correlationID, err := uuid.Parse(correlationIdString)
	if err != nil {
		return uuid.New()
	}
	return correlationID
}

func SetCorrelationID(ctx context.Context, correlationID uuid.UUID) context.Context {
	return context.WithValue(ctx, correlationIDKey, correlationID)
}
