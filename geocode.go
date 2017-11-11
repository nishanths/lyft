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
		log.Fatalf("%s must be set", geocodeEnv)
	}
	return key
}

var (
	gc     *maps.Client
	gcInit sync.Once
)

// geocodeClient returns a Google Maps client ready to make
// geocode requests. It logs a fatal error if the client cannot be created.
func geocodeClient() *maps.Client {
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
