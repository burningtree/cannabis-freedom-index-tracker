# Build, push to your registry, and configure RESEARCH_IMAGE with the resulting digest.
FROM node:24-bookworm-slim
RUN npm install --global opencode-ai@1.17.13 && test "$(opencode --version)" = "1.17.13"
COPY tracker-research /usr/local/bin/tracker-research
RUN chmod 0555 /usr/local/bin/tracker-research && mkdir /work
USER 65534:65534
WORKDIR /work
ENTRYPOINT ["/usr/local/bin/tracker-research"]
