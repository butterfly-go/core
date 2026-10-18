# Deployment

## Docker

The default configuration backend is Consul. In a container without Consul, select the file backend explicitly.

```dockerfile
FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o service .

FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /app/service .
COPY config.yaml .

ENV BUTTERFLY_CONFIG_TYPE=file \
    BUTTERFLY_CONFIG_FILE_PATH=/app/config.yaml

# Use the exec form of CMD, so the service is PID 1 and receives SIGTERM from
# `docker stop`. The shell form would put `sh` in between and swallow the signal.
CMD ["./service"]
```

For production you will usually leave out `config.yaml` and use Consul instead (`BUTTERFLY_CONFIG_TYPE=consul`, plus the Consul address). See [Configuration](configuration.md).

## Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: user-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: user-service
  template:
    metadata:
      labels:
        app: user-service
    spec:
      # Longer than ShutdownTimeout (default 30s), see below.
      terminationGracePeriodSeconds: 45
      containers:
      - name: user-service
        image: your-registry/user-service:latest
        ports:
        - name: http
          containerPort: 8080
        - name: grpc            # only when GRPCRegister is set
          containerPort: 9090
        - name: admin           # /metrics, /healthz, /readyz
          containerPort: 2223
        env:
        - name: BUTTERFLY_CONFIG_TYPE
          value: "consul"
        - name: BUTTERFLY_CONFIG_CONSUL_ADDRESS
          value: "consul:8500"
        - name: BUTTERFLY_TRACING_ENDPOINT
          value: "otel-collector:4317"   # gRPC is the default provider
        livenessProbe:
          httpGet:
            path: /healthz
            port: admin
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: admin
          periodSeconds: 5
```

Notes:

- **Probes go to the admin port.** The framework serves `/healthz` and `/readyz` itself, so do not implement them in your router. See [Health checks](observability.md#health-checks).
- **A gRPC-only service** can use the native probe instead, `readinessProbe: { grpc: { port: 9090 } }`. It reports the same state as `/readyz`.
- **The admin port serves metrics.** Point Prometheus at `:2223/metrics`, and do not expose that port outside the cluster.

### Rolling updates without dropped requests

When a pod is deleted, Kubernetes sends `SIGTERM` and, in parallel, removes the pod from the Service endpoints. The second takes a moment to propagate, so for a short time traffic still arrives at a pod that is shutting down. To cover that gap:

```yaml
server:
  shutdown_delay: 5s   # report not-ready, keep serving for 5s, then stop
```

The shutdown sequence is: report not-ready, wait `shutdown_delay`, drain in-flight requests, run teardown functions. All of it, including the delay, must fit in `ShutdownTimeout` (default 30s), and the pod's `terminationGracePeriodSeconds` must be longer than that, or Kubernetes sends `SIGKILL` before the pod finishes. The 45 above leaves some margin over the default timeout. See [Graceful shutdown](application.md#graceful-shutdown).
