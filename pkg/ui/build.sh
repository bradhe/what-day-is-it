#!/bin/bash

MD5SUM=`which md5sum`
TARGETS="./src/ ./dist/ ./package.json"

CHECKSUM_FILE=./.what-day-is-it.last-build.checksum
TMP_CHECKSUM_FILE=/tmp/.what-day-is-it.checksum.tmp

find ${TARGETS} -type f -exec ${MD5SUM} "{}" + > ${TMP_CHECKSUM_FILE}

function do_build {
	exec npm run-script build
}

function update_checksum {
	mv ${TMP_CHECKSUM_FILE} ${CHECKSUM_FILE}
}

if [ ! -f ${CHECKSUM_FILE} ];
then
	update_checksum
	do_build
elif [ "$(diff ${CHECKSUM_FILE} ${TMP_CHECKSUM_FILE})" != "" ];
then
	update_checksum
	do_build
else
	# Nothing to see here.
	exit 0
fi