package consultation

import (
	"context"
	"errors"
	"time"
)

var ErrRequestPayload = errors.New("request ID already used with different payload")
var ErrRequestBusy = errors.New("another consultation request is processing")
var ErrRequestCancelled = errors.New("consultation request cancelled before commit")
var ErrRequestFinished = errors.New("consultation request already terminal")

// RequestReceipt belongs to one authenticated owner. Its result is the exact saved
// turn, not whichever conversation happens to be latest when a client reconnects.
type RequestReceipt struct {
	ID              string
	Month           string
	NewConsultation bool
	PayloadHash     string
	Status          string
	ExpiresAt       time.Time
	CancelRequested bool
	ErrorCode       string
	Result          *Thread
}
type RequestRepository interface {
	ClaimChatRequest(context.Context, RequestReceipt) (RequestReceipt, bool, error)
	GetChatRequest(context.Context, string) (RequestReceipt, error)
	CancelChatRequest(context.Context, string) (RequestReceipt, error)
	FailChatRequest(context.Context, string, string) error
}
type RequestReference struct {
	ID             string
	Deadline       time.Time
	AllowCancelled bool
}
type requestReferenceKey struct{}

func WithRequestReference(ctx context.Context, ref RequestReference) context.Context {
	return context.WithValue(ctx, requestReferenceKey{}, ref)
}
func RequestReferenceFrom(ctx context.Context) (RequestReference, bool) {
	ref, ok := ctx.Value(requestReferenceKey{}).(RequestReference)
	return ref, ok
}
