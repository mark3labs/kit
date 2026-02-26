#!/usr/bin/env -S kit script
---
# This script uses the container-use MCP server from https://github.com/dagger/container-use
mcpServers:
  container-use:
    type: "local"
    command: ["container-use", "stdio"]
---
Create 2 variations of a simple hello world app using Flask and FastAPI. each in their own environment. Give me the URL of each app
