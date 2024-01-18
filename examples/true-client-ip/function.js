import cf from 'cloudfront';

const kvsId = "{{ must_env `KVS_ID` }}";
const kvsHandle = cf.kvs(kvsId);

async function handler(event) {
  const request = event.request;
  const clientIP = event.viewer.ip;
  const hostname = (await kvsHandle.exists(clientIP)) ? await kvsHandle.get(clientIP) : 'unknown';

  request.headers['true-client-ip'] = { value: clientIP };
  request.headers['x-hostname'] = { value: hostname };
  return request;
}
