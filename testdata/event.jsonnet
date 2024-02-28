local event = import 'libs/event.libsonnet';
event {
  context: {
    eventType: 'viewer-response',
  },
  viewer: {
    ip: '1.2.3.4',
  },
  request: {
    method: 'GET',
    uri: '/index.html',
    headers: {},
    cookies: {},
    querystring: {},
  },
  response: {
    statusCode: 200,
    statusDescription: 'OK',
    headers: {},
    cookies: {},
  },
}
