async function handler(event) {
  const response = event.response;
  const headers = response.headers;

  // Set the cache-control header
  headers['cache-control'] = { value: 'public, max-age=63072000' };
  console.log('[on the edge] Cache-Control header set.');

  // Return response to viewers
  return response;
}
