{
  version: '1.0',
  context: {
    eventType: 'viewer-response',
  },
  viewer: {
    ip: '1.2.3.4',
  },
  request: |||
    GET /index.html HTTP/1.1
    Host: example.com
    User-Agent: Mozilla/5.0
    Cookie: foo=bar, bar=baz
    Cookie: bar=xxx
  |||,
  response: |||
    HTTP/1.1 200 OK
    Content-Type: text/html
    Content-Length: 13
    Set-Cookie: foo=bar; Secure; Path=/; Domain=example.com

    Hello World!
  |||,
}
