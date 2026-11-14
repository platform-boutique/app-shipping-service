package shipping

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"

	pb "shipping-service/proto"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Service implements the gRPC ShippingService.
type Service struct {
	pb.UnimplementedShippingServiceServer
	log *logrus.Logger
}

// NewService creates a new shipping service.
func NewService(log *logrus.Logger) *Service {
	return &Service{log: log}
}

// itemCount sums the quantities of the items; non-positive quantities are ignored.
func itemCount(items []*pb.CartItem) int {
	count := 0
	for _, item := range items {
		if q := item.GetQuantity(); q > 0 {
			count += int(q)
		}
	}
	return count
}

// quoteZone picks the zone for GetQuote. No address or a blank country uses
// DefaultZone (the cart-page estimate sends none). A country that is given is
// checked with ResolveCountry, exactly as ShipOrder does, so a country that
// ShipOrder would reject fails here, at quote time, instead of after the
// card was charged.
func quoteZone(a *pb.Address) (Zone, error) {
	if strings.TrimSpace(a.GetCountry()) == "" {
		return DefaultZone, nil
	}
	code, err := ResolveCountry(a.GetCountry())
	if err != nil {
		return "", err
	}
	return ZoneForCountry(code), nil
}

// GetQuote produces a shipping quote (cost) in USD for the destination zone
// and the number of items. A country that is given but not recognised is
// rejected with INVALID_ARGUMENT, the same check ShipOrder applies.
func (s *Service) GetQuote(ctx context.Context, req *pb.GetQuoteRequest) (*pb.GetQuoteResponse, error) {
	// WithContext: the JSON formatter adds trace_id, span_id and request_id of this call.
	log := s.log.WithContext(ctx)
	log.Info("[GetQuote] received request")
	defer log.Info("[GetQuote] completed request")

	zone, err := quoteZone(req.GetAddress())
	if err != nil {
		log.WithError(err).Warn("[GetQuote] rejected country")
		return nil, status.Error(codes.InvalidArgument, "invalid address: "+err.Error())
	}
	count := itemCount(req.GetItems())
	quote := QuoteFor(zone, count)

	var days int32
	if count > 0 {
		days = Rates[zone].EstimatedDays
	}

	return &pb.GetQuoteResponse{
		CostUsd: &pb.Money{
			CurrencyCode: "USD",
			Units:        int64(quote.Dollars),
			Nanos:        int32(quote.Cents * 10_000_000), //#nosec G115  // Cents is always 0-99, overflow impossible
		},
		EstimatedDays: days,
		Zone:          string(zone),
	}, nil
}

// ShipOrder mocks shipping the requested items and returns a tracking ID.
// The address is validated (see ValidateAddress); a missing or invalid one is
// rejected with INVALID_ARGUMENT.
func (s *Service) ShipOrder(ctx context.Context, req *pb.ShipOrderRequest) (*pb.ShipOrderResponse, error) {
	log := s.log.WithContext(ctx)
	log.Info("[ShipOrder] received request")
	defer log.Info("[ShipOrder] completed request")

	addr := req.GetAddress()
	code, err := ValidateAddress(addr)
	if err != nil {
		log.WithError(err).Warn("[ShipOrder] rejected address")
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	id := TrackingID(OrderSalt(addr, code, req.GetItems()))
	log.WithFields(logrus.Fields{"country": code, "zone": ZoneForCountry(code)}).Info("[ShipOrder] shipped")

	return &pb.ShipOrderResponse{TrackingId: id}, nil
}

// RecoveryUnaryInterceptor turns a panic in a unary handler into an INTERNAL
// error instead of crashing the process (gRPC-Go does not recover handler
// panics). Defence in depth: the handlers are written not to panic.
func RecoveryUnaryInterceptor(log *logrus.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.WithContext(ctx).WithFields(logrus.Fields{
					"method": info.FullMethod,
					"panic":  fmt.Sprint(r),
					"stack":  string(debug.Stack()),
				}).Error("recovered from handler panic")
				resp, err = nil, status.Error(codes.Internal, "internal error")
			}
		}()
		return handler(ctx, req)
	}
}
