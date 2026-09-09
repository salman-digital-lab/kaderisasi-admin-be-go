// Command inventory reports handler coverage without opening a database or port.
package main

import (
	"encoding/json"
	"kaderisasi/admin/internal/httpapi"
	"os"
)

func main() {
	server := &httpapi.Server{}
	server.Handler()
	if err := json.NewEncoder(os.Stdout).Encode(server.ImplementationInventory()); err != nil {
		panic(err)
	}
}
