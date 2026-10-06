// Package container - workspace sandbox provider wiring.
package container

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// newSandboxManager is deliberately disabled. Every executable backend now
// comes from a named workspace configuration resolved at request time.
func newSandboxManager(
	_ *redis.Client,
	_ interfaces.SessionRepository,
) sandbox.Manager {
	return sandbox.NewDisabledManager()
}

func selectSessionBindingStore(
	redisClient *redis.Client,
	requireRedis bool,
) (sandbox.SessionSandboxBindingStore, string, error) {
	namespace := strings.TrimSpace(os.Getenv("WEKNORA_REDIS_NAMESPACE"))
	if namespace == "" {
		namespace = "weknora"
	}
	if redisClient != nil {
		store, err := sandbox.NewRedisSessionSandboxBindingStore(redisClient, namespace)
		if err != nil {
			return nil, "", fmt.Errorf("build redis binding store: %w", err)
		}
		return store, "redis", nil
	}
	_ = requireRedis
	logger.Warnf(context.Background(),
		"[sandbox] No Redis configured, using in-memory binding store (single-instance)")
	return sandbox.NewMemorySessionSandboxBindingStore(), "memory", nil
}

// sessionExistenceLookup is the narrow slice of SessionRepository the
// session existence checker actually needs. Declaring it here (rather than
// depending on interfaces.SessionRepository) keeps the checker easy to test
// and lets the container inject a nil repository in Lite mode without
// dragging the whole database contract along.
type sessionExistenceLookup interface {
	GetByID(ctx context.Context, tenantID uint64, id string) (*types.Session, error)
}

// sessionExistenceCheckerFor returns a SessionExistenceChecker backed by the
// tenant session repository. When the repository is unavailable (Lite mode
// without a database) the returned checker is permissive so single-process
// deployments still work; multi-instance production paths always resolve a
// real repository because the container refuses to boot without one.
func sessionExistenceCheckerFor(
	lookup sessionExistenceLookup,
) sandbox.SessionExistenceChecker {
	if lookup == nil {
		return sandbox.PermissiveSessionExistenceChecker{}
	}
	return &repositorySessionExistenceChecker{lookup: lookup}
}

// repositorySessionExistenceChecker adapts SessionRepository.GetByID onto the
// SessionExistenceChecker contract. gorm.ErrRecordNotFound → false, other
// errors propagate so the lifecycle coordinator preserves bindings under
// transient database failures.
type repositorySessionExistenceChecker struct {
	lookup sessionExistenceLookup
}

func (c *repositorySessionExistenceChecker) SessionExists(
	ctx context.Context,
	key sandbox.SessionSandboxKey,
) (bool, error) {
	if c == nil || c.lookup == nil {
		return true, nil
	}
	session, err := c.lookup.GetByID(ctx, key.TenantID, key.SessionID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, apperrors.ErrSessionNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("session existence check: %w", err)
	}
	return session != nil, nil
}

// buildGlobalSandboxConfig returns the process-wide *sandbox.Config that
// per-tenant overrides are merged onto.
func buildGlobalSandboxConfig() *sandbox.Config {
	cfg := sandbox.DefaultConfig()
	cfg.Type = sandbox.SandboxTypeDisabled
	return cfg
}

// newSharedSessionSandboxBindingStore is the process-wide singleton binding
// store. Every consumer of session-sandbox bindings — the tenant resolver
// that WRITES them, the preview no-egress checker and the craft lifecycle
// that READ them — must resolve this one instance: in Lite mode (no Redis)
// the store is an in-memory map, and two separately built instances would
// be mutually invisible, making the preview checker deny every issuance
// ("no current Docker sandbox binding") even though the resolver recorded
// a live none-network sandbox. Redis deployments are functionally shared
// anyway, but the singleton also avoids duplicate connection pools.
func newSharedSessionSandboxBindingStore(redisClient *redis.Client) sandbox.SessionSandboxBindingStore {
	store, kind, err := selectSessionBindingStore(redisClient, false)
	if err != nil {
		logger.Warnf(context.Background(),
			"[sandbox] shared session binding store unavailable: %v", err)
		return nil
	}
	logger.Infof(context.Background(),
		"[sandbox] shared session binding store configured: binding=%s (single instance for writers and readers)", kind)
	return store
}

// newTenantSandboxResolver wires the workspace-config resolver. The
// process-wide manager is disabled; agents without a selected config stay
// disabled as well. The binding store is the shared singleton (see
// newSharedSessionSandboxBindingStore) so preview no-egress checks observe
// this resolver's writes even in Lite mode.
func newTenantSandboxResolver(
	defaultManager sandbox.Manager,
	loader sandbox.TenantSandboxConfigLoader,
	store sandbox.SessionSandboxBindingStore,
	sessionRepo interfaces.SessionRepository,
	bootstrapper sandbox.SessionBootstrapper,
) sandbox.TenantSandboxResolver {
	ctx := context.Background()
	if store == nil {
		logger.Warnf(ctx, "Per-tenant sandbox config disabled: no shared binding store")
		return nil
	}
	resolver, err := sandbox.NewTenantSandboxResolver(sandbox.TenantSandboxResolverDeps{
		GlobalConfig:    buildGlobalSandboxConfig(),
		DefaultManager:  defaultManager,
		Loader:          loader,
		Store:           store,
		Checker:         sessionExistenceCheckerFor(sessionRepo),
		Bootstrapper:    bootstrapper,
		SharedTransport: sandbox.NewGuardedTransport(),
	})
	if err != nil {
		logger.Warnf(ctx,
			"Failed to initialize tenant sandbox resolver: %v "+
				"(per-tenant sandbox config disabled)", err)
		return nil
	}
	logger.Infof(ctx, "Tenant sandbox resolver configured (shared binding store)")
	return resolver
}
