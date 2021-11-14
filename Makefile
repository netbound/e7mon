GOPATH = $(shell go env GOPATH)

run:
	go run ./cmd/e7mon

execution:
	go run ./cmd/e7mon execution

beacon:
	go run ./cmd/e7mon beacon

install:
	@echo "Installing in $(GOPATH)/bin"
	go install
	@echo "Setting net capacity (requires root privileges)"
	sudo setcap 'CAP_NET_RAW,CAP_NET_ADMIN=eip' "$(GOPATH)/bin/e7mon"