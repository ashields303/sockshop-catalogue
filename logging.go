package catalogue

import (
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

// LoggingMiddleware logs method calls, parameters, results, and elapsed time.
func LoggingMiddleware(logger log.Logger) Middleware {
	return func(next Service) Service {
		return loggingMiddleware{
			next:   next,
			logger: logger,
		}
	}
}

type loggingMiddleware struct {
	next   Service
	logger log.Logger
}

func (mw loggingMiddleware) List(tags []string, order string, pageNum, pageSize int) (socks []Sock, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "List",
			"tags", strings.Join(tags, ", "),
			"order", order,
			"pageNum", pageNum,
			"pageSize", pageSize,
			"result", len(socks),
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.List(tags, order, pageNum, pageSize)
}

func (mw loggingMiddleware) Count(tags []string) (n int, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Count",
			"tags", strings.Join(tags, ", "),
			"result", n,
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.Count(tags)
}

func (mw loggingMiddleware) Get(id string) (s Sock, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Get",
			"id", id,
			"sock", s.ID,
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.Get(id)
}

func (mw loggingMiddleware) Stock(id string) (n int, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Stock",
			"id", id,
			"result", n,
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.Stock(id)
}

func (mw loggingMiddleware) AdjustStock(id string, delta int, reason string) (n int, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "AdjustStock",
			"id", id,
			"delta", delta,
			"reason", reason,
			"result", n,
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.AdjustStock(id, delta, reason)
}

func (mw loggingMiddleware) StockHistory(id string) (history []StockAdjustment, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "StockHistory",
			"id", id,
			"result", len(history),
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.StockHistory(id)
}

func (mw loggingMiddleware) LowStock(threshold int) (socks []Sock, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "LowStock",
			"threshold", threshold,
			"result", len(socks),
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.LowStock(threshold)
}

func (mw loggingMiddleware) Tags() (tags []string, err error) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Tags",
			"result", len(tags),
			"err", err,
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.Tags()
}

func (mw loggingMiddleware) Health() (health []Health) {
	defer func(begin time.Time) {
		mw.logger.Log(
			"method", "Health",
			"result", len(health),
			"took", time.Since(begin),
		)
	}(time.Now())
	return mw.next.Health()
}

// StockAuditMiddleware records each committed stock adjustment: one
// event=stock_adjusted log line and an increment of counter for the
// adjustment's reason. Rejected adjustments are not recorded. The counter must
// have a single "reason" label; every reason in StockReasons is initialised to
// 0 so it is exported before the first adjustment.
func StockAuditMiddleware(logger log.Logger, counter *prometheus.CounterVec) Middleware {
	for _, reason := range StockReasons {
		counter.WithLabelValues(reason)
	}
	return func(next Service) Service {
		return stockAuditMiddleware{
			Service: next,
			logger:  logger,
			counter: counter,
		}
	}
}

// stockAuditMiddleware passes every method other than AdjustStock straight
// through to the embedded Service.
type stockAuditMiddleware struct {
	Service
	logger  log.Logger
	counter *prometheus.CounterVec
}

func (mw stockAuditMiddleware) AdjustStock(id string, delta int, reason string) (int, error) {
	n, err := mw.Service.AdjustStock(id, delta, reason)
	if err != nil {
		return n, err
	}
	mw.logger.Log(
		"event", "stock_adjusted",
		"id", id,
		"delta", delta,
		"reason", reason,
		"new_count", n,
	)
	mw.counter.WithLabelValues(reason).Inc()
	return n, nil
}
