#!/bin/sh

sed -i "s|BACKEND_URL_TEMPLATE|${BACKEND_URL}|g" /var/cache/nginx/index.html
sed -i "s|BACKEND_PROTOCOL_TEMPLATE|${BACKEND_PROTOCOL}|g" /var/cache/nginx/index.html

exec nginx -g "daemon off;"