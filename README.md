## lyft

[![GoDoc](https://godoc.org/github.com/nishanths/lyft?status.svg)](https://godoc.org/github.com/nishanths/lyft)
[![Build Status](https://travis-ci.org/nishanths/lyft.svg?branch=master)](https://travis-ci.org/nishanths/lyft)

Create and manage Lyft rides from the command line.

```sh
# Install
go get github.com/nishanths/lyft

# Set up env vars (see 'Setup' heading below)
export GOOG_GEOCODE_KEY=<key>
export LYFT_CLIENT_ID=<key>
export LYFT_CLIENT_SECRET=<key>

# Manage rides
lyft ride create
lyft ride cancel <ride-id>
lyft ride status <ride-id>

# Save places for future use when creating rides
lyft place add    <name>
lyft place remove <name>...
lyft place show   [name]

# Help
lyft -help # or https://godoc.org/github.com/nishanths/lyft
```

Lyft Line isn't available on Lyft's web application (October 2017),
but this program can help you order Line rides from your computer.

## Setup

The program uses the following environment variables.

```
GOOG_GEOCODE_KEY
LYFT_CLIENT_ID
LYFT_CLIENT_SECRET
```

Refer to the [Setup](https://godoc.org/github.com/nishanths/lyft#hdr-Setup)
section in godoc to set these up.

## Example

<img src="https://i.imgur.com/uT0d4ln.gif" width=480>

## License

BSD 3-Clause.

Built with [`lyft-go`](https://github.com/nishanths/lyft-go).
