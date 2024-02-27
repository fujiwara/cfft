const handler = async (event) => {
  const request = event.request;
  request.headers['x-req2'] = { value: 'bar' };
  return request;
}
