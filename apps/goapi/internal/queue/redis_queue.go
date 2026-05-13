package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"goapi/config"
)

const redisKeyPrefix = "queue:v1:"

var popDueScript = redis.NewScript(`
local zkey = KEYS[1]
local now = tonumber(ARGV[1])
local jobs = redis.call('ZRANGEBYSCORE', zkey, '-inf', now, 'LIMIT', 0, 1)
if #jobs == 0 then return nil end
redis.call('ZREM', zkey, jobs[1])
return jobs[1]
`)

// RedisQueue persists jobs in Redis (sorted set schedule + JSON payloads).
type RedisQueue struct {
	rdb *redis.Client
	cfg *config.Config
}

// NewRedisQueue constructs a Redis-backed queue.
func NewRedisQueue(rdb *redis.Client, cfg *config.Config) *RedisQueue {
	return &RedisQueue{rdb: rdb, cfg: cfg}
}

func jobRedisKey(id string) string {
	return redisKeyPrefix + "job:" + id
}

func dueRedisKey() string {
	return redisKeyPrefix + "due"
}

func dlqRedisKey() string {
	return redisKeyPrefix + "dlq"
}

// Enqueue implements Enqueuer.
func (q *RedisQueue) Enqueue(ctx context.Context, jobType string, payload json.RawMessage, opts EnqueueOptions) error {
	if q.rdb == nil {
		return fmt.Errorf("queue: redis client is nil")
	}
	if jobType == "" {
		return fmt.Errorf("queue: empty job type")
	}
	id := uuid.New().String()
	now := time.Now().UTC()
	maxAttempts := opts.MaxAttempts
	if maxAttempts < 1 {
		maxAttempts = q.cfg.Queue.MaxAttempts
	}
	if maxAttempts < 1 {
		maxAttempts = 5
	}
	runAt := opts.RunAt
	if runAt.IsZero() {
		runAt = now
	}

	job := Job{
		ID:          id,
		Type:        jobType,
		Payload:     payload,
		Status:      StatusQueued,
		Attempts:    0,
		MaxAttempts: maxAttempts,
		RunAt:       runAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	raw, err := json.Marshal(job)
	if err != nil {
		return err
	}

	pipe := q.rdb.Pipeline()
	pipe.Set(ctx, jobRedisKey(id), raw, 0)
	pipe.ZAdd(ctx, dueRedisKey(), redis.Z{
		Score:  float64(runAt.UnixMilli()),
		Member: id,
	})
	_, err = pipe.Exec(ctx)
	return err
}

// PopDue removes and returns the next job ID whose run time is <= until (UTC semantics via UnixMilli).
func (q *RedisQueue) PopDue(ctx context.Context, until time.Time) (string, error) {
	if q.rdb == nil {
		return "", fmt.Errorf("queue: redis client is nil")
	}
	v, err := popDueScript.Run(ctx, q.rdb, []string{dueRedisKey()}, strconv.FormatInt(until.UnixMilli(), 10)).Result()
	if err == redis.Nil || v == nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("queue: unexpected pop script result type %T", v)
	}
	return s, nil
}

// LoadJob reads a job by ID.
func (q *RedisQueue) LoadJob(ctx context.Context, id string) (*Job, error) {
	raw, err := q.rdb.Get(ctx, jobRedisKey(id)).Bytes()
	if err == redis.Nil {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}
	var job Job
	if err := json.Unmarshal(raw, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// SaveJob persists job state.
func (q *RedisQueue) SaveJob(ctx context.Context, job *Job) error {
	if job == nil {
		return fmt.Errorf("queue: nil job")
	}
	raw, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return q.rdb.Set(ctx, jobRedisKey(job.ID), raw, 0).Err()
}

// DeleteJob removes job payload key.
func (q *RedisQueue) DeleteJob(ctx context.Context, id string) error {
	return q.rdb.Del(ctx, jobRedisKey(id)).Err()
}

// Requeue schedules job back onto the due zset and saves payload.
func (q *RedisQueue) Requeue(ctx context.Context, job *Job) error {
	if err := q.SaveJob(ctx, job); err != nil {
		return err
	}
	return q.rdb.ZAdd(ctx, dueRedisKey(), redis.Z{
		Score:  float64(job.RunAt.UnixMilli()),
		Member: job.ID,
	}).Err()
}

// PushDeadLetter stores a JSON snapshot and optionally trims the list.
func (q *RedisQueue) PushDeadLetter(ctx context.Context, snapshot []byte) error {
	if err := q.rdb.LPush(ctx, dlqRedisKey(), snapshot).Err(); err != nil {
		return err
	}
	capN := q.cfg.Queue.DeadLetterMaxCap
	if capN < 1 {
		capN = 10000
	}
	// Best-effort trim to cap (keep newest).
	return q.rdb.LTrim(ctx, dlqRedisKey(), 0, int64(capN-1)).Err()
}
