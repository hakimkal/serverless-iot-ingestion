package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"

	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/hakimkal/serverless-ingest/internal"
	"github.com/hakimkal/serverless-ingest/internal/config"
	"github.com/hakimkal/serverless-ingest/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Payload struct {
	Source string `json:"source,omitempty"`
	Data   any    `json:"data,omitempty"`
}

var queries *db.Queries
var pool *pgxpool.Pool
var redisUrl string

type apiKeyExtractor func(r *http.Request) string

func init() {
	ctx := context.Background()

	global, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Unable to load environment")
	}
	conn, err := pgx.Connect(ctx, global.DBUrl)

	if err != nil {
		log.Fatal(err)
	}

	//defer conn.Close(ctx)

	dbConfig, err := pgxpool.ParseConfig(global.DBUrl)

	if err != nil {
		log.Fatal(err)
	}

	dbConfig.MaxConns = 20
	dbConfig.MinConns = 5 // minimum idle connections
	dbConfig.MaxConnLifetime = 30 * time.Minute
	dbConfig.MaxConnIdleTime = 5 * time.Minute
	dbConfig.HealthCheckPeriod = 1 * time.Minute

	pool, err = pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		log.Fatal(err)
	}
	redisUrl = global.RedisUrl
	queries = db.New(conn)
	//defer pool.Close()

}

// Single Payload
//func ingestHandler(w http.ResponseWriter, r *http.Request) {
//	var p Payload
//	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
//		http.Error(w, "invalid json", http.StatusBadRequest)
//		return
//	}
//
//	// Accept raw JSON too (if no wrapper)
//	if p.Data == nil {
//		var raw map[string]any
//		if err := json.NewDecoder(r.Body).Decode(&raw); err == nil {
//			p.Data = raw
//		}
//	}
//
//	ctx := r.Context()
//
//	data, _ := convertPayloadData(p.Data)
//	err := queries.InsertEvent(ctx, db.InsertEventParams{
//		Payload: data,
//		Source:  pgtype.Text{String: p.Source, Valid: true},
//	})
//	if err != nil {
//		log.Printf("DB error: %v", err)
//		http.Error(w, "Failed to save", http.StatusInternalServerError)
//		return
//	}
//
//	w.WriteHeader(http.StatusCreated)
//	w.Header().Set("Content-Type", "application/json")
//	w.Write([]byte(`{"status":"saved"}`))
//}
//func convertPayloadData(data any) ([]byte, error) {
//	var payload []byte
//
//	switch v := data.(type) {
//	case []byte:
//		payload = v
//	case string:
//		payload = []byte(v)
//	default:
//		b, err := json.Marshal(v)
//		if err != nil {
//			return nil, err
//		}
//		payload = b
//	}
//	return payload, nil
//}

// ========================
// Batch Ingest Handler
// ========================
func ingestHandler(w http.ResponseWriter, r *http.Request) {
	var events []Payload
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	// Try to decode array first (batch)
	if err := decoder.Decode(&events); err != nil {
		// Fallback to single event
		var single Payload
		if err := decoder.Decode(&single); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		events = []Payload{single}
	}

	if len(events) == 0 || len(events) > 500 {
		http.Error(w, "batch size must be 1–500", http.StatusBadRequest)
		return
	}

	// Batch insert using COPY (fastest possible)
	ctx := r.Context()
	tx, err := pool.Begin(ctx)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	rows := make([][]any, 0, len(events))

	for _, e := range events {
		payloadJSON, err := json.Marshal(e.Data)
		if err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		rows = append(rows, []any{
			payloadJSON,
			e.Source,
		})
	}

	count, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"events"},
		[]string{"payload", "source"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		log.Printf("COPY failed: %v", err)
		http.Error(w, "failed to save", http.StatusInternalServerError)
		return
	}

	// COMMIT
	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "commit failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"status":  "saved",
		"count":   count,
		"batchId": time.Now().UnixNano(),
	})
}
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok"))
}

func main() {

	//defer db.Close()

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Get("/health", healthHandler)

	internal.NewRateLimiter(redisUrl)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(internal.RequireAPIKey)
		r.Use(internal.RateLimit)
		r.Post("/ingest", ingestHandler)
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		log.Printf("Server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Graceful shutdown (works on Cloud Run, Fly.io, etc.)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}
