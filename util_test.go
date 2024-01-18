package cfft_test

import (
	"encoding/json"
	"testing"

	"github.com/fujiwara/cfft"
	"github.com/google/go-cmp/cmp"
)

func TestTextCFFRequest(t *testing.T) {
	parsedRequest, err := cfft.ParseRequest(`GET /?foo=bar&foo=baz&bar=1 HTTP/1.1
Host: example.com
User-Agent: curl/7.64.1
Accept: */*
X-Forwarded-For: 127.0.0.1
X-Forwarded-For: 192.168.1.1
Cookie: foo=bar; baz=qux
Cookie: baz=quux

`)
	if err != nil {
		t.Errorf("ParseRequest returned an error: %v", err)
	}
	expect := `{
		"method": "GET",
		"uri": "/?foo=bar&foo=baz&bar=1",
		"headers": {
			"accept": {"value": "*/*"},
			"x-forwarded-for": {
				"value": "127.0.0.1",
				"multiValue": [{"value": "127.0.0.1"}, {"value": "192.168.1.1"}]
			},
			"host": {"value": "example.com"},
			"user-agent": {"value": "curl/7.64.1"}
		},
		"querystring": {
			"foo": {"value": "bar", "multiValue": [{"value": "bar"}, {"value": "baz"}]},
			"bar": {"value": "1"}
		},
		"cookies": {
			"foo": {"value": "bar"},
			"baz": {"value": "qux", "multiValue": [{"value": "qux"}, {"value": "quux"}]}
		}
	}`
	var expectRequest cfft.CFFRequest
	if err := json.Unmarshal([]byte(expect), &expectRequest); err != nil {
		t.Fatalf("failed to parse expect as CFFRequest: %v", err)
	}
	if d := cmp.Diff(parsedRequest, expectRequest); d != "" {
		t.Errorf("parsedRequest differs from expect: %s", d)
	}
}

func TestCFFTextToResponse(t *testing.T) {
	parsedResponse, err := cfft.ParseResponse(`HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8
Content-Length: 13
X-Foo: aaa
X-Foo: bbb
Set-Cookie: foo=bar; Expires=Wed, 13 Jan 2021 22:23:01 GMT; Max-Age=86400; HttpOnly; Secure
Set-Cookie: baz=qux; Path=/; Domain=example.com;

Hello, World!`)
	if err != nil {
		t.Errorf("ParseResponse returned an error: %v", err)
	}
	expect := `{
		"statusCode": 200,
		"statusDescription": "OK",
		"headers": {
			"content-type": {"value": "text/plain; charset=utf-8"},
			"content-length": {"value": "13"},
			"x-foo": {
				"value": "aaa",
				"multiValue": [{"value": "aaa"}, {"value": "bbb"}]
			}
		},
		"cookies": {
			"foo": {
				"value": "bar",
				"attributes": "Expires=Wed, 13 Jan 2021 22:23:01 GMT; Max-Age=86400; HttpOnly; Secure"
			},
			"baz": {
				"value": "qux",
				"attributes": "Path=/; Domain=example.com"
			}
		},
		"body": {
			"encoding": "text",
			"data": "Hello, World!"
		}
	}`
	var expectResponse cfft.CFFResponse
	if err := json.Unmarshal([]byte(expect), &expectResponse); err != nil {
		t.Fatalf("failed to parse expect as CFFResponse: %v", err)
	}
	if diff := cmp.Diff(parsedResponse, expectResponse); diff != "" {
		t.Errorf("parsedResponse differs from expect: %s", diff)
	}
}

func TestCFFTextToResponseWitoutBody(t *testing.T) {
	parsedResponse, err := cfft.ParseResponse(`HTTP/1.1 302 Found
Location: https://example.com/
`)
	if err != nil {
		t.Errorf("ParseResponse returned an error: %v", err)
	}
	expect := `{
		"statusCode": 302,
		"statusDescription": "Found",
		"cookies": {},
		"headers": {
			"location": {"value": "https://example.com/"}
		}
	}`
	var expectResponse cfft.CFFResponse
	if err := json.Unmarshal([]byte(expect), &expectResponse); err != nil {
		t.Fatalf("failed to parse expect as CFFResponse: %v", err)
	}
	if diff := cmp.Diff(parsedResponse, expectResponse); diff != "" {
		t.Errorf("parsedResponse differs from expect: %s", diff)
	}
	if parsedResponse.Body != nil {
		t.Errorf("parsedResponse.Body should be nil")
	}
}
