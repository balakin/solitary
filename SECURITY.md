# Security

Solitary runs coding agents in a Lima VM so that an agent which takes over its
container still cannot reach the host. That is the goal, and every way around
it is a vulnerability. It is not a guarantee.

## What can break the boundary

A cell's isolation rests on several layers, and a flaw in any one of them can
lose it:

- the hypervisor and host kernel — KVM or Apple's Virtualization framework, and
  QEMU with its emulated devices, plus virglrenderer and the host GPU driver
  for a cell with `gpu: true`;
- Lima, which builds the VM from its definition and runs the host-side agents
  for port forwarding and networking;
- the guest kernel, podman and nftables, which enforce the container and the
  network policy;
- solitary itself, which renders the machine definition, writes the firewall,
  moves files across the boundary and handles secrets.

Keep all of them up to date, use a restrictive network policy, and give a cell
only what you could afford to lose if the boundary failed.
[Limitations](https://solitary.balakin.io/docs/limitations#the-boundary-itself-can-fail)
covers this in more detail.

## Reporting a vulnerability

Report it privately through
[GitHub's vulnerability reporting](https://github.com/balakin/solitary/security/advisories/new),
not in a public issue. A report that only breaks a cell's isolation through a
bug in Lima, QEMU or a kernel is still welcome here — solitary may be able to
avoid the path while the upstream fix ships — but please report it upstream as
well.

Fixes ship in the next release; there are no backports to older versions.
Advisories are published once a fixed release is available.
