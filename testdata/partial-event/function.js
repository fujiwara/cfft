async function handler(event) {
  const request = event.request;
  request.headers['cache-control'] = { value: 'private; no-cache' };
  request.cookies = { 'cookie-name': { value: 'cookie-value' } };
  request.querystring = { n: { value: '10' } };
  return request;
}
