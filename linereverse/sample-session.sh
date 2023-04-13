#!/bin/bash
SESSION_ID=12345678

echo "/connect/${SESSION_ID}/" | nc -4u 127.0.0.1 9999
