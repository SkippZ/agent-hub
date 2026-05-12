package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"agent-hub/internal/api"
	"agent-hub/internal/config"
)

func main() {
	cfgPath := config.ConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	handler := api.New()

	mux := http.NewServeMux()
	handler.Register(mux)

	addr := ":8080"
	if v := os.Getenv("AGENT_HUB_PORT"); v != "" {
		addr = ":" + v
	}

	server := &http.Server{
		Addr:    addr,
		Handler: withCORS(mux),
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		log.Println("Shutting down...")
		server.Close()
	}()

	fmt.Printf("Agent Hub running at http://localhost%s\n", addr)
	fmt.Printf("Config: projects_dir=%s\n", cfg.ProjectsDir)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
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
