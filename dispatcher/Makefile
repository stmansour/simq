TOP=../..
BINDIR=${TOP}/dist/simq
THISDIR="dispatcher"
TEST_FAILURE_FILE=fail
THISDIR := $(notdir $(PWD))
BUILD_TIME := $(shell date "+%Y-%m-%d_%H:%M:%S")

sim: *.go
	go vet
	golint
	staticcheck
	go build -ldflags "-X 'github.com/stmansour/psim/util.buildID=$(BUILD_TIME)'" -o dispatcher
	@echo "*** $(THISDIR): completed $(THISDIR) ***"

clean:
	go clean
	rm -rf *.prof *.out extres.json5 fail
	@echo "*** ${THISDIR}: completed clean ***"

test: config
	@touch $(TEST_FAILURE_FILE)
	# @go test -coverprofile=coverage.out -coverpkg=../../...
	@go test -coverprofile=coverage.out -coverpkg=./
	@echo "*** ${THISDIR}: completed test ***"

config:
	if [ ! -f ./extres.json5 ]; then cp ~/.vault/extres.json5 .;fi

coverage:
	go tool cover -html=coverage.out 

package:
	@echo "*** ${THISDIR}: completed package ***"

releaseman:
	if [ -d /usr/local/share/man/man1 ] && [ -w /usr/local/share/man/man1 ]; then cp dispatcher.1 /usr/local/share/man/man1/ ; fi
	@echo "*** ${THISDIR}: completed releaseman ***"

refmt:
	fmt dispatcher.1 > dispatcher.11 ; mv dispatcher.11 dispatcher.1

