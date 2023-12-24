# cfft

cfft is a testing tool for [CloudFront Functions](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/cloudfront-functions.html).

## Usage

```
Usage: cfft <command>

Flags:
  -h, --help                  Show context-sensitive help.
  -c, --config="cfft.yaml"    config file

Commands:
  test
    test function

  init --name=STRING
    initialize files

  diff
    diff function code

  publish
    publish function

  version
    show version

Run "cfft <command> --help" for more information on a command.
```

### Example of initializing files for testing CloudFront Functions

`cfft init` creates a config file and a function file and an example event file.

```
Usage: cfft init --name=STRING

initialize function

Flags:
  -h, --help                           Show context-sensitive help.
  -c, --config="cfft.yaml"             config file

      --name=STRING                    function name
      --format="json"                  output event file format (json,jsonnet,yaml)
      --event-type="viewer-request"    event type (viewer-request,viewer-response)
```

If the function is already exists in the CloudFront Functions, cfft downloads the function code and creates a config file.

If the function is not found, cfft creates a new config file and a function file and example event file. You can edit the function file and event file and test the function with `cfft test --create-if-missing`.


## Example of testing CloudFront Functions

`cfft test` executes CloudFront Functions in the DEVELOPMENT stage and compares the result with the expect object if specified.

```
Usage: cfft test

test function

Flags:
  -h, --help                  Show context-sensitive help.
  -c, --config="cfft.yaml"    config file

      --create-if-missing     create function if missing
```

### Add Cache-Control header in viewer-response

See [examples/add-cache-control](examples/add-cache-control) directory.

```yaml
# cfft.yaml
name: my-function
function: function.js
testCases:
  - name: add-cache-control
    event: event.json
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

2023/12/23 15:11:33 [error] failed to run test case add-cache-control, expect and actual are not equal
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

### Event and Expect file format

The event and expect file format is JSON, Jsonnet or YAML.

```yaml
# cfft.yaml
name: my-function
function: function.js
testCases:
  - name: add-cache-control
    event: event.jsonnet
    expect: expect.yaml
```

cfft supports the following file extensions.
- .json
- .jsonnet
- .yaml
- .yml

### Diff function code

`cfft diff` compares the function code with the code in the CloudFront Functions.

```console
$ cfft diff
2023/12/23 17:57:17 [info] function cfft found
--- E3UN6WX5RRO2AG
+++ function.js
@@ -1,5 +1,5 @@
 async function handler(event) {
   const request = event.request;
-  console.log('hello cfft world');
+  console.log('hello cfft');
   return request;
 }
```

### Publish function

`cfft publish` publishes the function to the CloudFront Functions.

```console
$ cfft publish
```

`cfft publish` fails if the local function code differs from the CloudFront Functions code.

Before publishing the function, you need to run `cfft diff` to check the difference and run `cfft test` to check the function behavior.


## LICENSE

MIT

## Author

Fujiwara Shunichiro
