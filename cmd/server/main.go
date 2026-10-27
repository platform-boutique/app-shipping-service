package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"shipping-service/internal/shipping"
	pb "shipping-service/proto"

	"github.com/grafana/pyroscope-go"
	grpcprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

var log *logrus.Logger

func init() {
	log = logrus.New()
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout
}

func main() {
	// --- Telemetry: traces, metrics and logs pushed over OTLP/gRPC (Alloy → Tempo/Mimir/Loki) ---
	shutdownTelemetry, err := initTelemetry(context.Background(), "shipping-service", log)
	if err != nil {
		log.Warnf("telemetry disabled, continuing without it: %v", err)
	} else {
		log.Info("telemetry enabled (OTLP/gRPC)")
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = shutdownTelemetry(ctx)
		}()
	}

	// --- Profiling (pprof HTTP + Pyroscope push) ---
	if os.Getenv("ENABLE_PROFILING") == "1" {
		initProfiling()
	} else {
		log.Info("profiling disabled — set ENABLE_PROFILING=1 to enable")
	}

	port := "50051"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	log.Infof("starting grpc server on :%s", port)
	if err := run(port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func run(port string) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	// gRPC server with OTel tracing + Prometheus metrics interceptors.
	// Recovery runs innermost so a recovered panic is still counted as INTERNAL.
	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(grpcprom.UnaryServerInterceptor, shipping.RecoveryUnaryInterceptor(log)),
		grpc.StreamInterceptor(grpcprom.StreamServerInterceptor),
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

	// --- Metrics + pprof on the same HTTP server ---
	go func() {
		metricsMux := http.NewServeMux()
		metricsMux.Handle("/metrics", promhttp.Handler())
		metricsMux.HandleFunc("/debug/pprof/", pprof.Index)
		metricsMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		metricsMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		metricsMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		metricsMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		metricsPort := "9090"
		if p := os.Getenv("METRICS_PORT"); p != "" {
			metricsPort = p
		}
		log.Infof("metrics + pprof endpoint on :%s", metricsPort)
		httpSrv := &http.Server{
			Addr:         ":" + metricsPort,
			Handler:      metricsMux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		if err := httpSrv.ListenAndServe(); err != nil {
			log.Warnf("metrics server error: %v", err)
		}
	}()

	log.Infof("listening on %s", listener.Addr().String())
	return srv.Serve(listener)
}

// initProfiling starts the Pyroscope push-based profiler.
func initProfiling() {
	pyroscopeAddr := os.Getenv("PYROSCOPE_ADDR")
	if pyroscopeAddr == "" {
		pyroscopeAddr = "http://pyroscope:4040"
	}

	_, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: "shipping-service",
		ServerAddress:   pyroscopeAddr,
		Logger:          pyroscope.StandardLogger,
		ProfileTypes: []pyroscope.ProfileType{
			pyroscope.ProfileCPU,
			pyroscope.ProfileAllocObjects,
			pyroscope.ProfileAllocSpace,
			pyroscope.ProfileInuseObjects,
			pyroscope.ProfileInuseSpace,
		},
	})
	if err != nil {
		log.Warnf("pyroscope init failed, continuing without it: %v", err)
		return
	}
	log.Info("profiling enabled → pushing to " + pyroscopeAddr)
}
