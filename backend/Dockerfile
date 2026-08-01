# AvtoTest API — staging/production image.
# Self-migrates on boot (embedded migrations). Config is 100% env-driven;
# do not bake DATABASE_URL / secrets into the image.
#
# Build:  docker build -t avtotest-api:local -f backend/Dockerfile backend/
# Run:    see deploy/README.md

FROM golang:1.26-alpine@sha256:0178a641fbb4858c5f1b48e34bdaabe0350a330a1b1149aabd498d0699ff5fb2 AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/encryptpan ./cmd/encryptpan \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/healthcheck ./cmd/healthcheck

FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35
WORKDIR /
COPY --from=build /out/api /api
COPY --from=build /out/encryptpan /encryptpan
COPY --from=build /out/healthcheck /healthcheck
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
