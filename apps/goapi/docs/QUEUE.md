# Background queue (Redis jobs)

Transactional email (verification, password reset, organization invitations) is implemented as queue jobs. Behavior depends on Redis connectivity and configuration:

| Condition | Behavior |
|-----------|----------|
| Redis unavailable or queue/async disabled | Jobs run **inline** in the API process (synchronous). Startup logs: `queue: using inline synchronous job execution`. |
| Redis + async enabled + worker enabled | Jobs are stored in Redis and processed by the **worker** loop in the API process (`queue: async Redis worker enabled`). |
| Redis + async enabled + **worker disabled** (`JOB_WORKER_ENABLED=false`) | Jobs are **enqueued only**; nothing drains the queue unless another process runs the worker. Startup warns with guidance. Prefer enabling the worker or switching to inline mode for development. |

Operational checklist:

1. If you use Redis-backed enqueue in production, ensure at least one API or worker instance runs with `JOB_WORKER_ENABLED=true` (or your deployment equivalent).
2. Monitor Redis list/stream depth or use admin job health endpoints if exposed.
3. For local Docker when email seems delayed, confirm the worker is running and Redis is healthy.

See [`internal/queue/bundle.go`](../internal/queue/bundle.go) for the decision logic.
