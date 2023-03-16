/**
 * @Author:      thepoy
 * @Email:       thepoy@163.com
 * @File Name:   response.go
 * @Created At:  2021-07-24 13:34:44
 * @Modified At: 2023-03-16 11:04:57
 * @Modified By: thepoy
 */

package predator

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	ctx "github.com/go-predator/predator/context"
	"github.com/go-predator/predator/json"
)

// ErrIncorrectResponse is an error that occurs when the response status code is not 20X
var (
	ErrIncorrectResponse = errors.New("the response status code is not 20X")
)

// Response represents an HTTP response from a server
type Response struct {
	resp       *http.Response // resp stores the underlying http.Response
	StatusCode StatusCode     // StatusCode stores the response status code

	header http.Header // header stores the response headers

	Body      []byte      // Body stores the binary response body
	Ctx       ctx.Context `json:"-"` // Ctx stores the context shared between the request and the response
	Request   *Request    `json:"-"` // Request stores the request corresponding to the response
	FromCache bool        // FromCache indicates whether the response was obtained from cache
	clientIP  string      // clientIP stores the public IP address of the server that sent the response
	timeout   bool        // timeout indicates whether the response was caused by a timeout error
	invalid   bool        // invalid indicates whether the response is valid; html for invalid responses will not be parsed

	isJSON bool             // isJSON indicates whether the response body is a valid JSON
	json   *json.JSONResult // json stores the JSON result of the response body if it is a valid JSON
}

// Save writes response body to disk with given file name and permission mode 0644
func (r *Response) Save(fileName string) error {
	return os.WriteFile(fileName, r.Body, 0644)
}

// Invalidate marks the current response as invalid and skips the html parsing process
func (r *Response) Invalidate() {
	r.invalid = true
}

// Method returns the request method of the response
func (r *Response) Method() string {
	return r.Request.Method()
}

// ContentType returns the content type header of the response
func (r *Response) ContentType() string {
	return r.header.Get("Content-Type")
}

// ContentLength returns the content length header of the response if present,
// otherwise returns the length of the body in bytes
func (r *Response) ContentLength() uint64 {
	cl := r.header.Get("Content-Length")

	var (
		length uint64 // length stores content length in bytes
		err    error  // err stores any parsing error
	)

	if cl != "" { // if content length header exists
		length, err = strconv.ParseUint(cl, 10, 64) // parse it as unsigned integer in base 10 with bit size 64
		if err != nil {                             // if parsing error occurs
			panic(err) // panic with error message
		}
	} else { // if content length header does not exist
		length = uint64(len(r.Body)) // use body length as content length
	}

	return length // return content length in bytes
}

// GetSetCookie returns the set-cookie header of the response
func (r *Response) GetSetCookie() string { return r.resp.Header.Get("Set-Cookie") }

// JSON returns the JSON result of the response body if it is a valid JSON
func (r *Response) JSON() json.JSONResult {
	return *r.json
}

// String returns the string representation of the response body
func (r *Response) String() string {
	return string(r.Body)
}

// cachedHeaders is a struct that stores a subset of response headers for caching purposes
type cachedHeaders struct {
	StatusCode    StatusCode // StatusCode stores the response status code
	ContentType   string     // ContentType stores the content type header; this is the most important field
	ContentLength uint64     // ContentLength stores the content length header or the body length in bytes
	Server        []byte     // Server stores the server IP address in bytes
	Location      []byte     // Location stores the location header in bytes if present
}

// cachedResponse is a struct that stores a cached response body and headers
type cachedResponse struct {
	Body    []byte         // Body stores the response body in bytes
	Headers *cachedHeaders // Headers stores a pointer to a cachedHeaders struct
}

// convertHeaders converts the response headers to a cachedHeaders struct and returns it with any error
func (r *Response) convertHeaders() (*cachedHeaders, error) {
	ch := &cachedHeaders{}               // create an empty cachedHeaders struct
	ch.StatusCode = r.StatusCode         // assign status code from response
	ch.ContentType = r.ContentType()     // assign content type from response
	ch.ContentLength = r.ContentLength() // assign content length from response
	ch.Server = []byte(r.ClientIP())     // assign server IP from response

	if ch.StatusCode == StatusFound { // if status code is 302 (found)
		if ch.Location == nil { // if location header is nil
			return nil, ErrInvalidResponseStatus // return nil and invalid status error
		}
		ch.Location = []byte(r.resp.Header.Get("Location")) // assign location header from response
	}

	return ch, nil // return cached headers and nil error
}

// Marshal converts the response to a cachedResponse struct and marshals it to JSON bytes with any error
func (r *Response) Marshal() ([]byte, error) {
	// The cached response does not need to save all the response headers,
	// so the following code is not used to convert the response headers to bytes
	// var buf bytes.Buffer
	// b := bufio.NewWriter(&buf)
	// r.Headers.Write(b)
	// b.Flush()

	var (
		cr  cachedResponse
		err error
	)
	cr.Body = r.Body
	cr.Headers, err = r.convertHeaders()
	if err != nil {
		return nil, err
	}

	return json.Marshal(cr)
}

// Unmarshal unmarshals a cached response body into the current response object
func (r *Response) Unmarshal(cachedBody []byte) error {
	var (
		cr  cachedResponse
		err error
	)

	err = json.Unmarshal(cachedBody, &cr) // unmarshal the cached response
	if err != nil {
		return err
	}

	r.Body = cr.Body                       // set the response body
	r.StatusCode = cr.Headers.StatusCode   // set the response status code
	r.clientIP = string(cr.Headers.Server) // set the client IP address

	if r.header == nil { // create a new header if it does not exist
		r.header = make(http.Header)
	}

	r.header.Set("Content-Type", cr.Headers.ContentType)                             // set the content type header
	r.header.Set("Content-Length", strconv.FormatUint(cr.Headers.ContentLength, 10)) // set the content length header

	return nil
}

// ClientIP returns the client IP address of the response
func (r *Response) ClientIP() string {
	return r.clientIP
}

// IsTimeout returns true if the response was a timeout
func (r *Response) IsTimeout() bool {
	return r.timeout
}
