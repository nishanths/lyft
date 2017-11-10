package lyft // import "go.avalanche.space/lyft-go"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"strconv"
	"time"
)

// BaseURL is the base URL for Lyft's HTTP API.
const BaseURL = "https://api.lyft.com"

const TimeLayout = time.RFC3339

// Client is a client for the Lyft API.
// AccessToken must be set for a client to be ready to use. The rest of the
// fields are optional. Methods are goroutine safe, unless the
// client's fields are being modified at the same time.
type Client struct {
	AccessToken string
	// The following fields are optional.
	HTTPClient *http.Client // Uses http.DefaultClient if nil.
	Header     http.Header  // Extra request header to add.
	BaseURL    string       // The base URL of the API; uses the package-level BaseURL if empty. Useful in tests.
	debug      bool         // Dump requests/responses using package log's default logger.
}

func (c *Client) base() string {
	if c.BaseURL == "" {
		return BaseURL
	}
	return c.BaseURL
}

func (c *Client) do(r *http.Request) (*http.Response, error) {
	// Set up headers and add credentials.
	c.addHeader(r.Header)
	c.authorize(r.Header)

	// Determine the HTTP client to use.
	client := http.DefaultClient
	if c.HTTPClient != nil {
		client = c.HTTPClient
	}

	if c.debug {
		dump, err := httputil.DumpRequestOut(r, true)
		if err != nil {
			log.Printf("error dumping request: %s", err)
		} else {
			log.Printf("%s", dump)
		}
	}

	// Do the request.
	rsp, err := client.Do(r)

	if c.debug {
		dump, err := httputil.DumpResponse(rsp, true)
		if err != nil {
			log.Printf("error dumping response: %s", err)
		} else {
			log.Printf("%s", dump)
		}
	}

	return rsp, err
}

// addHeader adds the key/values in c.Header to h.
func (c *Client) addHeader(h http.Header) {
	for key, values := range c.Header {
		for _, v := range values {
			h.Add(key, v)
		}
	}
}

// authorize modifies the header to include the access token
// in the Authorization field, as expected by the Lyft API. Useful when
// constructing a request manually.
func (c *Client) authorize(h http.Header) {
	h.Add("Authorization", "Bearer "+c.AccessToken)
}

// Possible values for the Reason field in StatusError.
const (
	InvalidToken         = "invalid_token"
	TokenExpired         = "token_expired"
	InsufficientScope    = "insufficient_scope"
	UnsupportedGrantType = "unsupported_grant_type"
)

type ErrorInfo struct {
	Reason      string
	Details     []map[string]string
	Description string
}

func newErrorInfo(body io.Reader, h http.Header) ErrorInfo {
	var lyftErr lyftError
	decodeErr := unmarshal(body, &lyftErr)

	// Determine the value for the Reason field; from the header
	// otherwise from the body.
	var e string
	v := h["error"] // non-canonical
	if len(v) != 0 {
		e = v[0]
	} else if decodeErr == nil {
		e = lyftErr.Slug
	}

	// The Details and Description fields.
	var det []map[string]string
	var desc string
	if decodeErr == nil {
		det = lyftErr.Details
		desc = lyftErr.Description
	}

	return ErrorInfo{
		Reason:      e,
		Details:     det,
		Description: desc,
	}
}

var _ error = (*StatusError)(nil)

// StatusError is returned when the HTTP roundtrip succeeded, but there
// was error was indicated via the HTTP status code, typically due to an
// application-level error.
type StatusError struct {
	StatusCode   int
	ResponseBody bytes.Buffer
	ErrorInfo    // Fields may be empty
}

// NewStatusError is not meant for external use. It exists solely so that subpackages
// (such as package auth) can create a StatusError in a canonical way.
func NewStatusError(rsp *http.Response) *StatusError {
	return newStatusError(rsp)
}

// newStatusError constructs a StatusError from the response.
// Does not close rsp.Body.
//
// newStatusError should assume that rsp.Body may be drained subsequently,
// so it must copy rsp.Body if necessary. It is allowed to drain the
// incoming rsp.Body.
func newStatusError(rsp *http.Response) *StatusError {
	var buf bytes.Buffer // for the StatusError's ResponseBody field
	buf.ReadFrom(rsp.Body)
	buf2 := bytes.NewBuffer(buf.Bytes()) // another buffer for newErrorInfo to use.
	return &StatusError{
		StatusCode:   rsp.StatusCode,
		ResponseBody: buf,
		ErrorInfo:    newErrorInfo(buf2, rsp.Header),
	}
}

func (s *StatusError) Error() string {
	if s.Reason != "" {
		return fmt.Sprintf("%s: status code=%d", s.Reason, s.StatusCode)
	}
	return fmt.Sprintf("status code=%d", s.StatusCode)
}

// See https://developer.lyft.com/v1/docs/errors.
type lyftError struct {
	Slug        string              `json:"error"`
	Details     []map[string]string `json:"error_detail"`
	Description string              `json:"error_description"`
}

// IsRateLimit returns whether the error arose because of running into a
// rate limit.
func IsRateLimit(err error) bool {
	if se, ok := err.(*StatusError); ok {
		return se.StatusCode == 429
	}
	return false
}

// IsTokenExpired returns true if the error arose because the access token
// expired.
func IsTokenExpired(err error) bool {
	if se, ok := err.(*StatusError); ok {
		// https://developer.lyft.com/v1/docs/authentication#section-http-status-codes
		// There doesn't seem to be a canonical way?
		return (se.StatusCode == 401 && len(se.ResponseBody.Bytes()) == 0) || se.Reason == TokenExpired
	}
	return false
}

// RequestID gets the value of the Request-ID key from a response header.
func RequestID(h http.Header) string {
	return h.Get("Request-ID")
}

// RateRemaining returns the value of X-Ratelimit-Remaining.
func RateRemaining(h http.Header) (n int, ok bool) {
	return intHeaderValue(h, "X-Ratelimit-Remaining")
}

// RateRemaining returns the value of X-Ratelimit-Limit.
func RateLimit(h http.Header) (n int, ok bool) {
	return intHeaderValue(h, "X-Ratelimit-Limit")
}

func intHeaderValue(h http.Header, k string) (int, bool) {
	vals, ok := h[k]
	if !ok || len(vals) == 0 {
		return 0, false
	}
	i, err := strconv.Atoi(vals[0])
	if err != nil {
		return 0, false
	}
	return i, true
}

func drainAndClose(r io.ReadCloser) {
	io.Copy(ioutil.Discard, r)
	r.Close()
}

func unmarshal(r io.Reader, v interface{}) error {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
