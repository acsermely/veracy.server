package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/golang-jwt/jwt/v4"
	"gitlab.com/acsermely/permit-v0/server/src/db"
)

type key int

const (
	CONTEXT_USER_OBJECT_KEY key = 0
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		secret := []byte(os.Getenv("SECRET"))
		cookie, err := r.Cookie("token")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Missing Cookie"))
			return
		}
		tokenString := cookie.Value

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return secret, nil
		})
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}

		var userName string
		if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
			userName = claims["user"].(string)
		} else {
			fmt.Println("invalid token")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}

		user, err := db.GetUser(userName)
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), CONTEXT_USER_OBJECT_KEY, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}