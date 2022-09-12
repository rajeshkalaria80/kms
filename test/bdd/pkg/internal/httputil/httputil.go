/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package httputil

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/hyperledger/aries-framework-go/pkg/common/log"
)

const (
	contentType     = "Content-Type"
	applicationJSON = "application/json"
	authorization   = "Authorization"
)

var logger = log.New("kms-bdd")

// Response is an HTTP response.
type Response struct {
	Status       string
	StatusCode   int
	Body         []byte
	ErrorMessage string
}

// DoRequest makes an HTTP request.
func DoRequest(ctx context.Context, url string, opts ...Opt) (*Response, error) { //nolint:funlen
	op := &options{
		httpClient: http.DefaultClient,
		method:     http.MethodGet,
	}

	for _, fn := range opts {
		fn(op)
	}

	body, err := io.ReadAll(op.body)
	if err != nil {
		return nil, fmt.Errorf("request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, op.method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	req.Header.Add(contentType, applicationJSON)

	if op.gnapToken != "" {
		req.Header.Add(authorization, "GNAP "+op.gnapToken)
	}

	if op.signer != nil {
		if err = op.signer.Sign(req); err != nil {
			return nil, fmt.Errorf("sign http request: %w", err)
		}

		// recreate request with body again as Sign() above consumes the request Body reader.
		req.Body = io.NopCloser(bytes.NewReader(body))
	}

	resp, err := op.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http do: %w", err)
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Errorf("Failed to close response body: %s\n", closeErr.Error())
		}
	}()

	r := &Response{
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if len(body) > 0 {
		r.Body = body

		if resp.StatusCode != http.StatusOK {
			var errResp errorResponse

			if err = json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
				return nil, errors.New(errResp.Message)
			}

			return nil, errors.New(resp.Status)
		}
	}

	return r, nil
}

type errorResponse struct {
	Message string `json:"errMessage,omitempty"`
}

type requestSigner interface {
	Sign(req *http.Request) error
}

type options struct {
	httpClient     *http.Client
	method         string
	headers        []string
	body           io.Reader
	gnapToken      string
	signer         requestSigner
}

// Opt configures HTTP request options.
type Opt func(*options)

// WithHTTPClient specifies the custom HTTP client.
func WithHTTPClient(c *http.Client) Opt {
	return func(o *options) {
		o.httpClient = c
	}
}

// WithMethod specifies an HTTP method. Default is GET.
func WithMethod(method string) Opt {
	return func(o *options) {
		o.method = method
	}
}

// WithBody specifies HTTP request body.
func WithBody(val []byte) Opt {
	return func(o *options) {
		o.body = bytes.NewBuffer(val)
	}
}

// WithGNAPToken specifies an authorization GNAP token.
func WithGNAPToken(token string) Opt {
	return func(o *options) {
		o.gnapToken = token
	}
}

// WithSigner specifies a request signer for HTTP Signatures.
func WithSigner(signer requestSigner) Opt {
	return func(o *options) {
		o.signer = signer
	}
}
