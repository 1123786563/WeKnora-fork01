// Command connector-control runs the open-connector control worker: the
// only process that holds the open-connector runtime ADMIN credential.
//
// Ordinary API/Agent processes never receive this credential — they only
// enqueue rows into connector_operations_outbox. This binary reads the admin
// secret from a dedicated mount file (CONNECTOR_CONTROL_ADMIN_SECRET_FILE,
// default /run/secrets/connector-admin-token) and fails closed while the
// mount is absent.
//
// Environment:
//
//	DB_DRIVER=postgres DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME   (postgres)
//	DB_DRIVER=sqlite DB_PATH=./data/connector-control.db             (lite)
//	CONNECTOR_CONTROL_RUNTIME_BASE_URL   default http://open-connector:8080
//	CONNECTOR_CONTROL_RUNTIME_ALLOWLIST  comma-separated internal addresses
//	                                    (default: DefaultInternalAllowlist)
//	CONNECTOR_CONTROL_ADMIN_SECRET_FILE  default /run/secrets/connector-admin-token
//	CONNECTOR_CONTROL_SECRET_DIR         default ./data/connector-secrets
//	CONNECTOR_CONTROL_SECRET_KEY_FILE    REQUIRED — the secret sink is
//	                                    encrypted at rest (T16): a mounted
//	                                    0400 key file (64 hex / base64 / 32
//	                                    raw bytes); startup refuses without it
//	CONNECTOR_CONTROL_OWNER_ID           default connector-control
//	CONNECTOR_CONTROL_POLL_INTERVAL      default 500ms
package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/connectorcontrol"
	"github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type controlConfig struct {
	driver       string
	pgHost       string
	pgPort       string
	pgUser       string
	pgPassword   string
	pgName       string
	sqlitePath   string
	runtimeBase  string
	allowlist    []string
	secretFile   string
	secretDir    string
	secretKey    string
	ownerID      string
	pollInterval time.Duration
	adminTimeout time.Duration
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func loadConfig() (controlConfig, error) {
	cfg := controlConfig{
		driver:       os.Getenv("DB_DRIVER"),
		pgHost:       os.Getenv("DB_HOST"),
		pgPort:       os.Getenv("DB_PORT"),
		pgUser:       os.Getenv("DB_USER"),
		pgPassword:   os.Getenv("DB_PASSWORD"),
		pgName:       os.Getenv("DB_NAME"),
		sqlitePath:   env("DB_PATH", "./data/connector-control.db"),
		runtimeBase:  env("CONNECTOR_CONTROL_RUNTIME_BASE_URL", "http://open-connector:8080"),
		allowlist:    connectorcontrol.ParseAllowlist(os.Getenv("CONNECTOR_CONTROL_RUNTIME_ALLOWLIST")),
		secretFile:   env("CONNECTOR_CONTROL_ADMIN_SECRET_FILE", "/run/secrets/connector-admin-token"),
		secretDir:    env("CONNECTOR_CONTROL_SECRET_DIR", "./data/connector-secrets"),
		secretKey:    os.Getenv("CONNECTOR_CONTROL_SECRET_KEY_FILE"),
		ownerID:      env("CONNECTOR_CONTROL_OWNER_ID", "connector-control"),
		adminTimeout: connectorcontrol.AdminDefaultTimeout,
	}
	poll, err := time.ParseDuration(env("CONNECTOR_CONTROL_POLL_INTERVAL", "500ms"))
	if err != nil || poll <= 0 {
		return cfg, errors.New("invalid CONNECTOR_CONTROL_POLL_INTERVAL")
	}
	cfg.pollInterval = poll
	if cfg.secretKey == "" {
		return cfg, errors.New("CONNECTOR_CONTROL_SECRET_KEY_FILE is required: the secret sink is encrypted at rest (T16); refusing to run a plaintext sink")
	}
	switch cfg.driver {
	case "postgres", "sqlite":
	default:
		return cfg, fmt.Errorf("unsupported DB_DRIVER %q (want postgres or sqlite)", cfg.driver)
	}
	return cfg, nil
}

// openDB mirrors the server's DSN construction (key-value postgres DSN, WAL
// sqlite) plus _txlock=immediate on sqlite so concurrent control-worker
// statements serialize on the write lock from the start of a transaction.
func openDB(cfg controlConfig) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		Logger:  gormlogger.Default.LogMode(gormlogger.Warn),
		NowFunc: func() time.Time { return time.Now().UTC() },
	}
	switch cfg.driver {
	case "postgres":
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
			cfg.pgHost, cfg.pgPort, cfg.pgUser, url.QueryEscape(cfg.pgPassword), cfg.pgName)
		return gorm.Open(postgres.Open(dsn), gormCfg)
	case "sqlite":
		if err := os.MkdirAll(filepath.Dir(cfg.sqlitePath), 0o755); err != nil {
			return nil, err
		}
		dsn := cfg.sqlitePath + "?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on&_txlock=immediate"
		return gorm.Open(sqlite.Open(dsn), gormCfg)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q", cfg.driver)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := openDB(cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	adminSecret := connectorcontrol.FileAdminSecretSource(cfg.secretFile)
	adminClient, err := connectorcontrol.NewRuntimeAdminClient(connectorcontrol.AdminClientConfig{
		BaseURL:     cfg.runtimeBase,
		Allowlist:   cfg.allowlist,
		AdminSecret: adminSecret,
		Timeout:     cfg.adminTimeout,
	})
	if err != nil {
		return fmt.Errorf("admin client: %w", err)
	}
	sink, err := connectorcontrol.NewEncryptedFileSecretSink(
		cfg.secretDir, connectorcontrol.FileSecretKeySource(cfg.secretKey))
	if err != nil {
		return fmt.Errorf("secret sink: %w", err)
	}
	store := appconnector.NewOCStore(db)
	workerCfg := connectorcontrol.DefaultWorkerConfig(cfg.ownerID)
	workerCfg.PollInterval = cfg.pollInterval
	worker, err := connectorcontrol.NewControlWorker(store, store, adminClient, sink, adminSecret, workerCfg)
	if err != nil {
		return err
	}

	// Startup log: configuration facts only — never the secret, never the
	// password. The admin secret FILE path is operational data, its content
	// is not.
	allowlistSize := len(cfg.allowlist)
	if allowlistSize == 0 {
		allowlistSize = len(connectorcontrol.DefaultInternalAllowlist)
	}
	logger.Infof(ctx, "[connector-control] starting: owner=%s driver=%s runtime=%s allowlist=%d lease=%s renew=%s backoff<=%s poll=%s secret_file=%s",
		cfg.ownerID, cfg.driver, adminClient.Address(), allowlistSize,
		workerCfg.Lease, workerCfg.RenewInterval, workerCfg.BackoffMax, cfg.pollInterval, cfg.secretFile)
	return worker.RunForever(ctx, cfg.pollInterval)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "connector-control:", err)
		os.Exit(1)
	}
}
