# Stage 1: Build the Nuxt application
FROM node:22-alpine AS build

WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

# Stage 2: Minimal runtime image
FROM node:22-alpine

RUN addgroup -S app && adduser -S app -G app

WORKDIR /app

COPY --from=build /src/.output ./.output

RUN chown -R app:app /app
USER app

EXPOSE 3000

CMD ["node", ".output/server/index.mjs"]
