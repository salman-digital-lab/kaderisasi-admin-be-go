package storage

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type testHTTPClient func(*http.Request) (*http.Response, error)

func (client testHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return client(request)
}

func TestStorageDNSRetry(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		failures  int
		canceled  bool
		attempts  int
		delivered int
	}{
		{"transient lookup", 1, false, 2, 1},
		{"persistent lookup", 10, false, 3, 0},
		{"cancellation", 10, true, 0, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var delivered atomic.Int32
			endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				delivered.Add(1)
				body, err := io.ReadAll(r.Body)
				if err != nil || string(body) != "synthetic content" {
					t.Errorf("retried upload body: %q %v", body, err)
				}
				w.WriteHeader(200)
			}))
			defer endpoint.Close()
			attempts := 0
			transport := testHTTPClient(func(request *http.Request) (*http.Response, error) {
				attempts++
				if attempts <= scenario.failures {
					return nil, &net.DNSError{Name: "storage.example.test", Err: "no such host", IsNotFound: true}
				}
				return http.DefaultClient.Do(request)
			})
			client := s3.New(s3.Options{Region: "fixture", BaseEndpoint: aws.String(endpoint.URL), UsePathStyle: true, Credentials: credentials.NewStaticCredentialsProvider("synthetic", "synthetic", ""), HTTPClient: transport, Retryer: retry.AddWithMaxBackoffDelay(storageRetryer(), time.Millisecond)})
			store := S3{Client: client, Bucket: "fixture"}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario.canceled {
				cancel()
			}
			err := store.Put(ctx, "fixture/content.txt", []byte("synthetic content"), "text/plain")
			if scenario.delivered > 0 && err != nil {
				t.Fatal(err)
			}
			if scenario.delivered == 0 && err == nil {
				t.Fatal("failure was hidden")
			}
			if scenario.canceled && !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost: %v", err)
			}
			if scenario.delivered == 0 && !scenario.canceled {
				var lookup *net.DNSError
				if !errors.As(err, &lookup) || !lookup.IsNotFound {
					t.Fatalf("lookup failure lost: %v", err)
				}
			}
			if attempts != scenario.attempts || int(delivered.Load()) != scenario.delivered {
				t.Fatalf("attempts=%d delivered=%d", attempts, delivered.Load())
			}
		})
	}
}
