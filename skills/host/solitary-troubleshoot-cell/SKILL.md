---
name: solitary-troubleshoot-cell
description: Diagnose a solitary cell that will not start, stopped answering, or ignored a change to its cell.yaml. For the host, where the solitary CLI is installed. Use when solitary up fails or hangs, when solitary ls shows a cell as degraded, busy, unreachable, broken or orphaned, when an edit to vm, ports, network or vm.provision did not take, or when the user's work in a cell seems to have disappeared.
---

# Troubleshoot a cell

Start by reading the state rather than guessing at it:

```sh
solitary ls                # every cell and its state
solitary doctor            # the host: Lima, hypervisor, memory, disk, stored definitions
solitary inspect <name>    # what a cell runs on outside its container
```

`doctor` is for a cell that won't come up at all. `inspect` never reads the container's
environment, so its output is safe to show.

## States

| State           | What to do                                                                      |
| --------------- | ------------------------------------------------------------------------------- |
| `uninitialized` | No machine yet. `solitary up --detach <name>` creates one.                      |
| `stopped`       | `solitary up --detach <name>`.                                                  |
| `degraded`      | The container is gone, usually because `command` exited. `up` starts a new one, but fix `command` first: it must never exit. |
| `busy`          | Slow but alive: just booted, or a build holding podman's lock. Wait, then check again. |
| `unreachable`   | The machine doesn't answer. `solitary down <name> && solitary up --detach <name>` recovers it without touching the disk. |
| `broken`        | Lima reports it broken. Read what `up` says. `rm` is the last resort, and it's the user's call. |
| `orphaned`      | A machine whose cell directory was deleted or renamed. `solitary init <name>` gives it a definition again; `solitary rm --orphans` destroys them all, but only if the user asks. |

## Common failures

- **`no such host` while building or in the cell.** The cell has `network.allow` and the name
  isn't on it. Use the `solitary-restrict-cell-network` skill.
- **An edit to `vm`, `ports`, `network` or `gpu` did nothing.** Those are read at boot, and
  `up` printed a warning saying so. Run `solitary down <name> && solitary up --detach <name>`.
- **A change to `vm.provision`.** The old script's effects stay on the disk. Only
  `solitary up --new-machine <name>` gets a machine built by the new script alone. It keeps
  the home but asks first, so leave that question to the user.
- **The machine asks for more memory than `/dev/shm` holds (Linux).** `up` refuses and names
  the largest `vm.memory` that fits. Lower it in `cell.yaml`.
- **A `vm.disk` smaller than the machine has.** A disk can't shrink. Raising the setting to
  the current size costs nothing, because disks are sparse. The alternative is `rm`, which
  destroys the home.
- **`no terminal to prompt on`.** A required secret has no value. Ask the user to run
  `solitary secrets <name>` in their own terminal. Never ask for the value in the
  conversation.
- **`set up by a newer solitary`.** Two solitary binaries share the host. `solitary update`
  brings this one level.
- **`busy with another solitary command`.** Another `up`, `down`, `rm` or `patch` is still
  running on that cell, often in another terminal. Wait for it to finish rather than killing
  it.
- **`Permission denied` inside the cell.** Something runs as a non-root user. The cell needs
  `user: <name>` in `cell.yaml`, and the image has to give that user `sudo` and ownership of
  its tool directories.
- **Tools reverted after a restart.** The container starts over from its image at every boot,
  and only the home survives. Update tools with `solitary up --rebuild <name>`, not from
  inside the cell.
- **The agent in the cell doesn't have solitary's skills.** Check `agents:` in `cell.yaml`.
  Left out, an agent is only found once it has run and made its config directory, so run
  `solitary up <name>` again (joining a running cell is enough) and start a new agent
  session. Naming it, as in `agents: [claude-code]`, installs the skills from the start. A
  warning from `up` that the directory is outside the home means the image moved the agent
  there (for example `CLAUDE_CONFIG_DIR=/opt/claude`), and solitary writes only to the home.
  Point that variable into the home in the image instead. If the agent is moved in a way `up`
  can't see, such as an export in `.bashrc`, name its real directory with a mapping:
  `- {name: claude-code, skills: ~/.config/claude/skills}`.
- **Work disappeared.** Work lives in `/home/cell`, which survives everything except
  `solitary rm`. If it's gone after an `rm`, it can't be recovered. Say so plainly.

## Don't

- Don't run `rm`, `rm --orphans` or `up --new-machine` to clear an error unless the user asks.
  The first two destroy a cell's home.
- Don't repair a machine with `limactl` or by editing Lima's files. `up` puts the machine back
  to what `cell.yaml` describes, and reporting what you found is better than working around
  it.

More is at https://solitary.balakin.io/docs/troubleshooting.
