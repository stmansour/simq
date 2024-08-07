# test comment
DIRS=util data dispatcher psq simd installers
DIST=dist 
TEST_FAILURE_FILE=fail
# Temporary file for storing start time
TIMER_FILE := .build_timer

OS := $(shell uname -s | tr '[:upper:]' '[:lower:]' | sed 's/linux/linux/; s/darwin/macos/; /macos\|linux/!s/.*/unknown/')
CPU := $(shell uname -m | sed 's/x86_64/x86_64/; s/arm64\|aarch64/arm64/; /x86_64\|arm64/!s/.*/unknown/')

.PHONY: install-tools golint staticcheck test deps

doit:
	for dir in $(DIRS); do make -C $$dir;done

deps:
	go mod download
	go mod tidy


clean:
	rm -rf dist dist2
	for dir in $(DIRS); do make -C $$dir clean;done

test: do_tests check_tests

do_tests:
	@echo "------------------------------------------------------------------"
	@echo "                          TESTS"
	@echo "------------------------------------------------------------------"
	for dir in $(DIRS); do make -C $$dir test;done

check_tests:
	@echo "------------------------------------------------------------------"
	@echo "                      TESTS RESULTS"
	@echo "------------------------------------------------------------------"
	@echo
	@echo "UNIT TEST CODE COVERAGE"
	@echo "======================="
	@for dir in $(shell find . -name coverage.out); do \
		if [ "$$dir" != "./apps/simulator/coverage.out" ]; then \
		coverage=$$(go tool cover -func=$$dir | grep total | awk '{print $$NF}') ; \
		echo "`dirname $$dir` : $$coverage"; \
		fi \
	done
	@echo
	@echo "FUNCTIONAL TEST CODE COVERAGE"
	@echo "============================="
	@for dir in $(shell find ./apps -name coverage.out); do \
		coverage=$$(go tool cover -func=$$dir | grep total | awk '{print $$NF}') ; \
		echo "`dirname $$dir` : $$coverage"; \
	done
	@echo
	@if test -n "$(shell find . -name ${TEST_FAILURE_FILE})"; then \
		echo "Tests have failed in the following directories:"; \
		find . -name "${TEST_FAILURE_FILE}" -exec dirname {} \; ; \
			exit 1; \
		else \
			echo "****************************"; \
			echo "***   ALL TESTS PASSED   ***"; \
			echo "****************************"; \
		fi

package:
	mkdir -p dist/simq/bin
	for dir in $(DIRS); do make -C $$dir package;done
	./mkdist.sh -c

post:
	./mkdist.sh -p

all: starttimer clean doit package test stoptimer
	@echo "Completed"

build: starttimer clean doit package stoptimer

stats:
	@find . -name "*.go" | srcstats

release:
	@echo "use the 'make post' target"

refmt:
	fmt design.txt > design.txt1 ; mv design.txt1 design.txt
	fmt systemdesign.txt > systemdesign.txt1 ; mv systemdesign.txt1 systemdesign.txt

install-tools:
	go get -u github.com/go-sql-driver/mysql
	go install golang.org/x/lint/golint@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest

starttimer:
	@echo $$(date +%s) > $(TIMER_FILE)

stoptimer:
	@start=$$(cat $(TIMER_FILE)); \
	end=$$(date +%s); \
	elapsed=$$((end - start)); \
	hours=$$((elapsed / 3600)); \
	minutes=$$(( (elapsed / 60) % 60 )); \
	seconds=$$((elapsed % 60)); \
	if [ $$hours -gt 0 ]; then \
		echo "Elapsed time: $$hours hour(s) $$minutes minute(s) $$seconds second(s)"; \
	elif [ $$minutes -gt 0 ]; then \
		echo "Elapsed time: $$minutes minute(s) $$seconds second(s)"; \
	else \
		echo "Elapsed time: $$seconds second(s)"; \
	fi; \
	rm -f $(TIMER_FILE)

