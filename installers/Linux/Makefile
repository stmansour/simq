TOP=../..
DIST=${TOP}/dist
THISDIR=installers/Darwin
OS := $(shell uname -s)

build:
	@echo "done in ${THISDIR}"

clean:
	@echo "done in ${THISDIR}"

test:
	@echo "done in ${THISDIR}"

package:
	@mkdir -p ${TOP}/dist2/simq
	@cp -r ${THISDIR}/linuxdispintall.sh ${TOP}/dist2/simq
	@cp linuxinstall.sh "${DIST}/simq/"
	@echo "done in ${THISDIR}"
