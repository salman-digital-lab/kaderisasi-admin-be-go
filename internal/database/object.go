package database

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/validation"
	"sort"
	"strings"
	"time"
	"unicode"
)

//go:embed columns.json
var columnsJSON []byte

type Object = validation.Object
type JSONQueries struct{ DB dbgen.DBTX }

func (q JSONQueries) One(ctx context.Context, sql string, args ...interface{}) (Object, error) {
	var raw []byte
	err := q.DB.QueryRow(ctx, "SELECT row_to_json(result) FROM ("+sql+") result", args...).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var object Object
	err = json.Unmarshal(raw, &object)
	return object, err
}
func (q JSONQueries) All(ctx context.Context, sql string, args ...interface{}) ([]Object, error) {
	rows, err := q.DB.Query(ctx, "SELECT row_to_json(result) FROM ("+sql+") result", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	objects := []Object{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var object Object
		if err = json.Unmarshal(raw, &object); err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	return objects, rows.Err()
}
func (q JSONQueries) Count(ctx context.Context, sql string, args ...interface{}) (int64, error) {
	var total int64
	err := q.DB.QueryRow(ctx, "SELECT count(*) FROM ("+sql+") counted", args...).Scan(&total)
	return total, err
}
func tableColumns(table string) (map[string]bool, error) {
	var all map[string][]string
	if err := json.Unmarshal(columnsJSON, &all); err != nil {
		return nil, err
	}
	columns, ok := all[table]
	if !ok || strings.HasPrefix(table, "adonis_") {
		return nil, errors.New("unknown business table")
	}
	result := map[string]bool{}
	for _, column := range columns {
		result[column] = true
	}
	return result, nil
}
func Snake(key string) string {
	var b strings.Builder
	for i, r := range key {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
func (q JSONQueries) mutation(ctx context.Context, table string, id int32, input Object, insert bool) (Object, error) {
	columns, err := tableColumns(table)
	if err != nil {
		return nil, err
	}
	data := Object{}
	for key, value := range input {
		key = Snake(key)
		if !columns[key] || key == "id" {
			return nil, fmt.Errorf("invalid %s column %s", table, key)
		}
		data[key] = value
	}
	if insert && columns["created_at"] {
		data.Set("created_at", time.Now())
	}
	if columns["updated_at"] {
		data.Set("updated_at", time.Now())
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return q.One(ctx, "SELECT * FROM "+table+" WHERE id=$1", id)
	}
	quoted := make([]string, len(keys))
	for i, key := range keys {
		quoted[i] = pgx.Identifier{key}.Sanitize()
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	name := pgx.Identifier{table}.Sanitize()
	var command string
	args := []interface{}{encoded}
	if insert {
		list := strings.Join(quoted, ",")
		command = "INSERT INTO " + name + " (" + list + ") SELECT " + list + " FROM json_populate_record(NULL::" + name + ",$1::json) RETURNING *"
	} else {
		sets := make([]string, len(quoted))
		for i, col := range quoted {
			sets[i] = col + "=payload." + col
		}
		command = "UPDATE " + name + " SET " + strings.Join(sets, ",") + " FROM json_populate_record(NULL::" + name + ",$1::json) payload WHERE " + name + ".id=$2 RETURNING " + name + ".*"
		args = append(args, id)
	}
	var raw []byte
	err = q.DB.QueryRow(ctx, "WITH affected AS ("+command+") SELECT row_to_json(affected) FROM affected", args...).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var result Object
	err = json.Unmarshal(raw, &result)
	return result, err
}
func (q JSONQueries) Insert(ctx context.Context, table string, input Object) (Object, error) {
	return q.mutation(ctx, table, 0, input, true)
}
func (q JSONQueries) Update(ctx context.Context, table string, id int32, input Object) (Object, error) {
	return q.mutation(ctx, table, id, input, false)
}
func (q JSONQueries) Delete(ctx context.Context, table string, id int32) (bool, error) {
	if _, err := tableColumns(table); err != nil {
		return false, err
	}
	tag, err := q.DB.Exec(ctx, "DELETE FROM "+pgx.Identifier{table}.Sanitize()+" WHERE id=$1", id)
	return tag.RowsAffected() > 0, err
}

type Pagination struct {
	Total           int64      `json:"total"`
	PerPage         PageNumber `json:"per_page"`
	CurrentPage     PageNumber `json:"current_page"`
	LastPage        PageNumber `json:"last_page"`
	FirstPage       int        `json:"first_page"`
	FirstPageURL    string     `json:"first_page_url"`
	LastPageURL     string     `json:"last_page_url"`
	NextPageURL     *string    `json:"next_page_url"`
	PreviousPageURL *string    `json:"previous_page_url"`
}
type Page struct {
	Meta Pagination `json:"meta"`
	Data []Object   `json:"data"`
}

type RawPagination struct {
	Total           int64      `json:"total"`
	PerPage         PageNumber `json:"perPage"`
	CurrentPage     PageNumber `json:"currentPage"`
	LastPage        PageNumber `json:"lastPage"`
	FirstPage       int        `json:"firstPage"`
	FirstPageURL    string     `json:"firstPageUrl"`
	LastPageURL     string     `json:"lastPageUrl"`
	NextPageURL     *string    `json:"nextPageUrl"`
	PreviousPageURL *string    `json:"previousPageUrl"`
}

func (m Pagination) Raw() RawPagination {
	return RawPagination(m)
}

type RawPage struct {
	Meta RawPagination `json:"meta"`
	Data []Object      `json:"data"`
}

// RawPage preserves Lucid's database-query paginator, whose metadata uses camel
// case even when the application installs SnakeCaseNamingStrategy on models.
func (p Page) RawPage() RawPage {
	return RawPage{Meta: p.Meta.Raw(), Data: p.Data}
}

func (q JSONQueries) Paginate(ctx context.Context, sql string, args []interface{}, page, perPage float64) (Page, error) {
	total, err := q.Count(ctx, sql, args...)
	if err != nil {
		return Page{}, err
	}
	result := Page{Meta: Meta(total, page, perPage), Data: []Object{}}
	if total == 0 {
		return result, nil
	}
	limit, start, err := SQLPage(page, perPage)
	if err != nil {
		return result, err
	}
	offset := len(args) + 1
	args = append(args, limit, start)
	result.Data, err = q.All(ctx, sql+fmt.Sprintf(" LIMIT $%d OFFSET $%d", offset, offset+1), args...)
	return result, err
}
