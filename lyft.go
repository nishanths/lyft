/*
Command lyft can request and manage Lyft rides from the command line.

Usage

  lyft [flags] <ride|place> [args]

Flags

The command's optional flags are:

  -c <ride-type>  Ride type: line, lyft, premier, lux, or luxsuv (default line).
  -dry-run        Dry-run; don't actually create or modify rides (default false).
  -end <place>    Use saved place as the end location for the ride.
  -notify         Show desktop notifications (default false), macOS only.
  -start <place>  Use saved place as the start location for the ride.
  -watch          Watch ride status updates (default false).

Ride subcommand

The ride subcommand can create, cancel, and track the status of rides.

  lyft ride create
  lyft ride cancel <ride-id>
  lyft ride status <ride-id>

Place subcommand

The place subcommand can save ride start and end locations for future use,
so you don't have to enter full addresses each time you create a ride. If
a name isn't specified, the show subcommand prints all saved places.

  lyft place add    <name>
  lyft place remove <name>...
  lyft place show   [name]

Location input

When prompted to enter a start or an end location, the input can be in these two
formats.

  1. Latitude/longitude pair in the format "lat,lng"
  2. Street address

Setup

The program uses the following environment variables.

  GOOG_GEOCODE_KEY
  LYFT_CLIENT_ID
  LYFT_CLIENT_SECRET

GOOG_GEOCODE_KEY is the Google Maps Geocode API key used to geocode street
addresses. It can be obtained from:

  https://developers.google.com/maps/documentation/geocoding/get-api-key

The Lyft API keys can be obtained by following these steps.

  1. Sign in at https://www.lyft.com/developers/apps/new.
  2. Create a new app. Enter any values for the app name and description.
  3. Enter any unused local URL (e.g. http://localhost:90) for the Redirect URL.

(The first time you request a ride, the program will request authorization
to create rides on your behalf. Follow the instructions printed on screen.
You will only have to do this once.)

Notes

Only one passenger is supported for Lyft Line rides requested by the program.

You will receive notifications via your smartphone's Lyft app as you usual
for rides created by this program, so you can skip the -notify and -watch
flags if you wish.

The program stores program-relevant data in a directory named ".lyft" in the
user's home directory.
*/
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

const help = `usage: lyft [flags] <ride|place> [args...]

Flags

  -c <ride-type>  Ride type: line, lyft, premier, lux, or luxsuv (default line).
  -dry-run        Dry-run; don't actually create or modify rides (default false).
  -end <place>    Use saved place as the end location for the ride.
  -notify         Show desktop notifications (default false), macOS only.
  -start <place>  Use saved place as the start location for the ride.
  -watch          Watch ride status updates (default false).

The ride subcommand can create, cancel, and track the status of rides.

  lyft ride create
  lyft ride cancel <ride-id>
  lyft ride status <ride-id>

The place subcommand can save ride start and end locations for future use.

  lyft place add    <name>
  lyft place remove <name>...
  lyft place show   [name]

The program uses the following environment variables.

  GOOG_GEOCODE_KEY
  LYFT_CLIENT_ID
  LYFT_CLIENT_SECRET

See https://godoc.org/github.com/nishanths/lyft for details.
`

func usage() {
	fmt.Fprint(os.Stderr, help)
	os.Exit(2)
}

const (
	rootDir      = ".lyft"
	internalFile = "internal.json"
	placesFile   = "places.json"
)

const (
	permRootDir = 0740
	permDir     = 0750
	permFile    = 0640
)

func main() {
	log.SetFlags(0)

	car := flag.String("c", "line", "")
	startPlace := flag.String("start", "", "")
	endPlace := flag.String("end", "", "")
	notifications := flag.Bool("notify", false, "")
	dryRun := flag.Bool("dry-run", false, "")
	watch := flag.Bool("watch", false, "")

	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
	}

	flags := Flags{
		car:           *car,
		startPlace:    *startPlace,
		endPlace:      *endPlace,
		notifications: *notifications,
		dryRun:        *dryRun,
		watch:         *watch || *notifications,
	}

	switch args[0] {
	case "ride":
		cmdRide(args[1:], flags)
	case "place":
		cmdPlace(args[1:])
	default:
		usage()
	}
}

// Flags is the command line flags collected together to make it easy
// to pass around as a single argument.
type Flags struct {
	car           string
	startPlace    string
	endPlace      string
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

// Location is a latitude and longitude pair and an optional display
// street address.
type Location struct {
	Lat     float64
	Lng     float64
	Address string
}

// parseLocationInput attempts to parse str as as lat,lng pair
// or a street address. The maps client function is invoked
// only if str was not a lat,lng (and hence geocoding is required).
func parseLocationInput(str string, mapsc func() *maps.Client) (Location, error) {
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
	loc, err := locationFromStreetAddress(str, mapsc())
	if err != nil {
		return Location{}, fmt.Errorf("failed to determine coordinates for address %q: %s", "", err)
	}
	return loc, nil
}

// locationFromStreetAddress constructs a Location for the street address a.
// The returned Location's Address field may not be the same
// value as the supplied street address. It is typically a cleaned-up form.
func locationFromStreetAddress(a string, mapsc *maps.Client) (Location, error) {
	ctx := context.TODO()
	results, err := mapsc.Geocode(ctx, &maps.GeocodingRequest{Address: a})
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
	// NOTE: If this URL stops working, try:
	// https://maps.google.com/maps?q=24.197611,120.780512
	// https://stackoverflow.com/q/30544268/3309046
	return fmt.Sprintf("https://www.google.com/maps/place/%f,%f", lat, lng)
}

func standardTabWriter() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
}
