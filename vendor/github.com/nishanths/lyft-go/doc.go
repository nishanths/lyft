// Package lyft provides a client for Lyft's v1 HTTP API. Along with its subpackages, it
// supports authentication, webhooks, Lyft's debug headers, and most endpoints. Lyft's
// API reference is available at https://developer.lyft.com/v1/docs/overview.
//
// Errors
//
// When the HTTP roundtrip succeeds but there was an application-level error,
// the error from will be of type *StatusError (and sometimes, another more
// specific type documented per-method). The error can be inspected for more
// details on what went wrong.
//
// Response Header and Request-ID
//
// Methods on the client typically have a signature like:
//
//  func (c *Client) Foo() (T, http.Header, error)
//
// The returned header is the the HTTP response header. It is safe to access
// when the error is nil, of type *StatusError, or of a documented concrete
// error type.
//
// The returned header is useful for obtaining the rate limit header and the
// unique Request-ID header set by Lyft. For details, see
// https://developer.lyft.com/v1/docs/errors#section-detailed-information-on-error-codes.
//
// Miscellaneous formats
//
// According to http://petstore.swagger.io/?url=https://api.lyft.com/v1/spec#/,
// the format of the currency strings returned is ISO 4217.
//
// Usage
//
// This example shows how to obtain an access token and find the
// ride types available at a location.
//
//   // Obtain an access token using the two-legged or three-legged flows.
//   t, err := twoleg.GenerateToken(http.DefaultClient, lyft.BaseURL, os.Getenv("CLIENT_ID"), os.Getenv("CLIENT_SECRET"))
//   if err != nil {
//       log.Fatalf("error generating token: %s", err)
//   }
//
//   // Create a client.
//   c := lyft.NewClient(t.AccessToken)
//
//   // Make requests.
//   r, header, err := c.RideTypes(37.7, -122.2)
//   if err != nil {
//       log.Fatalf("error getting ride types: %s", err)
//   }
//   fmt.Printf("ride types: %+v\n", r)
//   fmt.Printf("Request-ID: %s\n", lyft.RequestID(header))
//
// Missing Features
//
// The package does not yet support the sandbox-specific routes and the ride rating route.
package lyft
