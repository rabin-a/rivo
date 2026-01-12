package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rabin-a/rivo"
)

// EmailPayload represents an email to send.
type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get database URL from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://localhost:5432/rivo?sslmode=disable"
	}

	// Create Rivo client
	client, err := rivo.NewClient(ctx, rivo.Config{
		DatabaseURL:  dbURL,
		AutoMigrate:  true,
		Workers:      5,
		PollInterval: 100 * time.Millisecond,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// Register job handler
	client.RegisterHandler("send-email", func(ctx context.Context, job *rivo.Job) error {
		var payload EmailPayload
		if err := job.UnmarshalPayload(&payload); err != nil {
			return err
		}

		fmt.Printf("Sending email to %s: %s\n", payload.To, payload.Subject)
		time.Sleep(100 * time.Millisecond)
		fmt.Printf("Email sent to %s (job #%d, attempt %d)\n", payload.To, job.ID, job.Attempt)
		return nil
	})

	// Start processing jobs
	if err := client.Start(ctx); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Rivo worker started. Press Ctrl+C to stop.")

	// Enqueue some example jobs
	for i := 0; i < 3; i++ {
		_, err := client.Enqueue(ctx, rivo.EnqueueParams{
			Kind: "send-email",
			Payload: EmailPayload{
				To:      fmt.Sprintf("user%d@example.com", i),
				Subject: "Welcome!",
				Body:    "Thanks for signing up.",
			},
		})
		if err != nil {
			log.Printf("Failed to enqueue job: %v", err)
		}
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
	client.Shutdown(context.Background())
}
