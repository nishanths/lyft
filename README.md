## lyft

Request Lyft rides from the command line.

`go get -u go.avalanche.space/lyft`

### Example

Create a ride using `lyft ride create` and watch status updates (`-t`):

```
$ lyft -t ride create
Enter start location: 120 ottley drive atlanta
Enter end location (optional): candler field emory university
Start: https://www.google.com/maps/place/33.807049,-84.384321
       120 Ottley Dr NE, Atlanta, GA 30324, USA
End:   https://www.google.com/maps/place/33.800894,-84.333355
       Candler Field, Atlanta, GA 30322, USA

Created Ride ID: 2323630782199829597

Ride ID:     2323630782199829597
Ride Type:   Lyft Line
Status:      Pending
Origin:      https://www.google.com/maps/place/33.807049,-84.384321
             120 Ottley Dr NE, Atlanta, GA 30324, USA
Destination: https://www.google.com/maps/place/33.800894,-84.333355
             Candler Field, Atlanta, GA 30322, USA

...
```

### Setup

1. Install the program using `go get -u go.avalanche.space/lyft`.
2. Create `$HOME/.lyft/config.json` with the following contents:
```json
{
  "ClientID": "<Lyft Client ID>",
  "ClientSecret": "<Lyft Client Secret>"
}
```
3. Add a Google Maps Geocode API key to your `.profile`:
```
export GOOG_GECODE_KEY=<key>
```
4. Begin request rides! See the example above or run `lyft -help`.
   The first time you request a ride, you will need to authorize the program.
   Follow the instructions on the command line. You will only need to do this 
   the first time.

[Create an issue](https://github.com/nishanths/lyft/issues) if you run into trouble.

### API keys

Google Maps Geocode API key:

https://developers.google.com/maps/documentation/geocoding/get-api-key

Lyft API keys: 

1. Visit https://www.lyft.com/developers/apps/new and sign in.
2. Create a new app. Use any values for the app name and description.
3. Enter `http://localhost:90` for the redirect URL, and hit Submit.
4. That's it. You should be able to see your Client ID and Client Secret.

### License

BSD 3-Clause.
