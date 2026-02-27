# Quickstart: Health Check Endpoint

## Basic Usage

### Check readiness
```bash
anvil status --ready
# or via curl to daemon socket:
curl --unix-socket ~/.anvil/daemon.sock http://daemon/ready
```

### Check liveness
```bash
curl --unix-socket ~/.anvil/daemon.sock http://daemon/live
```

### Get detailed status
```bash
curl --unix-socket ~/.anvil/daemon.sock http://daemon/status
```

## Kubernetes Integration

### Pod spec with health probes
```yaml
# Using socat sidecar for Unix socket → TCP bridge
containers:
  - name: anvil
    image: anvil:latest
    volumeMounts:
      - name: socket
        mountPath: /root/.anvil

  - name: health-proxy
    image: alpine/socat
    command: ["socat", "TCP-LISTEN:9090,fork,reuseaddr", "UNIX-CONNECT:/root/.anvil/daemon.sock"]
    ports:
      - containerPort: 9090
    volumeMounts:
      - name: socket
        mountPath: /root/.anvil

livenessProbe:
  httpGet:
    path: /live
    port: 9090
  initialDelaySeconds: 10
  periodSeconds: 5

readinessProbe:
  httpGet:
    path: /ready
    port: 9090
  initialDelaySeconds: 5
  periodSeconds: 10
```

### Or with optional TCP health port (anvil.yaml)
```yaml
health_port: 9090  # exposes /ready, /live, /status on TCP
```

```yaml
livenessProbe:
  httpGet:
    path: /live
    port: 9090
readinessProbe:
  httpGet:
    path: /ready
    port: 9090
```
