package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func main() {
	client := &InfraiEmailClient{APIKey: os.Getenv("INFRAI_API_KEY"), MaxRetries: 4}
	workflow := NewStoreWorkflow(client, envOr("PUBLIC_URL", "http://localhost:8080"))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			CustomerID string `json:"customer_id"`
			Email      string `json:"email"`
			OrderID    string `json:"order_id"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		signup, err := workflow.StartSignup(r.Context(), input.CustomerID, input.Email, input.OrderID)
		respond(w, signup, err)
	})
	mux.HandleFunc("GET /verify", func(w http.ResponseWriter, r *http.Request) {
		signup, err := workflow.Verify(r.URL.Query().Get("token"))
		respond(w, signup, err)
	})
	mux.HandleFunc("POST /orders/{customerID}/advance", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Stage OrderStage `json:"stage"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		signup, err := workflow.Advance(r.PathValue("customerID"), input.Stage)
		respond(w, signup, err)
	})
	mux.HandleFunc("GET /orders/{customerID}", func(w http.ResponseWriter, r *http.Request) {
		signup, ok := workflow.Lookup(r.PathValue("customerID"))
		if !ok {
			http.Error(w, "signup not found", http.StatusNotFound)
			return
		}
		respond(w, signup, nil)
	})
	addr := envOr("LISTEN_ADDR", ":8080")
	log.Printf("store verification service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func respond(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(value)
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
