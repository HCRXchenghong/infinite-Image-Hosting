package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Task struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

type TaskMessage struct {
	ID            string
	Task          Task
	CreatedAt     time.Time
	DeliveryCount int64
}

type RedisStreamStats struct {
	Reachable        bool                 `json:"reachable"`
	Stream           string               `json:"stream"`
	DeadLetterStream string               `json:"dead_letter_stream"`
	Group            string               `json:"group"`
	Length           int64                `json:"length"`
	DeadLetterLength int64                `json:"dead_letter_length"`
	Pending          int64                `json:"pending"`
	Lag              int64                `json:"lag"`
	LastGeneratedID  string               `json:"last_generated_id"`
	Consumers        []RedisConsumerStats `json:"consumers"`
}

type RedisConsumerStats struct {
	Name     string `json:"name"`
	Pending  int64  `json:"pending"`
	IdleMS   int64  `json:"idle_ms"`
	Inactive int64  `json:"inactive_ms"`
}

type DeadLetterTask struct {
	ID             string         `json:"id"`
	OriginalID     string         `json:"original_id"`
	OriginalStream string         `json:"original_stream"`
	Group          string         `json:"group"`
	Consumer       string         `json:"consumer"`
	Type           string         `json:"type"`
	Payload        map[string]any `json:"payload"`
	CreatedAt      string         `json:"created_at"`
	FailedAt       string         `json:"failed_at"`
	DeliveryCount  int64          `json:"delivery_count"`
	Reason         string         `json:"reason"`
}

type Queue interface {
	Enqueue(ctx context.Context, task Task) error
}

type Consumer interface {
	Receive(ctx context.Context, count int64, block time.Duration) ([]TaskMessage, error)
	ClaimStale(ctx context.Context, minIdle time.Duration, count int64) ([]TaskMessage, error)
	Ack(ctx context.Context, ids ...string) error
	MoveToDeadLetter(ctx context.Context, message TaskMessage, reason error) error
	Close() error
}

type InlineQueue struct{}

func (InlineQueue) Enqueue(_ context.Context, _ Task) error {
	return nil
}

type RedisQueue struct {
	client *redis.Client
	stream string
}

type RedisConfig struct {
	Addr             string
	DB               int
	Stream           string
	DeadLetterStream string
}

func NewRedisQueue(cfg RedisConfig) *RedisQueue {
	stream := cfg.Stream
	if stream == "" {
		stream = "yuexiang:image:tasks"
	}
	return &RedisQueue{
		client: redis.NewClient(&redis.Options{Addr: cfg.Addr, DB: cfg.DB}),
		stream: stream,
	}
}

func (q *RedisQueue) Enqueue(ctx context.Context, task Task) error {
	payload, err := json.Marshal(task.Payload)
	if err != nil {
		return err
	}
	return q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		Values: map[string]any{
			"type":       task.Type,
			"payload":    string(payload),
			"created_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	}).Err()
}

func (q *RedisQueue) Close() error {
	return q.client.Close()
}

func InspectRedis(ctx context.Context, cfg RedisConfig, group string) (RedisStreamStats, error) {
	queue := NewRedisQueue(cfg)
	defer queue.Close()

	group = strings.TrimSpace(group)
	if group == "" {
		group = "yuexiang-image-workers"
	}
	deadLetterStream := strings.TrimSpace(cfg.DeadLetterStream)
	if deadLetterStream == "" {
		deadLetterStream = queue.stream + ":dead"
	}

	stats := RedisStreamStats{
		Reachable:        true,
		Stream:           queue.stream,
		DeadLetterStream: deadLetterStream,
		Group:            group,
		DeadLetterLength: 0,
		Length:           0,
		Pending:          0,
		Lag:              0,
		Consumers:        []RedisConsumerStats{},
		LastGeneratedID:  "",
	}
	streamInfo, err := queue.client.XInfoStream(ctx, queue.stream).Result()
	if err != nil && !isMissingStreamInfo(err) {
		return stats, err
	}
	if err == nil {
		stats.Length = streamInfo.Length
		stats.LastGeneratedID = streamInfo.LastGeneratedID
	}
	deadLength, err := queue.client.XLen(ctx, deadLetterStream).Result()
	if err != nil && !isMissingStreamInfo(err) {
		return stats, err
	}
	if err == nil {
		stats.DeadLetterLength = deadLength
	}
	pending, err := queue.client.XPending(ctx, queue.stream, group).Result()
	if err != nil && !isMissingStreamInfo(err) {
		return stats, err
	}
	if err == nil && pending != nil {
		stats.Pending = pending.Count
	}
	groups, err := queue.client.XInfoGroups(ctx, queue.stream).Result()
	if err != nil && !isMissingStreamInfo(err) {
		return stats, err
	}
	if err == nil {
		for _, info := range groups {
			if info.Name == group {
				stats.Pending = info.Pending
				stats.Lag = info.Lag
				break
			}
		}
	}
	consumers, err := queue.client.XInfoConsumers(ctx, queue.stream, group).Result()
	if err != nil && !isMissingStreamInfo(err) {
		return stats, err
	}
	if err == nil {
		stats.Consumers = make([]RedisConsumerStats, 0, len(consumers))
		for _, consumer := range consumers {
			stats.Consumers = append(stats.Consumers, RedisConsumerStats{
				Name:     consumer.Name,
				Pending:  consumer.Pending,
				IdleMS:   consumer.Idle.Milliseconds(),
				Inactive: consumer.Inactive.Milliseconds(),
			})
		}
	}
	return stats, nil
}

func ListDeadLetters(ctx context.Context, cfg RedisConfig, count int64) ([]DeadLetterTask, error) {
	if count <= 0 || count > 100 {
		count = 20
	}
	queue := NewRedisQueue(cfg)
	defer queue.Close()
	deadLetterStream := strings.TrimSpace(cfg.DeadLetterStream)
	if deadLetterStream == "" {
		deadLetterStream = queue.stream + ":dead"
	}
	messages, err := queue.client.XRevRangeN(ctx, deadLetterStream, "+", "-", count).Result()
	if err != nil && !isMissingStreamInfo(err) {
		return nil, err
	}
	out := make([]DeadLetterTask, 0, len(messages))
	for _, message := range messages {
		out = append(out, parseDeadLetterMessage(message))
	}
	return out, nil
}

func RequeueDeadLetter(ctx context.Context, cfg RedisConfig, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("dead-letter id is required")
	}
	queue := NewRedisQueue(cfg)
	defer queue.Close()
	deadLetterStream := strings.TrimSpace(cfg.DeadLetterStream)
	if deadLetterStream == "" {
		deadLetterStream = queue.stream + ":dead"
	}
	messages, err := queue.client.XRangeN(ctx, deadLetterStream, id, id, 1).Result()
	if err != nil && !isMissingStreamInfo(err) {
		return err
	}
	if len(messages) == 0 {
		return fmt.Errorf("dead-letter message %s not found", id)
	}
	dead := parseDeadLetterMessage(messages[0])
	payload, err := json.Marshal(dead.Payload)
	if err != nil {
		return err
	}
	pipe := queue.client.TxPipeline()
	pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: queue.stream,
		Values: map[string]any{
			"type":                    dead.Type,
			"payload":                 string(payload),
			"created_at":              time.Now().UTC().Format(time.RFC3339Nano),
			"requeued_from_dead_id":   id,
			"requeued_from_failed_at": dead.FailedAt,
			"original_id":             dead.OriginalID,
		},
	})
	pipe.XDel(ctx, deadLetterStream, id)
	_, err = pipe.Exec(ctx)
	return err
}

type RedisConsumer struct {
	client           *redis.Client
	stream           string
	deadLetterStream string
	group            string
	consumer         string
}

func NewRedisConsumer(ctx context.Context, cfg RedisConfig, group, consumer string) (*RedisConsumer, error) {
	queue := NewRedisQueue(cfg)
	group = strings.TrimSpace(group)
	if group == "" {
		group = "yuexiang-image-workers"
	}
	consumer = strings.TrimSpace(consumer)
	if consumer == "" {
		consumer = "worker"
	}
	if err := queue.client.XGroupCreateMkStream(ctx, queue.stream, group, "0").Err(); err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		_ = queue.Close()
		return nil, err
	}
	deadLetterStream := strings.TrimSpace(cfg.DeadLetterStream)
	if deadLetterStream == "" {
		deadLetterStream = queue.stream + ":dead"
	}
	return &RedisConsumer{
		client:           queue.client,
		stream:           queue.stream,
		deadLetterStream: deadLetterStream,
		group:            group,
		consumer:         consumer,
	}, nil
}

func (c *RedisConsumer) Receive(ctx context.Context, count int64, block time.Duration) ([]TaskMessage, error) {
	if count <= 0 {
		count = 1
	}
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.consumer,
		Streams:  []string{c.stream, ">"},
		Count:    count,
		Block:    block,
	}).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var redisMessages []redis.XMessage
	for _, stream := range streams {
		redisMessages = append(redisMessages, stream.Messages...)
	}
	return c.parseRedisMessages(ctx, redisMessages)
}

func (c *RedisConsumer) ClaimStale(ctx context.Context, minIdle time.Duration, count int64) ([]TaskMessage, error) {
	if minIdle <= 0 {
		return nil, nil
	}
	if count <= 0 {
		count = 1
	}
	messages, _, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   c.stream,
		Group:    c.group,
		Consumer: c.consumer,
		MinIdle:  minIdle,
		Start:    "0-0",
		Count:    count,
	}).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return c.parseRedisMessages(ctx, messages)
}

func (c *RedisConsumer) Ack(ctx context.Context, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	return c.client.XAck(ctx, c.stream, c.group, ids...).Err()
}

func (c *RedisConsumer) MoveToDeadLetter(ctx context.Context, message TaskMessage, reason error) error {
	payload, err := json.Marshal(message.Task.Payload)
	if err != nil {
		payload = []byte(`{"marshal_error":"payload could not be encoded"}`)
	}
	reasonText := ""
	if reason != nil {
		reasonText = reason.Error()
	}
	pipe := c.client.TxPipeline()
	pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: c.deadLetterStream,
		Values: map[string]any{
			"original_id":     message.ID,
			"original_stream": c.stream,
			"group":           c.group,
			"consumer":        c.consumer,
			"type":            message.Task.Type,
			"payload":         string(payload),
			"created_at":      message.CreatedAt.Format(time.RFC3339Nano),
			"failed_at":       time.Now().UTC().Format(time.RFC3339Nano),
			"delivery_count":  message.DeliveryCount,
			"reason":          reasonText,
		},
	})
	pipe.XAck(ctx, c.stream, c.group, message.ID)
	_, err = pipe.Exec(ctx)
	return err
}

func (c *RedisConsumer) Close() error {
	return c.client.Close()
}

func (c *RedisConsumer) parseRedisMessages(ctx context.Context, messages []redis.XMessage) ([]TaskMessage, error) {
	out := make([]TaskMessage, 0, len(messages))
	for _, message := range messages {
		task, createdAt, err := parseRedisTask(message.Values)
		if err != nil {
			task = Task{
				Type: "invalid",
				Payload: map[string]any{
					"parse_error": err.Error(),
					"raw_values":  stringifyValues(message.Values),
				},
			}
		}
		deliveryCount, err := c.deliveryCount(ctx, message.ID)
		if err != nil {
			return nil, fmt.Errorf("load delivery count for %s: %w", message.ID, err)
		}
		out = append(out, TaskMessage{
			ID:            message.ID,
			Task:          task,
			CreatedAt:     createdAt,
			DeliveryCount: deliveryCount,
		})
	}
	return out, nil
}

func (c *RedisConsumer) deliveryCount(ctx context.Context, id string) (int64, error) {
	info, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: c.stream,
		Group:  c.group,
		Start:  id,
		End:    id,
		Count:  1,
	}).Result()
	if err == redis.Nil {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	if len(info) == 0 || info[0].RetryCount <= 0 {
		return 1, nil
	}
	return info[0].RetryCount, nil
}

func parseRedisTask(values map[string]any) (Task, time.Time, error) {
	taskType := fmt.Sprint(values["type"])
	if taskType == "" || taskType == "<nil>" {
		return Task{}, time.Time{}, fmt.Errorf("missing task type")
	}
	var payload map[string]any
	rawPayload := fmt.Sprint(values["payload"])
	if rawPayload != "" && rawPayload != "<nil>" {
		if err := json.Unmarshal([]byte(rawPayload), &payload); err != nil {
			return Task{}, time.Time{}, err
		}
	}
	var createdAt time.Time
	if rawCreatedAt := fmt.Sprint(values["created_at"]); rawCreatedAt != "" && rawCreatedAt != "<nil>" {
		createdAt, _ = time.Parse(time.RFC3339Nano, rawCreatedAt)
	}
	return Task{Type: taskType, Payload: payload}, createdAt, nil
}

func stringifyValues(values map[string]any) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = fmt.Sprint(value)
	}
	return out
}

func parseDeadLetterMessage(message redis.XMessage) DeadLetterTask {
	payload := map[string]any{}
	rawPayload := fmt.Sprint(message.Values["payload"])
	if rawPayload != "" && rawPayload != "<nil>" {
		_ = json.Unmarshal([]byte(rawPayload), &payload)
	}
	deliveryCount, _ := strconv.ParseInt(fmt.Sprint(message.Values["delivery_count"]), 10, 64)
	return DeadLetterTask{
		ID:             message.ID,
		OriginalID:     fmt.Sprint(message.Values["original_id"]),
		OriginalStream: fmt.Sprint(message.Values["original_stream"]),
		Group:          fmt.Sprint(message.Values["group"]),
		Consumer:       fmt.Sprint(message.Values["consumer"]),
		Type:           fmt.Sprint(message.Values["type"]),
		Payload:        payload,
		CreatedAt:      fmt.Sprint(message.Values["created_at"]),
		FailedAt:       fmt.Sprint(message.Values["failed_at"]),
		DeliveryCount:  deliveryCount,
		Reason:         fmt.Sprint(message.Values["reason"]),
	}
}

func isMissingStreamInfo(err error) bool {
	if err == redis.Nil {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such key") || strings.Contains(message, "nogroup")
}
