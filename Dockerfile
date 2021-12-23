FROM moby/buildkit:v0.9.3
WORKDIR /gorpa
COPY gorpa README.md /gorpa/
ENV PATH=/gorpa:$PATH
ENTRYPOINT [ "/bhojpur/gorpa" ]
