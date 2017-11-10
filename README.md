## lyft

Request Lyft rides from the command line.

Install: `go get -u go.avalanche.space/lyft`

Lyft Line isn't available on Lyft's web application as of October 2017. 
This program can help you order Line rides from your computer.

### Example

Create a ride using `lyft ride create`. You can watch updates to the ride using 
the `-w` flag and include desktop notifications using `-t`.

<img src="https://i.imgur.com/uT0d4ln.gif" width=480>

### Setup

1. Install the program using `go get -u go.avalanche.space/lyft`.
2. Add a Google Maps Geocode API key to your `.profile`:
```
export GOOG_GEOCODE_KEY=<key>
```
3. Create `$HOME/.lyft/config.json` with the following contents:
```json
{
  "ClientID": "<Lyft Client ID>",
  "ClientSecret": "<Lyft Client Secret>"
}
```
4. Begin requesting rides! See the example above or run `lyft -help` from Terminal.
   The first time you request a ride, you will need to authorize the program
   to order rides on your behalf. Follow the instructions printed on the 
   command line. You will only need to do this the first time.

[Create an issue](https://github.com/nishanths/lyft/issues) if you run into trouble.

### API keys

Google Maps Geocode API key:

https://developers.google.com/maps/documentation/geocoding/get-api-key

Lyft API keys: 

1. Visit https://www.lyft.com/developers/apps/new and sign in.
2. Create a new app. Use any values for the app name and description.
3. Enter `http://localhost:90` or any unused local URL for the Redirect 
   URL, and hit Submit.
4. That's it. You should be able to see your Client ID and Client Secret.

Built with [`lyft-go`](https://github.com/nishanths/lyft-go).

### License

BSD 3-Clause.
