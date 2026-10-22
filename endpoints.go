package catalogue

// endpoints.go contains the endpoint definitions, including per-method request
// and response structs. Endpoints are the binding between the service and
// transport.

import (
	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/tracing/opentracing"
	stdopentracing "github.com/opentracing/opentracing-go"
	"golang.org/x/net/context"
)

// Endpoints collects the endpoints that comprise the Service.
type Endpoints struct {
	ListEndpoint         endpoint.Endpoint
	CountEndpoint        endpoint.Endpoint
	GetEndpoint          endpoint.Endpoint
	StockEndpoint        endpoint.Endpoint
	AdjustStockEndpoint  endpoint.Endpoint
	StockHistoryEndpoint endpoint.Endpoint
	LowStockEndpoint     endpoint.Endpoint
	TagsEndpoint         endpoint.Endpoint
	HealthEndpoint       endpoint.Endpoint
}

// MakeEndpoints returns an Endpoints structure, where each endpoint is
// backed by the given service.
func MakeEndpoints(s Service, tracer stdopentracing.Tracer) Endpoints {
	return Endpoints{
		ListEndpoint:         opentracing.TraceServer(tracer, "GET /catalogue")(MakeListEndpoint(s)),
		CountEndpoint:        opentracing.TraceServer(tracer, "GET /catalogue/size")(MakeCountEndpoint(s)),
		GetEndpoint:          opentracing.TraceServer(tracer, "GET /catalogue/{id}")(MakeGetEndpoint(s)),
		StockEndpoint:        opentracing.TraceServer(tracer, "GET /catalogue/{id}/stock")(MakeStockEndpoint(s)),
		AdjustStockEndpoint:  opentracing.TraceServer(tracer, "PATCH /catalogue/{id}/stock")(MakeAdjustStockEndpoint(s)),
		StockHistoryEndpoint: opentracing.TraceServer(tracer, "GET /catalogue/{id}/stock/history")(MakeStockHistoryEndpoint(s)),
		LowStockEndpoint:     opentracing.TraceServer(tracer, "GET /catalogue/low-stock")(MakeLowStockEndpoint(s)),
		TagsEndpoint:         opentracing.TraceServer(tracer, "GET /tags")(MakeTagsEndpoint(s)),
		HealthEndpoint:       opentracing.TraceServer(tracer, "GET /health")(MakeHealthEndpoint(s)),
	}
}

// MakeListEndpoint returns an endpoint via the given service.
func MakeListEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(listRequest)
		socks, err := s.List(req.Tags, req.Order, req.PageNum, req.PageSize)
		return listResponse{Socks: socks, Err: err}, err
	}
}

// MakeCountEndpoint returns an endpoint via the given service.
func MakeCountEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(countRequest)
		n, err := s.Count(req.Tags)
		return countResponse{N: n, Err: err}, err
	}
}

// MakeGetEndpoint returns an endpoint via the given service.
func MakeGetEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(getRequest)
		sock, err := s.Get(req.ID)
		return getResponse{Sock: sock, Err: err}, err
	}
}

// MakeStockEndpoint returns an endpoint via the given service.
func MakeStockEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(stockRequest)
		stock, err := s.Stock(req.ID)
		return stockResponse{ID: req.ID, Stock: stock, Err: err}, err
	}
}

// MakeAdjustStockEndpoint returns an endpoint via the given service.
func MakeAdjustStockEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(adjustStockRequest)
		count, err := s.AdjustStock(req.ID, req.Delta, req.Reason)
		return adjustStockResponse{ID: req.ID, Count: count, Err: err}, serverError(err)
	}
}

// MakeStockHistoryEndpoint returns an endpoint via the given service.
func MakeStockHistoryEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(stockHistoryRequest)
		history, err := s.StockHistory(req.ID)
		return stockHistoryResponse{History: history, Err: err}, serverError(err)
	}
}

// MakeLowStockEndpoint returns an endpoint via the given service.
func MakeLowStockEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		req := request.(lowStockRequest)
		socks, err := s.LowStock(req.Threshold)
		return lowStockResponse{Socks: socks, Err: err}, serverError(err)
	}
}

// serverError returns err only when it is a server-side failure. Client
// errors (unknown id, bad input, insufficient stock) travel in the response
// instead, so they do not count as failures towards the circuit breaker: a
// run of 409s must not open the breaker and block valid adjustments.
func serverError(err error) error {
	switch err {
	case ErrNotFound, ErrInsufficientStock, ErrInvalidReason, ErrInvalidDelta, ErrInvalidThreshold:
		return nil
	}
	return err
}

// MakeTagsEndpoint returns an endpoint via the given service.
func MakeTagsEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		tags, err := s.Tags()
		return tagsResponse{Tags: tags, Err: err}, err
	}
}

// MakeHealthEndpoint returns current health of the given service.
func MakeHealthEndpoint(s Service) endpoint.Endpoint {
	return func(ctx context.Context, request interface{}) (response interface{}, err error) {
		health := s.Health()
		return healthResponse{Health: health}, nil
	}
}

type listRequest struct {
	Tags     []string `json:"tags"`
	Order    string   `json:"order"`
	PageNum  int      `json:"pageNum"`
	PageSize int      `json:"pageSize"`
}

type listResponse struct {
	Socks []Sock `json:"sock"`
	Err   error  `json:"err"`
}

type countRequest struct {
	Tags []string `json:"tags"`
}

type countResponse struct {
	N   int   `json:"size"` // to match original
	Err error `json:"err"`
}

type getRequest struct {
	ID string `json:"id"`
}

type getResponse struct {
	Sock Sock  `json:"sock"`
	Err  error `json:"err"`
}

type stockRequest struct {
	ID string `json:"id"`
}

type stockResponse struct {
	ID    string `json:"id"`
	Stock int    `json:"stock"`
	Err   error  `json:"-"`
}

type adjustStockRequest struct {
	ID     string `json:"-"`
	Delta  int    `json:"delta"`
	Reason string `json:"reason"`
}

type adjustStockResponse struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
	Err   error  `json:"-"`
}

type stockHistoryRequest struct {
	ID string `json:"id"`
}

type stockHistoryResponse struct {
	History []StockAdjustment
	Err     error
}

type lowStockRequest struct {
	Threshold int `json:"threshold"`
}

type lowStockResponse struct {
	Socks []Sock
	Err   error
}

type tagsRequest struct {
	//
}

type tagsResponse struct {
	Tags []string `json:"tags"`
	Err  error    `json:"err"`
}

type healthRequest struct {
	//
}

type healthResponse struct {
	Health []Health `json:"health"`
}
