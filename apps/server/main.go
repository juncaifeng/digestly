// digestly-server: digestly core 的 HTTP 外壳。
// 开发期独立运行(:3210)与 vite 联调；桌面端由 Tauri 作为 sidecar 拉起。
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/juncaifeng/digestly/core/collector"
	"github.com/juncaifeng/digestly/core/pipeline"
	"github.com/juncaifeng/digestly/core/scheduler"
	"github.com/juncaifeng/digestly/core/store"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:3210", "listen address")
	dataDir := flag.String("data", "data", "data directory")
	flag.Parse()

	if err := os.MkdirAll(filepath.Join(*dataDir, "collectors"), 0o755); err != nil {
		log.Fatal(err)
	}
	collector.JSDir = filepath.Join(*dataDir, "collectors")
	if err := collector.LoadJSScripts(); err != nil {
		log.Printf("load js collectors: %v", err)
	}

	st, err := store.Open(filepath.Join(*dataDir, "digestly.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	chain := pipeline.NewChain(st,
		pipeline.Source{Rules: map[string]string{}},
		pipeline.Summary{MaxRunes: 200},
		pipeline.Keywords{TopN: 5},
	)
	sched := scheduler.New(st, chain)
	sched.Start()
	defer sched.Stop()

	r := chi.NewRouter()
	r.Use(middleware.Logger, middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:*", "http://127.0.0.1:*", "tauri://localhost", "http://tauri.localhost"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type"},
	}))
	r.Route("/api", func(r chi.Router) {
		registerFeedRoutes(r, st, sched)
		registerItemRoutes(r, st)
		registerCollectorRoutes(r)
		registerPipelineRoutes(r, chain)
	})

	log.Printf("digestly server listening on %s (data: %s)", *addr, *dataDir)
	log.Fatal(http.ListenAndServe(*addr, r))
}
