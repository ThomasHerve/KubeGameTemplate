#!/bin/sh

envsubst '${BACKEND_URL} ${BACKEND_PROTOCOL}' \
    < /usr/share/nginx/html/index.html \
    > /usr/share/nginx/html/index.html


exec nginx -g "daemon off;"