# `e7mon`

> Tool for monitoring your Ethereum clients. Client-agnostic as it queries the standardized JSON-RPC APIs.

<img src="./docs/img/output.png" width=1200>

## Getting started
### Requirements
* [Install Go](https://golang.org/doc/install) (minimum version 1.16)

* Install dependencies:
```bash
sudo apt-get install libpcap-dev build-essential
```
### Installation
* **With Go**

```bash
# Install e7mon
go get -v github.com/netbound/e7mon

# Set necessary capabilities (required for latency scans)
sudo setcap 'CAP_NET_RAW,CAP_NET_ADMIN=eip' $GOBIN/e7mon
```
* **From source**

Build the binary:
```bash
git clone https://github.com/netbound/e7mon
cd e7mon
make install
```
This will put the binary in `$GOBIN`, which is in your `$PATH`. You can verify the installation with:
```bash
e7mon client-versions
```

## Usage

Requires the following APIs to be exposed on your execution client:
* `eth`: querying the chain
* `net`: getting P2P stats
* `web3`: client information

And the API should be enabled on your beacon node as well.

### Running
First, generate the YAML [config file](./config/config.yml). This is included in the binary and will be written to `$HOME/.config/e7mon/config.yml`.
```bash
e7mon init
```
Next up, change the config to match your settings and preferences. Important to fill out is the correct API endpoint for each client.

Now run the monitor program:
```bash
# Monitor both execution client, beacon node and validator
e7mon

# Execution client only
e7mon execution

# Beacon node only
e7mon beacon

# Validator only
e7mon validator
```

Use the help command for all the options:
```
e7mon help

NAME:
   e7mon - Monitors your Ethereum clients

USAGE:
   e7mon [global options] command [command options] [arguments...]

COMMANDS:
   init, i              initializes configs
   client-versions, cv  prints client versions
   execution, e         monitors the execution client (eth1)
   beacon, b            monitors the beacon node (eth2)
   validator, v         monitors the validator (eth2)
   help, h              Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h  show help (default: false)
```


## Features
- Execution monitor
	- [x] Block monitor
	- [x] P2P stats
   - [ ] MEV alerts
	- [ ] More generic stats
- Beacon monitor
	- [x] Block monitor
	- [x] P2P stats
      - [x] Peers avg latency
	- [ ] More generic stats
   - [ ] Finalized checkpoints
- Validator monitor
   - [ ] Attestations
   - [ ] Produced blocks
   - [ ] Sync committees
   - [ ] Rewards
   - [ ] Validator stats
- [ ] Verbosity levels
- [ ] Integrate with beaconcha.in
- [ ] Messaging services


## Disclaimer
This is alpha software. If you find bugs, please submit an issue. Things can and will break. Tested with the Erigon
execution client and Lighthouse consensus node.

## Sources:
* https://ethereum.github.io/beacon-APIs/#/
* https://geth.ethereum.org/docs/rpc/server
* https://notes.ethereum.org/@serenity/handbook
* https://github.com/ethereum/consensus-specs
* https://github.com/ethereum/consensus-specs/blob/dev/specs/phase0/beacon-chain.md
* https://github.com/attestantio/go-eth2-client