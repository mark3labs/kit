#!/usr/bin/env -S mcphost script
---
# Demo script showcasing default values in MCPHost scripts
model: "anthropic/claude-sonnet-4-5-20250929"
mcpServers:
  filesystem:
    type: "builtin"
    name: "fs"
    options:
      allowed_directories: ["${work_dir:-/tmp}", "${home_dir:-/home}"]
  bash:
    type: "builtin"
    name: "bash"
  todo:
    type: "builtin"
    name: "todo"
---
# Default Values Demo Script

Hello ${user_name:-Anonymous User}! 

This script demonstrates the new default values feature in MCPHost scripts.

## Your Configuration:
- Working directory: ${work_dir:-/tmp}
- Home directory: ${home_dir:-/home}
- Preferred editor: ${editor:-nano}
- Log level: ${log_level:-info}
- Output format: ${format:-text}

## Tasks to Complete:

1. **Directory Analysis**: Analyze the contents of ${work_dir:-/tmp}
2. **System Info**: Show system information using ${info_command:-uname -a}
3. **File Operations**: Create a test file named ${test_file:-demo_test.txt}
4. **Report Generation**: Generate a ${format:-text} format report

Please complete these tasks and provide a summary of what you accomplished.
