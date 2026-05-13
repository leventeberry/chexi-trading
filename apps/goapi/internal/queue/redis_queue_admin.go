package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// DueDepth returns the number of job IDs scheduled in the due sorted set.
func (q *RedisQueue) DueDepth(ctx context.Context) (int64, error) {
	if q.rdb == nil {
		return 0, fmt.Errorf("queue: redis client is nil")
	}
	return q.rdb.ZCard(ctx, dueRedisKey()).Result()
}

// DeadLetterDepth returns the length of the dead-letter list.
func (q *RedisQueue) DeadLetterDepth(ctx context.Context) (int64, error) {
	if q.rdb == nil {
		return 0, fmt.Errorf("queue: redis client is nil")
	}
	return q.rdb.LLen(ctx, dlqRedisKey()).Result()
}

// PeekDeadLetters returns DLQ snapshots from index start to stop (inclusive), 0 = newest.
func (q *RedisQueue) PeekDeadLetters(ctx context.Context, start, stop int64) ([]Job, error) {
	if q.rdb == nil {
		return nil, fmt.Errorf("queue: redis client is nil")
	}
	raws, err := q.rdb.LRange(ctx, dlqRedisKey(), start, stop).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Job, 0, len(raws))
	for _, raw := range raws {
		var j Job
		if err := json.Unmarshal([]byte(raw), &j); err != nil {
			continue
		}
		out = append(out, j)
	}
	return out, nil
}

// NewestDeadLetter returns the most recently pushed DLQ snapshot, if any.
func (q *RedisQueue) NewestDeadLetter(ctx context.Context) (*Job, error) {
	jobs, err := q.PeekDeadLetters(ctx, 0, 0)
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, nil
	}
	return &jobs[0], nil
}

// ReplayDeadLetterByID removes one DLQ snapshot for id, restores the job key, and schedules it on due.
func (q *RedisQueue) ReplayDeadLetterByID(ctx context.Context, id string) error {
	if q.rdb == nil {
		return fmt.Errorf("queue: redis client is nil")
	}
	if id == "" {
		return fmt.Errorf("queue: empty job id")
	}
	raws, err := q.rdb.LRange(ctx, dlqRedisKey(), 0, -1).Result()
	if err != nil {
		return err
	}
	for _, raw := range raws {
		var j Job
		if err := json.Unmarshal([]byte(raw), &j); err != nil {
			continue
		}
		if j.ID != id {
			continue
		}
		now := time.Now().UTC()
		j.Status = StatusQueued
		j.LastError = ""
		j.Attempts = 0
		j.RunAt = now
		j.UpdatedAt = now
		j.FinishedAt = nil
		payload, err := json.Marshal(&j)
		if err != nil {
			return err
		}
		pipe := q.rdb.TxPipeline()
		pipe.LRem(ctx, dlqRedisKey(), 1, raw)
		pipe.Set(ctx, jobRedisKey(id), payload, 0)
		pipe.ZAdd(ctx, dueRedisKey(), redis.Z{
			Score:  float64(now.UnixMilli()),
			Member: id,
		})
		_, err = pipe.Exec(ctx)
		return err
	}
	return ErrDeadLetterJobNotFound
}

// AdminRetryJob replays from DLQ if present; otherwise requeues an existing job key (immediate run).
func (q *RedisQueue) AdminRetryJob(ctx context.Context, id string) error {
	if q.rdb == nil {
		return fmt.Errorf("queue: redis client is nil")
	}
	err := q.ReplayDeadLetterByID(ctx, id)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrDeadLetterJobNotFound) {
		return err
	}
	job, err := q.LoadJob(ctx, id)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return ErrAdminRetryTargetNotFound
		}
		return err
	}
	now := time.Now().UTC()
	job.Status = StatusQueued
	job.LastError = ""
	job.Attempts = 0
	job.RunAt = now
	job.UpdatedAt = now
	job.FinishedAt = nil
	return q.Requeue(ctx, job)
}
