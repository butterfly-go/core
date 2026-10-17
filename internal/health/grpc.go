package health

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// grpcWatchInterval is how often Watch re-evaluates readiness.
const grpcWatchInterval = 2 * time.Second

// ServiceName is the fully qualified name of the standard gRPC health service.
const ServiceName = "grpc.health.v1.Health"

// GRPCServer implements the standard grpc.health.v1 service on top of a
// Registry, so Kubernetes' native gRPC probes see the same readiness as
// /readyz. Every service name, including the empty one, reports the overall
// readiness of the instance.
type GRPCServer struct {
	healthpb.UnimplementedHealthServer
	reg      *Registry
	interval time.Duration
}

// NewGRPCServer returns a health service backed by reg.
func NewGRPCServer(reg *Registry) *GRPCServer {
	return &GRPCServer{reg: reg, interval: grpcWatchInterval}
}

// Register adds the health service to s unless the application already
// registered its own, in which case the application's wins and nothing
// happens. It reports whether the service was registered.
func (g *GRPCServer) Register(s *grpc.Server) bool {
	if _, exists := s.GetServiceInfo()[ServiceName]; exists {
		return false
	}
	healthpb.RegisterHealthServer(s, g)
	return true
}

func (g *GRPCServer) servingStatus(ctx context.Context) healthpb.HealthCheckResponse_ServingStatus {
	if g.reg.Readiness(ctx).OK() {
		return healthpb.HealthCheckResponse_SERVING
	}
	return healthpb.HealthCheckResponse_NOT_SERVING
}

// Check implements grpc.health.v1.Health.
func (g *GRPCServer) Check(ctx context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: g.servingStatus(ctx)}, nil
}

// Watch implements grpc.health.v1.Health. It sends the current status, then
// again whenever it changes, until the client goes away.
func (g *GRPCServer) Watch(_ *healthpb.HealthCheckRequest, stream healthpb.Health_WatchServer) error {
	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()

	last := healthpb.HealthCheckResponse_ServingStatus(-1)
	for {
		cur := g.servingStatus(stream.Context())
		if cur != last {
			if err := stream.Send(&healthpb.HealthCheckResponse{Status: cur}); err != nil {
				return status.Error(codes.Unavailable, "stream closed")
			}
			last = cur
		}
		select {
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "client went away")
		case <-ticker.C:
		}
	}
}
