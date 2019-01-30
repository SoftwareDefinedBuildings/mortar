#!/bin/bash
mkdir -p certs
if [ ! \( -f "certs/mortar-client.key" -a -f "certs/mortar-client.pem" \) ]; then
    openssl req -x509 -nodes -days 365 -newkey rsa:2048 -keyout certs/mortar-client.key -out certs/mortar-client.pem
fi
