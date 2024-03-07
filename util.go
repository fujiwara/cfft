package cfft

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/samber/lo"
)

func f(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}

type UtilCmd struct {
	ParseRequest  ParseRequestCmd  `cmd:"" help:"parse HTTP request text from STDIN"`
	ParseResponse ParseResponseCmd `cmd:"" help:"parse HTTP response text from STDIN"`
}

type ParseRequestCmd struct{}

type ParseResponseCmd struct{}

func (app *CFFT) RunUtil(ctx context.Context, op string, opt *UtilCmd) error {
	switch op {
	case "parse-request":
		return app.UtilParseRequest(ctx, opt.ParseRequest)
	case "parse-response":
		return app.UtilParseResponse(ctx, opt.ParseResponse)
	default:
		return fmt.Errorf("unknown command %s", op)
	}
}

func (app *CFFT) UtilParseRequest(ctx context.Context, opt ParseRequestCmd) error {
	text, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read request from STDIN, %w", err)
	}
	req, err := ParseRequest(string(text))
	if err != nil {
		return fmt.Errorf("failed to parse request, %w", err)
	}
	enc := json.NewEncoder(app.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(req)
}

func (app *CFFT) UtilParseResponse(ctx context.Context, opt ParseResponseCmd) error {
	text, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read response from STDIN, %w", err)
	}
	resp, err := ParseResponse(string(text))
	if err != nil {
		return fmt.Errorf("failed to parse response, %w", err)
	}
	enc := json.NewEncoder(app.stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

type CFFEvent struct {
	Version  string       `json:"version,omitempty"`
	Context  *CFFContext  `json:"context,omitempty"`
	Viewer   *CFFViewer   `json:"viewer,omitempty"`
	Request  *CFFRequest  `json:"request,omitempty"`
	Response *CFFResponse `json:"response,omitempty"`
}

func (r *CFFRequest) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		r = nil
		return nil
	}
	switch b[0] {
	case '"':
		// request is string
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		_r, err := ParseRequest(s)
		if err != nil {
			return err
		}
		*r = _r
	case '{':
		// request is object
		var x cffrequest
		if err := json.Unmarshal(b, &x); err != nil {
			return err
		}
		*r = (CFFRequest)(x)
	default:
		return fmt.Errorf("invalid request object: %s", string(b))
	}

	// init by empty map if nil
	if r.Headers == nil {
		r.Headers = map[string]CFFValue{}
	}
	if r.QueryString == nil {
		r.QueryString = map[string]CFFValue{}
	}
	if r.Cookies == nil {
		r.Cookies = map[string]CFFCookieValue{}
	}
	return nil
}

func (e *CFFEvent) Bytes() []byte {
	b, _ := json.Marshal(e)
	return b
}

/*
https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/functions-event-structure.html#functions-event-structure-context
distributionDomainName
The CloudFront domain name (for example, d111111abcdef8.cloudfront.net) of the distribution that's associated with the event.

distributionId
The ID of the distribution (for example, EDFDVBD6EXAMPLE) that's associated with the event.

eventType
The event type, either viewer-request or viewer-response.

requestId
A string that uniquely identifies a CloudFront request (and its associated response).
*/
type CFFContext struct {
	DistributionDomainName string `json:"distributionDomainName,omitempty"`
	DistributionId         string `json:"distributionId,omitempty"`
	EventType              string `json:"eventType,omitempty"`
	RequestId              string `json:"requestId,omitempty"`
}

/*
https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/functions-event-structure.html#functions-event-structure-viewer

The viewer object contains an ip field whose value is the IP address of the viewer (client) that sent the request. If the viewer request came through an HTTP proxy or a load balancer, the value is the IP address of the proxy or load balancer.
*/
type CFFViewer struct {
	IP string `json:"ip"`
}

/*
https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/functions-event-structure.html#functions-event-structure-request

The request object contains the following fields:

method
The HTTP method of the request. If your function code returns a request, it cannot modify this field. This is the only read-only field in the request object.

uri
The relative path of the requested object. If your function modifies the uri value, note the following:
The new uri value must begin with a forward slash (/).
When a function changes the uri value, it changes the object that the viewer is requesting.
When a function changes the uri value, it doesn't change the cache behavior for the request or the origin that an origin request is sent to.

querystring
An object that represents the query string in the request. If the request doesn't include a query string, the request object still includes an empty querystring object.
The querystring object contains one field for each query string parameter in the request.

headers
An object that represents the HTTP headers in the request. If the request contains any Cookie headers, those headers are not part of the headers object. Cookies are represented separately in the cookies object.
The headers object contains one field for each header in the request. Header names are converted to lowercase in the event object, and header names must be lowercase when they're added by your function code. When CloudFront Functions converts the event object back into an HTTP request, the first letter of each word in header names is capitalized. Words are separated by a hyphen (-). For example, if your function code adds a header named example-header-name, CloudFront converts this to Example-Header-Name in the HTTP request.

cookies
An object that represents the cookies in the request (Cookie headers).
The cookies object contains one field for each cookie in the request.
*/

type CFFValue struct {
	Value      string     `json:"value"`
	MultiValue []CFFValue `json:"multiValue,omitempty"`
}

type CFFCookieValue struct {
	Value      string           `json:"value"`
	Attributes string           `json:"attributes,omitempty"`
	MultiValue []CFFCookieValue `json:"multiValue,omitempty"`
}

type cffrequest CFFRequest

type CFFRequest struct {
	Method      string                    `json:"method"`
	URI         string                    `json:"uri"`
	QueryString map[string]CFFValue       `json:"querystring"`
	Headers     map[string]CFFValue       `json:"headers"`
	Cookies     map[string]CFFCookieValue `json:"cookies"`
}

/*
https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/functions-event-structure.html#functions-event-structure-response
The response object contains the following fields:

statusCode
The HTTP status code of the response. This value is an integer, not a string.
Your function can generate or modify the statusCode.

statusDescription
The HTTP status description of the response. If your function code generates a response, this field is optional.

headers
An object that represents the HTTP headers in the response. If the response contains any Set-Cookie headers, those headers are not part of the headers object. Cookies are represented separately in the cookies object.
The headers object contains one field for each header in the response. Header names are converted to lowercase in the event object, and header names must be lowercase when they're added by your function code. When CloudFront Functions converts the event object back into an HTTP response, the first letter of each word in header names is capitalized. Words are separated by a hyphen (-). For example, if your function code adds a header named example-header-name, CloudFront converts this to Example-Header-Name in the HTTP response.

cookies
An object that represents the cookies in the response (Set-Cookie headers).
The cookies object contains one field for each cookie in the response.

body
Adding the body field is optional, and it will not be present in the response object unless you specify it in your function. Your function does not have access to the original body returned by the CloudFront cache or origin. If you don't specify the body field in your viewer response function, the original body returned by the CloudFront cache or origin is returned to viewer.
If you want CloudFront to return a custom body to the viewer, specify the body content in the data field, and the body encoding in the encoding field. You can specify the encoding as plain text ("encoding": "text") or as Base64-encoded content ("encoding": "base64").
As a shortcut, you can also specify the body content directly in the body field ("body": "<specify the body content here>"). When you do this, omit the data and encoding fields. CloudFront treats the body as plain text in this case.

encoding
The encoding for the body content (data field). The only valid encodings are text and base64.
If you specify encoding as base64 but the body is not valid base64, CloudFront returns an error.
data
The body content.
*/

type cffresponse CFFResponse

type CFFResponse struct {
	StatusCode        int                       `json:"statusCode,omitempty"`
	StatusDescription string                    `json:"statusDescription,omitempty"`
	Headers           map[string]CFFValue       `json:"headers"`
	Cookies           map[string]CFFCookieValue `json:"cookies"`
	Body              *CFFBody                  `json:"body,omitempty"`
}

func (r *CFFResponse) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		r = nil
		return nil
	}
	switch b[0] {
	case '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		_r, err := ParseResponse(s)
		if err != nil {
			return err
		}
		*r = _r
	case '{':
		// response is object
		var x cffresponse
		if err := json.Unmarshal(b, &x); err != nil {
			return err
		}
		*r = (CFFResponse)(x)
	default:
		return fmt.Errorf("invalid response object: %s", string(b))
	}
	return nil
}

type CFFBody struct {
	Encoding string `json:"encoding"`
	Data     string `json:"data"`
}

func ParseRequest(text string) (CFFRequest, error) {
	req, err := textToHTTPRequest(text)
	if err != nil {
		return CFFRequest{}, fmt.Errorf("failed to parse request, %w", err)
	}
	r := CFFRequest{
		Method:      req.Method,
		URI:         req.URL.String(),
		Headers:     map[string]CFFValue{},
		Cookies:     map[string]CFFCookieValue{},
		QueryString: map[string]CFFValue{},
	}
	// fill headers
	for k, v := range req.Header {
		k := strings.ToLower(k)
		if k == "cookie" {
			continue
		}
		if len(v) == 1 {
			r.Headers[k] = CFFValue{Value: v[0]}
		} else {
			r.Headers[k] = CFFValue{
				Value: v[0],
				MultiValue: lo.Map(v, func(s string, _ int) CFFValue {
					return CFFValue{Value: s}
				}),
			}
		}
	}
	// fill cookies
	for _, c := range req.Cookies() {
		if cv, ok := r.Cookies[c.Name]; !ok {
			r.Cookies[c.Name] = CFFCookieValue{
				Value:      c.Value,
				MultiValue: []CFFCookieValue{{Value: c.Value}},
			}
		} else {
			// multi-value
			cv.MultiValue = append(cv.MultiValue, CFFCookieValue{
				Value: c.Value,
			})
			r.Cookies[c.Name] = cv
		}
	}
	// remove multi-value if not needed
	for k, v := range r.Cookies {
		if len(v.MultiValue) <= 1 {
			v.MultiValue = nil
			r.Cookies[k] = v
		}
	}

	// fill query string
	for k, v := range req.URL.Query() {
		if len(v) == 1 {
			r.QueryString[k] = CFFValue{Value: v[0]}
		} else {
			r.QueryString[k] = CFFValue{
				Value: v[0],
				MultiValue: lo.Map(v, func(s string, _ int) CFFValue {
					return CFFValue{Value: s}
				}),
			}
		}
	}

	return r, nil
}

func ParseResponse(text string) (CFFResponse, error) {
	resp, err := textToHTTPResponse(text)
	if err != nil {
		return CFFResponse{}, fmt.Errorf("failed to parse response, %w", err)
	}
	r := CFFResponse{
		StatusCode:        resp.StatusCode,
		StatusDescription: resp.Status,
		Headers:           map[string]CFFValue{},
		Cookies:           map[string]CFFCookieValue{},
		Body:              nil,
	}
	// fill headers
	for k, v := range resp.Header {
		k := strings.ToLower(k)
		if k == "set-cookie" {
			continue
		}
		if len(v) == 1 {
			r.Headers[k] = CFFValue{Value: v[0]}
		} else {
			r.Headers[k] = CFFValue{
				Value: v[0],
				MultiValue: lo.Map(v, func(s string, _ int) CFFValue {
					return CFFValue{Value: s}
				}),
			}
		}
	}
	// fill cookies
	for _, c := range resp.Cookies() {
		if cv, ok := r.Cookies[c.Name]; !ok {
			r.Cookies[c.Name] = CFFCookieValue{
				Value:      c.Value,
				Attributes: strings.TrimPrefix(c.String(), c.Name+"="+c.Value+"; "),
			}
		} else {
			// multi-value
			cv.MultiValue = append(cv.MultiValue, CFFCookieValue{
				Value:      c.Value,
				Attributes: strings.TrimPrefix(c.String(), c.Name+"="+c.Value+"; "),
			})
			r.Cookies[c.Name] = cv
		}
	}

	// fill body
	if resp.Body != nil {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return CFFResponse{}, fmt.Errorf("failed to read body, %w", err)
		}
		r.Body = &CFFBody{
			Encoding: "text",
			Data:     string(body),
		}
	}
	return r, nil
}

func textToHTTPRequest(text string) (*http.Request, error) {
	buf := bytes.NewBufferString(text)
	reader := bufio.NewReader(buf)

	requestLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	parts := strings.Split(strings.TrimSpace(requestLine), " ")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid request line")
	}

	req, err := http.NewRequest(parts[0], parts[1], nil)
	if err != nil {
		return nil, err
	}
	req.Proto = parts[2]

	// read headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // End of header
		}
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header line")
		}
		req.Header.Add(parts[0], parts[1])
	}

	// read body
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewBuffer(body))

	return req, nil
}

func textToHTTPResponse(text string) (*http.Response, error) {
	buf := bytes.NewBufferString(text)
	reader := bufio.NewReader(buf)

	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	// parse status line
	parts := strings.SplitN(strings.TrimSpace(statusLine), " ", 3)
	switch len(parts) {
	case 0, 1:
		return nil, fmt.Errorf("invalid status line")
	case 2:
		parts = append(parts, "") // status description is optional
	}

	statusCode, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid status code")
	}

	resp := &http.Response{
		StatusCode: statusCode,
		Status:     strings.TrimSpace(parts[2]),
		Header:     make(http.Header),
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// When reached the EOF while reading header, body should be nil.
				resp.Body = nil
				return resp, nil
			}
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // End of header
		}
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header line")
		}
		resp.Header.Add(parts[0], parts[1])
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))

	return resp, nil
}

// RemoveHeaderComment is a regexp to remove header comment from function code
var RegexpCodeHeaderComment = regexp.MustCompile(`//cfft:[^\n]+\n`)

// RemoveHeaderComment removes header comment from function code
func RemoveHeaderComment(code []byte) []byte {
	found := RegexpCodeHeaderComment.Find(code)
	if found != nil {
		return bytes.Replace(code, found, nil, 1)
	}
	return code
}
