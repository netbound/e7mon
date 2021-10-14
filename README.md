# e7mon

Tool for monitoring your Ethereum clients. Client-agnostic as it queries the standardized JSON-RPC APIs.

## Installation
**With Go**
```bash
go install github.com/jonasbostoen/e7mon
```
## Usage
First, generate the YAML config file. This is included in the binary and will be written to `$HOME/.config/e7mon/config.yml`.
```bash
e7mon init
```
Next up, change the config to match your settings and preferences. Important to fill out is the correct API endpoint for each client.

Now run the monitor program:
```bash
# Monitor both clients
e7mon

# Execution client only
e7mon execution

# Beacon node only
e7mon beacon
```

Use the help command for all the options:
```
e7mon help

NAME:
   e7mon - Monitors your Ethereum clients

USAGE:
   e7mon [global options] command [command options] [arguments...]

COMMANDS:
   init       initializes configs
   execution  monitors the execution client (eth1)
   beacon     monitors the beacon node (eth2)
   help, h    Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h  show help (default: false)
```

## Example output
![Example output](./img/output.png)

## Todo
- [ ] Think about all the different config options
- Execution monitor
	- [x] Block monitor
	- [x] P2P stats
	- [ ] More generic stats

- Beacon monitor
	- [x] Block monitor
	- [x] P2P stats
	- [ ] More generic stats
- Validator monitor
- [ ] Write API connectors for message services

Sources:
* https://ethereum.github.io/beacon-APIs/#/
* https://github.com/attestantio/go-eth2-client