package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/freitasmatheusrn/rate_limiter/pkg/token"
)

func GetJWT(secretKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		acessToken := token.Generate([]byte(secretKey))
		response := map[string]string{
			"access_token": acessToken,
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}


