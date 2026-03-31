package middleware

import (
	"net"
	"net/http"

	"github.com/freitasmatheusrn/rate_limiter/pkg/ratelimiter"
	"github.com/freitasmatheusrn/rate_limiter/pkg/token"
)

func RateLimiterMiddleware(rl ratelimiter.Interface, secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var key, kind string
		apiKey := r.Header.Get("API_KEY")

		if apiKey != "" {
			if _, err := token.Validate(apiKey, []byte(secret)); err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			key = apiKey
			kind = "token"
		} else {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			key = ip
			kind = "ip"
		}

		if !rl.Allow(key, kind) {
			http.Error(w, "you have reached the maximum number of requests or actions allowed within a certain time frame", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)

	})
}
