package shortlink

import (
	"context"
	"crypto/rand"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

type Service struct {
	Pool    *pgxpool.Pool
	BaseURL string
}
type Input struct {
	OriginalURL string `json:"original_url"`
	Code        string `json:"code,omitempty"`
}
type Link struct {
	Code        string    `json:"code"`
	ShortURL    string    `json:"short_url"`
	OriginalURL string    `json:"original_url"`
	CreatedAt   time.Time `json:"created_at"`
	VisitCount  int64     `json:"visit_count"`
}
type Meta struct {
	Total       int64 `json:"total"`
	PerPage     int32 `json:"per_page"`
	CurrentPage int32 `json:"current_page"`
	LastPage    int64 `json:"last_page"`
}
type Page struct {
	Data    []Link `json:"data"`
	Meta    Meta   `json:"meta"`
	BaseURL string `json:"base_url"`
}

var codePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{3,10}$`)

func ValidateCode(code string) error {
	if !codePattern.MatchString(code) || code == "health" {
		return domain.Fail(422, "INVALID_SHORT_CODE")
	}
	return nil
}
func ValidateDestination(raw, base string) error {
	invalid := func() error { return domain.Fail(422, "INVALID_DESTINATION_URL") }
	if raw == "" || strings.ContainsFunc(raw, unicode.IsControl) || strings.ContainsAny(raw, "\\ ") {
		return invalid()
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Opaque != "" {
		return invalid()
	}
	own, err := url.Parse(base)
	if err != nil || own.Hostname() == "" {
		return invalid()
	}
	if strings.EqualFold(strings.TrimRight(u.Hostname(), "."), strings.TrimRight(own.Hostname(), ".")) {
		return invalid()
	}
	return nil
}
func generateCode() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var result [6]byte
	for i := range result {
		for {
			var b [1]byte
			if _, err := rand.Read(b[:]); err != nil {
				return "", err
			}
			if b[0] < 248 {
				result[i] = alphabet[int(b[0])%len(alphabet)]
				break
			}
		}
	}
	return string(result[:]), nil
}
func (s Service) link(row dbgen.Url) Link {
	return Link{Code: row.ID, ShortURL: s.BaseURL + "/" + row.ID, OriginalURL: row.OriginalUrl, CreatedAt: row.CreatedAt.Time, VisitCount: row.VisitCount}
}
func (s Service) List(ctx context.Context, search string, number, size int32) (Page, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	rows, err := q.ListShortLinks(ctx, dbgen.ListShortLinksParams{Search: search, PageLimit: size, PageOffset: (number - 1) * size})
	if err != nil {
		return Page{}, err
	}
	total, err := q.CountShortLinks(ctx, search)
	if err != nil {
		return Page{}, err
	}
	links := make([]Link, 0, len(rows))
	for _, row := range rows {
		links = append(links, s.link(row))
	}
	return Page{Data: links, Meta: Meta{total, size, number, max(1, (total+int64(size)-1)/int64(size))}, BaseURL: s.BaseURL}, tx.Commit(ctx)
}
func (s Service) Create(ctx context.Context, in Input) (Link, error) {
	return s.create(ctx, in, generateCode)
}
func (s Service) create(ctx context.Context, in Input, generate func() (string, error)) (Link, error) {
	if err := ValidateDestination(in.OriginalURL, s.BaseURL); err != nil {
		return Link{}, err
	}
	if in.Code != "" {
		if err := ValidateCode(in.Code); err != nil {
			return Link{}, err
		}
	}
	for attempt := 0; attempt < 5; attempt++ {
		code := in.Code
		if code == "" {
			var err error
			code, err = generate()
			if err != nil {
				return Link{}, err
			}
		}
		row, err := dbgen.New(s.Pool).CreateShortLink(ctx, dbgen.CreateShortLinkParams{ID: code, OriginalUrl: in.OriginalURL})
		if err == nil {
			return s.link(row), nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Link{}, err
		}
		if in.Code != "" {
			return Link{}, domain.Fail(409, "SHORT_CODE_TAKEN")
		}
	}
	return Link{}, domain.Fail(503, "SHORT_CODE_GENERATION_FAILED")
}
func (s Service) Update(ctx context.Context, code, destination string) (Link, error) {
	if err := ValidateDestination(destination, s.BaseURL); err != nil {
		return Link{}, err
	}
	row, err := dbgen.New(s.Pool).UpdateShortLink(ctx, dbgen.UpdateShortLinkParams{ID: code, OriginalUrl: destination})
	if errors.Is(err, pgx.ErrNoRows) {
		return Link{}, domain.Fail(404, "SHORT_LINK_NOT_FOUND")
	}
	if err != nil {
		return Link{}, err
	}
	return s.link(row), nil
}
func (s Service) Delete(ctx context.Context, code string) error {
	count, err := dbgen.New(s.Pool).DeleteShortLink(ctx, code)
	if err != nil {
		return err
	}
	if count == 0 {
		return domain.Fail(404, "SHORT_LINK_NOT_FOUND")
	}
	return nil
}
