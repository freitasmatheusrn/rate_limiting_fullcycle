package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func Generate(secretKey []byte) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	})

	tokenString, err := token.SignedString(secretKey)
	if err != nil {
		panic(err)
	}

	return tokenString
}

func Validate(tokenString string, secretKey []byte) (bool, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de assinatura inválido")
		}
		return secretKey, nil
	})

	if err != nil {
		return false, err
	}

	return token.Valid, nil
}
