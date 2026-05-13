package services

import "context"

type authRequestMetaCtxKey struct{}

// AuthRequestMeta carries optional client context for session telemetry.
type AuthRequestMeta struct {
	UserAgent string
	IPAddress string
}

// WithAuthRequestMeta stores client metadata in context for auth/session flows.
func WithAuthRequestMeta(parent context.Context, meta AuthRequestMeta) context.Context {
	return context.WithValue(parent, authRequestMetaCtxKey{}, meta)
}

func authRequestMetaFromContext(ctx context.Context) AuthRequestMeta {
	meta, ok := ctx.Value(authRequestMetaCtxKey{}).(AuthRequestMeta)
	if !ok {
		return AuthRequestMeta{}
	}
	return meta
}
