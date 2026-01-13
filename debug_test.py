#!/usr/bin/env python3

import socket
import json
import sys

def send_debug_command(command, args=None):
    """Send a command to the CLAI debug server via Unix socket."""
    try:
        client = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        client.connect("/tmp/clai.sock")

        message = {"command": command}
        if args:
            message["args"] = args

        json_message = json.dumps(message)
        client.send(json_message.encode('utf-8'))
        response = client.recv(8192).decode('utf-8')

        client.close()
        return response

    except Exception as e:
        print(f"Error communicating with debug server: {e}", file=sys.stderr)
        return None

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: python3 debug_test.py <command> [args_json]")
        sys.exit(1)

    command = sys.argv[1]
    args = None

    if len(sys.argv) > 2:
        try:
            args = json.loads(sys.argv[2])
        except json.JSONDecodeError as e:
            print(f"Invalid JSON args: {e}", file=sys.stderr)
            sys.exit(1)

    response = send_debug_command(command, args)
    if response:
        print(response)
    else:
        sys.exit(1)