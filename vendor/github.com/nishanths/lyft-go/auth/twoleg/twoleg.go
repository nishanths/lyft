// Package twoleg provides functions for working with the two-legged
// OAuth flow described at https://developer.lyft.com/v1/docs/authentication#section-client-credentials-2-legged-flow-for-public-endpoints.
package twoleg

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/nishanths/lyft-go"
)

// Token is returned by GenerateToken.
type Token struct {
	AccessToken string
	TokenType   string
	Expires     time.Duration
	Scopes      []string
}

type token struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Expires     int64  `json:"expires_in"` // seconds
	Scopes      string `json:"scope"`      // space delimited
}

// GenerateToken creates a new access token.
// The access token returned can be used in lyft.Client.
// baseURL is typically lyft.BaseURL.
func GenerateToken(c *http.Client, baseURL, clientID, clientSecret string) (Token, http.Header, error) {
	const body = `{"grant_type": "client_credentials", "scope": "public"}`
	r, err := http.NewRequest("POST", baseURL+"/oauth/token", strings.NewReader(body))
	if err != nil {
		return Token{}, nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	r.SetBasicAuth(clientID, clientSecret)

	rsp, err := c.Do(r)
	if err != nil {
		return Token{}, nil, err
	}
	defer drainAndClose(rsp.Body)

	if rsp.StatusCode != 200 {
		return Token{}, rsp.Header, lyft.NewStatusError(rsp)
	}

	var g token
	if err := unmarshal(rsp.Body, &g); err != nil {
		return Token{}, rsp.Header, err
	}
	return Token{
		AccessToken: g.AccessToken,
		TokenType:   g.TokenType,
		Expires:     time.Second * time.Duration(g.Expires),
		Scopes:      strings.Fields(g.Scopes),
	}, rsp.Header, nil
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
