<div align="center">
    <h1>Relay Util</h1>
    <img src="https://github.com/commoddity/relay-util/blob/main/.github/relay-util.png" alt="Relay Util logo" width="600"/>
    <br/>
    <big>Relay Util is a command-line tool written in Go that sends relays to a specified URL using configurable concurrency, delay and timeout settings. It supports various flags to control its behavior and is primarily designed for load testing services.</big>
</div>
<br/>

## Installation

```bash
go install github.com/commoddity/relay-util/v2@latest
```

## Usage

```bash
relay-util -u=<url> -d=<data> -H=<header> -x=<executions> -g=<goroutines> -w=<wait> -t=<timeout> [-b] 
```

### Flags

- `-u, --url`: [REQUIRED] The URL to send the requests to.
- `-d, --data`: [OPTIONAL] The request body that will be sent as the relay. Must be a valid JSON string.
- `-H, --headers`: [OPTIONAL] Custom headers to include in the relay request, specified as -H "Header-Name: value". Can be used multiple times. If required, the Service ID must be specified as `target-service-id`.
- `-x, --executions`: [OPTIONAL] The total number of relays to execute. This defines how many times the relay will be sent.
- `-g, --goroutines`: [OPTIONAL] The level of concurrency for sending relays. This defines how many goroutines will be used to send relays in parallel.
- `-w, --wait`: [OPTIONAL] The delay between individual relay requests, measured in milliseconds. This helps to control the rate at which relays are sent.
- `-t, --timeout`: [OPTIONAL] The timeout for individual relay requests, measured in seconds.
- `-b, --success-bodies`: [OPTIONAL] A flag that, when set, will cause the bodies of successful relay responses to be displayed in the log output.

## Example Usage

```bash
relay-util \
-u=https://path.rpc.grove.city/v1 \
-H="target-service-id: F00C" \
-H="Authorization: api_key_123" \
-H="Custom-Header: value" \
-d='{"jsonrpc": "2.0", "id": 1, "method": "eth_blockNumber", "params": []}' \
-x=3000 \
-w=10 \
-g=500 \
-t=20
```
