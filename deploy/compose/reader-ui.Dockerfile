FROM node:22.23.2-alpine3.23@sha256:46825fbbd4e996a78b7a2cdc08d75e38a5a505bdab95dcda55605359bf124bc6 AS build

WORKDIR /app
COPY reader-ui/package.json reader-ui/package-lock.json ./
RUN --mount=type=cache,id=moonbook-reader-npm,target=/root/.npm,sharing=locked npm ci
COPY reader-ui/ ./
ARG VITE_READER_API_BASE=/prod-api
ENV VITE_READER_API_BASE=$VITE_READER_API_BASE
RUN npm run build

FROM node:22.23.2-alpine3.23@sha256:46825fbbd4e996a78b7a2cdc08d75e38a5a505bdab95dcda55605359bf124bc6

ENV NODE_ENV=production \
    PORT=3000
WORKDIR /app
RUN apk upgrade --no-cache
COPY reader-ui/package.json reader-ui/package-lock.json ./
RUN --mount=type=cache,id=moonbook-reader-npm,target=/root/.npm,sharing=locked npm ci --omit=dev \
    && npm cache clean --force \
    && rm -rf /usr/local/lib/node_modules/npm \
    && rm -f /usr/local/bin/npm /usr/local/bin/npx
COPY --from=build /app/build ./build
USER node
EXPOSE 3000
CMD ["node", "node_modules/@react-router/serve/bin.js", "./build/server/index.js"]
