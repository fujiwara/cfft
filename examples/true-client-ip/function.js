import cf from 'cloudfront';
const hostnames = {
  '127.0.0.1': 'localhost',
  '192.168.1.1': 'home',
}

async function handler(event) {
  const request = event.request;
  const clientIP = event.viewer.ip;
  const hostname = hostnames[clientIP] || 'unknown';

  request.headers['true-client-ip'] = { value: clientIP };
  request.headers['x-hostname'] = { value: hostname };
  return request;
}
