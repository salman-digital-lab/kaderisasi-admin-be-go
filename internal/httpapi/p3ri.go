package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/p3ri"
)

// P3RI committee registration review. Permissions come from the route table;
// responses are {"message":"OK","data":…} and failures carry Indonesian
// messages the admin frontend shows as is.
func (s *Server) registerP3ri() {
	service := &p3ri.Service{Pool: s.Pool, Storage: s.Storage}
	register := func(action string, h func(http.ResponseWriter, *http.Request) (any, error)) {
		s.register("p3ri_controller", action, func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("Cache-Control", "private, no-store")
			data, err := h(w, r)
			if err != nil {
				return err
			}
			if data != nil {
				reply(w, 200, "OK", data)
			}
			return nil
		})
	}
	id := func(r *http.Request) (int32, error) {
		n, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
		if err != nil || n <= 0 {
			return 0, domain.Fail(404, "Pendaftar tidak ditemukan.")
		}
		return int32(n), nil
	}
	filter := func(r *http.Request) p3ri.Filter {
		q := r.URL.Query()
		rank, _ := strconv.Atoi(q.Get("choice_rank"))
		return p3ri.Filter{Status: q.Get("status"), Division: q.Get("division"), ChoiceRank: int32(rank),
			Gender: q.Get("gender"), Search: q.Get("search")}
	}

	register("form", func(_ http.ResponseWriter, _ *http.Request) (any, error) {
		return p3ri.FormJSON(), nil
	})
	register("index", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		page, perPage := pageParams(r, 20, 100)
		if page < 1 || perPage < 1 {
			return nil, domain.Fail(422, "Halaman tidak valid.")
		}
		return service.List(r.Context(), filter(r), int32(page), int32(perPage))
	})
	register("show", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		target, err := id(r)
		if err != nil {
			return nil, err
		}
		return service.Detail(r.Context(), target)
	})
	register("updateStatus", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		target, err := id(r)
		if err != nil {
			return nil, err
		}
		var in p3ri.StatusInput
		body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
		if err != nil || json.Unmarshal(body, &in) != nil {
			return nil, domain.Fail(422, "Data tidak terbaca.")
		}
		return service.SetStatus(r.Context(), actor(r).ID, target, in)
	})
	register("export", func(w http.ResponseWriter, r *http.Request) (any, error) {
		filename, body, err := service.Export(r.Context(), filter(r))
		if err != nil {
			return nil, err
		}
		sendWorkbook(w, filename, body)
		return nil, nil
	})
	register("file", func(w http.ResponseWriter, r *http.Request) (any, error) {
		data, mime, err := service.File(r.Context(), r.URL.Query().Get("key"))
		if err != nil {
			return nil, err
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", "inline")
		_, _ = w.Write(data)
		return nil, nil
	})
}
