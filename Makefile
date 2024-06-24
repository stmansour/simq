# test comment
DIRS=util data dispatcher
DIST=dist 
TEST_FAILURE_FILE=fail
# Temporary file for storing start time
TIMER_FILE := .build_timer

.PHONY: install-tools golint staticcheck test

doit:
	for dir in $(DIRS); do make -C $$dir;done

clean:
	rm -rf dist
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
	for dir in $(DIRS); do make -C $$dir package;done
	if [ -d /usr/local/simq/bin/data ]; then cd dist/simq ; rm -rf data ; cp -r /usr/local/simq/bin/data . ; fi
	cd dist ; rm -f simq.tar* ; tar cvf simq.tar simq ; gzip simq.tar

all: starttimer clean dispatcher package test stoptimer
	@echo "Completed"

build: starttimer clean psim package stoptimer

stats:
	@find . -name "*.go" | srcstats

release:
	@echo "Not implemented"

refmt:
	fmt design.txt > design.txt1 ; mv design.txt1 design.txt

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

