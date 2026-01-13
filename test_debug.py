#!/usr/bin/env python3
import socket
import json
import sys

# Connect to the Unix socket
sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
sock.connect('/tmp/clai.sock')

# Send the command
cmd = {"command": "inspect_styles"}
data = json.dumps(cmd).encode()
sock.send(data)

# Read response
response = sock.recv(4096)
print(response.decode())

sock.close()