package service

// Pass B test-support shim (Ruling 2026-09-24-TEST-SUPPORT-SHIM).
//
// The datasource service test files migrated from internal/application/service
// in 26-datasource B2-DS.3 share the host-package fake kbDeleteDSRepo defined
// in knowledgebase_delete_datasource_test.go (K owner, staying behind for its
// own K-side test cases). Four migrated files (credential_refresh / reindex /
// stream / sync_cancel) reference the type and its factory by name, including
// struct embedding and composite literals, so a same-name minimal definition
// in this package is the only compile-legal bridge.
//
// Definition is verbatim from the host original (fields byKB/deleted are set
// by migrated composite literals). Ledger: 「临时测试装置垫片（B5 清理范围）」,
// tracked in the B2-DS.3 report; remove_at: ib2 (first-come-first-delete, at
// the latest B5) — collapses back to a single definition when the K-side test
// fixtures migrate in the K4 supplementary window.

import (
	"context"
	"errors"
	"sync"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type kbDeleteDSRepo struct {
	mu        sync.Mutex
	byKB      map[string][]*types.DataSource
	deleted   map[string]bool
	deleteIDs []string
}

func newKBDeleteDSRepo(kbID string, ds ...*types.DataSource) *kbDeleteDSRepo {
	r := &kbDeleteDSRepo{
		byKB:    map[string][]*types.DataSource{kbID: ds},
		deleted: map[string]bool{},
	}
	return r
}

func (r *kbDeleteDSRepo) Create(_ context.Context, _ *types.DataSource) error { return nil }
func (r *kbDeleteDSRepo) FindByID(_ context.Context, id string) (*types.DataSource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted[id] {
		return nil, errors.New("data source not found")
	}
	for _, list := range r.byKB {
		for _, ds := range list {
			if ds.ID == id {
				return ds, nil
			}
		}
	}
	return nil, errors.New("data source not found")
}
func (r *kbDeleteDSRepo) FindByKnowledgeBase(_ context.Context, kbID string) ([]*types.DataSource, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var active []*types.DataSource
	for _, ds := range r.byKB[kbID] {
		if !r.deleted[ds.ID] {
			active = append(active, ds)
		}
	}
	return active, nil
}
func (r *kbDeleteDSRepo) Update(_ context.Context, _ *types.DataSource) error { return nil }
func (r *kbDeleteDSRepo) UpdateSyncState(_ context.Context, _ *types.DataSource) error {
	return nil
}
func (r *kbDeleteDSRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleted[id] = true
	r.deleteIDs = append(r.deleteIDs, id)
	return nil
}
func (r *kbDeleteDSRepo) FindActive(_ context.Context) ([]*types.DataSource, error) {
	return nil, nil
}

var _ interfaces.DataSourceRepository = (*kbDeleteDSRepo)(nil)
