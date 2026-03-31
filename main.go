package main

import (
	"net/http"

	"github.com/freitasmatheusrn/rate_limiter/config"
	"github.com/freitasmatheusrn/rate_limiter/internal/web/handlers"
	"github.com/freitasmatheusrn/rate_limiter/internal/web/middleware"
	"github.com/freitasmatheusrn/rate_limiter/pkg/ratelimiter"
	"github.com/go-redis/redis/v8"
)

func main() {
	cfg := config.LoadConfig(".env")
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	defer client.Close()
	rateLimiter := ratelimiter.NewRedisLimiter(
		client,
		cfg.IpRequestLimit, cfg.TokenRequestLimit,
		cfg.TimeWindow, cfg.BlockTime)
	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Request dentro do limite"))
	})
	router.HandleFunc("POST /token", handlers.GetJWT(cfg.SecretKey))

	handler := middleware.RateLimiterMiddleware(rateLimiter, cfg.SecretKey, router)
	http.ListenAndServe(":8080", handler)

}
