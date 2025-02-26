FROM gsoci.azurecr.io/giantswarm/alpine:3.20.3-giantswarm

WORKDIR /

ADD konfigure-operator manager

USER 65532:65532

ENTRYPOINT ["/chart-operator"]
