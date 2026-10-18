package certificate

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

// TemplateLock is the activity's certificate template choice. A locked choice
// is required before issuance and can be released only while no active
// certificate of the activity exists.
type TemplateLock struct {
	Template       *RecipientTemplate `json:"template"`
	Locked         bool               `json:"locked"`
	LockedAt       *string            `json:"locked_at"`
	PublishedCount int64              `json:"published_count"`
}

// savedLock is stored as additional_config.certificate_template_lock. The
// public activity API returns additional_config, so it holds no admin data.
type savedLock struct {
	LockedAt string `json:"locked_at"`
}

func activityLock(activity dbgen.Activity) *savedLock {
	var config struct {
		Lock *savedLock `json:"certificate_template_lock"`
	}
	if json.Unmarshal(activity.AdditionalConfig, &config) != nil {
		return nil
	}
	return config.Lock
}

// IsTemplateLocked reports whether the activity's template choice is locked.
// A published certificate locks the choice even without a saved lock.
func IsTemplateLocked(ctx context.Context, q *dbgen.Queries, activity dbgen.Activity) (bool, error) {
	if activityLock(activity) != nil {
		return true, nil
	}
	published, err := q.CountActiveIssuedCertificates(ctx, activity.ID)
	return published > 0, err
}

func (s Issuance) templateLock(ctx context.Context, q *dbgen.Queries, activity dbgen.Activity) (TemplateLock, error) {
	var out TemplateLock
	if templateID := activityTemplateIdentifier(activity); templateID != nil {
		template, err := q.CertificateTemplateByIdentifier(ctx, *templateID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return out, err
		}
		if err == nil {
			out.Template = &RecipientTemplate{ID: template.ID, Name: template.Name, Version: template.Version, Status: template.LifecycleStatus, Readiness: CheckReadinessValues(template.ID, template.Name, template.TemplateData)}
		}
	}
	var err error
	out.PublishedCount, err = q.CountActiveIssuedCertificates(ctx, activity.ID)
	if err != nil {
		return out, err
	}
	if lock := activityLock(activity); lock != nil {
		out.Locked = true
		if at, err := time.Parse(time.RFC3339Nano, lock.LockedAt); err == nil {
			value := ISO(at, s.Location)
			out.LockedAt = &value
		}
	}
	out.Locked = out.Locked || out.PublishedCount > 0
	return out, nil
}

func (s Issuance) TemplateLock(ctx context.Context, id int32) (TemplateLock, error) {
	q := dbgen.New(s.Pool)
	activity, err := q.CertificateActivityByIdentifier(ctx, database.JSNumber(float64(id)))
	if errors.Is(err, pgx.ErrNoRows) {
		return TemplateLock{}, Error("ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return TemplateLock{}, err
	}
	return s.templateLock(ctx, q, activity)
}

// SetTemplateLock locks or releases the activity's template choice. The
// activity row lock serializes this with assignment and issuance.
func (s Issuance) SetTemplateLock(ctx context.Context, id int32, locked bool) (TemplateLock, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return TemplateLock{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	identifier := database.JSNumber(float64(id))
	activity, err := q.LockCertificateActivityByIdentifier(ctx, identifier)
	if errors.Is(err, pgx.ErrNoRows) {
		return TemplateLock{}, Error("ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return TemplateLock{}, err
	}
	if locked {
		templateID := activityTemplateIdentifier(activity)
		if templateID == nil {
			return TemplateLock{}, Error("NO_CERTIFICATE_TEMPLATE")
		}
		template, err := q.CertificateTemplateByIdentifier(ctx, *templateID)
		if errors.Is(err, pgx.ErrNoRows) {
			return TemplateLock{}, Error("CERTIFICATE_TEMPLATE_NOT_FOUND")
		}
		if err != nil {
			return TemplateLock{}, err
		}
		if readiness := CheckReadinessValues(template.ID, template.Name, template.TemplateData); template.LifecycleStatus != "published" || !readiness.Ready {
			return TemplateLock{}, Error("CERTIFICATE_TEMPLATE_NOT_READY", readiness.Errors)
		}
		if err = q.LockActivityCertificateTemplate(ctx, activity.ID); err != nil {
			return TemplateLock{}, err
		}
	} else {
		published, err := q.CountActiveIssuedCertificates(ctx, activity.ID)
		if err != nil {
			return TemplateLock{}, err
		}
		if published > 0 {
			return TemplateLock{}, domain.Fail(409, "CERTIFICATE_TEMPLATE_LOCK_IN_USE")
		}
		if err = q.UnlockActivityCertificateTemplate(ctx, activity.ID); err != nil {
			return TemplateLock{}, err
		}
	}
	if activity, err = q.CertificateActivityByIdentifier(ctx, identifier); err != nil {
		return TemplateLock{}, err
	}
	out, err := s.templateLock(ctx, q, activity)
	if err != nil {
		return TemplateLock{}, err
	}
	return out, tx.Commit(ctx)
}
