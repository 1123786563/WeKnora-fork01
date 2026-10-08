// Command craft-egress-adapter runs the per-Run model-egress attempt
// authority (internal/craftegress) as the runtime-side producer of
// X-Craft-Activity-ID. Configuration comes exclusively from the environment:
//
//	CRAFT_EGRESS_LISTEN              listen address (default 127.0.0.1:8787)
//	CRAFT_EGRESS_GATEWAY_URL         required http(s) Craft model gateway base URL
//	CRAFT_EGRESS_CREDENTIAL          required execution credential (never a literal in source)
//	CRAFT_EGRESS_JOURNAL             required durable attempt-journal file path
//	CRAFT_EGRESS_ALLOW_PRIVATE_TARGET set to 1 to permit loopback/private gateway hosts
//	                                  (local development and same-host proofs only)
//
// The adapter must be the only configured provider endpoint of the sandboxed
// runtime; the gateway refuses model forwards without the identity this
// process mints and journals before every physical send.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Tencent/WeKnora/internal/craftegress"
)

func main() {
	listen := os.Getenv("CRAFT_EGRESS_LISTEN")
	if listen == "" {
		listen = "127.0.0.1:8787"
	}
	gatewayURL := os.Getenv("CRAFT_EGRESS_GATEWAY_URL")
	if gatewayURL == "" {
		log.Fatal("craft-egress-adapter: CRAFT_EGRESS_GATEWAY_URL is required")
	}
	credential := os.Getenv("CRAFT_EGRESS_CREDENTIAL")
	if credential == "" {
		log.Fatal("craft-egress-adapter: CRAFT_EGRESS_CREDENTIAL is required")
	}
	journalPath := os.Getenv("CRAFT_EGRESS_JOURNAL")
	if journalPath == "" {
		log.Fatal("craft-egress-adapter: CRAFT_EGRESS_JOURNAL is required")
	}
	allowPrivate := os.Getenv("CRAFT_EGRESS_ALLOW_PRIVATE_TARGET") == "1"
	if _, err := craftegress.ValidateGatewayTarget(gatewayURL, allowPrivate); err != nil {
		log.Fatalf("craft-egress-adapter: %v", err)
	}
	forwardTimeout := craftegress.DefaultForwardTimeout()
	if raw := strings.TrimSpace(os.Getenv("CRAFT_EGRESS_FORWARD_TIMEOUT")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			log.Fatalf("craft-egress-adapter: CRAFT_EGRESS_FORWARD_TIMEOUT must be a positive duration, got %q", raw)
		}
		// An adapter budget BELOW the gateway's forward budget aborts slow
		// generations mid-flight, parks the attempt as unknown-outcome and
		// deadlocks same-fingerprint retries on ACTIVITY_UNRESOLVED. Refuse
		// such configurations loudly instead of silently.
		if parsed < craftegress.DefaultForwardTimeout() {
			log.Fatalf("craft-egress-adapter: CRAFT_EGRESS_FORWARD_TIMEOUT (%s) must be >= the gateway-aligned default (%s); raise the gateway budget together, not the adapter alone", parsed, craftegress.DefaultForwardTimeout())
		}
		forwardTimeout = parsed
	}
	adapter, err := craftegress.NewCraftEgressAdapter(craftegress.CraftEgressAdapterConfig{
		GatewayBaseURL:     gatewayURL,
		Credential:         credential,
		JournalPath:        journalPath,
		ForwardTimeout:     forwardTimeout,
		AllowPrivateTarget: allowPrivate,
	})
	if err != nil {
		log.Fatalf("craft-egress-adapter: %v", err)
	}
	defer func() { _ = adapter.Close() }()

	server := &http.Server{Addr: listen, Handler: adapter, ReadHeaderTimeout: 10 * time.Second}
	serveErr := make(chan error, 1)
	go func() {
		log.Printf("craft-egress-adapter: listening on %s, gateway %s", listen, gatewayURL)
		// Serve errors flow back to the main goroutine so the deferred
		// adapter.Close() still runs; log.Fatal inside the goroutine would
		// os.Exit straight past it and leak the journal handle.
		serveErr <- server.ListenAndServe()
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	serveFailed := false
	select {
	case <-stop:
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("craft-egress-adapter: serve: %v", err)
			serveFailed = true
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("craft-egress-adapter: shutdown: %v", err)
	}
	if serveFailed {
		// Exit NON-zero: K8s onFailure restarts only non-zero exits, and
		// alerting treats zero as healthy — a bind failure must be visible.
		os.Exit(1)
	}
}
