async function handler(event) {
  const request = event.request;
  console.log('on the edge');
  return request;
}
