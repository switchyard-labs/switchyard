// Switchyard control plane. Serves the Nift-built web application and the JSON
// API backed by Trestle (coordination truth) and Cloudflare Artifacts (Git truth).
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

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
	if err := a.Provision(); err != nil {
		log.Fatalf("provision: %v", err)
	}
	if d, err := time.ParseDuration(*reconcile); err == nil {
		a.StartReconciler(context.Background(), d)
	}
	log.Printf("switchyard control plane listening on %s (trestle=%s, namespace=%s)", *listen, *treBase, *ns)
	log.Fatal(http.ListenAndServe(*listen, a.Handler()))
}