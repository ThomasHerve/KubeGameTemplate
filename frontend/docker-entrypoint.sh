#!/bin/sh

sed -i "s|BACKEND_URL_TEMPLATE|${BACKEND_URL}|g" /files/index.html
sed -i "s|BACKEND_PROTOCOL_TEMPLATE|${BACKEND_PROTOCOL}|g" /files/index.html

exec nginx -g "daemon off;"