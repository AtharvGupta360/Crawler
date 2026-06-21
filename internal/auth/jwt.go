package auth

import (
	"errors"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// contextKey is a private type to avoid key collisions in context values.
type contextKey string

// ClaimsContextKey is the key used to store Claims in request contexts.
const ClaimsContextKey contextKey = "jwt_claims"

// Claims holds the JWT payload.
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

// DefaultTTL is the access-token lifetime.
const DefaultTTL = 24 * time.Hour

// GenerateToken creates a signed JWT for the given user.
func GenerateToken(secret string, ttl time.Duration, u *models.User) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: u.ID,
		Email:  u.Email,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Issuer:    "jobcrawl",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken parses and validates a JWT string, returning the claims.
func ValidateToken(secret, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
