package api

import (
	"cityio/internal/models"
	"cityio/internal/services"
	"cityio/internal/ws"

	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/cors"
)

func DecodeBody[T any](request *http.Request) (T, error) {
	var obj T
	decoder := json.NewDecoder(request.Body)

	if err := decoder.Decode(&obj); err != nil {
		log.Printf("Error decoding request body: %s", err)
		return obj, err
	}

	return obj, nil
}

func GetClaims(request *http.Request) models.UserClaims {
	ctxClaims := request.Context().Value("claims").(jwt.MapClaims)
	var claims models.UserClaims

	if username, ok := ctxClaims["username"].(string); ok {
		claims.Username = username
	}

	if email, ok := ctxClaims["email"].(string); ok {
		claims.Email = email
	}

	if userId, ok := ctxClaims["userId"].(string); ok {
		claims.UserId = userId
	}
	return claims
}

func Start() {
	log.Printf("Serving at 0.0.0.0:%s...", os.Getenv("API_PORT"))

	router := mux.NewRouter()
	router.Use(recoverMiddleware)
	addRoutes(router)

	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173", "http://localhost:4173", "https://cityio.prayujt.com"},
		AllowCredentials: true,
		AllowedHeaders:   []string{"*"},
	})

	handler := c.Handler(router)

	server := &http.Server{
		Handler:      handler,
		Addr:         fmt.Sprintf("0.0.0.0:%s", os.Getenv("API_PORT")),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic occurred: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func authHandle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			log.Println("No token is given")
			response.WriteHeader(http.StatusUnauthorized)
			return
		}

		claims, err := services.ValidateToken(token)
		if err != nil {
			log.Printf("Error parsing JWT: %s", err)
			response.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(request.Context(), "claims", claims)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func authHandler(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}

		claims, err := services.ValidateToken(token)
		if err != nil {
			log.Printf("Error parsing JWT: %s", err)
			response.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(request.Context(), "claims", claims)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func addRoutes(router *mux.Router) {
	router.HandleFunc("/ws", ws.HandleWebSocket).Methods("GET")

	userRouter := router.PathPrefix("/users").Subrouter()

	userRouter.HandleFunc("/register", Register).Methods("POST")
	userRouter.HandleFunc("/login", Login).Methods("POST")
	userRouter.HandleFunc("/{userId}", DeleteUser).Methods("DELETE")
	userRouter.HandleFunc("/validate", authHandler(ValidateToken)).Methods("GET")
}
