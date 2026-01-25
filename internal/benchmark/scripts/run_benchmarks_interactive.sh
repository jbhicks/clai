#!/bin/bash

# Interactive Benchmark Test Runner
# Simple loop through tests 1-21 using make benchmark TEST=#

echo "=== Model Benchmark Tests ==="
echo ""

# Get test names for display
tests=($(grep -A1 "Name:" internal/llm/benchmarks.go | grep "Name:" | sed 's/.*Name: *"//' | sed 's/",//'))
total=21

echo "Found $total tests in unified suite"
echo ""

for i in $(seq 1 $total); do
    test_name="${tests[$((i-1))]}"
    echo "[$i/$total] $test_name"
    
    while true; do
        read -p "Press ENTER to run test #$i, 'r' to rerun, 's' to stop, 'q' to quit: " choice
        case $choice in
            '' ) 
                echo "Running test #$i: $test_name"
                echo "Command: make benchmark TEST=$i"
                echo ""
                make benchmark TEST=$i
                echo ""
                break
                ;;
            [Rr]* ) 
                echo "Rerunning test #$i: $test_name"
                echo "Command: make benchmark TEST=$i"
                echo ""
                make benchmark TEST=$i
                echo ""
                break
                ;;
            [Ss]* ) 
                echo "Stopping at test #$i ($test_name)"
                echo "Tests completed: $i/$total"
                exit 0
                ;;
            [Qq]* ) 
                echo "Quitting benchmark suite"
                echo "Tests completed: $((i-1))/$total"
                exit 0
                ;;
            * ) 
                echo "Press ENTER to run test #$i, 'r' to rerun, 's' to stop, or 'q' to quit"
                ;;
        esac
    done
    
    echo "----------------------------------------"
    echo ""
done

echo "=== All Benchmark Tests Complete ==="
echo "All $total tests processed"