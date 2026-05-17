package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"agent-hub/internal/api"
	"agent-hub/internal/config"
	"agent-hub/internal/db"
)

func main() {
	root := config.ProjectRoot()
	log.Printf("Project root: %s", root)

	cfgPath := config.ConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	dbPath := filepath.Join(root, "agent-hub.db")
	store, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer store.Close()

	handler := api.New(store, cfg, root)

	mux := http.NewServeMux()
	handler.Register(mux)

	// Serve frontend dist for non-API paths
	frontendDist := filepath.Join(root, "frontend", "dist")
	log.Printf("Frontend dist: %s", frontendDist)

	if info, err := os.Stat(frontendDist); err != nil {
		log.Printf("Warning: frontend dist not found at %s: %v", frontendDist, err)
	} else {
		log.Printf("Frontend dist found (isDir=%v)", info.IsDir())
	}

	addr := ":8080"
	if v := os.Getenv("AGENT_HUB_PORT"); v != "" {
		addr = ":" + v
	}

	server := &http.Server{
		Addr: addr,
		Handler: withLogging(withCORS(router(
			mux,
			http.FileServer(http.Dir(frontendDist)),
		))),
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("Shutting down...")
		handler.Shutdown()
		server.Close()
	}()

	fmt.Printf("Agent Hub running at http://localhost%s\n", addr)
	fmt.Printf("Config: projects_dir=%s\n", cfg.ProjectsDir)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

func router(apiHandler, fs http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		// SPA: serve index.html for any non-file path
		path := r.URL.Path
		if path != "/" && !strings.Contains(path, ".") {
			r.URL.Path = "/"
		}
		log.Printf("Serving static: %s", path)
		fs.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
