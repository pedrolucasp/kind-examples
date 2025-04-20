package main

import (
	"encoding/json"
	"net/http"
	"log"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type Star struct {
	Name     string `json:"name"`
	Distance int    `json:"distance"`
	Size     string `json:"size"`
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
	}))

	r.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("I want a Camel Blue"))
	})

	r.Get("/api/v1/stars", func(w http.ResponseWriter, r *http.Request) {
		stars := []Star{
			{Name: "Sirius", Distance: 8, Size: "Large"},
			{Name: "Proxima Centauri", Distance: 4, Size: "Small"},
			{Name: "Alpha Centauri", Distance: 4, Size: "Medium"},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(stars)
	})

	log.Println("Starting server on :3000")
	http.ListenAndServe(":3000", r)
}
