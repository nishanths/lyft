package main

import (
	"log"
	"os"
	"sync"

	"googlemaps.github.io/maps"
)

const geocodeEnv = "GOOG_GEOCODE_KEY"

// geocodeKey returns the Google Maps Geocode API key.
// If it is not found, it logs a fatal error.
func geocodeKey() string {
	key := os.Getenv(geocodeEnv)
	if key == "" {
		log.Fatalf("%s must be set to geocode addresses; see https://godoc.org/github.com/nishanths/lyft#hdr-Setup", geocodeEnv)
	}
	return key
}

var (
	gc     *maps.Client
	gcInit sync.Once
)

// mapsClient returns a Google Maps client ready to make
// geocode requests. It logs a fatal error if the client cannot be created.
func mapsClient() *maps.Client {
	gcInit.Do(func() {
		key := geocodeKey()
		client, err := maps.NewClient(maps.WithAPIKey(key))
		if err != nil {
			log.Fatalf("making google maps client: %s", err)
		}
		gc = client
	})
	return gc
}
