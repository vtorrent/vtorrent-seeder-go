#!/bin/bash

LOGDIR=${HOME}/vtorrent-seeder-go-logs/

mkdir -p ${LOGDIR}

gzip ${LOGDIR}/*.log

cd
echo
echo "======= Run the Go Language vtorrent-seeder-go ======="
echo
${HOME}/go/bin/vtorrent-seeder-go -p 53 -v -w 8880 -netfile ${HOME}/vtorrent.json 2>&1 | tee ${LOGDIR}/$(date +%F-%s)-goseeder.log
