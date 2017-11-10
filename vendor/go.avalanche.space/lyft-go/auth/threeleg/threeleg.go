// Package threeleg provides functions for working with the three-legged
// OAuth flow described at https://developer.lyft.com/v1/docs/authentication#section-3-legged-flow-for-accessing-user-specific-endpoints.
package threeleg // import "go.avalanche.space/lyft-go/auth/threeleg"

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.avalanche.space/lyft-go"
)

// AuthorizationURL constructs the URL that a user should be directed to, in order for the user
// to grant the list of permissions your application is requesting.
func AuthorizationURL(clientID string, scopes []string, state string) string {
	v := make(url.Values)
	v.Set("client_id", clientID)
	v.Set("response_type", "code") // only possible value
	v.Set("scope", strings.Join(scopes, " "))
	v.Set("state", state)
	return fmt.Sprintf("https://api.lyft.com/oauth/authorize?%s", v.Encode())
}

// AuthorizationCode retrieves the authorization code sent in the
// authorization redirect request from Lyft.
// If ReadForm hasn't been called on the request already, it will be
// called during the process.
func AuthorizationCode(r *http.Request) string {
	return r.FormValue("code")
}

// Token is returned by GenerateToken.
type Token struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Expires      time.Duration
	Scopes       []string
}

type token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Expires      int64  `json:"expires_in"` // seconds
	Scopes       string `json:"scope"`      // space delimited
}

// RefreshedToken is returned by RefreshToken.
type RefreshedToken struct {
	AccessToken string
	TokenType   string
	Expires     time.Duration
	Scopes      []string
}

type refreshedToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Expires     int64  `json:"expires_in"` // seconds
	Scopes      string `json:"scope"`      // space delimited
}

// GenerateToken creates a new access token using the authorization code
// obtained from Lyft's authorization redirect. The access token
// returned can be used in lyft.Client. baseURL is typically lyft.BaseURL.
func GenerateToken(c *http.Client, baseURL, clientID, clientSecret, code string) (Token, http.Header, error) {
	body := fmt.Sprintf(`{"grant_type": "authorization_code", "code": "%s"}`, code)
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
		AccessToken:  g.AccessToken,
		RefreshToken: g.RefreshToken,
		TokenType:    g.TokenType,
		Expires:      time.Second * time.Duration(g.Expires),
		Scopes:       strings.Fields(g.Scopes),
	}, rsp.Header, nil
}

// RefreshToken refreshes the access token associated with refreshToken.
// See Token for obtaining access/refresh token pairs.
// baseURL is typically lyft.BaseURL.
func RefreshToken(c *http.Client, baseURL, clientID, clientSecret, refreshToken string) (RefreshedToken, http.Header, error) {
	body := fmt.Sprintf(`{"grant_type": "refresh_token", "refresh_token": "%s"}`, refreshToken)
	r, err := http.NewRequest("POST", baseURL+"/oauth/token", strings.NewReader(body))
	if err != nil {
		return RefreshedToken{}, nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	r.SetBasicAuth(clientID, clientSecret)

	rsp, err := c.Do(r)
	if err != nil {
		return RefreshedToken{}, nil, err
	}
	defer drainAndClose(rsp.Body)

	if rsp.StatusCode != 200 {
		return RefreshedToken{}, rsp.Header, lyft.NewStatusError(rsp)
	}

	var ref refreshedToken
	if err := unmarshal(rsp.Body, &ref); err != nil {
		return RefreshedToken{}, rsp.Header, err
	}
	return RefreshedToken{
		AccessToken: ref.AccessToken,
		TokenType:   ref.TokenType,
		Expires:     time.Second * time.Duration(ref.Expires),
		Scopes:      strings.Fields(ref.Scopes),
	}, rsp.Header, nil
}

// RevokeToken revokes the supplied access token.
// baseURL is typically lyft.BaseURL.
func RevokeToken(c *http.Client, baseURL, clientID, clientSecret, accessToken string) (http.Header, error) {
	// NOTE: There is some disagreement on the naming of the params in the API
	// reference regrading refresh token vs. access token.
	body := fmt.Sprintf(`{"token": "%s"}`, accessToken)
	r, err := http.NewRequest("POST", baseURL+"/oauth/revoke_refresh_token", strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	r.Header.Set("Content-Type", "application/json")
	r.SetBasicAuth(clientID, clientSecret)

	rsp, err := c.Do(r)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(rsp.Body)

	if rsp.StatusCode != 200 {
		return rsp.Header, lyft.NewStatusError(rsp)
	}
	return rsp.Header, nil
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
