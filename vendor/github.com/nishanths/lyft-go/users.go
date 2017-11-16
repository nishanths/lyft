package lyft

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Ride statuses.
const (
	StatusPending    = "pending"
	StatusAccepted   = "accepted"
	StatusArrived    = "arrived"
	StatusPickedUp   = "pickedUp"
	StatusDroppedOff = "droppedOff"
	StatusCanceled   = "canceled"
	StatusUnknown    = "unknown"
)

func RideStatusDisplay(s string) string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusAccepted:
		return "Accepted"
	case StatusArrived:
		return "Arrived"
	case StatusPickedUp:
		return "Picked up"
	case StatusDroppedOff:
		return "Dropped off"
	case StatusCanceled:
		return "Canceled"
	case StatusUnknown:
		return "Unknown"
	}
	return s
}

// Ride profiles.
const (
	ProfileBusiness = "business"
	ProfilePersonal = "personal"
)

// Auxiliary type for unmarshaling.
// NOTE: Time parsing will fail if the corresponding string is field empty.
// So to be safe, we only parse times if the field is non-empty. Particularly
// Pickup and Dropoff (which contain time.Time) are not present in webhook events.
type rideDetail struct {
	RideID              string            `json:"ride_id"`
	RideStatus          string            `json:"status"`
	RideType            string            `json:"ride_type"`
	Origin              rideLocation      `json:"origin"`
	Pickup              rideLocation      `json:"pickup"`
	Destination         rideLocation      `json:"destination"`
	Dropoff             rideLocation      `json:"dropoff"`
	Location            VehicleLocation   `json:"location"`
	Passenger           Person            `json:"passenger"` // The Phone field will not be set
	Driver              Person            `json:"driver"`
	Vehicle             Vehicle           `json:"vehicle"`
	PrimetimePercentage string            `json:"primetime_percentage"`
	Distance            float64           `json:"distance_miles"`
	Duration            float64           `json:"duration_seconds"` // Documented as float64
	Price               Price             `json:"price"`
	LineItems           []LineItem        `json:"line_items"`
	Requested           string            `json:"requested_at"`
	RideProfile         string            `json:"ride_profile"`
	BeaconColor         string            `json:"beacon_string"`
	PricingDetailsURL   string            `json:"pricing_details_url"`
	RouteURL            string            `json:"route_url"`
	CanCancel           []string          `json:"can_cancel"`
	CanceledBy          string            `json:"canceled_by"`
	CancellationPrice   cancellationPrice `json:"cancellation_price"`
	Rating              int               `json:"rating"`
	Feedback            string            `json:"feedback"`
}

func (r rideDetail) convert(res *RideDetail) error {
	var err error
	res.RideID = r.RideID
	res.RideStatus = r.RideStatus
	res.RideType = r.RideType
	err = r.Origin.convert(&res.Origin)
	if err != nil {
		return err
	}
	err = r.Pickup.convert(&res.Pickup)
	if err != nil {
		return err
	}
	err = r.Destination.convert(&res.Destination)
	if err != nil {
		return err
	}
	err = r.Dropoff.convert(&res.Dropoff)
	if err != nil {
		return err
	}
	res.Location = r.Location
	res.Passenger = r.Passenger
	res.Driver = r.Driver
	res.Vehicle = r.Vehicle
	res.PrimetimePercentage = r.PrimetimePercentage
	res.Distance = r.Distance
	res.Duration = time.Second * time.Duration(r.Duration)
	res.Price = r.Price
	res.LineItems = r.LineItems
	if r.Requested != "" {
		requested, err := time.Parse(TimeLayout, r.Requested)
		if err != nil {
			return err
		}
		res.Requested = requested
	}
	res.RideProfile = r.RideProfile
	res.BeaconColor = r.BeaconColor
	res.PricingDetailsURL = r.PricingDetailsURL
	res.RouteURL = r.RouteURL
	res.CanCancel = r.CanCancel
	res.CanceledBy = r.CanceledBy
	err = r.CancellationPrice.convert(&res.CancellationPrice)
	if err != nil {
		return err
	}
	res.Rating = r.Rating
	res.Feedback = r.Feedback
	return nil
}

type rideLocation struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
	Address   string  `json:"address"`
	ETA       float64 `json:"eta_seconds"` // Documented differently for origin vs. destination vs. dropoff, so float64 is safest
	Time      string  `json:"time"`
}

func (l rideLocation) convert(res *RideLocation) error {
	res.Latitude = l.Latitude
	res.Longitude = l.Longitude
	res.Address = l.Address
	res.ETA = time.Second * time.Duration(l.ETA) // TODO: consider not truncating
	if l.Time != "" {
		t, err := time.Parse(TimeLayout, l.Time)
		if err != nil {
			return err
		}
		res.Time = t
	}
	return nil
}

type cancellationPrice struct {
	Amount        int    `json:"amount"`
	Currency      string `json:"currency"`
	Token         string `json:"token"`
	TokenDuration int64  `json:"token_duration"` // seconds; documented as int
}

func (c cancellationPrice) convert(res *CancellationPrice) error {
	res.Amount = c.Amount
	res.Currency = c.Currency
	res.Token = c.Token
	res.TokenDuration = time.Second * time.Duration(c.TokenDuration) // TODO: consider not truncating
	return nil
}

// RideDetail is returned by the client's RideDetail and RideHistory methods.
// Some fields are available only if certain conditions are true
// at the time of making the request. See the API reference for details.
// The "generated_at" field is not supported.
type RideDetail struct {
	RideID              string
	RideStatus          string
	RideType            string
	Origin              RideLocation // Requested location of pickup. The Time field will not be set.
	Pickup              RideLocation // Actual location of pickup. The ETA field will not be set.
	Destination         RideLocation // Requested location of dropoff. The Time field will not be set.
	Dropoff             RideLocation // Actual location of dropoff. The ETA field will not be set.
	Location            VehicleLocation
	Passenger           Person
	Driver              Person
	Vehicle             Vehicle
	PrimetimePercentage string
	Distance            float64
	Duration            time.Duration
	Price               Price
	LineItems           []LineItem
	Requested           time.Time
	RideProfile         string
	BeaconColor         string
	PricingDetailsURL   string
	RouteURL            string
	CanCancel           []string
	CanceledBy          string
	CancellationPrice   CancellationPrice
	Rating              int
	Feedback            string
}

type RideLocation struct {
	Latitude  float64
	Longitude float64
	Address   string
	ETA       time.Duration
	Time      time.Time
}

type VehicleLocation struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
	Bearing   float64 `json:"bearing"` // Bearing of the car in degrees.
}

type Person struct {
	UserID    string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	ImageURL  string `json:"image_url"`
	Rating    string `json:"rating"`
	Phone     string `json:"phone_number"`
}

type Vehicle struct {
	Make              string `json:"make"`
	Model             string `json:"model"`
	Year              int    `json:"year"`
	LicensePlate      string `json:"license_plate"`
	LicensePlateState string `json:"license_plate_state"`
	Color             string `json:"color"`
	ImageURL          string `json:"image_url"`
}

type Price struct {
	Amount      int    `json:"amount"`
	Currency    string `json:"currency"`
	Description string `json:"description"`
}

type LineItem struct {
	Amount      int    `json:"amount"`
	Currency    string `json:"currency"`
	Description string `json:"type"`
}

type CancellationPrice struct {
	Amount        int
	Currency      string
	Token         string
	TokenDuration time.Duration
}

func (r *RideDetail) UnmarshalJSON(p []byte) error {
	var aux rideDetail
	if err := json.Unmarshal(p, &aux); err != nil {
		return err
	}
	return aux.convert(r)
}

// RideHistory returns the authenticated user's current and past rides.
// See the Lyft API reference for details on how far back the
// start and end times can go. If end is the zero time it is ignored.
// Limit specifies the maximum number of rides to return. If limit is -1,
// RideHistory requests the maximum limit documented in the API reference (50).
//
// Implementation detail: The times, in UTC, are formatted using "2006-01-02T15:04:05Z".
// For example: start.UTC().Format("2006-01-02T15:04:05Z").
func (c *Client) RideHistory(start, end time.Time, limit int32) ([]RideDetail, http.Header, error) {
	const layout = "2006-01-02T15:04:05Z"

	vals := make(url.Values)
	vals.Set("start_time", start.UTC().Format(layout))
	if !end.UTC().IsZero() {
		vals.Set("end_time", end.UTC().Format(layout))
	}
	if limit == -1 {
		limit = 50 // max limit documented in the Lyft API reference
	}
	vals.Set("limit", strconv.FormatInt(int64(limit), 10))
	r, err := http.NewRequest("GET", c.base()+"/v1/rides?"+vals.Encode(), nil)
	if err != nil {
		return nil, nil, err
	}

	rsp, err := c.do(r)
	if err != nil {
		return nil, nil, err
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != 200 {
		return nil, rsp.Header, NewStatusError(rsp)
	}

	var response struct {
		R []RideDetail `json:"ride_history"`
	}
	if err := unmarshal(rsp.Body, &response); err != nil {
		return nil, rsp.Header, err
	}
	return response.R, rsp.Header, nil
}

// UserProfile is returned by the client's UserProfile method.
type UserProfile struct {
	ID        string `json:"id"` // Authenticated user's ID.
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Ridden    bool   `json:"has_taken_a_ride"` // Whether the user has taken at least one ride.
}

// UserProfile returns the authenticated user's profile info.
func (c *Client) UserProfile() (UserProfile, http.Header, error) {
	r, err := http.NewRequest("GET", c.base()+"/v1/profile", nil)
	if err != nil {
		return UserProfile{}, nil, err
	}

	rsp, err := c.do(r)
	if err != nil {
		return UserProfile{}, nil, err
	}
	defer rsp.Body.Close()

	if rsp.StatusCode != 200 {
		return UserProfile{}, rsp.Header, NewStatusError(rsp)
	}

	var p UserProfile
	if err := unmarshal(rsp.Body, &p); err != nil {
		return UserProfile{}, rsp.Header, err
	}
	return p, rsp.Header, nil
}
