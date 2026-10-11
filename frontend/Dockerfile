FROM node:24.21.0-alpine3.23 AS build

WORKDIR /app
RUN npm install --global npm@12.1.0
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:24.21.0-alpine3.23 AS runtime

ENV NODE_ENV=production
ENV PORT=4000
ENV API_PUBLIC_BASE_URL=http://localhost:8080/api/v1
WORKDIR /app
RUN npm install --global npm@12.1.0
COPY package.json package-lock.json ./
RUN npm ci --omit=dev && npm cache clean --force
COPY --from=build /app/dist/frontend ./dist/frontend
USER node
EXPOSE 4000
HEALTHCHECK --interval=10s --timeout=3s --retries=5 --start-period=10s CMD node -e "fetch('http://127.0.0.1:4000/healthz').then((r) => process.exit(r.ok ? 0 : 1)).catch(() => process.exit(1))"
CMD ["node", "dist/frontend/server/server.mjs"]
