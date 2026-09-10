package auth

import (
	"context"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
	"graphql-go/persistence"
	"net/http"
	"os"
	"strings"
)

// A private key for context that only this package can access. This is important
// to prevent collisions between different context uses
var userCtxKey = &contextKey{"user"}

type contextKey struct {
	name string
}

// Middleware decodes the share session cookie and packs the session into context
func Middleware(db *gorm.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Special handling for WebSocket connections
			// Check if this is a WebSocket upgrade request
			if websocket := r.Header.Get("Upgrade"); websocket == "websocket" {
				// Allow WebSocket upgrade requests to proceed without authentication
				// The subscription resolvers can implement their own auth if needed
				next.ServeHTTP(w, r)
				return
			}

			// Check for JWT in Authorization header
			authHeader := r.Header.Get("Authorization")

			if authHeader == "" {
				http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
				return
			}

			// The header should be in the format `Bearer <token>`
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
				return
			}

			// Validate the JWT
			tokenString := parts[1]
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				// Make sure the token method is what we expect
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}

				// Return the secret key (this should be more secure in production)
				// Replace "your_secret_key" with your actual secret key
				return []byte(os.Getenv("JWT_SECRET")), nil
			})
			if err != nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
				// You can access the token claims here if needed
				// get the user from the database
				sub, _ := claims.GetSubject()
				user := &persistence.User{ID: sub}
				db.First(user)

				// put it in context
				ctx := context.WithValue(r.Context(), userCtxKey, user)

				// and call the next with our new context
				r = r.WithContext(ctx)
				next.ServeHTTP(w, r)
			} else {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

		})
	}
}

// WithUser returns a context carrying the given user, as Middleware would. It lets
// tests and other entry points drive resolvers without minting a JWT.
func WithUser(ctx context.Context, user *persistence.User) context.Context {
	return context.WithValue(ctx, userCtxKey, user)
}

// ForContext finds the user from the context. REQUIRES Middleware to have run.
func ForContext(ctx context.Context) *persistence.User {
	raw, _ := ctx.Value(userCtxKey).(*persistence.User)
	return raw
}
