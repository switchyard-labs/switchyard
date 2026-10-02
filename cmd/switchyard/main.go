// Switchyard control plane. Serves the Nift-built web application and the JSON
// API backed by Trestle (coordination truth) and Cloudflare Artifacts (Git truth).
package main

import (
	"context"
	"crypto/rand"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"switchyard/internal/agent"

	"switchyard/internal/app"
	"switchyard/internal/artifacts"
	"switchyard/internal/trestle"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	listen := flag.String("listen", envOr("SWITCHYARD_LISTEN", "127.0.0.1:8080"), "listen address")
	treBase := flag.String("trestle", envOr("SWITCHYARD_TRESTLE_URL", "http://127.0.0.1:7350"), "Trestle base URL")
	treUser := flag.String("trestle-user", envOr("SWITCHYARD_TRESTLE_USER", "admin"), "Trestle admin user")
	trePass := flag.String("trestle-pass", envOr("SWITCHYARD_TRESTLE_PASS", ""), "Trestle admin password")
	acc := flag.String("artifacts-account", envOr("SWITCHYARD_ARTIFACTS_ACCOUNT", ""), "Cloudflare account id")
	ns := flag.String("artifacts-namespace", envOr("SWITCHYARD_ARTIFACTS_NAMESPACE", "switchyard-cp0"), "Artifacts namespace")
	tokCmd := flag.String("artifacts-token-cmd", envOr("SWITCHYARD_ARTIFACTS_TOKEN_CMD", "/opt/cp0/switchyard/token.sh"), "token helper")
	static := flag.String("static", envOr("SWITCHYARD_STATIC_DIR", "./public"), "Nift build output directory")
	data := flag.String("data", envOr("SWITCHYARD_DATA_DIR", "./data"), "control-plane data dir")
	strutBin := flag.String("strut-bin", envOr("SWITCHYARD_STRUT_BIN", "/opt/cp0/switchyard/deterministic-worker"), "Strut worker binary")
	reconcile := flag.String("reconcile-interval", envOr("SWITCHYARD_RECONCILE_INTERVAL", "15s"), "reconciliation interval")
	queueID := flag.String("queue-id", envOr("SWITCHYARD_QUEUE_ID", ""), "Cloudflare queue id for Artifacts events (fast path)")
	queueInt := flag.String("queue-pull-interval", envOr("SWITCHYARD_QUEUE_PULL_INTERVAL", "5s"), "queue pull interval")
	wfInt := flag.String("workflow-interval", envOr("SWITCHYARD_WORKFLOW_INTERVAL", "2s"), "workflow runner interval")
	flag.Parse()

	if *trePass == "" {
		log.Fatal("SWITCHYARD_TRESTLE_PASS is required")
	}
	if *acc == "" {
		log.Fatal("SWITCHYARD_ARTIFACTS_ACCOUNT is required")
	}

	tre := trestle.New(*treBase, *treUser, *trePass)
	art := artifacts.New(*acc, *ns, *tokCmd)
	a := app.New(tre, art, *static, *data)
	a.StrutBin = *strutBin
	// agent substrate: credential store keyed from env or a persisted data key
	key := loadOrCreateKey(filepath.Join(*data, "secret.key"))
	a.Secrets = agent.NewCredentialStore(
		func(v map[string]any, idem string) error { _, _, e := tre.CreateRecord("credentials", v, idem); return e },
		func() ([]map[string]any, error) { return tre.ListRecords("credentials", "") },
		func(id string) error {
			rid, ver, vals, e := tre.FindRecord("credentials", `id = "`+id+`"`)
			if e != nil {
				return e
			}
			if rid == "" || len(vals) == 0 {
				return os.ErrNotExist
			}
			return tre.DeleteRecord("credentials", rid, ver)
		},
		key)
	a.Roles = agent.BuiltinRoles
	a.Runner = &agent.DeterministicRunner{
		StrutBin: *strutBin, WorkDir: filepath.Join(*data, "scratch"),
		Apply: func(exec *agent.Execution, path, content string) (string, error) {
			return "", nil // handled by the run handler via refs
		},
	}
	if err := a.Provision(); err != nil {
		log.Fatalf("provision: %v", err)
	}
	if d, err := time.ParseDuration(*reconcile); err == nil {
		a.StartReconciler(context.Background(), d)
	}
	// event-driven fast path (Cloudflare queue) — optional; reconciliation
	// remains the safety net if no queue is configured.
	if *queueID != "" {
		a.Queue = app.NewQueueConsumer(*acc, *queueID, art.AccountToken)
	}
	if qi, err := time.ParseDuration(*queueInt); err == nil {
		a.StartEventConsumer(context.Background(), qi)
	}
	// durable workflow runner (CP7)
	if wi, err := time.ParseDuration(*wfInt); err == nil {
		a.StartWorkflowRunner(context.Background(), wi)
	}
	log.Printf("switchyard control plane listening on %s (trestle=%s, namespace=%s)", *listen, *treBase, *ns)
	log.Fatal(http.ListenAndServe(*listen, a.Handler()))
}
// loadOrCreateKey returns a 32-byte AES key from env or a persisted file.
func loadOrCreateKey(path string) []byte {
	if k := os.Getenv("SWITCHYARD_SECRET_KEY"); len(k) >= 32 {
		return []byte(k)[:32]
	}
	if b, err := os.ReadFile(path); err == nil && len(b) == 32 {
		return b
	}
	k := make([]byte, 32)
	_, _ = rand.Read(k)
	_ = os.WriteFile(path, k, 0600)
	return k
}
