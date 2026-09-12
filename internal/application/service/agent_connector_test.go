package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openConnectorAssemblyDB builds the installation tables the visibility gate
// reads. The wiring under test is the production assembly itself; nothing
// here constructs a registry or a facade by hand.
func openConnectorAssemblyDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&repoappconn.InstallationRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func connectorAssemblyCtx(tenant uint64) context.Context {
	ctx := context.Background()
	if tenant != 0 {
		ctx = context.WithValue(ctx, types.TenantIDContextKey, tenant)
	}
	return ctx
}

// TestRealSessionAssemblyMountsAppConnectorTool proves the wiring on the REAL
// authenticated session creation path: the engine is built by
// CreateAgentEngine — the exact entry the session QA path calls — and the
// tool the engine offers to the model is observed on the model boundary. A
// registry unit test cannot substitute for this (T13 F-1 lesson).
func TestRealSessionAssemblyMountsAppConnectorTool(t *testing.T) {
	db := openConnectorAssemblyDB(t)
	if err := db.Create(&repoappconn.InstallationRow{
		ID: "inst-7", TenantID: 7, AppID: "github-oc", AppVersion: "1.0.0",
		State: appconn.InstallationActive,
	}).Error; err != nil {
		t.Fatal(err)
	}
	chatModel := &fakeAgentChatModel{}
	svc := &agentService{db: db}

	engine, err := svc.CreateAgentEngine(
		connectorAssemblyCtx(7),
		&types.AgentConfig{AllowedTools: []string{tools.ToolThinking}},
		chatModel, nil, nil, "sess-1", "msg-1",
	)
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	if _, err := engine.Execute(connectorAssemblyCtx(7), "sess-1", "msg-1", "hello", nil); err != nil {
		t.Fatalf("engine execute: %v", err)
	}
	if !toolOffered(chatModel.lastToolNames, tools.ToolAppConnector) {
		t.Fatalf("app_connector must be offered on the real session path, got %v", chatModel.lastToolNames)
	}
}

// Installation visibility gates the mounting: no active installation (or a
// disabled one) means the tenant has no connector surface at all and the
// tool must not be advertised.
func TestRealSessionAssemblyGatesAppConnectorOnInstallationVisibility(t *testing.T) {
	t.Run("no installation rows at all", func(t *testing.T) {
		db := openConnectorAssemblyDB(t)
		chatModel := &fakeAgentChatModel{}
		svc := &agentService{db: db}
		engine, err := svc.CreateAgentEngine(
			connectorAssemblyCtx(7),
			&types.AgentConfig{AllowedTools: []string{tools.ToolThinking}},
			chatModel, nil, nil, "sess-1", "msg-1",
		)
		if err != nil {
			t.Fatalf("create engine: %v", err)
		}
		if _, err := engine.Execute(connectorAssemblyCtx(7), "sess-1", "msg-1", "hello", nil); err != nil {
			t.Fatalf("engine execute: %v", err)
		}
		if toolOffered(chatModel.lastToolNames, tools.ToolAppConnector) {
			t.Fatal("tool must not be offered without any installation")
		}
	})
	t.Run("installation disabled", func(t *testing.T) {
		db := openConnectorAssemblyDB(t)
		if err := db.Create(&repoappconn.InstallationRow{
			ID: "inst-7", TenantID: 7, AppID: "github-oc", AppVersion: "1.0.0",
			State: appconn.InstallationDisabled,
		}).Error; err != nil {
			t.Fatal(err)
		}
		chatModel := &fakeAgentChatModel{}
		svc := &agentService{db: db}
		engine, err := svc.CreateAgentEngine(
			connectorAssemblyCtx(7),
			&types.AgentConfig{AllowedTools: []string{tools.ToolThinking}},
			chatModel, nil, nil, "sess-1", "msg-1",
		)
		if err != nil {
			t.Fatalf("create engine: %v", err)
		}
		if _, err := engine.Execute(connectorAssemblyCtx(7), "sess-1", "msg-1", "hello", nil); err != nil {
			t.Fatalf("engine execute: %v", err)
		}
		if toolOffered(chatModel.lastToolNames, tools.ToolAppConnector) {
			t.Fatal("tool must not be offered for a disabled installation")
		}
	})
	t.Run("no tenant in context", func(t *testing.T) {
		db := openConnectorAssemblyDB(t)
		if err := db.Create(&repoappconn.InstallationRow{
			ID: "inst-7", TenantID: 7, AppID: "github-oc", AppVersion: "1.0.0",
			State: appconn.InstallationActive,
		}).Error; err != nil {
			t.Fatal(err)
		}
		chatModel := &fakeAgentChatModel{}
		svc := &agentService{db: db}
		engine, err := svc.CreateAgentEngine(
			connectorAssemblyCtx(0),
			&types.AgentConfig{AllowedTools: []string{tools.ToolThinking}},
			chatModel, nil, nil, "sess-1", "msg-1",
		)
		if err != nil {
			t.Fatalf("create engine: %v", err)
		}
		if _, err := engine.Execute(connectorAssemblyCtx(0), "sess-1", "msg-1", "hello", nil); err != nil {
			t.Fatalf("engine execute: %v", err)
		}
		if toolOffered(chatModel.lastToolNames, tools.ToolAppConnector) {
			t.Fatal("tool must not be offered without a tenant")
		}
	})
}
