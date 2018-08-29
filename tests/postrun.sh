#! /bin/bash
LOGFILE="/tmp/postrun.log"
rm ${LOGFILE}

if [ $# -eq 0 ]; then
  echo "Nothing to do" | tee -a ${LOGFILE}
  exit 0
fi

for argument in "$@"; do
  echo "postrun command wrapper script recieved argument: ${argument}" | tee -a ${LOGFILE}
done
