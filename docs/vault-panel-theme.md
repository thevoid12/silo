# Vault Panel & UI Theme Update

## What changed

### Vault panel
- A lock icon button (🔐) appears in the top-left rail header of the shell.
- Clicking it opens a modal panel titled **Vault**.
- The panel first prompts for your vault password to authenticate.
- Once unlocked, all stored secrets are listed. Each entry shows the key name and a masked value (dots).
- Per-entry actions: **Edit** (inline input), **Save**, **Cancel**, **Delete**.
- An **+ Add secret** button lets you store new key/value pairs.
- The panel can be closed via the ✕ button or by clicking outside it.

### UI theme
- New light blue + white palette replacing the dark theme:
  - Background: soft blue-grey (`#f0f4f9`)
  - Panel sidebar: slightly deeper (`#e8edf5`)
  - Cards/chat area: white
  - Accent: Claude-style blue (`#4a7cf7`)
  - Borders: subtle translucent blue
- All primary buttons now use the accent blue colour.
- Active session in the sidebar uses a light blue tint.

## How vault secrets are stored

The vault panel calls the same `silo vault` CLI commands used from the terminal — `vault list`, `vault get`, `vault set`, `vault delete` — driven through a PTY so the password prompts work correctly. No new backend code was required.

## Playwright tests

New test file: `desktop/playwright_tests/vault.spec.ts`

Covers:
- Vault button opens the panel with password prompt
- ✕ button closes the panel
- Clicking the backdrop closes the panel
- Unlock button disabled when password empty, enabled when filled
- Shows secrets (or empty state + Add button) after correct password
- Shows error on wrong password
