#!/bin/bash
if [ ! \( -f "mortar-client.key" -a -f "mortar-client.pem" \) ]; then
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout mortar-client.key -out mortar-client.pem
fi
