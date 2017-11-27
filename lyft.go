// Command lyft can request and manage Lyft rides from the command line.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/tabwriter"

	"github.com/nishanths/lyft-go"
	"googlemaps.github.io/maps"
)

// TODO: implement ride update <ride-id>

const help = `usage: lyft [flags] <ride|route>

Ride subcommand:
The ride subcommand can create, cancel, or track the status of rides.

  lyft ride create
  lyft ride cancel <ride-id>
  lyft ride status <ride-id>

Route subcommand:
The route subcommand lets you save routes for future use, so you don't have
to enter full locations each time you create a ride. If name isn't specified,
the show prints all saved routes.

  lyft route add    <name>
  lyft route remove <name>...
  lyft route show   [name]

Location input:
When prompted to enter a location, the input can be in either of these
two formats: (1) a latitude/longitude pair in the format "lat,lng"; or
(2) a street address.

Setup:
The program expects the following environment variables. GOOG_GEOCODE_KEY
is used as the Google Maps Geocode API key for gecoding street addresses to
latitudes/longitudes. It isn't required if you are not running commands that
perform geocoding. See README.md for instructions on obtaining these keys.

  export GOOG_GEOCODE_KEY=<key>
  export LYFT_CLIENT_ID=<key>
  export LYFT_CLIENT_SECRET=<key>

Flags:
  -c type    Ride type to order; one of: line, lyft, premier, lux, luxsuv (default line).
             Only 1 passenger is allowed for line rides requested by the program.
  -n         Dry-run; don't actually create or modify rides (default false).
  -r route   Use the named route for the ride.
  -t         Show desktop notifications for significant status updates (default false).
             Implies -w; supported on macOS only. If you use the Lyft app on your phone,
             you will still receive notifications on your phone as you usually would.
  -w         Continuously watch ride status (default false). Applicable only
             for 'route add' and 'route status'.
`

func usage() {
	fmt.Fprint(os.Stderr, help)
	os.Exit(2)
}

const (
	rootDir      = ".lyft"
	internalFile = "internal.json"
	routesFile   = "routes.json"
)

const (
	permRootDir = 0740
	permDir     = 0750
	permFile    = 0640
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("lyft: ")

	car := flag.String("c", "line", "")
	routeName := flag.String("r", "", "")
	notifications := flag.Bool("t", false, "")
	dryRun := flag.Bool("n", false, "")
	watch := flag.Bool("w", false, "")

	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
	}

	flags := Flags{
		car:           *car,
		routeName:     *routeName,
		notifications: *notifications,
		dryRun:        *dryRun,
		watch:         *watch || *notifications,
	}

	switch args[0] {
	case "ride":
		cmdRide(args[1:], flags)
	case "route":
		cmdRoute(args[1:], flags)
	default:
		usage()
	}
}

// Flags is the command line flags collected together to make it easy
// to pass around as a single argument.
type Flags struct {
	car           string
	routeName     string
	notifications bool
	dryRun        bool
	watch         bool
}

// rideType returns the ride type for the specified flag,
// or exits with a fatal error if the ride type is invalid.
func (f Flags) rideType() string {
	s := flagToRideType(f.car)
	if s == "" {
		log.Fatalf("unknown ride type %q", f.car)
	}
	return s
}

// flagToRideType returns the ride type for the flag argument,
// or an empty string if a matching ride type wasn't found.
func flagToRideType(r string) string {
	switch r {
	case "line":
		return lyft.RideTypeLine
	case "lyft":
		return lyft.RideTypeLyft
	case "premier":
		return lyft.RideTypePremier
	case "lux":
		return lyft.RideTypeLux
	case "luxsuv":
		return lyft.RideTypeLuxSUV
	}
	return ""
}

// parseLocation attempts to parse str as as lat,lng pair
// or a street address. The client function is invoked
// only if str was not a lat,lng (and hence geocoding is required).
func parseLocation(str string, client func() *maps.Client) (Location, error) {
	// Does it look like a lat,lng?
	// NOTE: we need to check that the length is at least 2, otherwise
	// maps.ParseLatLng panics internally due to an out of bounds access.
	if len(strings.Split(str, ",")) == 2 {
		// Let's try parsing. If it succeeds, it was a latitude/longitude.
		ll, err := maps.ParseLatLng(str)
		if err == nil {
			return Location{ll.Lat, ll.Lng, ""}, nil
		}
	}
	// OK, need to geocode street address.
	loc, err := locationForAddress(str, client())
	if err != nil {
		return Location{}, fmt.Errorf("failed to determine coordinates for address %q: %s", "", err)
	}
	return loc, nil
}

// Route is a path from the start location to the end location.
type Route struct {
	Start, End *Location
}

// Location is a latitude and longitude pair and an optional display
// street address.
type Location struct {
	Lat     float64
	Lng     float64
	Address string
}

// locationForAddress constructs a Location for the street address a.
// The returned Location's Address field may not be the same
// value as the supplied street address. It is typically a cleaned-up form.
func locationForAddress(a string, client *maps.Client) (Location, error) {
	ctx := context.Background()
	results, err := client.Geocode(ctx, &maps.GeocodingRequest{Address: a})
	if err != nil {
		return Location{}, err
	}
	if len(results) == 0 {
		// Can this happen? Wish they would document this; they literally
		// own both the HTTP API and this client, so it really isn't that hard.
		return Location{}, errors.New("zero results for address")
	}
	return Location{
		results[0].Geometry.Location.Lat,
		results[0].Geometry.Location.Lng,
		results[0].FormattedAddress,
	}, nil
}

// HOME returns the value of $HOME (or its equivalent on windows).
// Logs a fatal error if the home directory isn't set.
func HOME() string {
	var h string
	if runtime.GOOS == "windows" {
		h = os.Getenv("HOMEPATH")
	} else {
		h = os.Getenv("HOME")
	}
	if h == "" {
		log.Fatal("home directory not set")
	}
	return h
}

// notify displays a system notification with the supplied arguments, if the
// we know how to do so for the runtime operating system.
// All arguments are optional.
func notify(message, title, subtitle string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	// FWIW, this also has support for sound -- but we don't use sound here.
	var s string
	switch {
	case title != "" && subtitle != "":
		s = fmt.Sprintf(`display notification "%s" with title "%s" subtitle "%s"`, message, title, subtitle)
	case title != "":
		s = fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
	default:
		s = fmt.Sprintf(`display notification "%s"`, message)
	}
	return exec.Command("osascript", "-e", s).Run()
}

// interactiveInput scans one line of input from standard input,
// panics on error.
func interactiveInput(prompt string) string {
	fmt.Fprint(os.Stdout, prompt)
	str, err := scanLine()
	if err != nil {
		panic(err)
	}
	return str
}

func scanLine() (string, error) {
	sc := bufio.NewScanner(os.Stdin)
	sc.Scan()
	return sc.Text(), sc.Err()
}

// googleMapsURL returns a deep-link to a Google Map for the
// supplied lat,lng pair.
func googleMapsURL(lat, lng float64) string {
	// If this URL stops working, try:
	// https://maps.google.com/maps?q=24.197611,120.780512
	// https://stackoverflow.com/q/30544268/3309046
	return fmt.Sprintf("https://www.google.com/maps/place/%f,%f", lat, lng)
}

func standardTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
}
