FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG REVISION=unknown
ARG BRANCH=unknown
ARG TARGETARCH

RUN CGO_ENABLED=0 GOARCH=$TARGETARCH go build \
    -ldflags "-X github.com/prometheus/common/version.Version=${VERSION} \
              -X github.com/prometheus/common/version.Revision=${REVISION} \
              -X github.com/prometheus/common/version.Branch=${BRANCH}" \
    -o /out/cluster_autoscaler_status_exporter .

FROM gcr.io/distroless/static:nonroot
LABEL maintainer="Bringg DevOps <devops@bringg.com>"

COPY --from=builder /out/cluster_autoscaler_status_exporter /

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/cluster_autoscaler_status_exporter"]
