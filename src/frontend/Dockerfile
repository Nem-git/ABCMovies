
################### FRONTEND BASE ###############

FROM node:alpine AS frontend-base

WORKDIR /srv/www

ENV PROTOCOL_HEADER=X-Forwarded-Proto
ENV HOST_HEADER=X-Forwarded-Host
ENV HOST=0.0.0.0
ENV PORT=8001

COPY package*.json ./
COPY svelte.config.js ./
COPY tsconfig.json ./
COPY vite.config.ts ./

################### FRONTEND DEV ###############

FROM frontend-base AS frontend-dev

ENV NODE_ENV=development

RUN npm ci

CMD ["npm", "run", "dev"]

################# FRONTEND PROD BASE ################

FROM frontend-base AS frontend-prod-base

ENV NODE_ENV=production

RUN npm ci --omit=dev

################ FRONTEND PROD #################

FROM frontend-prod-base AS frontend-prod

COPY --from=builder /srv/www/build ./build
COPY --from=builder /srv/www/package*.json ./

CMD ["node", "build"]