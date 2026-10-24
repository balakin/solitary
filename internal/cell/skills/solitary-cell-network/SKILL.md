---
name: solitary-cell-network
description: Recognise when a solitary cell's firewall refused a connection, and tell the user what to allow rather than working around it. For an agent running inside a cell, where SOLITARY_CELL is set. Use when a download, install, git fetch, API call or build fails with "no such host", NXDOMAIN, ENOTFOUND, EAI_AGAIN, "could not resolve host" or "Temporary failure in name resolution".
---

# A name that doesn't resolve

A solitary cell can be given an allow list. The machine then runs a resolver that answers
only for listed names, and a firewall that allows only what it resolved. A name that isn't
listed doesn't resolve at all, so a refusal shows up as a DNS error, not a timeout:

```text
dial tcp: lookup registry.npmjs.org on 127.0.0.1:53: no such host
curl: (6) Could not resolve host: pypi.org
npm ERR! code ENOTFOUND
```

When a well-known public name fails like this, assume the cell's allow list refused it.
Retrying won't help.

## What to do

1. **Stop and tell the user.** Name the exact host that was refused and what it was
   needed for: "`pypi.org` was refused; `pip install` needs it to download packages."
2. **Say how to allow it.** On their machine, they add the name to `network.allow` in the
   cell's definition, then restart the cell, since the list is read when the machine
   boots:

   ```text
   ~/.config/solitary/cells/<cell>/cell.yaml
     network:
       allow:
         - pypi.org

   solitary down <cell> && solitary up <cell>
   ```

   Use the name from `$SOLITARY_CELL`. A domain covers its subdomains, so `pypi.org`
   also allows `files.pypi.org`, but a different domain the same service uses (such as
   `files.pythonhosted.org`) needs its own entry. Mention the ones you can predict.

   GitHub's storage is the exception: ask for a preset, not the name. A refused
   `productionresultssa<N>.blob.core.windows.net` (`gh run view --log`, an artifact
   download) needs `github:actions-results`, since the next job lands on a different
   account. A refused `*registryv2prod.blob.core.windows.net` or
   `pkg-containers.githubusercontent.com` needs `github:packages`. Never suggest
   `blob.core.windows.net`: it opens every Azure storage account.
3. **Restarting ends this session.** Everything in the cell's home survives it. Say so,
   so the user isn't surprised.

## Don't

- Don't work around the list: no other mirror, proxy, raw IP address, or DNS-over-HTTPS
  to reach the same thing. The list is the user's decision about what this cell may
  reach.
- Don't loop retrying, and don't switch to another tool that needs the same host.
- Don't assume the whole network is down. Other names may resolve fine; only unlisted
  ones fail.
