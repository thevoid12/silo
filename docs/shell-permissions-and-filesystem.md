# Shell: Real Filesystem & Persistent Permissions

## What Changed

### Real Filesystem Execution
Commands now run in the user's **actual current working directory** instead of an isolated temp directory. Files created by the agent (e.g. `hello.txt`) appear where the user is running silo.

### Smart Command Construction
The shell tool now has a dedicated `script` field for anything involving shell features:

| Field | Use for |
|-------|---------|
| `script` | pipes (`\|`), redirection (`>`, `>>`), `&&`, `;`, quotes — **preferred** |
| `command` + `args` | simple single-binary calls only (e.g. `ls -la`, `date`) |

Examples the agent uses:
- `script="echo 'hello,silo' > hello_silo.txt"` — create file
- `script="grep -r 'pattern' . | head -20"` — search with pipe
- `script="mkdir -p foo && touch foo/bar.txt"` — multi-step

`sh` and `bash` are in the default allowlist so scripts run without a prompt.

### Persistent Permissions (`allowed_permissions.md`)

When the agent asks for permission to run a command, you now have three options:

```
Allow "write"? [y/n/a=always]:
```

| Answer | Meaning |
|--------|---------|
| `y` | Allow this once |
| `n` | Deny |
| `a` | Always allow — saved to `~/.silo/allowed_permissions.md` |

Commands saved with `a` are automatically allowed in all future sessions. The file is plain markdown and can be edited by hand.

**Location:** `~/.silo/allowed_permissions.md`
