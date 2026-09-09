package httpapi

import (
	"encoding/json"
	"errors"
	"kaderisasi/admin/internal/certificate"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/validation"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var indexedIDs = regexp.MustCompile(`^registration_ids\[([0-9]+)\]$`)

func certificateQuery(r *http.Request) database.Object {
	params := r.URL.Query()
	out := database.Object{}
	for key, values := range params {
		if !strings.HasPrefix(key, "registration_ids[") {
			if len(values) == 1 {
				out.Set(key, values[0])
			} else {
				out.Set(key, values)
			}
		}
	}
	if values, ok := params["registration_ids[]"]; ok {
		out.Set("registration_ids", values)
	}
	indices := []int{}
	indexed := map[int][]string{}
	invalid := false
	object := database.Object{}
	for key, values := range params {
		if !strings.HasPrefix(key, "registration_ids[") || key == "registration_ids[]" {
			continue
		}
		match := indexedIDs.FindStringSubmatch(key)
		if match == nil {
			invalid = true
			object.Set(key, values)
			continue
		}
		index, err := strconv.Atoi(match[1])
		if err != nil || index > 200 || strconv.Itoa(index) != match[1] {
			invalid = true
			object.Set(match[1], values)
			continue
		}
		indices = append(indices, index)
		indexed[index] = values
	}
	if invalid {
		out.Set("registration_ids", object)
	} else if len(indices) > 0 {
		sort.Ints(indices)
		values := []string{}
		for _, index := range indices {
			values = append(values, indexed[index]...)
		}
		out.Set("registration_ids", values)
	}
	return out
}
func validatedCertificateQuery(w http.ResponseWriter, r *http.Request) (database.Object, bool) {
	data, issues := validation.Validate("recipientsValidator", certificateQuery(r))
	if len(issues) > 0 {
		write(w, 422, struct {
			Message string             `json:"message"`
			Errors  []validation.Issue `json:"errors"`
		}{"VALIDATION_ERROR", issues})
		return nil, false
	}
	if !certificateIDsFit(data, true) {
		message(w, 500, "GENERAL_ERROR")
		return nil, false
	}
	return data, true
}
func selectedIDs(data database.Object, key string) []int32 {
	if !data.Has(key) {
		return nil
	}
	out := []int32{}
	_ = json.Unmarshal(data[key], &out)
	return out
}
func (s *Server) registerCertificates() {
	controller := "certificates_controller"
	service := certificate.Issuance{Pool: s.Pool, Location: s.Config.Location, Logger: s.Logger}
	s.register(controller, "recipients", func(w http.ResponseWriter, r *http.Request) error {
		raw := r.PathValue("activityId")
		id, err := issuedPathID(raw, "INVALID_ACTIVITY_ID")
		if err != nil {
			return err
		}
		data, ok := validatedCertificateQuery(w, r)
		if !ok {
			return nil
		}
		page, size := int(data.ID("page")), int(data.ID("per_page"))
		if page == 0 {
			page = 1
		}
		if size == 0 {
			size = 50
		}
		result, err := service.Recipients(r.Context(), int32(id), certificate.RecipientOptions{Page: page, PerPage: size, SortOrder: data.String("sort_order"), Search: data.String("search"), State: data.String("state"), RegistrationIDs: selectedIDs(data, "registration_ids")})
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "prepare", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := certificateInput(w, r, "prepareValidator")
		if !ok {
			return nil
		}
		result, err := service.Prepare(r.Context(), data.ID("activity_id"), selectedIDs(data, "registration_ids"))
		if err != nil {
			return err
		}
		reply(w, 200, "CERTIFICATE_REVIEW_READY", result)
		return nil
	})
	s.register(controller, "index", func(w http.ResponseWriter, r *http.Request) error {
		id := int32(0)
		if r.URL.Query().Has("activity_id") {
			value, err := strconv.ParseFloat(r.URL.Query().Get("activity_id"), 64)
			if err != nil || value <= 0 || value != float64(int64(value)) || value > 9007199254740991 {
				return domain.Fail(400, "INVALID_ACTIVITY_ID")
			}
			if value > math.MaxInt32 {
				return errors.New("integer out of range")
			}
			id = int32(value)
		}
		data, ok := validatedCertificateQuery(w, r)
		if !ok {
			return nil
		}
		page, size := boundedPageParams(r, 20)
		result, err := service.List(r.Context(), id, selectedIDs(data, "registration_ids"), page, size)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "lookup", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := certificateInput(w, r, "lookupCertificatesValidator")
		if !ok {
			return nil
		}
		ids := certificate.UniqueIDs(selectedIDs(data, "registration_ids"))
		result, err := service.List(r.Context(), data.ID("activity_id"), ids, 1, float64(len(ids)))
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result.Data)
		return nil
	})
	for _, action := range []string{"show", "showByCode", "verify"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			var result certificate.Response
			var err error
			if action == "show" {
				id, parseErr := issuedPathID(r.PathValue("id"), "INVALID_CERTIFICATE_ID")
				if parseErr != nil {
					return parseErr
				}
				result, err = service.ByID(r.Context(), id)
			} else {
				result, err = service.ByCode(r.Context(), r.PathValue("code"))
			}
			if err != nil {
				if action == "verify" {
					var d *domain.Error
					if errors.As(err, &d) {
						write(w, 404, struct {
							Message string      `json:"message"`
							Data    interface{} `json:"data"`
						}{d.Message, struct {
							Valid bool `json:"valid"`
						}{false}})
						return nil
					}
				}
				return err
			}
			if action == "verify" {
				reply(w, 200, "CERTIFICATE_VERIFIED", struct {
					Valid       bool                     `json:"valid"`
					Certificate *certificate.IssuedData  `json:"certificate"`
					Participant certificate.Participant  `json:"participant"`
					Activity    certificate.ActivityData `json:"activity"`
				}{result.Certificate.RevokedAt == nil, result.Certificate, result.Participant, result.Activity})
			} else {
				reply(w, 200, "GET_DATA_SUCCESS", result)
			}
			return nil
		})
	}
	s.register(controller, "issueSingle", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := certificateInput(w, r, "registrationCertificateValidator")
		if !ok {
			return nil
		}
		actorID := actor(r).ID
		result, err := service.Issue(r.Context(), data.ID("registration_id"), &actorID, r.Header.Get("X-Request-ID"), nil)
		if err != nil {
			return err
		}
		status := 200
		msg := "CERTIFICATE_ALREADY_ISSUED"
		if result.Created {
			status = 201
			msg = "CERTIFICATE_ISSUED"
		}
		reply(w, status, msg, result.Data)
		return nil
	})
	s.register(controller, "issueBulk", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := certificateInput(w, r, "issueBulkCertificateValidator")
		if !ok {
			return nil
		}
		var expected *certificate.Expectation
		if data.Has("expected") {
			_ = json.Unmarshal(data["expected"], &expected)
		}
		actorID := actor(r).ID
		var ids []int64
		if err := json.Unmarshal(data["registration_ids"], &ids); err != nil {
			return err
		}
		result, err := service.Bulk(r.Context(), ids, &actorID, r.Header.Get("X-Request-ID"), expected)
		if err != nil {
			return err
		}
		if data.String("response_mode") != "compact" {
			reply(w, 200, "CERTIFICATES_ISSUED", result)
			return nil
		}
		if !certificateIDsFit(data, true) {
			return errors.New("integer out of range")
		}
		names, err := service.RecipientNames(r.Context(), selectedIDs(data, "registration_ids"))
		if err != nil {
			return err
		}
		results := []database.Object{}
		for _, group := range []struct {
			items []certificate.Response
			state string
		}{{result.Created, "created"}, {result.AlreadyIssued, "already_issued"}} {
			for _, item := range group.items {
				row := database.Object{}
				row.Set("registration_id", item.Participant.RegistrationID)
				row.Set("name", item.Participant.Name)
				row.Set("state", group.state)
				row.Set("certificate_id", item.Certificate.ID)
				row.Set("certificate_code", item.Certificate.Code)
				if group.state == "already_issued" && item.Certificate.RevokedAt != nil {
					row.Set("state", "skipped")
					row.Set("reason", "CERTIFICATE_ALREADY_REVOKED")
				}
				results = append(results, row)
			}
		}
		for _, group := range []struct {
			items []certificate.Failure
			state string
		}{{result.Skipped, "skipped"}, {result.Failed, "failed"}} {
			for _, item := range group.items {
				row := database.Object{}
				name := names[int32(item.RegistrationID)]
				if name == "" {
					name = "Peserta"
				}
				row.Set("registration_id", item.RegistrationID)
				row.Set("reason", item.Reason)
				row.Set("name", name)
				row.Set("state", group.state)
				results = append(results, row)
			}
		}
		reply(w, 200, "CERTIFICATES_ISSUED", struct {
			Results      []database.Object `json:"results"`
			Paused       bool              `json:"paused"`
			RemainingIDs []int64           `json:"remaining_ids"`
		}{results, result.Paused, result.RemainingIDs})
		return nil
	})
	s.register(controller, "revoke", func(w http.ResponseWriter, r *http.Request) error {
		id, parseErr := issuedPathID(r.PathValue("id"), "INVALID_CERTIFICATE_ID")
		if parseErr != nil {
			return parseErr
		}
		data, ok := certificateInput(w, r, "revokeCertificateValidator")
		if !ok {
			return nil
		}
		result, err := service.Revoke(r.Context(), id, data.String("reason"), actor(r).ID, r.Header.Get("X-Request-ID"))
		if err != nil {
			return err
		}
		reply(w, 200, "CERTIFICATE_REVOKED", result)
		return nil
	})
	s.register(controller, "generateSingle", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := certificateInput(w, r, "registrationCertificateValidator")
		if !ok {
			return nil
		}
		result, err := service.Preview(r.Context(), data.ID("registration_id"))
		if err != nil {
			return err
		}
		reply(w, 200, "CERTIFICATE_DATA_GENERATED", result)
		return nil
	})
	s.register(controller, "generate", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := certificateInput(w, r, "generateCertificatesValidator")
		if !ok {
			return nil
		}
		status := data.String("status")
		if status == "" {
			status = certificate.EligibleStatus
		}
		result, err := service.PreviewBulk(r.Context(), data.ID("activity_id"), status)
		if err != nil {
			return err
		}
		reply(w, 200, "CERTIFICATE_DATA_GENERATED", result)
		return nil
	})
}
