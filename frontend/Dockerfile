FROM nginxinc/nginx-unprivileged:alpine

USER root

RUN apk add --no-cache gettext

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY docker-entrypoint.sh /docker-entrypoint.sh

RUN chmod +x /docker-entrypoint.sh

USER 101

COPY public/index.html /var/cache/nginx/index.html

ENTRYPOINT ["/docker-entrypoint.sh"]
