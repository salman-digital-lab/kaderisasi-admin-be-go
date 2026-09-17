//go:build integration

package jobs

import (
	"context"
	"errors"
	"fmt"
	"kaderisasi/admin/internal/storage"
	"testing"
	"time"
)

type cleanupStore struct {
	storage.Store
	deleted []string
	fail    bool
}

func (s *cleanupStore) Delete(_ context.Context, key string) error {
	if s.fail {
		return errors.New("storage unavailable")
	}
	s.deleted = append(s.deleted, key)
	return nil
}

func TestFormUploadCleanupRetainsClaimedAndLiveAttachments(t *testing.T) {
	ctx := context.Background()
	pool := jobPool(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var formID int32
	if err = tx.QueryRow(ctx, `INSERT INTO custom_forms(form_name, feature_type) VALUES('Cleanup fixture','independent_form') RETURNING id`).Scan(&formID); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
		expiry := time.Now().Add(-time.Hour)
		if i == 2 {
			expiry = time.Now().Add(time.Hour)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO custom_form_sessions(id,form_id,token_hash,schema_hash,expires_at) VALUES($1,$2,$3,$3,$4)`, id, formID, fmt.Sprintf("%064d", i), expiry); err != nil {
			t.Fatal(err)
		}
		var claimed *time.Time
		if i == 3 {
			now := time.Now()
			claimed = &now
		}
		if _, err = tx.Exec(ctx, `INSERT INTO custom_form_attachments(id,session_id,field_key,storage_key,original_name,download_name,mime_type,size_bytes,source_size_bytes,claimed_at) VALUES($1,$1,'proof',$2,'proof.pdf','proof.pdf','application/pdf',100,100,$3)`, id, fmt.Sprintf("cleanup-fixture/%d", i), claimed); err != nil {
			t.Fatal(err)
		}
	}
	store := &cleanupStore{fail: true}
	runner := Runner{DB: tx, Storage: store}
	if _, err = runner.Run(ctx, "forms:clean-uploads", time.Now()); err == nil {
		t.Fatal("storage failure must retain the record")
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM custom_form_attachments WHERE storage_key LIKE 'cleanup-fixture/%'`).Scan(&count); err != nil || count != 3 {
		t.Fatal(count, err)
	}
	store.fail = false
	result, err := runner.Run(ctx, "forms:clean-uploads", time.Now())
	if err != nil || result.Count != 1 {
		t.Fatal(result, err)
	}
	if len(store.deleted) != 1 || store.deleted[0] != "cleanup-fixture/1" {
		t.Fatal(store.deleted)
	}
	result, err = runner.Run(ctx, "forms:clean-uploads", time.Now())
	if err != nil || result.Count != 0 {
		t.Fatal(result, err)
	}
}
