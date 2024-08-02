#!/bin/bash

increments=5
from=40
to=$((from + increments))

while [ $to -lt 322 ]; do
  to=$((to + increments))
  from=$((from + increments))
  echo ""
  echo "-----------------------------------"
  echo "go run cmd/main.go -from ${from} -to ${to}"
  echo ""
  go run cmd/main.go -from "${from}" -to "${to}"
done

echo "all done"
