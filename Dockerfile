FROM golang:1.27.1-alpine3.24 AS go-deps
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

FROM go-deps AS api-build
WORKDIR /src
COPY . .
RUN mkdir -p /out \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/api ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/netprobe ./cmd/netprobe

FROM node:26.7.0-alpine3.24 AS mock-assets-build
WORKDIR /src
COPY mock/package.json mock/package-lock.json ./mock/
RUN cd mock && npm ci --ignore-scripts --no-audit --no-fund
COPY mock ./mock
COPY internal/mockserver/static ./internal/mockserver/static
RUN cd mock && npm run build

FROM go-deps AS mock-build
WORKDIR /src
COPY . .
COPY --from=mock-assets-build /src/internal/mockserver/static/app.js /src/internal/mockserver/static/app.js
RUN mkdir -p /out \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/mock ./cmd/mock

FROM scratch AS api
COPY --from=api-build --chown=65532:65532 /out/api /api
COPY db/migrations /migrations
COPY planning/openapi.yaml /openapi.yaml
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/api"]

FROM scratch AS mock
COPY --from=mock-build --chown=65532:65532 /out/mock /mock
USER 65532:65532
EXPOSE 8081
ENTRYPOINT ["/mock"]

FROM scratch AS netprobe
COPY --from=api-build --chown=65532:65532 /out/netprobe /netprobe
USER 65532:65532
ENTRYPOINT ["/netprobe"]

FROM node:24.21.0-alpine3.23@sha256:9ec4a2e289874ed0d722e1772ec2de45d2801541db8612f3638b26f128c69ac2 AS frontend-runner
WORKDIR /runner
COPY scripts/verify-frontend-stack.mjs ./verify-frontend-stack.mjs
USER node
ENTRYPOINT ["node", "verify-frontend-stack.mjs"]

FROM postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677 AS database
USER root
COPY scripts/postgres/entrypoint-wrapper.sh /usr/local/bin/espacigo-postgres-entrypoint
COPY scripts/postgres/init-runtime-user.sh /docker-entrypoint-initdb.d/20-espacigo-runtime-user.sh
RUN chmod 0755 /usr/local/bin/espacigo-postgres-entrypoint /docker-entrypoint-initdb.d/20-espacigo-runtime-user.sh
ENTRYPOINT ["/usr/local/bin/espacigo-postgres-entrypoint"]
CMD ["postgres"]
