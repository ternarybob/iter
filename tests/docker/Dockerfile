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

# Pre-create .claude directory structure with correct ownership
# This allows credentials to be mounted without permission issues
RUN mkdir -p /home/testuser/.claude/plugins/marketplaces \
    /home/testuser/.claude/plugins/cache \
    /home/testuser/.claude/skills \
    && chown -R testuser:testuser /home/testuser/.claude

USER testuser
WORKDIR /home/testuser

# Copy plugin files
COPY --chown=testuser:testuser bin/ /home/testuser/iter-plugin/

# Default entrypoint - tests use --entrypoint bash with inline scripts
ENTRYPOINT ["bash"]
CMD ["-c", "echo 'Use --entrypoint bash -c \"<script>\" to run tests'"]
