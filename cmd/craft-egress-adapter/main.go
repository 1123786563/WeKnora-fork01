// Command craft-egress-adapter runs the per-Run model-egress attempt
// authority (internal/modules/craftegress) as the runtime-side producer of
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
	"syscall"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craftegress"
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
	adapter, err := craftegress.NewCraftEgressAdapter(craftegress.CraftEgressAdapterConfig{
		GatewayBaseURL: gatewayURL,
		Credential:     credential,
		JournalPath:    journalPath,
	})
	if err != nil {
		log.Fatalf("craft-egress-adapter: %v", err)
	}
	defer func() { _ = adapter.Close() }()

	server := &http.Server{Addr: listen, Handler: adapter, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Printf("craft-egress-adapter: listening on %s, gateway %s", listen, gatewayURL)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("craft-egress-adapter: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("craft-egress-adapter: shutdown: %v", err)
	}
}
