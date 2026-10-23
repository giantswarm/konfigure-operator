FROM gsoci.azurecr.io/giantswarm/alpine:3.20.3-giantswarm

WORKDIR /

ARG TARGETARCH
COPY konfigure-operator-linux-${TARGETARCH} /manager

USER 65532:65532

ENTRYPOINT ["/manager"]
