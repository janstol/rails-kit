# Agent Skill

rails-kit includes a skill for Claude Code and Codex. Installation is global by default.

```sh
rails-kit skill install
rails-kit skill install --target codex
rails-kit skill install --target all
rails-kit skill install --local --target codex
rails-kit skill uninstall --target codex
rails-kit skill uninstall --local --target all
```

The default target is `claude`; select `codex` or `all` with `--target`.

Global destinations:

- Claude Code: `~/.claude/skills/rails-kit`
- Codex: `~/.agents/skills/rails-kit`

Codex installation includes UI metadata in `agents/openai.yaml`. Use `--local` for the corresponding skill directory inside the detected Rails root or the directory selected by `--root`.

The former `--global` flag remains accepted as a deprecated compatibility alias, but global scope no longer requires a flag.
