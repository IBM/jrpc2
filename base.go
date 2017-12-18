package jrpc2

import (
	"encoding/json"
	"errors"
	"fmt"
)

// A Request is a request message from a client to a server.
type Request struct {
	id     json.RawMessage // the request ID, nil for notifications
	method string          // the name of the method being requested
	params json.RawMessage // method parameters
}

// IsNotification reports whether the request is a notification, and thus does
// not require a value response.
func (r *Request) IsNotification() bool { return r.id == nil }

// ID returns the request identifier for r, or "" if r is a notification.
func (r *Request) ID() string { return string(r.id) }

// Method reports the method name for the request.
func (r *Request) Method() string { return r.method }

// UnmarshalParams decodes the parameters into v.
func (r *Request) UnmarshalParams(v interface{}) error { return json.Unmarshal(r.params, v) }

// A Response is a response message from a server to a client.
type Response struct {
	id     json.RawMessage
	err    *Error
	result json.RawMessage
}

// ID returns the request identifier for r.
func (r *Response) ID() string { return string(r.id) }

// Error returns a non-nil error of concrete type *Error if the response
// contains an error.
func (r *Response) Error() error {
	if r.err != nil {
		return r.err
	}
	return nil
}

// UnmarshalResult decodes the result message into v. If the request failed, an
// error is reported with concrete type *Error, and v is unmodified.
func (r *Response) UnmarshalResult(v interface{}) error {
	if r.err != nil {
		return r.err
	}
	return json.Unmarshal(r.result, v)
}

// jrequests is either a single request or a slice of requests.  This handles
// the decoding of batch requests in JSON-RPC 2.0.
type jrequests []*jrequest

func (j jrequests) MarshalJSON() ([]byte, error) {
	if len(j) == 1 {
		return json.Marshal(j[0])
	}
	return json.Marshal([]*jrequest(j))
}

func (j *jrequests) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return errors.New("empty request message")
	} else if data[0] != '[' {
		*j = jrequests{new(jrequest)}
		return json.Unmarshal(data, (*j)[0])
	}
	return json.Unmarshal(data, (*[]*jrequest)(j))
}

// jrequest is the transmission format of a request message.
type jrequest struct {
	V  string          `json:"jsonrpc"`      // must be Version
	ID json.RawMessage `json:"id,omitempty"` // rendered by the constructor, may be nil
	M  string          `json:"method"`
	P  jparams         `json:"params"` // rendered by the constructor
}

// jparams is a raw parameters message, including a check that the value is
// either an array or an object.
type jparams json.RawMessage

func (j jparams) MarshalJSON() ([]byte, error) { return []byte(j), nil }

func (j *jparams) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || (data[0] != '[' && data[0] != '{') {
		return errors.New("parameters must be list or object")
	}
	*j = append((*j)[:0], data...)
	return nil
}

// jresponses is a slice of responses, encoded as a single response if there is
// exactly one.
type jresponses []*jresponse

func (j jresponses) MarshalJSON() ([]byte, error) {
	if len(j) == 1 {
		return json.Marshal(j[0])
	}
	return json.Marshal([]*jresponse(j))
}

func (j *jresponses) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return errors.New("empty request message")
	} else if data[0] != '[' {
		*j = jresponses{new(jresponse)}
		return json.Unmarshal(data, (*j)[0])
	}
	return json.Unmarshal(data, (*[]*jresponse)(j))
}

// jresponse is the transmission format of a response message.
type jresponse struct {
	V  string          `json:"jsonrpc"`          // must be Version
	ID json.RawMessage `json:"id,omitempty"`     // set if request had an ID
	E  *jerror         `json:"error,omitempty"`  // set on error
	R  json.RawMessage `json:"result,omitempty"` // set on success
}

// jerror is the transmission format of an error object.
type jerror struct {
	Code int32           `json:"code"`
	Msg  string          `json:"message,omitempty"` // optional
	Data json.RawMessage `json:"data,omitempty"`    // optional
}

// toError converts a wire-format error object into an *Error.
func (e *jerror) toError() *Error {
	if e == nil {
		return nil
	}
	return &Error{
		Code:    Code(e.Code),
		Message: e.Msg,
		data:    e.Data,
	}
}

func jerrorf(code Code, msg string, args ...interface{}) *jerror {
	return &jerror{
		Code: int32(code),
		Msg:  fmt.Sprintf(msg, args...),
	}
}

// marshalParams validates and marshals params to JSON for a request.  It's
// okay for the parameters to be empty, but if they are not they must be valid
// JSON. We check for the required structural properties also.
func marshalParams(params interface{}) (json.RawMessage, error) {
	if params == nil {
		return nil, nil // no parameters, that is OK
	}
	bits, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	if len(bits) != 0 && bits[0] != '[' && bits[0] != '{' {
		// JSON-RPC requires that if parameters are provided at all, they are
		// an array or an object
		return nil, Errorf(E_InvalidRequest, "invalid parameters: array or object required")
	}
	return bits, err
}
