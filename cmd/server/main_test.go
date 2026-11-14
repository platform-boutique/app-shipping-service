package main

import (
	"context"
	"net"
	"testing"
	"time"

	pb "shipping-service/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// CODE-04: when the context ends (SIGTERM in production) the server stops accepting, serve
// returns nil, and the port is closed. Deadlines are generous: no timing is asserted.
func TestServe_StopsGracefullyWhenContextEnds(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, ln, 5*time.Second) }()

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	callCtx, callCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer callCancel()
	resp, err := healthpb.NewHealthClient(conn).Check(callCtx, &healthpb.HealthCheckRequest{}, grpc.WaitForReady(true))
	if err != nil || resp.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("health before shutdown: %v %v", resp, err)
	}
	if _, err := pb.NewShippingServiceClient(conn).GetQuote(callCtx, &pb.GetQuoteRequest{}); err != nil {
		t.Fatalf("GetQuote: %v", err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned %v after a graceful stop", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("serve did not return after the context ended")
	}
	if c, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second); err == nil {
		c.Close()
		t.Error("port still open after shutdown")
	}
}

// A context that ended before serve started must not turn into a startup error.
func TestServe_ContextAlreadyEnded(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { done <- serve(ctx, ln, 5*time.Second) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("serve did not return")
	}
}
