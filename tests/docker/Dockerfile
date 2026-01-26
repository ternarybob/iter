FROM node:20-slim

# Install dependencies
RUN apt-get update && apt-get install -y \
    curl \
    git \
    jq \
    coreutils \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# Create test user (claude doesn't like running as root)
RUN useradd -m -s /bin/bash testuser
USER testuser
WORKDIR /home/testuser

# Copy plugin files
COPY --chown=testuser:testuser bin/ /home/testuser/iter-plugin/

# Copy test script
COPY --chown=testuser:testuser tests/docker/test-plugin.sh /home/testuser/test-plugin.sh
RUN chmod +x /home/testuser/test-plugin.sh

# Set entrypoint
ENTRYPOINT ["/home/testuser/test-plugin.sh"]
