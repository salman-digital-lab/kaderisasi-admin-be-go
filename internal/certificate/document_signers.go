package certificate

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"unicode/utf8"
)

// DocumentSigner is a certificate identity, independent of administrator accounts.
type DocumentSigner struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

var defaultDocumentSigners = []DocumentSigner{{Key: "oktofa-yudha-sudrajad", Name: "Oktofa Yudha Sudrajad, S.T., M.S.M., Ph.D.", Title: "Ketua Bidang Mahasiswa, Kaderisasi, dan Alumni"}}

var (
	signersMu         sync.RWMutex
	configuredSigners = defaultDocumentSigners
)

// ParseDocumentSigners validates a JSON signer catalog. The first entry signs direct publications.
func ParseDocumentSigners(raw string) ([]DocumentSigner, error) {
	var signers []DocumentSigner
	if err := json.Unmarshal([]byte(raw), &signers); err != nil || len(signers) == 0 {
		return nil, errors.New("CERTIFICATE_DOCUMENT_SIGNERS must be a non-empty JSON array")
	}
	seen := map[string]bool{}
	for i, signer := range signers {
		signer.Key, signer.Name, signer.Title = strings.TrimSpace(signer.Key), strings.TrimSpace(signer.Name), strings.TrimSpace(signer.Title)
		if signer.Key == "" || signer.Name == "" || signer.Title == "" || seen[signer.Key] || utf8.RuneCountInString(signer.Title) > 120 || utf8.RuneCountInString(signer.Name) > 255 {
			return nil, errors.New("CERTIFICATE_DOCUMENT_SIGNERS entries need unique key, name, and title")
		}
		seen[signer.Key] = true
		signers[i] = signer
	}
	return signers, nil
}

// ConfigureDocumentSigners replaces the signer catalog. An empty list restores the default.
func ConfigureDocumentSigners(signers []DocumentSigner) {
	signersMu.Lock()
	defer signersMu.Unlock()
	if len(signers) == 0 {
		configuredSigners = defaultDocumentSigners
		return
	}
	configuredSigners = append([]DocumentSigner(nil), signers...)
}

func DocumentSigners() []DocumentSigner {
	signersMu.RLock()
	defer signersMu.RUnlock()
	return append([]DocumentSigner(nil), configuredSigners...)
}

// DefaultDocumentSigner signs certificates published directly by an administrator.
func DefaultDocumentSigner() *DocumentSigner {
	signer := DocumentSigners()[0]
	return &signer
}

func documentSigner(key string) *DocumentSigner {
	for _, signer := range DocumentSigners() {
		if signer.Key == key {
			return &signer
		}
	}
	return nil
}
