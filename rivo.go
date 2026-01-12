// Package rivo provides a Postgres-native workflow and background job execution platform.
//
// Rivo is designed to be embedded directly into Go applications, providing durable
// job execution with at-least-once delivery guarantees.
//
// # Quick Start
//
//	client, err := rivo.NewClient(ctx, rivo.Config{
//	    DatabaseURL: "postgres://localhost:5432/rivo",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Register a job handler
//	client.RegisterHandler("send-email", func(ctx context.Context, job *rivo.Job) error {
//	    var payload EmailPayload
//	    if err := job.UnmarshalPayload(&payload); err != nil {
//	        return err
//	    }
//	    return sendEmail(payload.To, payload.Subject, payload.Body)
//	})
//
//	// Enqueue a job
//	_, err = client.Enqueue(ctx, rivo.EnqueueParams{
//	    Kind:    "send-email",
//	    Payload: EmailPayload{To: "user@example.com"},
//	})
//
//	// Start processing
//	client.Start(ctx)
package rivo
