FROM node:22.18.0-alpine3.22 AS build

WORKDIR /app
COPY reader-ui/package.json reader-ui/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY reader-ui/ ./
ARG VITE_READER_API_BASE=/prod-api
ENV VITE_READER_API_BASE=$VITE_READER_API_BASE
RUN npm run build

FROM node:22.18.0-alpine3.22

ENV NODE_ENV=production \
    PORT=3000
WORKDIR /app
COPY reader-ui/package.json reader-ui/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --omit=dev \
    && npm cache clean --force
COPY --from=build /app/build ./build
USER node
EXPOSE 3000
CMD ["npm", "run", "start"]
