// Package webhook provides types and utility functions for handling
// Lyft webhooks.
package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/nishanths/lyft-go"
)

// Event types.
const (
	RideStatusUpdated = "ride.status.updated"
	RideReceiptReady  = "ride.receipt.ready"
)

const SandboxEventPrefix = "sandboxevent"

// Event represents an event from a Lyft webhook.
// It implements json.Unmarshaler in a manner suitable for decoding
// incoming webhook request bodies.
type Event struct {
	EventID   string
	URL       string
	Occurred  time.Time
	EventType string
	// Some fields may not be set.
	// See details for each event type: https://developer.lyft.com/v1/docs/webhooks
	Detail lyft.RideDetail
}

func (e *Event) IsSandbox() bool {
	return strings.HasPrefix(e.EventID, SandboxEventPrefix)
}

func (e *Event) UnmarshalJSON(p []byte) error {
	type event struct {
		EventID   string          `json:"event_id"`
		URL       string          `json:"href"`
		Occurred  string          `json:"occurred_at"`
		EventType string          `json:"event_type"`
		Detail    lyft.RideDetail `json:"event"`
	}
	var aux event
	if err := json.Unmarshal(p, &aux); err != nil {
		return err
	}
	e.EventID = aux.EventID
	e.URL = aux.URL
	if aux.Occurred != "" {
		o, err := time.Parse(lyft.TimeLayout, aux.Occurred)
		if err != nil {
			return err
		}
		e.Occurred = o
	}
	e.EventID = aux.EventType
	e.Detail = aux.Detail
	return nil
}

// Signature returns the value of "X-Lyft-Signature" from an incoming
// webhook request header. The "sha256=" prefix will have been trimmed
// in the returned string.
func Signature(h http.Header) string {
	return strings.TrimPrefix(h.Get("X-Lyft-Signature"), "sha256=")
}

// Verify checks whether the incoming webhook request originated from
// Lyft. The signature is the value of the X-Lyft-Signature header value
// with the "sha256=" prefix already trimmed. The verification token
// can be be found in the Lyft Developer Portal.
func Verify(requestBody, signature, verificationToken []byte) (bool, error) {
	mac := hmac.New(sha256.New, verificationToken)
	_, err := mac.Write(requestBody)
	if err != nil {
		return false, err
	}
	expectedMAC := mac.Sum(nil)

	// Base64 encode the body MAC.
	var buf bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &buf)
	_, err = enc.Write(expectedMAC)
	if err != nil {
		return false, err
	}
	err = enc.Close()
	if err != nil {
		return false, err
	}

	return hmac.Equal(buf.Bytes(), signature), nil
}

// ErrVerify is returned by DecodeEvent if the request could not be
// verified to have been originating from Lyft.
var ErrVerify = errors.New("failed to verify request")

// DecodeEvent decodes an incoming webhook request's body and header
// into an Event. DecodeEvent will verify that the request was received from
// Lyft. The error will be ErrVerify if verification fails.
// The request body will always be drained and closed, even if an error is returned.
// Errors from draining and closing are not reported.
func DecodeEvent(requestBody io.ReadCloser, h http.Header, verificationToken []byte) (Event, error) {
	defer drainAndClose(requestBody)
	var e Event

	// Create copies of required readers.
	var verifyBuf bytes.Buffer // copy of buffer to use in verification
	if _, err := verifyBuf.ReadFrom(requestBody); err != nil {
		return e, err
	}
	decodeBuf := bytes.NewBuffer(verifyBuf.Bytes()) // for JSON decoding Event

	if ok, err := Verify(verifyBuf.Bytes(), []byte(Signature(h)), verificationToken); err != nil {
		return e, err // generic internal error
	} else if !ok {
		return e, ErrVerify // verification failed
	}

	return e, unmarshal(decodeBuf, &e)
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
