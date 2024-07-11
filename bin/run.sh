#!/bin/bash

# Define the path to your YAML file
yaml_file="config/config.yml"

# Use grep to find the lines and awk to print the second column
N=$(grep '^N:' $yaml_file | awk '{print $2}')
R=$(grep '^R:' $yaml_file | awk '{print $2}')
D=$(grep '^D:' $yaml_file | awk '{print $2}')
L=$(grep '^L:' $yaml_file | awk '{print $2}')

# Print the results
echo "N: $N"
echo "R: $R"
echo "D: $D"
echo "L: $L"

# Start the bulletin board
echo "Running bulletin board in background"
go run cmd/bulletin-board/main.go > "out/bulletin_board.log" 2>&1 &
bb_pid=$!

# Declare an array to store all process IDs
declare -a pids
pids+=("$bb_pid")  # Include the bulletin board's PID for later termination

# Start nodes and collect their PIDs
for (( id=1; id<=N; id++ )); do
    echo "Running node.go with ID: $id in the background"
    go run cmd/node/main.go -id "$id" > "out/nodes/$id.log" 2>&1 &
    pids+=($!)
done

# Start clients and collect their PIDs
for (( id=1; id<=R; id++ )); do
    echo "Running client.go with ID: $id in the background"
    go run cmd/clients/main.go -id "$id" > "out/clients/$id.log" 2>&1 &
    pids+=($!)
done

# Define the countdown time
countdown_time=10

echo ""

# Start the countdown
for (( i=countdown_time; i>0; i-- )); do
    echo "Starting metrics in $i seconds..."
    sleep 1
done

echo "Go!"

echo "Running metrics"
go run cmd/metrics/metrics.go

# Terminate all processes after collecting metrics
echo "Terminating all started processes..."
for pid in "${pids[@]}"; do
  echo "Terminating process with PID: $pid"
  kill $pid
done

pkill -f '/var/folders/ss/8mkcky815pdf4wxyn2trh0wh0000gn/T/go-build'

