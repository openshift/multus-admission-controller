FROM openshift/origin-release:golang-1.10 as builder

# ENV GOPATH /go
# RUN mkdir $GOPATH
# RUN yum install -y golang make

ENV PKG_NAME=github.com/K8sNetworkPlumbingWG/net-attach-def-admission-controller
ENV PKG_PATH=$GOPATH/src/$PKG_NAME
RUN mkdir -p $PKG_PATH

COPY . $PKG_PATH/
# ENV GLIDE_PATH=$PKG_PATH/vendor/github.com/Masterminds/glide/
# WORKDIR $GLIDE_PATH
# RUN make build

WORKDIR $PKG_PATH
# RUN $GLIDE_PATH/glide install --strip-vendor
RUN go install ./...

CMD ["webhook"]

