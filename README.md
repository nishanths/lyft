## lyft [![Build Status](https://travis-ci.org/nishanths/lyft.svg?branch=master)](https://travis-ci.org/nishanths/lyft)

Create and manage Lyft rides from the command line.

```sh
# Install
go get -u github.com/nishanths/lyft

# Follow setup steps described below
# ...

# Create rides
lyft ride create
lyft ride cancel <ride-id>
lyft ride status <ride-id>

# Save routes for future use
lyft route add    <name>
lyft route remove <name>...
lyft route show   [name]

# Help documentation
lyft -help
```

Lyft Line isn't available on Lyft's web application (October 2017),
but this program can help you order Line rides from your computer.

## Setup

[![GoDoc](https://godoc.org/github.com/nishanths/lyft?status.svg)](https://godoc.org/github.com/nishanths/lyft)

The program uses the following environment variables.

```
export GOOG_GEOCODE_KEY=<key>
export LYFT_CLIENT_ID=<key>
export LYFT_CLIENT_SECRET=<key>
```

See the [Setup](https://godoc.org/github.com/nishanths/lyft#hdr-Setup)
section in godoc to set them up.

(The first time you request a ride, the program will request authorization
to create rides on your behalf. Follow the instructions printed on screen.
You will only have to do this step once.)

## Example

<img src="https://i.imgur.com/uT0d4ln.gif" width=480>

## License

BSD 3-Clause.

Built with [`lyft-go`](https://github.com/nishanths/lyft-go).
