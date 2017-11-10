package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"go.avalanche.space/lyft-go"
	"go.avalanche.space/lyft-go/auth"
	"go.avalanche.space/lyft-go/auth/threeleg"
)

type Config struct {
	ClientID     string
	ClientSecret string
}

type Internal struct {
	ClientID     string
	ClientSecret string
	AccessToken  string
	RefreshToken string
	// TODO: we could save an extra trip by saving the expiry as well.
}

func (i Internal) matches(c Config) bool {
	return i.ClientID == c.ClientID && i.ClientSecret == c.ClientSecret
}

func readConfig() (c Config, err error) {
	cfg, err := ioutil.ReadFile(filepath.Join(HOME(), rootDir, configFile))
	if err != nil {
		if os.IsNotExist(err) {
			return c, fmt.Errorf("failed to find config.json at %s", filepath.Join(HOME(), rootDir, configFile))
		}
		return c, fmt.Errorf("reading config.json: %s", err)
	}
	if err := json.Unmarshal(cfg, &c); err != nil {
		return c, fmt.Errorf("unmarshaling config: %s", err)
	}
	return c, nil
}

func getInternal() Internal {
	c, err := readConfig()
	if err != nil {
		log.Fatal(err)
	}

	return ensureInternal(c)
}

func ensureInternal(c Config) Internal {
	var inter Internal
	internalFilepath := filepath.Join(HOME(), rootDir, internalFile)
	b, fileErr := ioutil.ReadFile(internalFilepath)

	if fileErr == nil {
		// Good. It's there.
		if err := json.Unmarshal(b, &inter); err != nil {
			log.Fatalf("unmarshaling internal config: %s", err)
		}

		// Still in sync, hopefully?
		if inter.matches(c) {
			// It is still in sync. We're done.
			return inter
		}
		// Out of sync. Let's revoke the tokens here, before we
		// end up razing the file in the upcoming steps.
		revokeToken(c.ClientID, c.ClientSecret, inter.AccessToken)
	}

	// At this point, we failed to read internal file (does not exist, permissions, etc.),
	// or it was out of sync. Either way, we need new content in the internal file.
	// First, let's remove the file.
	if !os.IsNotExist(fileErr) {
		if err := os.Remove(internalFilepath); err != nil {
			log.Fatalf("removing internal file: %s", err)
		}
	}

	// Try to obtain the access and refresh tokens.
	code := obtainAuthorizationCode(c)
	t, _, err := threeleg.GenerateToken(http.DefaultClient, lyft.BaseURL, c.ClientID, c.ClientSecret, code)
	if err != nil {
		log.Fatalf("generating access token: %s", err)
	}

	inter = Internal{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
	}
	data, err := json.Marshal(inter)
	if err != nil {
		revokeToken(c.ClientID, c.ClientSecret, inter.AccessToken)
		log.Fatalf("marshaling internal config: %s", err)
	}
	if err := os.MkdirAll(filepath.Join(HOME(), rootDir), permRootDir); err != nil {
		revokeToken(c.ClientID, c.ClientSecret, inter.AccessToken)
		log.Fatalf("making .%s directory: %s", rootDir, err)
	}
	if err := ioutil.WriteFile(internalFilepath, data, permFile); err != nil {
		revokeToken(c.ClientID, c.ClientSecret, inter.AccessToken)
		log.Fatalf("writing internal file: %s", err)
	}
	return inter
}

func refreshAndWriteToken(inter Internal) (accessToken string) {
	refreshed, _, err := threeleg.RefreshToken(http.DefaultClient, lyft.BaseURL, inter.ClientID, inter.ClientSecret, inter.RefreshToken)
	if err != nil {
		log.Fatalf("refreshing expired token: %s", err)
	}
	data, err := json.Marshal(inter)
	if err == nil {
		ioutil.WriteFile(filepath.Join(HOME(), rootDir, internalFile), data, permFile) // ignore error, we have the access token in-memory for now
	}
	return refreshed.AccessToken
}

func revokeToken(clientID, clientSecret, a string) (http.Header, error) {
	return threeleg.RevokeToken(http.DefaultClient, lyft.BaseURL, clientID, clientSecret, a)
}

func obtainAuthorizationCode(c Config) string {
	u := threeleg.AuthorizationURL(c.ClientID, []string{auth.Public, auth.RidesRead, auth.Offline, auth.RidesRequest}, "")

	fmt.Fprintf(os.Stdout, "Go to the following link in your browser and click the Accept button: %s\n", u)
	time.Sleep(1 * time.Second) // waiting helps make the successive prompt more understandable.
	link := interactiveInput("Copy and paste the URL you were redirected to: ")

	k, err := url.Parse(link)
	if err != nil {
		log.Fatalf("failed to parse entered URL: %s", err)
	}
	code := k.Query().Get("code")
	if code == "" {
		log.Fatalf("failed to get authorization code; did you enter the correct URL?")
	}
	return code
}
