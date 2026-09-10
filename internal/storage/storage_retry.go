package storage

import (
	"errors"
	"net"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
)

func storageRetryer() aws.Retryer {
	return retry.NewStandard(func(options *retry.StandardOptions) {
		// The configured S3 resolver intermittently returns NXDOMAIN for a live
		// endpoint. DNS failure occurs before a request reaches storage, so a
		// bounded retry cannot duplicate a completed upload or copy.
		options.Retryables = append([]retry.IsErrorRetryable{
			retry.IsErrorRetryableFunc(func(err error) aws.Ternary {
				var lookup *net.DNSError
				if errors.As(err, &lookup) && lookup.IsNotFound {
					return aws.TrueTernary
				}
				return aws.UnknownTernary
			}),
		}, options.Retryables...)
	})
}
