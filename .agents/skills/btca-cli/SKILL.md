---
name: btca-cli
description: Operate the btca CLI for local resources and source-first answers. Use when setting up btca in a project, connecting a provider, adding or managing resources, and asking questions via btca commands. Invoke this skill when the user says "use btca" or needs to do more detailed research on a specific library or framework.
---

# btca CLI

`btca` is a source-first research CLI. It hydrates resources (git, local, npm) into searchable context, then answers questions grounded in those sources. Use configured resources for ongoing work, or one-off anonymous resources directly in `btca ask`.

Full CLI reference: https://docs.btca.dev/guides/cli-reference

Add resources:

```bash
# Git resource
btca add -n svelte-dev https://github.com/sveltejs/svelte.dev

# Local directory
btca add -n my-docs -t local /absolute/path/to/docs

# npm package
btca add npm:@types/node@22.10.1 -n node-types -t npm
```

Verify resources:

```bash
btca resources
```

Ask a question:

```bash
btca ask -r svelte-dev -q "How do I define remote functions?"
```

## Common Tasks

- Ask with multiple resources:

```bash
btca ask -r react -r typescript -q "How do I type useState?"
```

- Ask with anonymous one-off resources (not saved to config):

```bash
# One-off git repo
btca ask -r https://github.com/sveltejs/svelte -q "Where is the implementation of writable stores?"

# One-off npm package
btca ask -r npm:react@19.0.0 -q "How is useTransition exported?"
```

## Config Overview

- Config lives in `btca.config.jsonc` (project) and `~/.config/btca/btca.config.jsonc` (global).
- Project config overrides global and controls provider/model and resources.

## Troubleshooting

- "No resources configured": add resources with `btca add ...` and re-run `btca resources`.
- "Provider not connected": run `btca connect` and follow the prompts.
- "Unknown resource": use `btca resources` for configured names, or pass a valid HTTPS git URL / `npm:<package>` as an anonymous one-off in `btca ask`.
