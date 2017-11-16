package lyft

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Ride types. May not be an exhaustive list.
const (
	RideTypeLyft    = "lyft"
	RideTypePlus    = "lyft_plus"
	RideTypeLine    = "lyft_line"
	RideTypePremier = "lyft_premier"
	RideTypeLux     = "lyft_lux"
	RideTypeLuxSUV  = "lyft_luxsuv"
)

// RideTypeDisplay returns a nice display string for the supplied ride type.
func RideTypeDisplay(r string) string {
	switch r {
	case RideTypeLyft:
		return "Lyft"
	case RideTypePlus:
		return "Lyft Plus"
	case RideTypeLine:
		return "Lyft Line"
	case RideTypePremier:
		return "Lyft Premier"
	case RideTypeLux:
		return "Lyft Lux"
	case RideTypeLuxSUV:
		return "Lyft Lux SUV"
	}
	return r
}

type CostTokenInfo struct {
	PrimetimePercentage string
	PrimetimeMultiplier float64
	PrimetimeToken      string
	CostToken           string
	TokenDuration       time.Duration
	ErrorURI            string
}

func newCostTokenInfo(body io.Reader) (CostTokenInfo, error) {
	var c CostTokenInfo
	return c, unmarshal(body, &c)
}

func (c *CostTokenInfo) UnmarshalJSON(p []byte) error {
	type costTokenInfo struct {
		PrimetimePercentage string  `json:"primetime_percentage"`
		PrimetimeMultiplier float64 `json:"primetime_multiplier"`
		PrimetimeToken      string  `json:"primetime_confirmation_token"`
		CostToken           string  `json:"cost_token"`
		// Is this seriously a string? Even Swagger says so.
		// http://petstore.swagger.io/?url=https://api.lyft.com/v1/spec
		TokenDuration string `json:"token_duration"` // in seconds
		ErrorURI      string `json:"error_uri"`
	}
	var aux costTokenInfo
	if err := json.Unmarshal(p, &aux); err != nil {
		return err
	}
	c.PrimetimePercentage = aux.PrimetimePercentage
	c.PrimetimeMultiplier = aux.PrimetimeMultiplier
	c.PrimetimeToken = aux.PrimetimeToken
	c.CostToken = aux.CostToken
	i, err := strconv.ParseInt(aux.TokenDuration, 10, 64)
	if err != nil {
		return err
	}
	c.TokenDuration = time.Second * time.Duration(i)
	c.ErrorURI = aux.ErrorURI
	return nil
}

var _ error = (*RideRequestError)(nil)

type RideRequestError struct {
	ErrorInfo                // Fields may be empty
	Cost      *CostTokenInfo // May be nil
}

func newRideRequestError(rsp *http.Response) *RideRequestError {
	var eiBuf bytes.Buffer
	eiBuf.ReadFrom(rsp.Body)
	ciBuf := bytes.NewBuffer(eiBuf.Bytes())

	ei := newErrorInfo(&eiBuf, rsp.Header)
	ci, err := newCostTokenInfo(ciBuf)

	ret := &RideRequestError{
		ErrorInfo: ei,
	}
	if err == nil {
		ret.Cost = &ci
	}
	return ret
}

func (c *RideRequestError) Error() string {
	if c.Reason != "" && c.Description != "" {
		return fmt.Sprintf("%s: %s", c.Reason, c.Description)
	} else if c.Reason != "" {
		return c.Reason
	} else if c.Description != "" {
		return c.Description
	}
	return "<ride request error>"
}

// RideRequest is the paramters for the client's RequestRide method.
type RideRequest struct {
	Origin      Location `json:"origin"`      // Latitude and Longitude fields are required
	Destination Location `json:"destination"` // Latitude and Longitude fields are required
	RideType    string   `json:"ride_type"`   // Required
	CostToken   string   `json:"cost_token"`  // Optional
}

// CreatedRide is returned by the client's RequestRide method.
type CreatedRide struct {
	RideID      string   `json:"ride_id"`
	RideStatus  string   `json:"status"` // StatusPending for newly requested rides
	RideType    string   `json:"ride_type"`
	Origin      Location `json:"origin"`
	Destination Location `json:"destination"`
	Passenger   Person   `json:"passenger"` // The Phone field will not be set
}

type Location struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lng"`
	Address   string  `json:"address"`
}

// RequestRide requests a ride for a user.
// As of 2017-11-05, Lyft Line is not fully supported. See
// https://developer.lyft.com/reference#ride-request for details.
//
// If further action (such as confirming the cost) is required before the
// ride can be successfully created, the error will be of type *RideRequestError.
// This corresponds to the 400 status code documented in Lyft's API reference.
func (c *Client) RequestRide(req RideRequest) (CreatedRide, http.Header, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(req); err != nil {
		return CreatedRide{}, nil, err
	}
	r, err := http.NewRequest("POST", c.base()+"/v1/rides", &buf)
	if err != nil {
		return CreatedRide{}, nil, err
	}
	r.Header.Set("Content-Type", "application/json")

	rsp, err := c.do(r)
	if err != nil {
		return CreatedRide{}, nil, err
	}
	defer drainAndClose(rsp.Body)

	switch rsp.StatusCode {
	case 201:
		var cr CreatedRide
		if err := unmarshal(rsp.Body, &cr); err != nil {
			return CreatedRide{}, rsp.Header, err
		}
		return cr, rsp.Header, nil
	case 400:
		return CreatedRide{}, rsp.Header, newRideRequestError(rsp)
	default:
		return CreatedRide{}, rsp.Header, NewStatusError(rsp)
	}
}

// SetDestination updates the ride's destination to the supplied location.
// The location's Address field is optional.
func (c *Client) SetDestination(rideID string, loc Location) (Location, http.Header, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(loc); err != nil {
		return Location{}, nil, err
	}
	r, err := http.NewRequest("PUT", fmt.Sprintf("%s/v1/rides/%s/destination", c.base(), rideID), &buf)
	if err != nil {
		return Location{}, nil, err
	}
	r.Header.Set("Content-Type", "application/json")

	rsp, err := c.do(r)
	if err != nil {
		return Location{}, nil, err
	}
	defer drainAndClose(rsp.Body)

	switch rsp.StatusCode {
	case 200:
		var ret Location
		if err := unmarshal(rsp.Body, &ret); err != nil {
			return Location{}, rsp.Header, err
		}
		return ret, rsp.Header, nil
	default:
		return Location{}, rsp.Header, NewStatusError(rsp)
	}
}

// RideReceipt is returned by the client's RideReceipt method.
type RideReceipt struct {
	RideID      string
	Price       Price
	LineItems   []LineItem
	Charges     []Charge
	Requested   time.Time
	RideProfile string
}

func (r *RideReceipt) UnmarshalJSON(p []byte) error {
	type rideReceipt struct {
		RideID      string     `json:"ride_id"`
		Price       Price      `json:"price"`
		LineItems   []LineItem `json:"line_items"`
		Charges     []Charge   `json:"charges"`
		Requested   string     `json:"requested_at"`
		RideProfile string     `json:"ride_profile"`
	}
	var aux rideReceipt
	if err := json.Unmarshal(p, &aux); err != nil {
		return err
	}
	r.RideID = aux.RideID
	r.Price = aux.Price
	r.LineItems = aux.LineItems
	r.Charges = aux.Charges
	if aux.Requested != "" {
		requested, err := time.Parse(TimeLayout, aux.Requested)
		if err != nil {
			return err
		}
		r.Requested = requested
	}
	r.RideProfile = aux.RideProfile
	return nil
}

type Charge struct {
	Amount        int    `json:"amount"`
	Currency      string `json:"currency"`
	PaymentMethod string `json:"payment_method"`
}

// RideReceipt retrieves the receipt for the specified ride.
func (c *Client) RideReceipt(rideID string) (RideReceipt, http.Header, error) {
	r, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/rides/%s/receipt", c.base(), rideID), nil)
	if err != nil {
		return RideReceipt{}, nil, err
	}

	rsp, err := c.do(r)
	if err != nil {
		return RideReceipt{}, nil, err
	}
	defer drainAndClose(rsp.Body)

	if rsp.StatusCode != 200 {
		return RideReceipt{}, rsp.Header, NewStatusError(rsp)
	}

	var rec RideReceipt
	if err := unmarshal(rsp.Body, &rec); err != nil {
		return RideReceipt{}, rsp.Header, err
	}
	return rec, rsp.Header, nil
}

var _ error = (*CancelRideError)(nil)

type CancelRideError struct {
	ErrorInfo
	Amount        float64
	Currency      string
	Token         string
	TokenDuration time.Duration
}

func newCancelRideError(rsp *http.Response) *CancelRideError {
	ret := &CancelRideError{}

	type aux struct {
		Amount        float64 `json:"amount"`
		Currency      string  `json:"currency"`
		Token         string  `json:"token"`
		TokenDuration int64   `json:"token_duration"` // seconds
	}

	var eiBuf bytes.Buffer
	eiBuf.ReadFrom(rsp.Body)
	otherBuf := bytes.NewBuffer(eiBuf.Bytes())

	ret.ErrorInfo = newErrorInfo(&eiBuf, rsp.Header)

	var a aux
	err := unmarshal(otherBuf, &a)
	if err == nil {
		ret.Amount = a.Amount
		ret.Currency = a.Currency
		ret.Token = a.Token
		ret.TokenDuration = time.Second * time.Duration(a.TokenDuration)
	}

	return ret
}

func (c *CancelRideError) Error() string {
	if c.Reason != "" && c.Description != "" {
		return fmt.Sprintf("%s: %s", c.Reason, c.Description)
	} else if c.Reason != "" {
		return c.Reason
	} else if c.Description != "" {
		return c.Description
	}
	return "<cancel ride error>"
}

// CancelRide cancels the specificed ride. cancelToken is the cancel confirmation
// token; it is optional. See https://developer.lyft.com/reference#ride-request-cancel
// for more details on the token.
//
// If more action is required to cancel the ride, a returned error of
// type *CancelRideError will have more details.
func (c *Client) CancelRide(rideID, cancelToken string) (http.Header, error) {
	var body io.Reader
	if cancelToken != "" {
		body = strings.NewReader(fmt.Sprintf(`{"cancel_confirmation_token": "%s"}`, cancelToken))
	}
	r, err := http.NewRequest("POST", fmt.Sprintf("%s/v1/rides/%s/cancel", c.base(), rideID), body)
	if err != nil {
		return nil, err
	}
	if cancelToken != "" {
		r.Header.Set("Content-Type", "application/json")
	}

	rsp, err := c.do(r)
	if err != nil {
		return nil, err
	}
	defer drainAndClose(rsp.Body)

	switch rsp.StatusCode {
	case 204:
		return rsp.Header, nil
	case 400:
		return rsp.Header, newCancelRideError(rsp)
	default:
		return rsp.Header, NewStatusError(rsp)
	}
}

func (c *Client) RideDetail(rideID string) (RideDetail, http.Header, error) {
	r, err := http.NewRequest("GET", fmt.Sprintf("%s/v1/rides/%s", c.base(), rideID), nil)
	if err != nil {
		return RideDetail{}, nil, err
	}

	rsp, err := c.do(r)
	if err != nil {
		return RideDetail{}, nil, err
	}
	defer drainAndClose(rsp.Body)

	if rsp.StatusCode != 200 {
		return RideDetail{}, rsp.Header, NewStatusError(rsp)
	}

	var det RideDetail
	if err := unmarshal(rsp.Body, &det); err != nil {
		return RideDetail{}, rsp.Header, err
	}
	return det, rsp.Header, nil
}

// TODO: Implement this: func (c *Client) RateRide()
