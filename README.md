# cfft

cfft is a testing tool for [CloudFront Functions](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/cloudfront-functions.html).

### Description

cfft is a testing tool for CloudFront Functions. cfft helps you to test CloudFront Functions in development stage.

cfft supports the following features.

- Initialize files for testing CloudFront Functions.
- Test CloudFront Functions in development stage.
- Compare the result with the expect object.
- Ignore fields in the expect object.
- Diff function code.
- Publish function.

cfft supports management of [CloudFront KeyValueStore](https://docs.aws.amazon.com/ja_jp/AmazonCloudFront/latest/DeveloperGuide/kvs-with-functions.html).

## Install

### Homebrew

```console
$ brew install fujiwara/tap/cfft
```

### Download binary

Download the binary from [GitHub Releases](https://github.com/fujiwara/cfft/releases).

### aqua

[aquaproj](https://aquaproj.github.io/) supports cfft. `fujiwara/cfft` is available in [aqua-registory](https://github.com/aquaproj/aqua-registry).

```console
$ aqua init
$ aqua g -i fujiwara/cfft
```

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

  kvs list
    list key values

  kvs get <key>
    get value of key

  kvs put <key> <value>
    put value of key

  kvs delete <key>
    delete key

  kvs info
    show info of key value store

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

### Use CloudFront KeyValueStore

cfft supports [CloudFront KeyVakueStore](https://docs.aws.amazon.com/ja_jp/AmazonCloudFront/latest/DeveloperGuide/kvs-with-functions.html).

```yaml
# cfft.yaml
name: function-with-kvs
function: function.js
kvs:
  name: hostnames
```

If you specify the `kvs` element in the config file, cfft creates a KeyValueStore with the name, if not exsites, and associates the KeyValueStore with the function. You can use the KeyValueStore in the function code.

In a function code, the KVS id is available in the `KVS_ID` environment variable.

```js
import cf from 'cloudfront';

const kvsId = "{{ must_env `KVS_ID` }}";
const kvsHandle = cf.kvs(kvsId);

async function handler(event) {
  const request = event.request;
  const clientIP = event.viewer.ip;
  const hostname = (await kvsHandle.exists(clientIP)) ? await kvsHandle.get(clientIP) : 'unknown';

  request.headers['x-hostname'] = { value: hostname };
  return request;
}
```

#### Manage KVS key values with `cfft kvs` command

`cfft kvs` command manages KVS key values.

- `cfft kvs list` lists all key values.
- `cfft kvs get <key>` gets the value of the key.
- `cfft kvs put <key> <value>` puts the value of the key.
- `cfft kvs delete <key>` deletes the key.
- `cfft kvs info` shows the information of the KeyValueStore.

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

## Template syntax

cfft read files (config, function, event, and expect) with the following template syntax by [kayac/go-config](https://github.com/kayac/go-config).

`must_env` function renders the environment variable value.

```
{{ must_env `FOO` }}
```

If the environment variable `FOO` is not set, cfft exits with a non-zero status code. You can use `env` function to set a default value.

```
{{ env `BAR` `default_of_BAR` }}
```

See [examples/true-client-ip](examples/true-client-ip) directory to see how to use the template syntax.

```yaml
testCases:
  - name: localhost
    event: event.json
    expect: expect.json
    env:
      IP: 127.0.0.1
      HOSTNAME: localhost
  - name: home
    event: event.json
    expect: expect.json
    env:
      IP: 192.168.1.1
      HOSTNAME: home
```

In `testCases`, `env` overrides the environment variables. These values are used in `event.json` and `expect.json`.

event.json
```json
{
    "version": "1.0",
    "context": {
        "eventType": "viewer-request"
    },
    "viewer": {
        "ip": "{{ env `IP` `127.0.0.2` }}"
    },
    "request": {
        "method": "GET",
        "uri": "/index.html",
        "headers": {},
        "cookies": {},
        "querystring": {}
    }
}
```

expect.json
```json
{
    "request": {
        "cookies": {},
        "headers": {
            "true-client-ip": {
                "value": "{{ env `IP` `127.0.0.2` }}"
            },
            "x-hostname": {
                "value": "{{ env `HOSTNAME` `unknown` }}"
            }
        },
        "method": "GET",
        "querystring": {},
        "uri": "/index.html"
    }
}
```

## LICENSE

MIT

## Author

Fujiwara Shunichiro
