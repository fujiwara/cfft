function handler(event) {
  var request = event.request;
  console.log('on the edge');
  return request;
}
