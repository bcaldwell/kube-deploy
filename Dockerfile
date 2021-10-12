FROM golang:1.17-alpine AS builder

WORKDIR $GOPATH/src/github.com/bcaldwell/kube-deploy

COPY . ./
RUN go build -o /kube-deploy ./cmd/kube-deploy/main.go


# Alpine linux with docker installed
FROM alpine:3.13

# <os>/<arch>
ARG TARGETPLATFORM

ENV HELM_VERSION=3.7.0
ENV KUBECTL_VERSION=1.22.2

# install git, helm and kubectl
RUN apk add --update --no-cache curl ca-certificates git bash && \
    curl -L "https://get.helm.sh/helm-v${HELM_VERSION}-${TARGETPLATFORM/\//-}.tar.gz" | tar xvz && \
    mv "${TARGETPLATFORM/\//-}/helm" /usr/bin/helm && \
    chmod +x /usr/bin/helm && \
    rm -rf "${TARGETPLATFORM/\//-}" && \
    curl -LO "https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/${TARGETPLATFORM}/kubectl" && \
    mv ./kubectl /usr/bin/kubectl && \
    chmod +x /usr/bin/kubectl && \
    apk del curl && \
    rm -f /var/cache/apk/*

COPY --from=builder /kube-deploy /usr/bin/kube-deploy

ENTRYPOINT [ "/bin/bash" ]
