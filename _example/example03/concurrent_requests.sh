#!/bin/bash

# Make 5 concurrent requests to http://localhost:8000/long

for i in {1..5}
do
  curl -s http://localhost:8000/long &
done

wait
echo "All 5 requests completed."