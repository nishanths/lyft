// Package auth and its subpackages define types and functions
// related to Lyft's OAuth flows.
package auth // import "go.avalanche.space/lyft-go/auth"

// Scopes.
const (
	Public       = "public"
	RidesRead    = "rides.read"
	Offline      = "offline"
	RidesRequest = "rides.request"
	Profile      = "profile"
)

func AllScopes() []string {
	return []string{Public, RidesRead, Offline, RidesRequest, Profile}
}

// SandboxSecret returns the sandboxed form of an non-sandboxed client secret.
// See https://developer.lyft.com/v1/docs/sandbox.
func SandboxSecret(clientSecret string) string {
	return "SANDBOX-" + clientSecret
}
