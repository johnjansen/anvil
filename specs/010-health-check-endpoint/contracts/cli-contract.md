# API Contract: Health Check Endpoints

## Endpoints

### GET /ready
Readiness probe — returns whether daemon can accept new tasks.

**Response 200 (ready):**
```json
{"ready": true}
```

**Response 503 (not ready):**
```json
{"ready": false, "reason": "worker pool full (10/10 slots in use)"}
```

Reasons include:
- "worker pool full (N/M slots in use)"
- "no projects loaded"
- "daemon is draining"
- "daemon is shutting down"

### GET /live
Liveness probe — returns daemon process health.

**Response 200:**
```json
{"alive": true}
```

### GET /status
Detailed status with worker, project, and task metrics.

**Response 200:**
```json
{
  "ready": true,
  "workers": {"available": 8, "max": 10},
  "projects": 3,
  "running_tasks": 2,
  "pending_tasks": 5,
  "uptime": "2h35m12s",
  "draining": false
}
```

## Notes
- All endpoints return Content-Type: application/json
- /ready and /live are designed for Kubernetes probe integration
- /status is designed for monitoring dashboards and alerting
- Endpoints are accessible via Unix socket (existing) or optional TCP health port
