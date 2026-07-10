#!/bin/sh

envsubst '${BACKEND_URL}' \
    < /usr/share/nginx/html/config.template.js \
    > /usr/share/nginx/html/config.js


exec nginx -g "daemon off;"