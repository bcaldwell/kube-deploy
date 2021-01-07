FROM golang:1.15 AS builder

WORKDIR $GOPATH/src/github.com/bcaldwell/kube-deploy

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix nocgo -o /kube-deploy ./cmd/kube-deploy/main.go


# Alpine linux with docker installed
FROM alpine:3.12

ENV HELM_VERSION=3.4.2
ENV KUBECTL_VERSION=1.20.1

# install git, helm and kubectl
RUN apk add --update --no-cache curl ca-certificates git bash && \
    curl -L https://get.helm.sh/helm-v${HELM_VERSION}-linux-amd64.tar.gz | tar xvz && \
    mv linux-amd64/helm /usr/bin/helm && \
    chmod +x /usr/bin/helm && \
    rm -rf linux-amd64 && \
    curl -LO https://storage.googleapis.com/kubernetes-release/release/v${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    mv ./kubectl /usr/bin/kubectl && \
    chmod +x /usr/bin/kubectl && \
    apk del curl && \
    rm -f /var/cache/apk/*

COPY --from=builder /kube-deploy /usr/bin/kube-deploy

ENTRYPOINT [ "/bin/bash" ]