package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/segmentio/kafka-go"
)

type SuccessResponse struct {
	Status string `json:"status"`
}

type HealthResponse struct {
	Status bool `json:"status"`
}

func main() {
	port := getenv("PORT", "8082")
	brokers := getenv("KAFKA_BROKERS", "kafka:9092")

	// Topics
	topics := map[string]string{
		"movie":   getenv("KAFKA_TOPIC_MOVIE", "movie-events"),
		"user":    getenv("KAFKA_TOPIC_USER", "user-events"),
		"payment": getenv("KAFKA_TOPIC_PAYMENT", "payment-events"),
	}

	// Запускаю consumers в фоне
	go consumeTopic(brokers, topics["movie"], "events-service")
	go consumeTopic(brokers, topics["user"], "events-service")
	go consumeTopic(brokers, topics["payment"], "events-service")

	mux := http.NewServeMux()

	mux.HandleFunc("/api/events/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, HealthResponse{Status: true})
	})

	mux.HandleFunc("/api/events/movie", func(w http.ResponseWriter, r *http.Request) {
		handleEvent(w, r, brokers, topics["movie"], "movie")
	})
	mux.HandleFunc("/api/events/user", func(w http.ResponseWriter, r *http.Request) {
		handleEvent(w, r, brokers, topics["user"], "user")
	})
	mux.HandleFunc("/api/events/payment", func(w http.ResponseWriter, r *http.Request) {
		handleEvent(w, r, brokers, topics["payment"], "payment")
	})

	log.Printf("Events service started on :%s (brokers=%s)", port, brokers)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func handleEvent(w http.ResponseWriter, r *http.Request, brokers, topic, eventType string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	b, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "failed to marshal payload", http.StatusInternalServerError)
		return
	}

	if err := produce(brokers, topic, b); err != nil {
		log.Printf("[ERROR] produce failed (type=%s topic=%s): %v", eventType, topic, err)
		http.Error(w, "failed to produce event", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, SuccessResponse{Status: "success"})
}

func produce(brokers, topic string, value []byte) error {
	w := &kafka.Writer{
		Addr:         kafka.TCP(brokers),
		Topic:        topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireAll,
	}
	defer w.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return w.WriteMessages(ctx, kafka.Message{
		Value: value,
		Time:  time.Now(),
	})
}

func consumeTopic(brokers, topic, groupID string) {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{brokers},
		Topic:       topic,
		GroupID:     groupID,
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafka.FirstOffset,
	})
	defer r.Close()

	log.Printf("[CONSUMER] started topic=%s group=%s", topic, groupID)

	for {
		msg, err := r.ReadMessage(context.Background())
		if err != nil {
			log.Printf("[CONSUMER][ERROR] topic=%s: %v", topic, err)
			time.Sleep(1 * time.Second)
			continue
		}
		log.Printf("[EVENT] topic=%s offset=%d value=%s", topic, msg.Offset, string(msg.Value))
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
