FROM nginxinc/nginx-unprivileged:alpine

USER root

RUN apk add --no-cache gettext

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY public/ /usr/share/nginx/html/
COPY docker-entrypoint.sh /docker-entrypoint.sh

RUN chmod +x /docker-entrypoint.sh

USER 101

ENTRYPOINT ["/docker-entrypoint.sh"]
