package validation

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
)

func orderedKeys(raw json.RawMessage) []string {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	if _, err := decoder.Token(); err != nil {
		panic(err)
	}
	keys := []string{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			panic(err)
		}
		keys = append(keys, key.(string))
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil {
			panic(err)
		}
	}
	index := func(key string) (uint64, bool) {
		n, err := strconv.ParseUint(key, 10, 32)
		return n, err == nil && n < 4294967295 && strconv.FormatUint(n, 10) == key
	}
	sort.SliceStable(keys, func(i, j int) bool {
		a, ai := index(keys[i])
		b, bi := index(keys[j])
		if ai != bi {
			return ai
		}
		return ai && a < b
	})
	return keys
}
