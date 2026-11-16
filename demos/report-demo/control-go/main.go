package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	path := flag.String("config", os.Getenv("REPORT_DEMO_CONFIG"), "private absolute config JSON path")
	migrate := flag.Bool("migrate", false, "apply embedded migrations with separate migrator DSN and exit")
	backup := flag.String("backup", "", "export scoped PG facts and immutable content to a standalone private JSON package")
	replay := flag.String("replay", "", "verify a standalone snapshot offline, without loading config or opening any connection")
	restore := flag.String("restore", "", "restore snapshot to empty isolated test DB from DEMO_RESTORE_DSN")
	restoreRoot := flag.String("restore-content-root", "", "isolated content directory for test restore")
	contractDir := flag.String("export-contracts", "", "export JSON schemas inferred from the actual application types, without loading private config")
	canonical := flag.String("canonical-json", "", "strictly parse and JCS-hash a JSON file offline for cross-language contract vectors")
	flag.Parse()
	if *contractDir != "" {
		if err := ExportContracts(*contractDir); err != nil {
			log.Fatal("contract export failed: ", err)
		}
		writeCLI(map[string]any{"status": "CONTRACTS_EXPORTED", "private_config_loaded": false})
		return
	}
	if *canonical != "" {
		out, err := CanonicalJSONFile(*canonical)
		if err != nil {
			log.Fatal("strict canonical input rejected")
		}
		writeCLI(out)
		return
	}
	if *replay != "" {
		out, err := ReplaySnapshot(*replay)
		if err != nil {
			log.Fatal("offline verification failed: ", err)
		}
		writeCLI(out)
		return
	}
	cfg, cat, material, err := LoadConfig(*path)
	if err != nil {
		log.Fatalf("configuration failed: %s", err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *restore != "" {
		snapshot, err := ReadSnapshot(*restore)
		if err != nil {
			log.Fatal("snapshot invalid: ", err)
		}
		out, err := RestoreSnapshot(ctx, snapshot, os.Getenv("DEMO_RESTORE_DSN"), cfg.DSN, *restoreRoot, cfg.ArtifactRoot)
		if err != nil {
			log.Fatalf("isolated restore failed (%T); inspect/reconcile the original target before retrying", err)
		}
		writeCLI(out)
		return
	}
	dsn := cfg.DSN
	if *migrate {
		dsn = cfg.MigratorDSN
	}
	store, err := OpenStore(ctx, dsn, cfg.Scope, cfg.LeaseSeconds)
	if err != nil {
		log.Fatal("database unavailable (connection details withheld)")
	}
	defer store.Pool.Close()
	if *backup != "" {
		store.ExecutionProfile = cfg.ExecutionProfile
		snapshot, err := store.Backup(ctx, Files{Root: cfg.ArtifactRoot}, cat, material)
		if err != nil {
			log.Fatalf("snapshot preparation failed (%T)", err)
		}
		if err = WriteSnapshot(*backup, snapshot); err != nil {
			log.Fatal("snapshot publish failed")
		}
		writeCLI(map[string]any{"status": "BACKED_UP", "digest": snapshot.Digest, "artifacts": len(snapshot.Payload.Contents), "sessions_included": false, "model_credentials_included": false})
		return
	}
	if *migrate {
		if err = store.Migrate(ctx); err != nil {
			log.Fatalf("migration failed (%T)", err)
		}
		log.Print("schema migration complete")
		return
	}
	srv := &http.Server{Addr: cfg.Listen, Handler: NewServer(store, cfg, cat, material).Routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = srv.Shutdown(shutdown)
	}()
	log.Printf("report control listening at %s; session=%s", cfg.Listen, store.Session)
	if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func writeCLI(value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		log.Fatal("CLI response encoding failed")
	}
	fmt.Println(string(raw))
}
