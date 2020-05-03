package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func cmdPlace(args []string) {
	if len(args) == 0 {
		usage()
	}

	home := HOME()

	switch args[0] {
	case "add":
		cmdPlaceAdd(args[1:], home)
	case "remove":
		cmdPlaceRemove(args[1:], home)
	case "show":
		cmdPlaceShow(args[1:], home)
	default:
		usage()
	}
}

func cmdPlaceAdd(args []string, home string) {
	// Whoops?
	if len(args) == 0 {
		log.Fatalf("must specify a <name> for the place to add")
	}
	name := args[0]

	if err := os.MkdirAll(filepath.Join(home, rootDir), permRootDir); err != nil {
		log.Fatalf("making %s directory: %s", rootDir, err)
	}

	// Does the places file exist?
	_, err := os.Stat(filepath.Join(home, rootDir, placesFile))
	if err != nil {
		if os.IsNotExist(err) {
			// Create an empty file. That way, the code below can have less
			// branching.
			if err := ioutil.WriteFile(filepath.Join(home, rootDir, placesFile), []byte("{}"), permFile); err != nil {
				log.Fatalf("creating places.json: %s", err)
			}
		} else {
			log.Fatalf("stat places.json: %s", err)
		}
	}

	// Parse the existing places.
	b, err := ioutil.ReadFile(filepath.Join(home, rootDir, placesFile))
	if err != nil {
		log.Fatalf("reading places.json: %s", err)
	}
	var existing map[string]Location
	if err := json.Unmarshal(b, &existing); err != nil {
		log.Fatalf("unmarshaling places: %s", err)
	}

	// Reject if named place already exists.
	if _, ok := existing[name]; ok {
		log.Fatalf("place %q already exists; remove before re-adding", name)
	}

	loc, err := parseLocationInput(interactiveInput("Enter location (street address or lat,lng): "), mapsClient)
	if err != nil {
		log.Fatal(err)
	}

	// Update the places file with the new place.
	existing[name] = loc
	if err := writePlaces(existing); err != nil {
		log.Fatalf("saving place: %s", err)
	}

	// Print the added place.
	w := standardTabWriter()
	fmt.Fprintf(w, "Added:\t%s\n", googleMapsURL(loc.Lat, loc.Lng))
	if loc.Address != "" {
		fmt.Fprintf(w, "\t%s\n", loc.Address)
	}
	w.Flush()

	os.Exit(0)
}

func printRoute(startLoc, endLoc *Location) {
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

func cmdPlaceRemove(args []string, home string) {
	if len(args) == 0 {
		log.Fatalf("must specify a <name> for the place to remove")
	}

	b, err := ioutil.ReadFile(filepath.Join(home, rootDir, placesFile))
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("no places found? not making any changes.")
		}
		log.Fatalf("opening places file: %s", err)
	}

	var existing map[string]Location
	if err := json.Unmarshal(b, &existing); err != nil {
		log.Fatalf("unmarshaling places: %s", err)
	}

	for _, name := range args {
		_, ok := existing[name]
		if !ok {
			log.Fatalf("place %q not found; not making any changes.", name)
		}

		delete(existing, name)
	}

	if err := writePlaces(existing); err != nil {
		log.Fatalf("saving places: %s", err)
	}
	os.Exit(0)
}

func cmdPlaceShow(args []string, home string) {
	b, err := ioutil.ReadFile(filepath.Join(home, rootDir, placesFile))
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stdout, "no existing places. add one using 'lyft place add <name>'.\n")
			os.Exit(0)
		}
		log.Fatalf("opening places file: %s", err)
	}

	var places map[string]Location
	if err := json.Unmarshal(b, &places); err != nil {
		log.Fatalf("unmarshaling places: %s", err)
	}

	if len(places) == 0 {
		fmt.Fprintf(os.Stdout, "no existing places. add one using 'lyft place add <name>'.\n")
		os.Exit(0)
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
	place, ok := places[name]
	if !ok {
		log.Fatalf("place %q not found", name)
	}
	// Print it as JSON.
	data, err := json.MarshalIndent(place, "", " ")
	if err != nil {
		log.Fatalf("marshaling places: %s", err)
	}
	fmt.Fprintf(os.Stdout, "%s\n", data)
	os.Exit(0)
}

func placeByName(name string) (Location, error) {
	places, err := readPlaces()
	if err != nil {
		return Location{}, err
	}
	loc, ok := places[name]
	if !ok {
		return Location{}, fmt.Errorf("place %q not found", name)
	}
	return loc, nil
}

func writePlaces(m map[string]Location) error {
	home := HOME()
	if m == nil {
		m = map[string]Location{} // so that it marshals to: {}
	}
	contents, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(home, rootDir, placesFile), contents, permFile)
}

// readPlaces returns the existing places or an empty, non-nil map
// if no places exist yet.
func readPlaces() (map[string]Location, error) {
	home := HOME()

	b, err := ioutil.ReadFile(filepath.Join(home, rootDir, placesFile))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Location{}, nil
		}
		return nil, err
	}
	var places map[string]Location
	err = json.Unmarshal(b, &places)
	if err != nil {
		return nil, err
	}
	if places == nil {
		// need to make sure a non-nil map is returned.
		return map[string]Location{}, nil
	}
	return places, nil
}
