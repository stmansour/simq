TOP=..
BINDIR=${TOP}/dist/simq
THISDIR=data
TEST_FAILURE_FILE=fail

data: *.go
	go vet
	@golint ./... | grep -v "use underscores in Go names;" | ( ! grep . )
	staticcheck
	go build
	go install
	@echo "*** ${THISDIR}: completed ${THISDIR} ***"

clean:
	go clean
	@rm -rf *.out extres.json5
	@echo "*** ${THISDIR}: completed clean ***"

test: config
	@touch $(TEST_FAILURE_FILE)
	@if [ ! -f ./extres.json5 ]; then echo "*** WARNING *** No vault, skipping test"; else \
	@go test github.com/stmansour/psim/${THISDIR} && rm -f $(TEST_FAILURE_FILE)	\
	@go test -coverprofile=coverage.out || (echo "*** ERROR ***  go test failed!" && exit 1) \
	fi
	@echo "*** ${THISDIR}: completed test ***"

config:
	@if [ -f ~/.vault/extres.json5 ]; then cp ~/.vault/extres.json5 .; fi;

coverage:
	go tool cover -html=coverage.out

package:
	@echo "*** ${THISDIR}: completed package ***"
