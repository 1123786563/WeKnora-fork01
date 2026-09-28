package service

import (
	"context"

	"github.com/Tencent/WeKnora/internal/modules/career/repository"
)

type Service struct{ store repository.Store }

func New(store repository.Store) *Service { return &Service{store: store} }

func (s *Service) Get(ctx context.Context, scope repository.Scope, id string) (repository.Record, error) {
	if s == nil || s.store == nil {
		return repository.Record{}, repository.ErrNotFound
	}
	return s.store.Get(ctx, scope, id)
}

func (s *Service) Put(ctx context.Context, scope repository.Scope, record repository.Record) error {
	if s == nil || s.store == nil {
		return repository.ErrNotFound
	}
	return s.store.Put(ctx, scope, record)
}
