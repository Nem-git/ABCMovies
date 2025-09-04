
################# COMPOSER #################

FROM composer AS composer-base

WORKDIR /srv/www

################# PHP BASE #################

FROM php:fpm-alpine AS backend-base

WORKDIR /srv/www

RUN apk --no-cache add bento4
COPY --from=composer-base /usr/bin/composer /usr/bin/composer

COPY ./core.ini /usr/local/etc/php
COPY ./dev.ini /usr/local/etc/php

COPY ./composer.* ./

EXPOSE 9000

################# PHP DEV #################

FROM backend-base AS backend-dev

RUN composer install --no-interaction

COPY ./ ./

################# PHP PROD #################

FROM backend-base AS backend-prod

RUN composer install --no-dev --no-interaction

COPY ./ ./