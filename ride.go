package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.avalanche.space/lyft-go"
)

func cmdRide(args []string, flags Flags) {
	if len(args) == 0 {
		usage()
	}

	switch args[0] {
	case "create":
		cmdRideCreate(args[1:], flags)
	case "cancel":
		cmdRideCancel(args[1:], flags)
	case "status":
		cmdRideStatus(args[1:], flags)
	default:
		usage()
	}
}

func cmdRideCreate(args []string, flags Flags) {
	inter := getInternal()
	lyftClient := lyft.Client{AccessToken: inter.AccessToken}

	var r Route
	if flags.routeName != "" {
		var err error
		r, err = routeByName(flags.routeName)
		if err != nil {
			log.Fatalf("route %q not found", flags.routeName)
		}
	} else {
		s, e := interactiveRouteInput(geocodeClient)
		r.Start = s
		r.End = e
		printRoute(s, e)
		fmt.Fprintln(os.Stdout)
	}

	req := lyft.RideRequest{
		Origin:   lyft.Location{Latitude: r.Start.Lat, Longitude: r.Start.Lng, Address: r.Start.Address},
		RideType: flags.rideType(),
	}
	if r.End != nil {
		req.Destination = lyft.Location{Latitude: r.End.Lat, Longitude: r.End.Lng, Address: r.End.Address}
	}

	if flags.dryRun {
		os.Exit(0)
	}

	created, _, err := lyftClient.RequestRide(req)
	if err != nil {
		if lyft.IsTokenExpired(err) {
			lyftClient.AccessToken = refreshAndWriteToken(inter)
			created, _, err = lyftClient.RequestRide(req)
		}
		if err != nil { // still an error?
			log.Fatalf("creating ride: %s", err)
		}
	}
	fmt.Fprintf(os.Stdout, "Created Ride ID: %s\n", created.RideID)

	if flags.watch {
		cmdRideStatus([]string{created.RideID}, flags) // kinda gross to call it like this.
	} else {
		os.Exit(0)
	}
}

func cmdRideCancel(args []string, flags Flags) {
	if len(args) == 0 {
		usage()
	}

	inter := getInternal()
	lyftClient := lyft.Client{AccessToken: inter.AccessToken}

	if flags.dryRun {
		os.Exit(0)
	}

	var cancelToken string
	for {
		_, err := lyftClient.CancelRide(args[0], cancelToken)
		if err == nil {
			os.Exit(0)
		}

		if ce, ok := err.(*lyft.CancelRideError); ok {
			input := interactiveInput(fmt.Sprintf("You will be charged %s%f for canceling. Continue? [Y/n]: ", ce.Currency, ce.Amount))
			if parseYes(input) {
				cancelToken = ce.Token
				continue
			}
			fmt.Fprintf(os.Stdout, "Not making any changes.")
			os.Exit(0)
		} else {
			if lyft.IsTokenExpired(err) {
				lyftClient.AccessToken = refreshAndWriteToken(inter)
				_, err = lyftClient.CancelRide(args[0], cancelToken)
				// TODO: this is a lot of copy-paste from above. consider reorganizing.
				if ce, ok := err.(*lyft.CancelRideError); ok {
					input := interactiveInput(fmt.Sprintf("You will be charged %s%f for canceling. Continue? [Y/n]: ", ce.Currency, ce.Amount))
					if parseYes(input) {
						cancelToken = ce.Token
						continue
					}
					fmt.Fprintf(os.Stdout, "Not making any changes.")
					os.Exit(0)
				}
			}
			log.Fatalf("failed to cancel ride %s: %s", args[0], err)
		}
	}
}

// Parses the string s as the value of a yes/no input.
// Defaults to 'yes' if it's unclear what was said.
func parseYes(s string) (yes bool) {
	switch strings.ToLower(s) {
	case "no", "n":
		return false
	default:
		// empty string, "yes", "y", anything else.
		return true
	}
}

// Parses the string s as the value of a yes/no input.
// Defaults to 'no' if it's unclear what was said.
func parseNo(s string) (no bool) {
	switch strings.ToLower(s) {
	case "yes", "y":
		return false
	default:
		// empty string, "no", "n", anything else.
		return true
	}
}

func cmdRideStatus(args []string, flags Flags) {
	if len(args) == 0 {
		usage()
	}

	inter := getInternal()
	lyftClient := lyft.Client{AccessToken: inter.AccessToken}

	detail, _, err := lyftClient.RideDetail(args[0])
	if err != nil {
		if lyft.IsTokenExpired(err) {
			lyftClient.AccessToken = refreshAndWriteToken(inter)
			detail, _, err = lyftClient.RideDetail(args[0])
		}
		if err != nil { // still an error?
			log.Fatalf("fetching ride status: %s", err)
		}
	}

	loopSleep := 10 * time.Second
	notified := map[string]struct{}{}
	w := standardTabWriter()
	fmt.Fprintln(os.Stdout)
	fmt.Fprintf(w, "Ride ID:\t%s\n", detail.RideID)
	fmt.Fprintf(w, "Ride Type:\t%s\n", lyft.RideTypeDisplay(detail.RideType))

	// None of this is expected to run into the rate limit.

	for {
		// Print info.
		fmt.Fprintf(w, "Status:\t%s\n", lyft.RideStatusDisplay(detail.RideStatus))
		switch detail.RideStatus {
		case lyft.StatusPending:
			orig, dest := detail.Origin, detail.Destination
			fmt.Fprintf(w, "Origin:\t%s\n", googleMapsURL(orig.Latitude, orig.Longitude))
			if orig.Address != "" {
				fmt.Fprintf(w, "\t%s\n", orig.Address)
			}
			fmt.Fprintf(w, "Destination:\t%s\n", googleMapsURL(dest.Latitude, dest.Longitude))
			if dest.Address != "" {
				fmt.Fprintf(w, "\t%s\n", dest.Address)
			}
		case lyft.StatusAccepted, lyft.StatusArrived:
			orig, dest := detail.Origin, detail.Destination
			fmt.Fprintf(w, "Origin:\t%s\n", googleMapsURL(orig.Latitude, orig.Longitude))
			if orig.Address != "" {
				fmt.Fprintf(w, "\t%s (ETA=%s)\n", orig.Address, orig.ETA)
			}
			fmt.Fprintf(w, "Destination:\t%s\n", googleMapsURL(dest.Latitude, dest.Longitude))
			if dest.Address != "" {
				fmt.Fprintf(w, "\t%s (ETA=%s)\n", dest.Address, dest.ETA)
			}
			fmt.Fprintf(w, "Location:\t%s\n", googleMapsURL(detail.Location.Latitude, detail.Location.Longitude))
			fmt.Fprintf(w, "Driver:\t%s %s, %s\n", detail.Driver.FirstName, detail.Driver.LastName, detail.Driver.Rating)
			v := detail.Vehicle
			fmt.Fprintf(w, "Vehicle:\t%s %s %s\n", v.Color, v.Make, v.Model)
			fmt.Fprintf(w, "\t%s (%d)\n", v.LicensePlate, v.Year)
		case lyft.StatusCanceled:
			fmt.Fprintf(w, "Cancellation fee:\t%s%d\n", detail.CancellationPrice.Currency, detail.CancellationPrice.Amount)
			if detail.CanceledBy != "" {
				fmt.Fprintf(w, "Canceled by:\t%s\n", detail.CanceledBy)
			}

		case lyft.StatusUnknown, lyft.StatusPickedUp, lyft.StatusDroppedOff:
			// Nothing extra.
		}
		w.Flush()
		fmt.Fprintln(os.Stdout)

		// Notifications.
		if flags.notifications {
			switch detail.RideStatus {
			case lyft.StatusCanceled, lyft.StatusAccepted:
				// notify if we haven't already
				if _, ok := notified[detail.RideStatus]; !ok {
					notified[detail.RideStatus] = struct{}{}
					notify("", "Lyft "+lyft.RideStatusDisplay(detail.RideStatus), "")
				}
			case lyft.StatusArrived:
				// always notify
				notified[detail.RideStatus] = struct{}{}
				message := fmt.Sprintf("%s %s (%s)", detail.Vehicle.Color, detail.Vehicle.Make, detail.Vehicle.LicensePlate)
				notify(message, "Lyft "+lyft.RideStatusDisplay(detail.RideStatus), "")
			}
		}

		if flags.watch {
			// Set loop wait times / break.
			switch detail.RideStatus {
			case lyft.StatusPending:
				// nothing. keep going.
			case lyft.StatusAccepted:
				loopSleep = 30 * time.Second
				if detail.Origin.ETA != 0 && detail.Origin.ETA < 120*time.Second {
					loopSleep = 10 * time.Second
				}
			case lyft.StatusArrived, lyft.StatusCanceled, lyft.StatusUnknown, lyft.StatusPickedUp, lyft.StatusDroppedOff:
				break
			}
		} else {
			break
		}

		time.Sleep(loopSleep)

		// Update for next round.
		detail, _, err = lyftClient.RideDetail(args[0])
		if err != nil {
			if lyft.IsTokenExpired(err) {
				lyftClient.AccessToken = refreshAndWriteToken(inter)
				detail, _, err = lyftClient.RideDetail(args[0])
			}
			if err != nil { // still an error?
				log.Fatalf("fetching ride status: %s", err)
			}
		}
	}

	if flags.watch {
		var c chan struct{}
		<-c // infinite wait
	}

	os.Exit(0)
}
