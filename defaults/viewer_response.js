async function handler(event) {
  const response = event.response;
  console.log('on the edge');
  return response;
}
