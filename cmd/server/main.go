package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"shipping-service/internal/obs"
	"shipping-service/internal/shipping"
	pb "shipping-service/proto"

	grpcprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

var log = logrus.New()

// drainTimeout is how long in-flight calls get after SIGTERM. With the telemetry flush
// bound (obs.DefaultShutdownTimeout, 4 s) the process exits within 9 s (CODE-04: 10 s).
const drainTimeout = 5 * time.Second

func main() {
	// Logger, env contract (OTEL_*, PYROSCOPE_ADDR), OTLP pipelines, Pyroscope, build_info and
	// /metrics on :9090 live in internal/obs. pprof is served only if PPROF_ADDR is set.
	o := obs.Start(context.Background(), obs.Options{
		Service: "shipping-service",
		Loggers: []*logrus.Logger{log},
	})
	defer o.Recover()
	defer o.Shutdown()

	port := "50051"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	log.Infof("starting grpc server on :%s", port)
	if err := run(port); err != nil {
		o.Fatal("server error", err)
	}
}

func run(port string) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", port, err)
	}
	// SIGTERM/SIGINT end the context; serve then drains and returns nil.
	sigCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSignals()
	return serve(sigCtx, listener, drainTimeout)
}

// serve runs the gRPC server on listener until ctx ends, then reports NOT_SERVING, stops
// accepting, lets in-flight calls finish (at most drain) and returns nil.
func serve(ctx context.Context, listener net.Listener, drain time.Duration) error {
	// gRPC server with OTel tracing + Prometheus metrics interceptors.
	// Recovery runs innermost so a recovered panic is still counted as INTERNAL.
	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(obs.UnaryServerInterceptor(), grpcprom.UnaryServerInterceptor, shipping.RecoveryUnaryInterceptor(log)),
		grpc.ChainStreamInterceptor(obs.StreamServerInterceptor(), grpcprom.StreamServerInterceptor),
	)

	svc := shipping.NewService(log)
	pb.RegisterShippingServiceServer(srv, svc)

	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	reflection.Register(srv)

	// Register all gRPC server metrics with Prometheus
	grpcprom.Register(srv)
	grpcprom.EnableHandlingTimeHistogram()

	// Serve returns nil after a graceful stop, so main returns and the deferred telemetry
	// flush runs.
	go func() {
		<-ctx.Done()
		log.Info("shutdown signal received, draining")
		healthSrv.Shutdown()
		stopped := make(chan struct{})
		go func() { srv.GracefulStop(); close(stopped) }()
		select {
		case <-stopped:
		case <-time.After(drain):
			log.Warnf("drain did not finish within %s, stopping", drain)
			srv.Stop()
		}
	}()

	log.Infof("listening on %s", listener.Addr().String())
	if err := srv.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}
