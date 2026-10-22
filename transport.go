package catalogue

// transport.go contains the binding from endpoints to a concrete transport.
// In our case we just use a REST-y HTTP transport.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/kit/circuitbreaker"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/tracing/opentracing"
	httptransport "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	stdopentracing "github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sony/gobreaker"
	"golang.org/x/net/context"
)

// MakeHTTPHandler mounts the endpoints into a REST-y HTTP handler.
func MakeHTTPHandler(ctx context.Context, e Endpoints, imagePath string, logger log.Logger, tracer stdopentracing.Tracer) *mux.Router {
	r := mux.NewRouter().StrictSlash(false)
	options := []httptransport.ServerOption{
		httptransport.ServerErrorLogger(logger),
		httptransport.ServerErrorEncoder(encodeError),
	}

	// GET /catalogue       List
	// GET /catalogue/size  Count
	// GET /catalogue/low-stock?threshold=N  LowStock
	// GET /catalogue/{id}  Get
	// GET /catalogue/{id}/stock  Stock
	// PATCH /catalogue/{id}/stock  AdjustStock
	// GET /catalogue/{id}/stock/history  StockHistory
	// GET /tags            Tags
	// GET /health		Health Check
	//
	// Fixed paths under /catalogue (size, low-stock) must be registered before
	// /catalogue/{id}, or mux matches them as an id.

	r.Methods("GET").Path("/catalogue").Handler(httptransport.NewServer(
		ctx,
		circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "List",
			Timeout: 30 * time.Second,
		}))(e.ListEndpoint),
		decodeListRequest,
		encodeListResponse,
		append(options, httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "GET /catalogue", logger)))...,
	))
	r.Methods("GET").Path("/catalogue/size").Handler(httptransport.NewServer(
		ctx,
		circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "Count",
			Timeout: 30 * time.Second,
		}))(e.CountEndpoint),
		decodeCountRequest,
		encodeResponse,
		append(options, httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "GET /catalogue/size", logger)))...,
	))
	r.Methods("GET").Path("/catalogue/low-stock").Handler(httptransport.NewServer(
		ctx,
		circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "LowStock",
			Timeout: 30 * time.Second,
		}))(e.LowStockEndpoint),
		decodeLowStockRequest,
		encodeLowStockResponse,
		append(options,
			httptransport.ServerErrorEncoder(encodeStockError),
			httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "GET /catalogue/low-stock", logger)),
		)...,
	))
	r.Methods("GET").Path("/catalogue/{id}").Handler(httptransport.NewServer(
		ctx,
		circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "Get",
			Timeout: 30 * time.Second,
		}))(e.GetEndpoint),
		decodeGetRequest,
		encodeGetResponse, // special case, this one can have an error
		append(options, httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "GET /catalogue/{id}", logger)))...,
	))
	r.Methods("GET").Path("/catalogue/{id}/stock").Handler(httptransport.NewServer(
		ctx,
		circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "Stock",
			Timeout: 30 * time.Second,
		}))(e.StockEndpoint),
		decodeStockRequest,
		encodeStockResponse,
		append(options,
			httptransport.ServerErrorEncoder(encodeStockError),
			httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "GET /catalogue/{id}/stock", logger)),
		)...,
	))
	r.Methods("PATCH").Path("/catalogue/{id}/stock").Handler(httptransport.NewServer(
		ctx,
		circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "AdjustStock",
			Timeout: 30 * time.Second,
		}))(e.AdjustStockEndpoint),
		decodeAdjustStockRequest,
		encodeAdjustStockResponse,
		append(options,
			httptransport.ServerErrorEncoder(encodeStockError),
			httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "PATCH /catalogue/{id}/stock", logger)),
		)...,
	))
	r.Methods("GET").Path("/catalogue/{id}/stock/history").Handler(httptransport.NewServer(
		ctx,
		circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "StockHistory",
			Timeout: 30 * time.Second,
		}))(e.StockHistoryEndpoint),
		decodeStockHistoryRequest,
		encodeStockHistoryResponse,
		append(options,
			httptransport.ServerErrorEncoder(encodeStockError),
			httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "GET /catalogue/{id}/stock/history", logger)),
		)...,
	))
	r.Methods("GET").Path("/tags").Handler(httptransport.NewServer(
		ctx,
		circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "Tags",
			Timeout: 30 * time.Second,
		}))(e.TagsEndpoint),
		decodeTagsRequest,
		encodeResponse,
		append(options, httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "GET /tags", logger)))...,
	))
	r.Methods("GET").PathPrefix("/catalogue/images/").Handler(http.StripPrefix(
		"/catalogue/images/",
		http.FileServer(http.Dir(imagePath)),
	))
	r.Methods("GET").PathPrefix("/health").Handler(httptransport.NewServer(
		ctx,
		circuitbreaker.Gobreaker(gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:    "Health",
			Timeout: 30 * time.Second,
		}))(e.HealthEndpoint),
		decodeHealthRequest,
		encodeHealthResponse,
		append(options, httptransport.ServerBefore(opentracing.FromHTTPRequest(tracer, "GET /health", logger)))...,
	))
	r.Handle("/metrics", promhttp.Handler())
	return r
}

// ErrMalformedBody is returned when a request body is not valid JSON of the
// expected shape.
var ErrMalformedBody = errors.New("malformed request body")

func encodeError(_ context.Context, err error, w http.ResponseWriter) {
	code := http.StatusInternalServerError
	switch err {
	case ErrNotFound:
		code = http.StatusNotFound
	case ErrInsufficientStock:
		code = http.StatusConflict
	case ErrInvalidReason, ErrInvalidDelta, ErrInvalidThreshold, ErrMalformedBody:
		code = http.StatusBadRequest
	}
	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":       err.Error(),
		"status_code": code,
		"status_text": http.StatusText(code),
	})
}

func decodeListRequest(_ context.Context, r *http.Request) (interface{}, error) {
	pageNum := 1
	if page := r.FormValue("page"); page != "" {
		pageNum, _ = strconv.Atoi(page)
	}
	pageSize := 10
	if size := r.FormValue("size"); size != "" {
		pageSize, _ = strconv.Atoi(size)
	}
	order := "id"
	if sort := r.FormValue("sort"); sort != "" {
		order = strings.ToLower(sort)
	}
	tags := []string{}
	if tagsval := r.FormValue("tags"); tagsval != "" {
		tags = strings.Split(tagsval, ",")
	}
	return listRequest{
		Tags:     tags,
		Order:    order,
		PageNum:  pageNum,
		PageSize: pageSize,
	}, nil
}

// encodeListResponse is distinct from the generic encodeResponse because our
// clients expect that we will encode the slice (array) of socks directly,
// without the wrapping response object.
func encodeListResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(listResponse)
	return encodeResponse(ctx, w, resp.Socks)
}

func decodeCountRequest(_ context.Context, r *http.Request) (interface{}, error) {
	tags := []string{}
	if tagsval := r.FormValue("tags"); tagsval != "" {
		tags = strings.Split(tagsval, ",")
	}
	return countRequest{
		Tags: tags,
	}, nil
}

func decodeGetRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return getRequest{
		ID: mux.Vars(r)["id"],
	}, nil
}

// encodeGetResponse is distinct from the generic encodeResponse because we need
// to special-case when the getResponse object contains a non-nil error.
func encodeGetResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(getResponse)
	if resp.Err != nil {
		encodeError(ctx, resp.Err, w)
		return nil
	}
	return encodeResponse(ctx, w, resp.Sock)
}

func decodeStockRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return stockRequest{
		ID: mux.Vars(r)["id"],
	}, nil
}

// encodeStockResponse special-cases a non-nil error in the same way as
// encodeGetResponse.
func encodeStockResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(stockResponse)
	if resp.Err != nil {
		encodeStockError(ctx, resp.Err, w)
		return nil
	}
	return encodeResponse(ctx, w, resp)
}

// encodeStockError applies the encodeError status mapping to the stock routes.
// The go-kit server wraps decode and endpoint errors in an httptransport.Error,
// which encodeError does not unwrap, so it would map ErrNotFound to 500. The
// unwrap and the Content-Type fix are scoped to these routes to leave the error
// behaviour of the existing routes unchanged.
func encodeStockError(ctx context.Context, err error, w http.ResponseWriter) {
	if e, ok := err.(httptransport.Error); ok {
		err = e.Err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	encodeError(ctx, err, w)
}

// decodeAdjustStockRequest validates the JSON body {"delta": int, "reason": string}.
// A missing or zero delta is invalid. A non-integer delta, or a body that is
// not a JSON object, fails to decode and is reported as ErrMalformedBody.
func decodeAdjustStockRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var body struct {
		Delta  *int    `json:"delta"`
		Reason *string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, ErrMalformedBody
	}
	if body.Delta == nil || *body.Delta == 0 {
		return nil, ErrInvalidDelta
	}
	if body.Reason == nil || !contains(StockReasons, *body.Reason) {
		return nil, ErrInvalidReason
	}
	return adjustStockRequest{
		ID:     mux.Vars(r)["id"],
		Delta:  *body.Delta,
		Reason: *body.Reason,
	}, nil
}

func encodeAdjustStockResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(adjustStockResponse)
	if resp.Err != nil {
		encodeStockError(ctx, resp.Err, w)
		return nil
	}
	return encodeResponse(ctx, w, resp)
}

func decodeStockHistoryRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return stockHistoryRequest{
		ID: mux.Vars(r)["id"],
	}, nil
}

// encodeStockHistoryResponse encodes the adjustments as a bare JSON array.
func encodeStockHistoryResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(stockHistoryResponse)
	if resp.Err != nil {
		encodeStockError(ctx, resp.Err, w)
		return nil
	}
	return encodeResponse(ctx, w, resp.History)
}

// decodeLowStockRequest requires threshold to be a non-negative integer. Unlike
// decodeListRequest it does not fall back to a default on a bad value.
func decodeLowStockRequest(_ context.Context, r *http.Request) (interface{}, error) {
	threshold, err := strconv.Atoi(r.FormValue("threshold"))
	if err != nil || threshold < 0 {
		return nil, ErrInvalidThreshold
	}
	return lowStockRequest{
		Threshold: threshold,
	}, nil
}

// encodeLowStockResponse encodes the socks as a bare JSON array, like the list route.
func encodeLowStockResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	resp := response.(lowStockResponse)
	if resp.Err != nil {
		encodeStockError(ctx, resp.Err, w)
		return nil
	}
	return encodeResponse(ctx, w, resp.Socks)
}

func decodeTagsRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return struct{}{}, nil
}

func decodeHealthRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return struct{}{}, nil
}

func encodeHealthResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	return encodeResponse(ctx, w, response.(healthResponse))
}

func encodeResponse(_ context.Context, w http.ResponseWriter, response interface{}) error {
	// All of our response objects are JSON serializable, so we just do that.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}
