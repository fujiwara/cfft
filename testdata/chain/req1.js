async function handler(event) {
  const request = event.request;
  request.headers['x-req1'] = { value: 'foo' };
  return request;
}
