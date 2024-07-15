TOP=..
BINDIR=${TOP}/dist/simq
THISDIR="simd"
TEST_FAILURE_FILE=fail
BUILD_TIME := $(shell date "+%Y-%m-%d %H:%M:%S")
MACHINENAME := $(shell hostname)

simd: *.go
	go vet
	golint
	staticcheck
	go build -ldflags "-X 'github.com/stmansour/simq/util.buildID=$(BUILD_TIME)'" -o simd
	@echo "*** $(THISDIR): completed $(THISDIR) ***"

clean:
	go clean
	rm -rf *.prof *.out extres.json5 fail [a-z][0-9] DISPATCHER.log serverresponse config.json5 simulations finrep.csv simstats.csv
	@echo "*** ${THISDIR}: completed clean ***"

test: config
	@echo "*** ${THISDIR}: completed test ***"

config:
	@if [ ! -f ./config.json5 ]; then cp /usr/local/plato/bin/config.json5 .;echo "copied config.json5";fi
	@if [ ! -f ./extres.json5 ]; then cp ~/.vault/extres.json5 .;echo "copied extres.json5";fi
	@if [ "${MACHINENAME}" = "StevesMcBookPro.attlocal.net" ]; then cp simdconfsman.json5 simdconf.json5; echo "simdconfig set for ${MACHINENAME}"; fi
	@if [ "${MACHINENAME}" = "plato" ]; then cp simdconfplato.json5 simdconfig.json5; echo "simdconfig set for ${MACHINENAME}"; fi
	@if [ "${MACHINENAME}" = "Steves-2020-Pro.attlocal.net" ]; then cp simdsmanx86.json5 simdconfig.json5; echo "simdconfig set for ${MACHINENAME}"; fi

coverage:
	go tool cover -html=coverage.out 

package:
	cp simd ${BINDIR}/
	@echo "*** ${THISDIR}: completed package ***"

release:
	mkdir -p /usr/local/simq/simd
	cp simd /usr/local/simq/simd/

releaseman:
	if [ -d /usr/local/share/man/man1 ] && [ -w /usr/local/share/man/man1 ]; then cp simd.1 /usr/local/share/man/man1/ ; fi
	@echo "*** ${THISDIR}: completed releaseman ***"

refmt:
	fmt simd.1 > simd.11 ; mv simd.11 simd.1

