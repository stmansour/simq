TOP=..
BINDIR=${TOP}/dist/simq
THISDIR="dispatcher"
TEST_FAILURE_FILE=fail
BUILD_TIME := $(shell date "+%Y%m%dT%H%M%S")
MACHINENAME := $(shell hostname)
OS := $(shell uname -s)

dispatcher: *.go
	go vet
	golint
	staticcheck
	go build -ldflags "-X 'github.com/stmansour/simq/util.buildID=$(BUILD_TIME)'" -o dispatcher
	@echo "*** $(THISDIR): completed $(THISDIR) ***"

clean:
	go clean
	rm -rf config.* *.prof *.out qdconfigs extres.json5 fail [a-z][0-9] DISPATCHER.log serverresponse simulations simstats* received_config.json5 finrep*
	@echo "*** ${THISDIR}: completed clean ***"

test: config
	@touch $(TEST_FAILURE_FILE)
	@go test -timeout 15s github.com/stmansour/simq/${THISDIR}
	@./functest.sh && rm -f $(TEST_FAILURE_FILE) 
	@go test -coverprofile=coverage.out -coverpkg=./
	@echo "*** ${THISDIR}: completed test ***"

config:
	@if [ ! -f ./config.json5 ]; then cp /usr/local/plato/bin/config.json5 .;echo "copied config.json5";fi
	@if [ ! -f ./extres.json5 ]; then cp ~/.vault/extres.json5 .;echo "copied extres.json5";fi

coverage:
	go tool cover -html=coverage.out 

package:
ifeq ($(OS),Darwin)
	@echo "Skipping package commands on macOS."
else
	mkdir -p ${BINDIR}/dispatcher
	cp dispatcher ${BINDIR}/dispatcher/
	@echo "*** ${THISDIR}: completed package ***"
endif

release: config
	mkdir -p /usr/local/simq/dispatcher/
	cp dispatcher /usr/local/simq/dispatcher/
	@if [ "${MACHINENAME}" = "StevesMcBookPro.attlocal.net" ]; then cp dispatcher.json5 /usr/local/simq/dispatcher/; fi
	

releaseman:
	if [ -d /usr/local/share/man/man1 ] && [ -w /usr/local/share/man/man1 ]; then cp dispatcher.1 /usr/local/share/man/man1/ ; fi
	@echo "*** ${THISDIR}: completed releaseman ***"

refmt:
	fmt dispatcher.1 > dispatcher.11 ; mv dispatcher.11 dispatcher.1

