async function handler(event) {
  const request = event.request;
  const clientIP = event.viewer.ip;
  request.headers['true-client-ip'] = { value: clientIP };
  return request;
}
