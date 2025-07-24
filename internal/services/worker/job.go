package worker

import (
	"context"
	"log"
)

type RinhaJobFunc func(ctx context.Context, data []byte) error

func ExampleLoggingJob(ctx context.Context, data []byte) error {
	log.Printf("[worker] received data: %s", string(data))
	return nil
}
