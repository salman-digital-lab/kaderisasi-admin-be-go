package storage

import (
	"encoding/json"
	"os"
	"sync"
)

// Journal records attempted writes before S3 receives them, including copy
// destinations whose requests later fail. The harness owns and cleans this file.
func Journal(path string) (func(string) error, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var keys []string
	if err = json.Unmarshal(raw, &keys); err != nil {
		return nil, err
	}
	var mutex sync.Mutex
	return func(key string) error {
		mutex.Lock()
		defer mutex.Unlock()
		keys = append(keys, key)
		raw, err := json.Marshal(keys)
		if err != nil {
			return err
		}
		if err = os.WriteFile(path+".pending", raw, 0600); err != nil {
			return err
		}
		return os.Rename(path+".pending", path)
	}, nil
}
