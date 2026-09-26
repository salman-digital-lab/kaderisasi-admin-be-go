package certificate

// DocumentSigner is a certificate identity, independent of administrator accounts.
type DocumentSigner struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

func DocumentSigners() []DocumentSigner {
	return []DocumentSigner{{Key: "oktofa-yudha-sudrajad", Name: "Oktofa Yudha Sudrajad, S.T., M.S.M., Ph.D.", Title: "Ketua Bidang Mahasiswa, Kaderisasi, dan Alumni"}}
}

func documentSigner(key string) *DocumentSigner {
	for _, signer := range DocumentSigners() {
		if signer.Key == key {
			return &signer
		}
	}
	return nil
}
