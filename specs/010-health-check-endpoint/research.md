# Research: Health Check Endpoint

## Decision 1: Endpoint Transport
- **Decision**: Use existing Unix socket HTTP server (add routes to existing ServeMux)
- **Rationale**: Daemon already has a full HTTP server at ~/.anvil/daemon.sock with /health, /status, /metrics, /ps, etc. Adding /ready and /live to the same mux is trivial and consistent.
- **Alternatives**: Separate TCP listener for health probes. Rejected — adds complexity and config; Unix socket works with Kubernetes via socat sidecar or TCP health-check proxy, and the existing /health is already socket-based.

## Decision 2: Readiness Logic
- **Decision**: Readiness = (workers available > 0) AND (projects > 0) AND (not draining) AND (not shutting down)
- **Rationale**: Mirrors the existing HealthResponse.Healthy logic but exposed at a dedicated endpoint with proper HTTP status codes (200 vs 503).
- **Alternatives**: Include pending task count in readiness. Rejected — pending tasks don't mean the daemon can't accept more.

## Decision 3: Response Format
- **Decision**: JSON responses for all three endpoints with Content-Type: application/json
- **Rationale**: Consistent with existing /health and /status endpoints. JSON is the standard for health check responses in cloud-native environments.
- **Alternatives**: Plain text. Rejected — less structured and harder to parse programmatically.

## Decision 4: Existing /health and /status Overlap
- **Decision**: Keep existing /health and /status endpoints unchanged. Add /ready, /live, and enhance /status with additional fields.
- **Rationale**: Backward compatibility is critical. Existing monitoring setups depend on current response formats. The new /ready endpoint provides proper HTTP 503 semantics that /health lacks.
- **Alternatives**: Modify /health to return 503. Rejected — breaking change for existing users.

## Decision 5: Kubernetes Integration Pattern
- **Decision**: Document Unix socket access via socat sidecar or TCP proxy pattern. Also support direct TCP listener via optional health_port config.
- **Rationale**: Kubernetes probes need TCP access. Unix sockets require a sidecar. An optional TCP health port is the cleanest solution for container environments.
- **Alternatives**: Only Unix socket. Too limiting for Kubernetes without documentation. Only TCP. Changes the security model.
