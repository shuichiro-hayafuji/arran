package domain

import "errors"

// ErrNotFound is the transport-neutral absence result returned by repository
// adapters. It keeps database-specific errors out of application and handler.
var ErrNotFound = errors.New("spendable today: not found")

// ErrNotificationNotSent denotes failure before any external request was sent.
var ErrNotificationNotSent = errors.New("notification was not sent")
