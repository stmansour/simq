TOP=..
BINDIR=${TOP}/dist/simq
THISDIR=psq
TEST_FAILURE_FILE=fail
BUILD_TIME := $(shell date "+%Y-%m-%d_%H:%M:%S")

dispatcher: *.go
	go vet
	golint
	staticcheck
	go build -ldflags "-X 'github.com/stmansour/simq/util.buildID=$(BUILD_TIME)'" -o $(THISDIR)
	@echo "*** $(THISDIR): completed $(THISDIR) ***"

clean:
	go clean
	rm -rf *.prof *.out extres.json5 fail [a-z][0-9] psq config.json5 
	@echo "*** ${THISDIR}: completed clean ***"

test: config
	@echo "*** ${THISDIR}: completed test ***"

config:
	cp /usr/local/plato/bin/config.json5 .

coverage:
	go tool cover -html=coverage.out 

package:
	cp ${THISDIR} ${BINDIR}/
	@echo "*** ${THISDIR}: completed package ***"

releaseman:
	if [ -d /usr/local/share/man/man1 ] && [ -w /usr/local/share/man/man1 ]; then cp dispatcher.1 /usr/local/share/man/man1/ ; fi
	@echo "*** ${THISDIR}: completed releaseman ***"

refmt:
	fmt dispatcher.1 > dispatcher.11 ; mv dispatcher.11 dispatcher.1

release:
	cp psq /usr/local/simq/bin/psq
