package catalogue

// service.go contains the definition and implementation (business logic) of the
// catalogue service. Everything here is agnostic to the transport (HTTP).

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/jmoiron/sqlx"
)

// Service is the catalogue service, providing operations on a saleable
// catalogue of sock products and their stock levels.
type Service interface {
	List(tags []string, order string, pageNum, pageSize int) ([]Sock, error) // GET /catalogue
	Count(tags []string) (int, error)                                        // GET /catalogue/size
	Get(id string) (Sock, error)                                             // GET /catalogue/{id}
	Stock(id string) (int, error)                                            // GET /catalogue/{id}/stock
	AdjustStock(id string, delta int, reason string) (int, error)            // PATCH /catalogue/{id}/stock
	StockHistory(id string) ([]StockAdjustment, error)                       // GET /catalogue/{id}/stock/history
	LowStock(threshold int) ([]Sock, error)                                  // GET /catalogue/low-stock
	Tags() ([]string, error)                                                 // GET /tags
	Health() []Health                                                        // GET /health
}

// Middleware decorates a Service.
type Middleware func(Service) Service

// Sock describes the thing on offer in the catalogue.
type Sock struct {
	ID          string   `json:"id" db:"id"`
	Name        string   `json:"name" db:"name"`
	Description string   `json:"description" db:"description"`
	ImageURL    []string `json:"imageUrl" db:"-"`
	ImageURL_1  string   `json:"-" db:"image_url_1"`
	ImageURL_2  string   `json:"-" db:"image_url_2"`
	Price       float32  `json:"price" db:"price"`
	Count       int      `json:"count" db:"count"`
	Tags        []string `json:"tag" db:"-"`
	TagString   string   `json:"-" db:"tag_name"`
}

// StockAdjustment is one row of the stock_adjustments table: a signed change
// to a sock's stock count and the reason for it.
type StockAdjustment struct {
	ID        int       `json:"-" db:"id"`
	SockID    string    `json:"-" db:"sock_id"`
	Delta     int       `json:"delta" db:"delta"`
	Reason    string    `json:"reason" db:"reason"`
	CreatedAt time.Time `json:"timestamp" db:"created_at"`
}

// StockReasons lists the accepted reasons for a stock adjustment. It matches
// the stock_adjustments.reason ENUM in the catalogue-db schema.
var StockReasons = []string{"sale", "return", "restock", "correction"}

// Health describes the health of a service
type Health struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Time    string `json:"time"`
}

// ErrNotFound is returned when there is no sock for a given ID.
var ErrNotFound = errors.New("not found")

// ErrDBConnection is returned when connection with the database fails.
var ErrDBConnection = errors.New("database connection error")

// ErrInsufficientStock is returned when an adjustment would take stock below zero.
var ErrInsufficientStock = errors.New("insufficient stock")

// ErrInvalidReason is returned when an adjustment reason is not one of StockReasons.
var ErrInvalidReason = errors.New("invalid reason")

// ErrInvalidDelta is returned when an adjustment delta is zero or not an integer.
var ErrInvalidDelta = errors.New("invalid delta")

// ErrInvalidThreshold is returned when a low-stock threshold is missing,
// negative or not an integer.
var ErrInvalidThreshold = errors.New("invalid threshold")

var baseQuery = "SELECT sock.sock_id AS id, sock.name, sock.description, sock.price, sock.count, sock.image_url_1, sock.image_url_2, GROUP_CONCAT(tag.name) AS tag_name FROM sock JOIN sock_tag ON sock.sock_id=sock_tag.sock_id JOIN tag ON sock_tag.tag_id=tag.tag_id"

// NewCatalogueService returns an implementation of the Service interface,
// with connection to an SQL database.
func NewCatalogueService(db *sqlx.DB, logger log.Logger) Service {
	return &catalogueService{
		db:     db,
		logger: logger,
	}
}

type catalogueService struct {
	db     *sqlx.DB
	logger log.Logger
}

func (s *catalogueService) List(tags []string, order string, pageNum, pageSize int) ([]Sock, error) {
	var socks []Sock
	query := baseQuery

	var args []interface{}

	for i, t := range tags {
		if i == 0 {
			query += " WHERE tag.name=?"
			args = append(args, t)
		} else {
			query += " OR tag.name=?"
			args = append(args, t)
		}
	}

	query += " GROUP BY id"

	if order != "" {
		query += " ORDER BY ?"
		args = append(args, order)
	}

	query += ";"

	err := s.db.Select(&socks, query, args...)
	if err != nil {
		s.logger.Log("database error", err)
		return []Sock{}, ErrDBConnection
	}
	for i, s := range socks {
		socks[i].ImageURL = []string{s.ImageURL_1, s.ImageURL_2}
		socks[i].Tags = strings.Split(s.TagString, ",")
	}

	// DEMO: Change 0 to 850
	time.Sleep(0 * time.Millisecond)

	socks = cut(socks, pageNum, pageSize)

	return socks, nil
}

func (s *catalogueService) Count(tags []string) (int, error) {
	query := "SELECT COUNT(DISTINCT sock.sock_id) FROM sock JOIN sock_tag ON sock.sock_id=sock_tag.sock_id JOIN tag ON sock_tag.tag_id=tag.tag_id"

	var args []interface{}

	for i, t := range tags {
		if i == 0 {
			query += " WHERE tag.name=?"
			args = append(args, t)
		} else {
			query += " OR tag.name=?"
			args = append(args, t)
		}
	}

	query += ";"

	sel, err := s.db.Prepare(query)

	if err != nil {
		s.logger.Log("database error", err)
		return 0, ErrDBConnection
	}
	defer sel.Close()

	var count int
	err = sel.QueryRow(args...).Scan(&count)

	if err != nil {
		s.logger.Log("database error", err)
		return 0, ErrDBConnection
	}

	return count, nil
}

func (s *catalogueService) Get(id string) (Sock, error) {
	query := baseQuery + " WHERE sock.sock_id =? GROUP BY sock.sock_id;"

	var sock Sock
	err := s.db.Get(&sock, query, id)
	if err != nil {
		s.logger.Log("database error", err)
		return Sock{}, ErrNotFound
	}

	sock.ImageURL = []string{sock.ImageURL_1, sock.ImageURL_2}
	sock.Tags = strings.Split(sock.TagString, ",")

	return sock, nil
}

var stockQuery = "SELECT sock.count FROM sock WHERE sock.sock_id =?;"

func (s *catalogueService) Stock(id string) (int, error) {
	var stock int
	err := s.db.Get(&stock, stockQuery, id)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	if err != nil {
		s.logger.Log("database error", err)
		return 0, ErrDBConnection
	}

	return stock, nil
}

var (
	adjustSelectQuery = "SELECT sock.count FROM sock WHERE sock.sock_id =? FOR UPDATE;"
	adjustUpdateQuery = "UPDATE sock SET count =? WHERE sock_id =?;"
	adjustInsertQuery = "INSERT INTO stock_adjustments (sock_id, delta, reason) VALUES (?, ?, ?);"
)

// AdjustStock applies a signed delta to a sock's stock count and records the
// adjustment, in one transaction. The row lock taken by SELECT ... FOR UPDATE
// serialises concurrent adjustments to the same sock, so stock cannot go
// below zero.
func (s *catalogueService) AdjustStock(id string, delta int, reason string) (int, error) {
	if delta == 0 {
		return 0, ErrInvalidDelta
	}
	if !contains(StockReasons, reason) {
		return 0, ErrInvalidReason
	}

	tx, err := s.db.Beginx()
	if err != nil {
		s.logger.Log("database error", err)
		return 0, ErrDBConnection
	}
	defer tx.Rollback() // no-op once committed

	var count int
	err = tx.Get(&count, adjustSelectQuery, id)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	if err != nil {
		s.logger.Log("database error", err)
		return 0, ErrDBConnection
	}

	newCount := count + delta
	if newCount < 0 {
		return 0, ErrInsufficientStock
	}

	if _, err = tx.Exec(adjustUpdateQuery, newCount, id); err != nil {
		s.logger.Log("database error", err)
		return 0, ErrDBConnection
	}
	if _, err = tx.Exec(adjustInsertQuery, id, delta, reason); err != nil {
		s.logger.Log("database error", err)
		return 0, ErrDBConnection
	}
	if err = tx.Commit(); err != nil {
		s.logger.Log("database error", err)
		return 0, ErrDBConnection
	}

	return newCount, nil
}

var (
	sockExistsQuery = "SELECT 1 FROM sock WHERE sock.sock_id =?;"
	// created_at is read as a Unix timestamp so the result does not depend on
	// the session time zone or the driver's parseTime setting.
	historyQuery = "SELECT delta, reason, UNIX_TIMESTAMP(created_at) FROM stock_adjustments WHERE sock_id =? ORDER BY created_at DESC, id DESC LIMIT 50;"
)

// StockHistory returns the 50 most recent adjustments for a sock, newest first.
func (s *catalogueService) StockHistory(id string) ([]StockAdjustment, error) {
	var exists int
	err := s.db.Get(&exists, sockExistsQuery, id)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		s.logger.Log("database error", err)
		return nil, ErrDBConnection
	}

	rows, err := s.db.Query(historyQuery, id)
	if err != nil {
		s.logger.Log("database error", err)
		return nil, ErrDBConnection
	}
	defer rows.Close()

	history := []StockAdjustment{}
	for rows.Next() {
		var (
			a       StockAdjustment
			created int64
		)
		if err := rows.Scan(&a.Delta, &a.Reason, &created); err != nil {
			s.logger.Log("database error", err)
			return nil, ErrDBConnection
		}
		a.CreatedAt = time.Unix(created, 0).UTC()
		history = append(history, a)
	}
	if err := rows.Err(); err != nil {
		s.logger.Log("database error", err)
		return nil, ErrDBConnection
	}

	return history, nil
}

var lowStockQuery = baseQuery + " WHERE sock.count <= ? GROUP BY sock.sock_id ORDER BY sock.count ASC, sock.sock_id ASC;"

// LowStock returns the socks whose stock count is at or below threshold,
// lowest count first.
func (s *catalogueService) LowStock(threshold int) ([]Sock, error) {
	if threshold < 0 {
		return []Sock{}, ErrInvalidThreshold
	}

	socks := []Sock{}
	err := s.db.Select(&socks, lowStockQuery, threshold)
	if err != nil {
		s.logger.Log("database error", err)
		return []Sock{}, ErrDBConnection
	}
	for i, s := range socks {
		socks[i].ImageURL = []string{s.ImageURL_1, s.ImageURL_2}
		socks[i].Tags = strings.Split(s.TagString, ",")
	}

	return socks, nil
}

func (s *catalogueService) Health() []Health {
	var health []Health
	dbstatus := "OK"

	err := s.db.Ping()
	if err != nil {
		dbstatus = "err"
	}

	app := Health{"catalogue", "OK", time.Now().String()}
	db := Health{"catalogue-db", dbstatus, time.Now().String()}

	health = append(health, app)
	health = append(health, db)

	return health
}

func (s *catalogueService) Tags() ([]string, error) {
	var tags []string
	query := "SELECT name FROM tag;"
	rows, err := s.db.Query(query)
	if err != nil {
		s.logger.Log("database error", err)
		return []string{}, ErrDBConnection
	}
	var tag string
	for rows.Next() {
		err = rows.Scan(&tag)
		if err != nil {
			s.logger.Log("database error", err)
			continue
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func cut(socks []Sock, pageNum, pageSize int) []Sock {
	if pageNum == 0 || pageSize == 0 {
		return []Sock{} // pageNum is 1-indexed
	}
	start := (pageNum * pageSize) - pageSize
	if start > len(socks) {
		return []Sock{}
	}
	end := (pageNum * pageSize)
	if end > len(socks) {
		end = len(socks)
	}
	return socks[start:end]
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
