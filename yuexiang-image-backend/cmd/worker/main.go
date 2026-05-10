package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yuexiang/image-backend/internal/api"
	"github.com/yuexiang/image-backend/internal/config"
	"github.com/yuexiang/image-backend/internal/queue"
)

func main() {
	cfg := config.Load()
	if cfg.QueueDriver != "redis" {
		log.Fatalf("worker requires QUEUE_DRIVER=redis, got %q", cfg.QueueDriver)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := api.NewServer(cfg)
	consumer, err := queue.NewRedisConsumer(ctx, queue.RedisConfig{
		Addr:             cfg.RedisAddr,
		DB:               cfg.RedisDB,
		Stream:           cfg.QueueStream,
		DeadLetterStream: cfg.QueueDeadLetterStream,
	}, cfg.QueueGroup, cfg.WorkerName)
	if err != nil {
		log.Fatalf("initialize redis consumer: %v", err)
	}
	defer consumer.Close()

	log.Printf("worker started stream=%s dead_letter=%s group=%s consumer=%s batch=%d retry_limit=%d claim_idle=%s", cfg.QueueStream, cfg.QueueDeadLetterStream, cfg.QueueGroup, cfg.WorkerName, cfg.WorkerBatchSize, cfg.WorkerRetryLimit, cfg.WorkerClaimIdle)
	for {
		stale, err := consumer.ClaimStale(ctx, cfg.WorkerClaimIdle, cfg.WorkerBatchSize)
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			log.Print("worker stopped")
			return
		}
		if err != nil {
			log.Printf("claim stale tasks failed: %v", err)
		}
		for _, message := range stale {
			processMessage(ctx, server, consumer, message, cfg.WorkerRetryLimit)
		}

		messages, err := consumer.Receive(ctx, cfg.WorkerBatchSize, cfg.WorkerPollInterval)
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			log.Print("worker stopped")
			return
		}
		if err != nil {
			log.Printf("receive task failed: %v", err)
			time.Sleep(time.Second)
			continue
		}
		for _, message := range messages {
			processMessage(ctx, server, consumer, message, cfg.WorkerRetryLimit)
		}
	}
}

func processMessage(ctx context.Context, server *api.Server, consumer queue.Consumer, message queue.TaskMessage, retryLimit int64) {
	if retryLimit <= 0 {
		retryLimit = 1
	}
	if err := server.ProcessTask(ctx, message.Task); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			log.Printf("task %s interrupted during shutdown type=%s", message.ID, message.Task.Type)
			return
		}
		if message.DeliveryCount >= retryLimit {
			if moveErr := consumer.MoveToDeadLetter(ctx, message, err); moveErr != nil {
				log.Printf("task %s dead-letter failed type=%s deliveries=%d err=%v move_err=%v", message.ID, message.Task.Type, message.DeliveryCount, err, moveErr)
				return
			}
			log.Printf("task %s moved to dead-letter type=%s deliveries=%d err=%v", message.ID, message.Task.Type, message.DeliveryCount, err)
			return
		}
		log.Printf("task %s failed type=%s deliveries=%d/%d: %v", message.ID, message.Task.Type, message.DeliveryCount, retryLimit, err)
		return
	}
	if err := consumer.Ack(ctx, message.ID); err != nil {
		log.Printf("ack task %s failed: %v", message.ID, err)
	}
}
