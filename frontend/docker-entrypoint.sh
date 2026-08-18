#!/bin/sh

sed -i "s|BACKEND_URL_TEMPLATE|${BACKEND_URL}|g" /usr/share/nginx/html/index.html
sed -i "s|BACKEND_PROTOCOL_TEMPLATE|${BACKEND_PROTOCOL}|g" /usr/share/nginx/html/index.html

exec nginx -g "daemon off;"