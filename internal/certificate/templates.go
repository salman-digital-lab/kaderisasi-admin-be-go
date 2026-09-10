package certificate

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/storage"
	"net/url"
	"regexp"
	"strings"
)

type Templates struct {
	Pool    *pgxpool.Pool
	Storage storage.Store
}

var DefaultTemplate = json.RawMessage(`{"backgroundUrl":null,"elements":[],"canvasWidth":800,"canvasHeight":566}`)

func rawObject(raw []byte) database.Object {
	out := database.Object{}
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = database.Object{}
	}
	return out
}
func notReady(readiness Readiness) error {
	return domain.Details(422, "CERTIFICATE_TEMPLATE_NOT_READY", map[string]interface{}{"errors": readiness.Errors})
}
func AssetKey(value string, id int32) (string, error) {
	prefix := fmt.Sprintf("certificate/templates/%d/", id)
	path := value
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		u, err := url.Parse(value)
		if err != nil {
			return "", err
		}
		decoded, err := url.PathUnescape(u.EscapedPath())
		if err != nil {
			return "", err
		}
		path = strings.TrimPrefix(decoded, "/")
	}
	key := ""
	if strings.HasPrefix(path, prefix) {
		key = path
	} else if i := strings.Index(path, "/"+prefix); i >= 0 {
		key = path[i+1:]
	}
	if !strings.HasPrefix(key, prefix) || strings.Contains(key, "?") {
		return "", errors.New("INVALID_CERTIFICATE_ASSET_KEY")
	}
	for _, part := range strings.Split(key, "/") {
		if part == "." || part == ".." {
			return "", errors.New("INVALID_CERTIFICATE_ASSET_KEY")
		}
	}
	return key, nil
}

var extensionPattern = regexp.MustCompile(`\.[a-zA-Z0-9]+$`)
