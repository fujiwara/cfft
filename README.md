# cfft

cfft is a testing tool for [CloudFront Functions](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/cloudfront-functions.html).

## Usage

```
Usage: cfft test

test function

Flags:
  -h, --help                  Show context-sensitive help.
  -c, --config="cfft.yaml"    config file

      --create-if-missing     create function if missing
```

## Example

### Add Cache-Control header in viewer-response

See [examples/add-cache-control](examples/add-cache-control) directory.

```yaml
# cfft.yaml
name: my-function
function: function.js
testCases:
  - name: add-cache-control
    event: event.json
    # expect: expect.json
```

```js
// function.js
async function handler(event) {
  const response = event.response;
  const headers = response.headers;

  // Set the cache-control header
  headers['cache-control'] = { value: 'public, max-age=63072000' };
  console.log('[on the edge] Cache-Control header set.');

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
$ cfft test
2023/12/23 15:09:04 [info] function my-function found
2023/12/23 15:09:04 [info] function code is not changed
2023/12/23 15:09:04 [info] testing function my-function with case add-cache-control...
2023/12/23 15:09:05 [info] ComputeUtilization:30
2023/12/23 15:09:05 [on the edge] Cache-Control header set.
{
  "response": {
    "cookies": {},
    "headers": {
      "cache-control": {
        "value": "public, max-age=63072000"
      }
    },
    "statusCode": 200,
    "statusDescription": "OK"
  }
}
```

cfft executes `my-function` with `event.json` at CloudFront Functions in development stage.

- If my-function is not found, `cfft test --create-if-missing` creates a new function with the name and runtime `cloudfront-js-2.0`.
- If the function is found and the code is different from the `function.js`, cfft updates the function code.
- Shows logs and compute utilization of the function after the execution.

### Compare the result with the expect object

When you specify the `expect` element in test cases, cfft compares the result with the expect object.

```yaml
# cfft.yaml
name: my-function
function: function.js
testCases:
  - name: add-cache-control
    event: event.json
    expect: expect.json
```

If the result is different from the `expect.json`, cfft exits with a non-zero status code.

```console
2023/12/23 15:11:33 [info] function my-function found
2023/12/23 15:11:33 [info] function code is not changed
2023/12/23 15:11:33 [info] testing function my-function with case add-cache-control...
2023/12/23 15:11:33 [info] ComputeUtilization:31
2023/12/23 15:11:33 [on the edge] Cache-Control header set.
{
  "response": {
    "cookies": {},
    "headers": {
      "cache-control": {
        "value": "public, max-age=63072000"
      }
    },
    "statusCode": 200,
    "statusDescription": "OK"
  }
}
2023/12/23 15:11:33 [error] failed to run test case add-cache-control, expect and actual are not equal:
--- expect
+++ actual
@@ -3,7 +3,7 @@
     "cookies": {},
     "headers": {
       "cache-control": {
-        "value": "public, max-age=6307200"
+        "value": "public, max-age=63072000"
       }
     },
     "statusCode": 200,
```

expect.json
```json
{
    "response": {
        "headers": {
            "cache-control": {
                "value": "public, max-age=6307200"
            }
        },
        "statusDescription": "OK",
        "cookies": {},
        "statusCode": 200
    }
}
```

### Ignore fields in the expect object

If you want to ignore some fields in the expect object, you can use the `ignore` element in test cases.

```yaml
# cfft.yaml
name: my-function
function: function.js
testCases:
  - name: add-cache-control
    event: event.json
    expect: expect.json
    ignore: ".response.cookies, .response.headers.date"
```

The `.response.cookies` and `.response.headers.date` are ignored in the expect object.


## LICENSE

MIT

## Author

Fujiwara Shunichiro
