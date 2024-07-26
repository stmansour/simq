DIRS=linux macos
THISDIR=installers

doit:
	@echo "Done in ${THISDIR}"

package:
	for dir in $(DIRS); do make -C $$dir package;done

clean:
	for dir in $(DIRS); do make -C $$dir clean;done


test:
	@echo "Done in ${THISDIR}"
