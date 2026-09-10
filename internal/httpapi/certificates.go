package httpapi

import (
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
func validatedCertificateQuery(w http.ResponseWriter, r *http.Request) (certificate.RecipientOptions, bool) {
	data, issues := validation.Validate("recipientsValidator", certificateQuery(r))
	if len(issues) > 0 {
		write(w, 422, struct {
			Message string             `json:"message"`
			Errors  []validation.Issue `json:"errors"`
		}{"VALIDATION_ERROR", issues})
		return certificate.RecipientOptions{}, false
	}
	return decodeInputAs[certificate.RecipientOptions](w, data)
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
		if data.Page == 0 {
			data.Page = 1
		}
		if data.PerPage == 0 {
			data.PerPage = 50
		}
		result, err := service.Recipients(r.Context(), id, data)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "prepare", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := certificateInputAs[certificate.SelectionInput](w, r, "prepareValidator")
		if !ok {
			return nil
		}
		result, err := service.Prepare(r.Context(), data.ActivityID, data.RegistrationIDs)
		if err != nil {
			return err
		}
		reply(w, 200, "CERTIFICATE_REVIEW_READY", result)
		return nil
	})
	s.register(controller, "index", func(w http.ResponseWriter, r *http.Request) error {
		id := float64(0)
		if r.URL.Query().Has("activity_id") {
			value, err := strconv.ParseFloat(database.NumberIdentifier(r.URL.Query().Get("activity_id")), 64)
			if err != nil || value <= 0 || value != math.Trunc(value) || value > 9007199254740991 {
				return domain.Fail(400, "INVALID_ACTIVITY_ID")
			}

			id = value
		}
		data, ok := validatedCertificateQuery(w, r)
		if !ok {
			return nil
		}
		page, size := boundedPageParams(r, 20)
		result, err := service.List(r.Context(), id, data.RegistrationIDs, page, size)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "lookup", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := certificateInputAs[certificate.SelectionInput](w, r, "lookupCertificatesValidator")
		if !ok {
			return nil
		}
		ids := certificate.UniqueIDs(data.RegistrationIDs)
		result, err := service.List(r.Context(), data.ActivityID, ids, 1, float64(len(ids)))
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
		data, ok := certificateInputAs[certificate.RegistrationInput](w, r, "registrationCertificateValidator")
		if !ok {
			return nil
		}
		actorID := actor(r).ID
		result, err := service.Issue(r.Context(), data.RegistrationID, &actorID, r.Header.Get("X-Request-ID"), nil)
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
		data, ok := certificateInputAs[certificate.BulkInput](w, r, "issueBulkCertificateValidator")
		if !ok {
			return nil
		}
		actorID := actor(r).ID
		result, err := service.Bulk(r.Context(), data.RegistrationIDs, &actorID, r.Header.Get("X-Request-ID"), data.Expected)
		if err != nil {
			return err
		}
		if data.ResponseMode != "compact" {
			reply(w, 200, "CERTIFICATES_ISSUED", result)
			return nil
		}
		compact, err := service.Compact(r.Context(), result, data.RegistrationIDs)
		if err != nil {
			return err
		}
		reply(w, 200, "CERTIFICATES_ISSUED", compact)
		return nil
	})
	s.register(controller, "revoke", func(w http.ResponseWriter, r *http.Request) error {
		id, parseErr := issuedPathID(r.PathValue("id"), "INVALID_CERTIFICATE_ID")
		if parseErr != nil {
			return parseErr
		}
		data, ok := certificateInputAs[certificate.RevocationInput](w, r, "revokeCertificateValidator")
		if !ok {
			return nil
		}
		result, err := service.Revoke(r.Context(), id, data.Reason, actor(r).ID, r.Header.Get("X-Request-ID"))
		if err != nil {
			return err
		}
		reply(w, 200, "CERTIFICATE_REVOKED", result)
		return nil
	})
	s.register(controller, "generateSingle", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := certificateInputAs[certificate.RegistrationInput](w, r, "registrationCertificateValidator")
		if !ok {
			return nil
		}
		result, err := service.Preview(r.Context(), data.RegistrationID)
		if err != nil {
			return err
		}
		reply(w, 200, "CERTIFICATE_DATA_GENERATED", result)
		return nil
	})
	s.register(controller, "generate", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := certificateInputAs[certificate.GenerationInput](w, r, "generateCertificatesValidator")
		if !ok {
			return nil
		}
		status := data.Status
		if status == "" {
			status = certificate.EligibleStatus
		}
		result, err := service.PreviewBulk(r.Context(), data.ActivityID, status)
		if err != nil {
			return err
		}
		reply(w, 200, "CERTIFICATE_DATA_GENERATED", result)
		return nil
	})
}
