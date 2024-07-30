DIRS=Linux Darwin
THISDIR=installers
OS=$(shell uname)

build:
	@echo "Done in ${THISDIR}"

clean:
	@echo "Done in ${THISDIR}"

test:
	@echo "Done in ${THISDIR}"

package:
	@for dir in $(DIRS); do if [ $$dir = $(OS) ]; then make -C $$dir package; fi ; done

