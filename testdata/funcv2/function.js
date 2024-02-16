async function handler(event) {
  const request = event.request;
  console.log(`on the edge uri: ${request.uri}`);
  return request;
}
