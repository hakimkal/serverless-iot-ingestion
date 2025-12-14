package internal

import (
	"context"
	"log"
	"net/http"

	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ========================
// API Key Configuration
// ========================
type APIKeyConfig struct {
	RatePerMin float64 // tokens per minute
	Capacity   float64 // max tokens
}

// Whitelisted API | Can be injected from environment file or db at scale
var APIKeys = map[string]APIKeyConfig{
	"iot-node-satoshi":  {RatePerMin: 100, Capacity: 100},
	"iot-node-nakamoto": {RatePerMin: 200, Capacity: 200},
}

// ========================
// Rate Limiter
// ========================
type RateLimiter struct {
	mu    sync.Mutex
	local map[string]*TokenBucket
}

type TokenBucket struct {
	Tokens      float64
	LastUpdated time.Time
}

var redisClient *redis.Client
var rl *RateLimiter

// NewRateLimiter initializes Redis client if URL is provided
func NewRateLimiter(redisURL string) *RateLimiter {
	r := &RateLimiter{
		local: make(map[string]*TokenBucket),
	}

	if redisURL != "" {
		opt, _ := redis.ParseURL(redisURL)
		redisClient = redis.NewClient(opt)
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			log.Printf("Redis unreachable, falling back to in-memory limiter: %v", err)
			redisClient = nil
		} else {
			log.Println("Rate limiting using Redis")
		}
	}

	return r
}

// Allow checks if a request is allowed for a given key
func (rl *RateLimiter) Allow(key string) bool {
	cfg, ok := APIKeys[key]
	if !ok {
		return false // unknown key
	}

	if redisClient != nil {
		return rl.allowRedis(key, cfg)
	}
	return rl.allowLocal(key, cfg)
}

// ========================
// Redis Implementation
// ========================
func (rl *RateLimiter) allowRedis(key string, cfg APIKeyConfig) bool {
	script := `
local key = KEYS[1]
local ts_key = KEYS[2]
local now = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local capacity = tonumber(ARGV[3])

local tokens = tonumber(redis.call("GET", key))
local last_ts = tonumber(redis.call("GET", ts_key))

if not tokens then
	tokens = capacity
	last_ts = now
end

local delta = math.max(0, now - last_ts)
tokens = math.min(capacity, tokens + delta * rate)
if tokens < 1 then
	redis.call("SET", key, tokens)
	redis.call("SET", ts_key, now)
	return 0
end

tokens = tokens - 1
redis.call("SET", key, tokens)
redis.call("SET", ts_key, now)
return 1
`
	now := float64(time.Now().UnixNano()) / 1e9
	res, err := redisClient.Eval(context.Background(), script,
		[]string{"rl:" + key, "rl:" + key + ":ts"},
		now, cfg.RatePerMin/60.0, cfg.Capacity).Result()
	if err != nil {
		log.Printf("Redis rate limiter error: %v", err)
		return false
	}
	return res.(int64) == 1
}

// ========================
// In-Memory Fallback
// ========================
func (rl *RateLimiter) allowLocal(key string, cfg APIKeyConfig) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	tb, exists := rl.local[key]
	if !exists {
		rl.local[key] = &TokenBucket{
			Tokens:      cfg.Capacity - 1,
			LastUpdated: now,
		}
		return true
	}

	// refill tokens
	delta := now.Sub(tb.LastUpdated).Seconds()
	tb.Tokens = min(cfg.Capacity, tb.Tokens+delta*cfg.RatePerMin/60.0)
	tb.LastUpdated = now

	if tb.Tokens < 1 {
		return false
	}

	tb.Tokens -= 1
	return true
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// ========================
// Rate Limit Middleware
// ========================
func RateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, ok := r.Context().Value("api_key").(string)
		if !ok || key == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if !rl.Allow(key) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
