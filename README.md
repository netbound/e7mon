# e7mon

Tool for monitoring your Ethereum clients.

## Usage
Install the binary:
```bash
go install github.com/jonasbostoen/e7mon
```
Check the installation:
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
Now we have to generate the configuration file. This will be at `$HOME/.config/e7mon/config.yml`.
```bash
e7mon init
```
The final step is to modify the configuration file to match your settings and preferences. After this
you can run the program:
```bash
# Monitor both clients
e7mon

# Execution client only
e7mon execution

# Beacon node only
e7mon beacon
```

## Todo
- [ ] Think about all the different config options
- Execution monitor
	- [x] Block monitor
	- [x] P2P stats
	- [ ] More generic stats

- Beacon monitor
	- [ ] Block monitor
	- [ ] P2P stats
	- [ ] More generic stats
- Validator monitor
- [ ] Write API connectors for message services

Sources:
* https://ethereum.github.io/beacon-APIs/#/
* https://github.com/attestantio/go-eth2-client