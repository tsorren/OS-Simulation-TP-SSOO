#!/bin/bash
echo "========================="
echo "IP: $(ifconfig | grep -m 1 'inet ' | grep -v '127.0.0.1' | awk '{print $2}')"
echo "========================="  