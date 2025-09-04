

################# NGINX #################

FROM nginx:alpine-slim AS nginx-base

WORKDIR /srv/www

COPY ./nginx.conf /etc/nginx/conf.d/default.conf

COPY ./src/backend/ ./

EXPOSE 8080

