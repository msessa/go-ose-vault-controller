FROM engineering-docker-registry-public.artifactory.mlctech.io/docker-centos-base:7.4.1708

COPY target/go-ose-vault-controller /vault-controller
CMD /vault-controller
