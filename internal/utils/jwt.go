package utils

import (
	"fmt"
	"immo-lux/internal/models"
	"immo-lux/internal/server_error"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	UserId     int64 `json:"userId"`
	SessionId  int64 `json:"sessionId"`
	RememberMe bool  `json:"rememberMe"`
	jwt.RegisteredClaims
}

func GenerateJWT(userId models.RecordId, sessionId models.RecordId, rememberMe bool, secret string, issuer string) (string, error) {
	var expirationTime time.Time
	if rememberMe {
		expirationTime = time.Now().Add(30 * 24 * time.Hour)
	} else {
		expirationTime = time.Now().Add(1 * time.Hour)
	}

	claims := JWTClaims{
		UserId:     int64(userId),
		SessionId:  int64(sessionId),
		RememberMe: rememberMe,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", server_error.Wrap("JWT_GENERATION", "failed to sign JWT token", err)
	}

	return tokenString, nil
}

func ValidateJWT(tokenString string, secret string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, server_error.New("JWT_VALIDATION", fmt.Sprintf("unexpected signing method: %v", token.Header["alg"]))
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, server_error.Wrap("JWT_VALIDATION", "failed to parse JWT token", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, server_error.New("JWT_VALIDATION", "invalid JWT token")
}
