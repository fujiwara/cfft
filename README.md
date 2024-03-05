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
- Output JSON for Terraform. See [Cooperate with Terraform](#cooperate-with-terraform).

cfft supports management of [CloudFront KeyValueStore](https://docs.aws.amazon.com/ja_jp/AmazonCloudFront/latest/DeveloperGuide/kvs-with-functions.html). See [Use CloudFront KeyValueStore](#use-cloudfront-keyvaluestore).

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
      --debug                 enable debug log
      --log-format="text"     log format (text,json)

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

  render
    render function code

  tf
    output JSON for tf

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
2024-01-19T22:35:26+09:00 [info] function my-function found
2024-01-19T22:35:26+09:00 [info] function code is not changed
2024-01-19T22:35:26+09:00 [info] [testcase:add-cache-control] testing function
2024-01-19T22:35:26+09:00 [info] [testcase:add-cache-control] ComputeUtilization: 31 optimal
2024-01-19T22:35:26+09:00 [info] [testcase:add-cache-control] [from:my-function] [on the edge] Cache-Control header set.
2024-01-19T22:35:26+09:00 [info] [testcase:add-cache-control] OK
2024-01-19T22:35:27+09:00 [info] 1 testcases passed
```

cfft executes `my-function` with `event.json` at CloudFront Functions in development stage.

About an event object, see also [CloudFront Functions event structure](https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/functions-event-structure.html).

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
2024-01-19T22:39:33+09:00 [info] function my-function found
2024-01-19T22:39:33+09:00 [info] function code or kvs association is changed, updating...
2024-01-19T22:39:34+09:00 [info] [testcase:add-cache-control] testing function
2024-01-19T22:39:35+09:00 [info] [testcase:add-cache-control] ComputeUtilization: 29
2024-01-19T22:39:35+09:00 [info] [testcase:add-cache-control] [from:my-function] [on the edge] Cache-Control header set.
--- expect
+++ actual
@@ -4,7 +4,7 @@
     "statusDescription": "OK",
     "headers": {
       "cache-control": {
-        "value": "public, max-age=6307200"
+        "value": "public, max-age=63072000"
       }
     }
   }

2024-01-19T22:39:35+09:00 [error] failed to run test case add-cache-control, expect and actual are not equal
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

### HTTP text format for Request and Response objects

cfft supports an HTTP text format for Request and Response objects.

The following example is the HTTP text format of the Request object.

```
GET /index.html HTTP/1.1
Host: example.com
```

The request object is converted to the following JSON object.

```json
{
  "method": "GET",
  "uri": "/index.html",
  "headers": {
    "host": {
      "value": "example.com"
    }
  }
}
```

The following example is the HTTP text format of the Response object.

```
HTTP/1.1 302 Found
Location: https://example.com/
```

The response object is converted to the following JSON object.

```json
{
  "statusCode": 302,
  "statusDescription": "Found",
  "headers": {
    "location": {
      "value": "https://example.com/"
    }
  }
}
```

You can convert from HTTP text to JSON object with `cfft util parse-request` and `cfft util parse-response` commands.

```console
$ cfft util parse-request < request.txt
{
  "method": "GET",
  "uri": "/index.html",
  "headers": {
    "host": {
      "value": "example.com"
    }
  }
}
```

For use of the text format, I recommend using YAML or Jsonnet format for the event and expect files instead of plain JSON. YAML and Jsonnet support multiline strings.

```yaml
# event.yaml
---
version: "1.0"
context:
  eventType: viewer-response
viewer:
  ip: 1.2.3.4
request: |
  GET /index.html HTTP/1.1
  Host: example.com
response: |
  HTTP/1.1 302 Found
  Location: https://example.com/
```

```jsonnet
{
  version: '1.0',
  context: {
    eventType: 'viewer-response',
  },
  viewer: {
    ip: '1.2.3.4',
  },
  request: |||
    GET /index.html HTTP/1.1
    Host: example.com
  |||,
  response: |||
    HTTP/1.1 302 Found
    Location: https://example.com/
  |||,
}
```

### Chain multiple functions

cfft supports chaining multiple functions. The feature is useful to test the combined function.

```yaml
# cfft.yaml
name: my-function
runtime: cloudfront-js-2.0 # required
function:
  event-type: viewer-request
  functions:
    - function1.js
    - function2.js
  filter_command: "npx esbuild --minify"
testCases:
## ...
```

The `runtime` must be `cloudfront-js-2.0`.

- `function` element allows you to specify multiple function files.
  - `event-type` must be `viewer-request` or `viewer-response`. required.
  - `functions` element is an array of function files.
  - `filter_command` is a command to filter the chained function code. optional.

When you specify the multiple `functions` in `function`, cfft automatically creates a combined function chained with all functions.

The combined function works as the following steps.
1. The first function in the `functions` array is evaluated.
2. The result of the first function is passed to the second function.
3. ...(repeat)

When the event-type is `viewer-response` and any step returns a response object(includes `statusCode`), the response object is returned to the viewer immidiately. The following functions are not evaluated.

The `filter_command` is a command to filter the chained function code. The command must accepts the function code from stdin and outputs the filtered function code to stdout. For example, use `npx esbuild --minify` to minify the function code.

Note: `esbuild --minify` may change identifiers in js code, so it may not work for js file includes `import` syntax.

You can review the generated combined function code with `cfft render` command.

### Use CloudFront KeyValueStore

cfft supports [CloudFront KeyVakueStore](https://docs.aws.amazon.com/ja_jp/AmazonCloudFront/latest/DeveloperGuide/kvs-with-functions.html).

```yaml
# cfft.yaml
name: function-with-kvs
function: function.js
kvs:
  name: hostnames
```

If you specify the `kvs` element in the config file, `cfft test --create-if-missing` creates a KeyValueStore with the name if not exsites, and associates the KeyValueStore with the function. You can use the KeyValueStore in the function code.

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
2024-01-19T22:41:18+09:00 [info] function my-function found
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

### Render function code, event and expect object

`cfft render` renders the function code or event object or expect object to STDOUT.

```
Usage: cfft render [<target>]

render function code

Arguments:
  [<target>]    render target (function,event,expect)

Flags:
       --test-case="" test case name (for target event or expect)
```

```console
$ cfft render
```

You can use `cfft render` to check the function code after rendering the template syntax.

`cfft render event --test-case=foo` renders the event object of the test case named 'foo'.

The `--test-case` flag is available only for the `event` and `expect` targets. If `--test-case` is not specified, cfft renders the event or expect object of the first test case.

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

## Cooperate with Terraform

cfft is desined to use with [Terraform](https://www.terraform.io).

cfft has two methods to cooperate with Terraform, `cfft tf` generates tf.json, and `cfft tf --external` generates JSON for Terraform's external data sources.

### Generate tf.json

`cfft tf` command outputs a JSON defines a Terraform [aws_cloudfront_function](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudfront_function) resource. The JSON file is read by Terraform as [JSON Configuration Syntax](https://developer.hashicorp.com/terraform/language/syntax/json).

```console
$ cfft tf > cff.tf.json
```

cff.tf.json
```json
{
  "//": "This file is generated by cfft. DO NOT EDIT.",
  "resource": {
    "aws_cloudfront_function": {
      "some-function": {
        "name": "some-function",
        "runtime": "cloudfront-js-2.0",
        "code": "....(function code)....",
        "comment": "comment of the function // sha256:...",
      }
    }
  }
}
```

Terraform creates or updates the function with the JSON. If you want to publish the function into the "LIVE" stage by `terraform apply`, set `cfft tf --publish` flag.

If you want to run `cfft test` before `terraform (plan|apply)`, execute `cfft test --create-if-missing` to create a function in the DEVELOPMENT stage.

In this case, you have to define the `import` block in a `.tf` file because the function is already created by cfft, but Terraform does not know the function. After `terraform apply`, you can remove the `import` block.

```hcl
import {
  to = aws_cloudfront_function.some-function
  id = "some-function"
}
```

When a function code contains `${`, this syntax conflicts with Terraform's interpolation syntax. In this case, cfft outputs the function code into Terraform's variable, and the aws_cloudfront_function resource refers to the variable.

The variable's default value is not parsed as Terraform's interpolation syntax. See also [variable-blocks](https://developer.hashicorp.com/terraform/language/syntax/json#variable-blocks).

```json
{
  "//": "This file is generated by cfft. DO NOT EDIT.",
  "variable": {
    "code_of_some-function": {
      "type": "string",
      "default": "...(function code)..."
    }
  },
  "resource": {
    "aws_cloudfront_function": {
      "some-function": {
        "name": "some-function",
        "code": "${var.code_of_some-function}",
        "comment": "comment of the function // sha256:...",
        "publish": true,
        "runtime": "cloudfront-js-2.0"
      }
    }
  }
}
```

`cfft tf --resource-name foo` outputs the JSON with the tf resource name `foo` instead of the function name.

### Generate JSON for Terraform external data sources

`cfft tf --external` command outputs a JSON for Terraform [external data sources](https://registry.terraform.io/providers/hashicorp/external/latest/docs/data-sources/external).

```console
$ cfft tf --external
{
  "name": "some-function",
  "code": "....(function code)....",
  "comment": "comment of the function // sha256:...",
  "runtime": "cloudfront-js-2.0"
}
```

You can define the `aws_cloudfront_function` resource with the `data.external` data source calling `cfft tf --external`.

When you run `terraform apply`, `cfft tf --external` is executed and the function is created or updated. If `publish` is true, Terraform will publish the function into the "LIVE" stage.

Note: `cfft tf --external` does not output a `publish` attribute because the external data source does not accept non-string values.

```hcl
resource "aws_cloudfront_function" "some-function" {
  name    = data.external.some-function.result["name"]
  runtime = data.external.some-function.result["runtime"]
  code    = data.external.some-function.result["code"]
  comment = data.external.some-function.result["comment"]
  publish = true
}

data "external" "some-function" {
  program = ["cfft", "--config", "cfft.yaml", "tf", "--external"]
}
```

If you want to execute `cfft test` before `terraform apply`, or you use the KeyValueStore, `cfft test --create-if-missing` creates a KeyValueStore and associates the KeyValueStore with the function. In this case, you have to define the `import` block in a `.tf` file because the function is already created by cfft, but Terraform does not know the function. After `terraform apply`, you can remove the `import` block.

```hcl
import {
  to = aws_cloudfront_function.some-function
  id = "some-function"
}
```

## LICENSE

MIT

## Author

Fujiwara Shunichiro
