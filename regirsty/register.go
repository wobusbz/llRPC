package regirsty

import (
	"context"
)

type Registry interface {
	Name() string
	Init(ctx context.Context, opts ...Option) (err error)
	Register(ctx context.Context, service *Service) (err error)
	Unregister(ctx context.Context, service *Service) (err error)
	GetService(ctx context.Context, name string) (service *Service, err error)
}
