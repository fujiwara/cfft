# cfft

cfft is a testing tool for [CloudFront Functions](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/cloudfront-functions.html).

## Usage

```
Usage: cfft <name> <function> <event> [<expect>]

Arguments:
  <name>        function name
  <function>    function code file
  <event>       event object file
  [<expect>]    expect object file

Flags:
  -h, --help             Show context-sensitive help.
  -i, --ignore=STRING    ignore fields in the expect object by jq syntax
```

## Example

### Add Cache-Control header in viewer-response

See [examples/add-cache-control](examples/add-cache-control) directory.

```js
// function.js
async function handler(event) {
  const response = event.response;
  const headers = response.headers;

  // Set the cache-control header
  headers['cache-control'] = { value: 'public, max-age=63072000' };

  // Return response to viewers
  return response;
}
```

event.json
```json
{
    "version": "1.0",
    "context": {
        "eventType": "viewer-response"
    },
    "viewer": {
        "ip": "1.2.3.4"
    },
    "request": {
        "method": "GET",
        "uri": "/index.html",
        "headers": {},
        "cookies": {},
        "querystring": {}
    },
    "response": {
        "statusCode": 200,
        "statusDescription": "OK",
        "headers": {},
        "cookies": {}
    }
}
```

```console
$ cfft my-function function.js event.json
2023/12/23 01:47:02 [info] loading function add-cache-control from function.js
2023/12/23 01:47:03 [info] function add-cache-control found
2023/12/23 01:47:04 [info] function code is not changed
2023/12/23 01:47:04 [info] testing function add-cache-control with event:event.json expect: ignore:
2023/12/23 01:47:04 [info] ComputeUtilization:27
{"response":{"headers":{"cache-control":{"value":"public, max-age=63072000"}},"statusDescription":"OK","cookies":{},"statusCode":200}}
```

cfft executes `my-function` with `event.json` at CloudFront Functions in development stage.

- If my-function is not found, cfft create a new function with the name.
- If the function is found and the code is different from the `function.js`, cfft updates the function code.
- cfft shows the compute utilization of the function after the execution.

### Compare the result with the expect object

When you specify the expect object, cfft compares the result with the expect object.

If the result is different from the `expect.json`, cfft exits with a non-zero status code.

```console
$ cfft my-function function.js event.json expect.json
2023/12/23 01:54:14 [info] loading function add-cache-control from function.js
2023/12/23 01:54:16 [info] function add-cache-control found
2023/12/23 01:54:16 [info] function code is not changed
2023/12/23 01:54:16 [info] testing function add-cache-control with event:event.json expect:expect.json ignore:
2023/12/23 01:54:16 [info] ComputeUtilization:47
{"response":{"headers":{"cache-control":{"value":"public, max-age=63072000"}},"statusDescription":"OK","cookies":{},"statusCode":200}}
2023/12/23 01:54:16 [info] expect and actual are equal
```

expect.json
```json
{
    "response": {
        "headers": {
            "cache-control": {
                "value": "public, max-age=63072000"
            }
        },
        "statusDescription": "OK",
        "cookies": {},
        "statusCode": 200
    }
}
```

### Ignore fields in the expect object

If you want to ignore some fields in the expect object, you can use the `--ignore` (`-i`) option.

```console
$ cfft my-function function.js event.json expect.json --ignore '.response.cookies, .response.headers.date'
```

The `.response.cookies` and `.response.headers.date` are ignored in the expect object.


## LICENSE

MIT

## Author

Fujiwara Shunichiro
