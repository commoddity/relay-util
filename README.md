<div align="center">
    <h1>Relay Util</h1>
    <img src=".github/relay-util.png" alt="Relay Util logo" width="600"/>
    <br/>
    <big>Relay Util is a command-line tool written in Go that sends relays to a specified blockchain and logs the results.</big>
    <big>It supports various flags to control its behavior.</big>
</div>
<br/>

## Installation

```bash
go install github.com/commoddity/relay-util/v2@latest
```

## Usage

```bash
relay-util -c=<chain> -e=<environment> -p=<planType> -x=<executions> -r=<request> [-l] [-s] [-o=<overrideURL>] [-g=<goroutines>] [-d=<delay>]
```

### Flags

- `-c, --chain`: The chain alias to which the relays will be sent.
- `-e, --env`: The environment where the relays will be sent. Valid values are 'production' or 'staging'.
- `-p, --planType`: The plan type under which the relays are sent. Valid values are 'starter' or 'enterprise'.
- `-x, --executions`: The total number of relays to execute. This defines how many times the relay will be sent.
- `-r, --request`: The JSON RPC request body that will be sent as the relay. Must be a valid JSON string.
- `-l, --local`: A flag to indicate if the relays should be sent to a local environment. Useful for testing locally.
- `-s, --success-bodies`: A flag that, when set, will cause the bodies of successful relay responses to be displayed in the log output.
- `-o, --override-url`: A custom URL to override the default endpoint. This allows you to specify a different URL for sending relays.
- `-g, --goroutines`: The level of concurrency for sending relays. This defines how many goroutines will be used to send relays in parallel.
- `-d, --delay`: The delay between individual relay requests, measured in milliseconds. This helps to control the rate at which relays are sent.

## Example Usage

```bash
relay-util \
-c=eth-mainnet \
-e=production \
-p=enterprise \
-r='{"jsonrpc": "2.0", "id": 1, "method": "eth_blockNumber" }' \
-x=1000 \
-g=100 \
-d=25
```

## Environment Variables

The project uses environment variables to configure various aspects of its operation. These variables are loaded from a `.env.relayutil` file in the user's home directory. If this file does not exist, the program will walk through the setup for creating it when `relay-util` is run.
