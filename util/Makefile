TOP=..
THISDIR="util"
BINDIR=${TOP}/dist/simq

util: *.go
	@touch fail
	go vet
	golint
	staticcheck
	go build
	go install
	@rm -f fail
	@echo "*** ${THISDIR}: completed compile ***"

clean:
	rm -rf *.out fail
	@echo "*** ${THISDIR}: completed clean ***"

install:
	go install

test:
	@touch fail
	@go test github.com/stmansour/psim/util && rm -f $(TEST_FAILURE_FILE)
	@go test -coverprofile=coverage.out || (echo "*** ERROR ***  go test failed!" && exit 1)
	@echo "*** ${THISDIR}: completed test ***"
	@rm -f fail

coverage:
	go tool cover -html=coverage.out

package:
	@echo "*** ${THISDIR}: completed package ***"
