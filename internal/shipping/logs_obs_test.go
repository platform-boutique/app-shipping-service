package shipping

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"shipping-service/internal/obs"
	pb "shipping-service/proto"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

func unaryInfo(m string) *grpc.UnaryServerInfo { return &grpc.UnaryServerInfo{FullMethod: m} }

func ctxWithSpan() context.Context {
	tid, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	sid, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled,
	}))
	return obs.WithRequestID(ctx, "req-9")
}

func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("not JSON: %q", l)
		}
		out = append(out, m)
	}
	return out
}

// OBS-30: every line a handler writes is JSON in the contract's shape and, inside a request,
// carries that request's trace id, span id and request id; the address itself is never logged.
func TestHandlers_LogsCarryTraceIDsAndNoAddress(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(obs.NewFormatter("shipping-service", "test"))
	svc := NewService(l)

	addr := &pb.Address{StreetAddress: "1 Secret Street", City: "Springfield", Country: "US", ZipCode: 12345}
	if _, err := svc.ShipOrder(ctxWithSpan(), &pb.ShipOrderRequest{Address: addr}); err != nil {
		t.Fatal(err)
	}
	// rejected address: warning with the error field
	if _, err := svc.ShipOrder(ctxWithSpan(), &pb.ShipOrderRequest{Address: &pb.Address{Country: "US"}}); err == nil {
		t.Fatal("expected a rejection")
	}
	if _, err := svc.GetQuote(ctxWithSpan(), &pb.GetQuoteRequest{Address: &pb.Address{Country: "Atlantis"}}); err == nil {
		t.Fatal("expected a rejection")
	}

	lines := logLines(t, &buf)
	if len(lines) < 8 {
		t.Fatalf("expected the handlers to log, got:\n%s", buf.String())
	}
	warnings := 0
	for _, m := range lines {
		if m["trace_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" || m["span_id"] != "00f067aa0ba902b7" || m["request_id"] != "req-9" {
			t.Errorf("line without the request's ids: %v", m)
		}
		if m["service"] != "shipping-service" || m["version"] != "test" || m["severity"] == nil || m["timestamp"] == nil || m["message"] == nil {
			t.Errorf("shape: %v", m)
		}
		if m["severity"] == "warning" {
			warnings++
			if e, _ := m["error"].(string); e == "" {
				t.Errorf("warning without the error field: %v", m)
			}
		}
	}
	if warnings != 2 {
		t.Errorf("expected 2 rejection warnings, got %d", warnings)
	}
	if strings.Contains(buf.String(), "Secret Street") || strings.Contains(buf.String(), "Springfield") {
		t.Errorf("address in the logs:\n%s", buf.String())
	}
}

// Outside a span the ids are absent (not empty strings, not zeros).
func TestHandlers_NoIDsWithoutSpan(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(obs.NewFormatter("shipping-service", "test"))
	svc := NewService(l)
	if _, err := svc.GetQuote(context.Background(), &pb.GetQuoteRequest{}); err != nil {
		t.Fatal(err)
	}
	for _, m := range logLines(t, &buf) {
		for _, k := range []string{"trace_id", "span_id", "request_id"} {
			if _, ok := m[k]; ok {
				t.Errorf("%s present outside a span: %v", k, m)
			}
		}
	}
}

// A recovered panic is logged with the ids of the request.
func TestRecoveryInterceptor_LogsPanicWithTraceID(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(obs.NewFormatter("shipping-service", "test"))
	_, err := RecoveryUnaryInterceptor(l)(ctxWithSpan(), nil, unaryInfo("/hipstershop.ShippingService/ShipOrder"),
		func(context.Context, any) (any, error) { panic("boom") })
	if err == nil {
		t.Fatal("expected INTERNAL")
	}
	lines := logLines(t, &buf)
	if len(lines) != 1 || lines[0]["severity"] != "error" || lines[0]["trace_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("panic line: %s", buf.String())
	}
}
