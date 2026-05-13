package queue

import "errors"

var (
	// ErrUnknownJobType is returned when no handler is registered for a job type.
	ErrUnknownJobType = errors.New("queue: unknown job type")

	// ErrInvalidPayload indicates JSON decoding failed or payload constraints violated.
	ErrInvalidPayload = errors.New("queue: invalid job payload")

	// ErrJobNotFound is returned when a job ID does not exist in the backend.
	ErrJobNotFound = errors.New("queue: job not found")

	// ErrDeadLetterJobNotFound is returned when no DLQ snapshot matches the job id.
	ErrDeadLetterJobNotFound = errors.New("queue: dead letter job not found")

	// ErrAdminRetryTargetNotFound is returned when retry is requested for an id not in DLQ and not in active job storage.
	ErrAdminRetryTargetNotFound = errors.New("queue: job not found for admin retry")
)
