---
description: File a GitHub issue with proper formatting
---

File a GitHub issue for the repository. The user wants to create an issue about: $@

## Steps

1. **Understand the issue** from `$@` - ask clarifying questions if:
   - The bug description is unclear
   - The expected vs actual behavior is missing
   - You don't know which component is affected

2. **Determine the issue type**:
   - `bug` - something is broken
   - `feat` - new feature request
   - `docs` - documentation improvement
   - `refactor` - code cleanup without behavior change

3. **Craft the title** using conventional format:
   - `<type>: <short description>`
   - Lowercase, imperative mood, ≤72 chars
   - Examples:
     - `fix: ToolRenderConfig BorderColor ignored during rendering`
     - `feat: add keyboard shortcut for clearing input`
     - `docs: clarify extension widget lifecycle`

4. **Write the body** with these sections:
   - **Bug Description** - what happened
   - **Steps to Reproduce** (for bugs) - numbered list
   - **Expected Behavior** - what should happen
   - **Actual Behavior** - what actually happened
   - **Code/Context** - relevant code snippets, file paths, error messages
   - **Proposed Fix** (optional) - your suggested approach

5. **File the issue** using `gh issue create`:
   ```bash
   gh issue create --title "type: description" --body "..."
   ```

6. **Confirm success** by showing:
   - The issue URL
   - The issue number

## Guidelines

- Include file paths and line numbers when you know them
- Use triple backticks for code blocks (the shell handles escaping)
- Keep the body factual - avoid speculation unless in "Proposed Fix" section
- If you're unsure about technical details, say so in the issue
- For UI bugs, describe what you see vs what you expect
- For API bugs, include the relevant struct/function names

## Example Usage

User: `/file-issue The ToolRenderConfig BorderColor field is documented but never used in rendering`

You: Create issue #42 with proper technical details about the bug.
