package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"googlemaps.github.io/maps"
)

func cmdRoute(args []string, flags Flags) {
	if len(args) == 0 {
		usage()
	}

	home := HOME()

	switch args[0] {
	case "add":
		cmdRouteAdd(args[1:], flags, home)
	case "remove":
		cmdRouteRemove(args[1:], home)
	case "show":
		cmdRouteShow(args[1:], home)
	default:
		usage()
	}
}

func cmdRouteAdd(args []string, flags Flags, home string) {
	// Whoops?
	if len(args) == 0 {
		log.Fatalf("must specify a <name> for the route to add")
	}
	name := args[0]

	if err := os.MkdirAll(filepath.Join(home, rootDir), permRootDir); err != nil {
		log.Fatalf("making .%s directory: %s", rootDir, err)
	}

	// Does the routes file exist?
	_, err := os.Stat(filepath.Join(home, rootDir, routesFile))
	if err != nil {
		if os.IsNotExist(err) {
			// Create an empty file. That way, the code below can have less
			// branching.
			if err := ioutil.WriteFile(filepath.Join(home, rootDir, routesFile), []byte("{}"), permFile); err != nil {
				log.Fatalf("creating routes.json: %s", err)
			}
		} else {
			log.Fatalf("stat routes.json: %s", err)
		}
	}

	// Parse the existing routes.
	b, err := ioutil.ReadFile(filepath.Join(home, rootDir, routesFile))
	if err != nil {
		log.Fatalf("reading routes.json: %s", err)
	}
	var existing map[string]Route
	if err := json.Unmarshal(b, &existing); err != nil {
		log.Fatalf("unmarshaling routes: %s", err)
	}

	// Reject if named route already exists.
	if _, ok := existing[name]; ok {
		log.Fatalf("route %q already exists; remove before re-adding", name)
	}

	startLoc, endLoc := interactiveRouteInput("Enter start location: ", "Enter end location (can be empty): ", geocodeClient)

	// Update the routes file with the new route.
	existing[name] = Route{Start: startLoc, End: endLoc}
	if err := writeRoutes(existing); err != nil {
		log.Fatalf("saving routes: %s", err)
	}

	// Print the added route.
	printRoute(startLoc, endLoc)
	os.Exit(0)
}

func printRoute(startLoc, endLoc *Location) {
	// Print the added route.
	w := standardTabWriter()
	fmt.Fprintf(w, "Start:\t%s\n", googleMapsURL(startLoc.Lat, startLoc.Lng))
	if startLoc.Address != "" {
		fmt.Fprintf(w, "\t%s\n", startLoc.Address)
	}
	if endLoc != nil {
		fmt.Fprintf(w, "End:\t%s\n", googleMapsURL(endLoc.Lat, endLoc.Lng))
		if endLoc.Address != "" {
			fmt.Fprintf(w, "\t%s\n", endLoc.Address)
		}
	}
	w.Flush()
}

// interactiveRouteInput gets the start and end location by interactive input.
// The end location is optional and can be nil.
func interactiveRouteInput(start, end string, client func() *maps.Client) (*Location, *Location) {
	startLoc, err := parseLocation(interactiveInput(start), client)
	if err != nil {
		log.Fatal(err)
	}

	var endLoc *Location
	str := interactiveInput(end)
	if str != "" {
		e, err := parseLocation(str, client)
		if err != nil {
			log.Fatal(err)
		}
		endLoc = &e
	}

	return &startLoc, endLoc
}

func cmdRouteRemove(args []string, home string) {
	if len(args) == 0 {
		log.Fatalf("must specify a <name> for the route to remove")
	}
	name := args[0]

	b, err := ioutil.ReadFile(filepath.Join(home, rootDir, routesFile))
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("no routes found? not making any changes.")
		}
		log.Fatalf("opening routes file: %s", err)
	}

	var existing map[string]Route
	if err := json.Unmarshal(b, &existing); err != nil {
		log.Fatalf("unmarshaling routes: %s", err)
	}
	_, ok := existing[name]
	if !ok {
		log.Fatalf("route %q not found; not making any changes.", name)
	}

	delete(existing, name)
	if err := writeRoutes(existing); err != nil {
		log.Fatalf("saving routes: %s", err)
	}
	os.Exit(0)
}

func cmdRouteShow(args []string, home string) {
	b, err := ioutil.ReadFile(filepath.Join(home, rootDir, routesFile))
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stdout, "No routes found? Add one using 'lyft route add <name>'.\n")
			os.Exit(0)
		}
		log.Fatalf("opening routes file: %s", err)
	}

	var routes map[string]Route
	if err := json.Unmarshal(b, &routes); err != nil {
		log.Fatalf("unmarshaling routes: %s", err)
	}

	// No name specified. Print all.
	if len(args) == 0 {
		// Print the raw JSON. We can rely on the fact that the object keys
		// will be sorted, because they would have been written in sorted
		// order initially.
		fmt.Fprintf(os.Stdout, "%s\n", b)
		os.Exit(0)
	}

	name := args[0]
	route, ok := routes[name]
	if !ok {
		log.Fatalf("route %q not found", name)
	}
	// Print it as JSON.
	data, err := json.MarshalIndent(route, "", " ")
	if err != nil {
		log.Fatalf("marshaling routes: %s", err)
	}
	fmt.Fprintf(os.Stdout, "%s\n", data)
	os.Exit(0)
}

func routeByName(name string) (Route, error) {
	routes, err := readRoutes()
	if err != nil {
		return Route{}, err
	}
	route, ok := routes[name]
	if !ok {
		return Route{}, fmt.Errorf("route %q not found", name)
	}
	return route, nil
}

func writeRoutes(m map[string]Route) error {
	home := HOME()
	if m == nil {
		m = map[string]Route{} // so that it marshals to: {}
	}
	contents, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(home, rootDir, routesFile), contents, permFile)
}

// readRoutes returns the existing routes or an empty, non-nil map
// if no routes exist yet.
func readRoutes() (map[string]Route, error) {
	home := HOME()

	b, err := ioutil.ReadFile(filepath.Join(home, rootDir, routesFile))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Route{}, nil
		}
		return nil, err
	}
	var routes map[string]Route
	err = json.Unmarshal(b, &routes)
	if err != nil {
		return nil, err
	}
	if routes == nil {
		// need to make sure a non-nil map is returned.
		return map[string]Route{}, nil
	}
	return routes, nil
}
