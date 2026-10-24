---
name: solitary-read-inbox
description: Find files the user sent into a solitary cell, including pasted screenshots. For an agent running inside a cell, where SOLITARY_CELL is set. Use when the user says they sent, pasted, attached, dropped or shared a file or a screenshot, or refers to an image or document you can't see.
---

# Read what the user sent

Nothing from the user's machine is mounted in this cell. What they send arrives in the
inbox:

```sh
ls -lt ~/inbox       # newest first
artifact --list      # the inbox and the outbox together
```

- **Files the user sends** with `solitary send <cell> <file>` keep their names.
- **A pasted screenshot** (`solitary send <cell> --clipboard`) arrives as
  `clipboard-<timestamp>.png`. When the user says "look at the screenshot" without naming
  one, it's the newest of those. Open it as an image.
- **Directories can't be sent.** If the user means a folder, it will be an archive; unpack
  it somewhere in the home rather than in the inbox.

If the inbox is empty or doesn't hold what the user describes, say so and tell them how
to send it, with the cell's name from `$SOLITARY_CELL`:

```sh
solitary send <cell> path/to/file
solitary send <cell> --clipboard     # the image on their clipboard
```

The inbox is the cell's own, and files stay in it until someone removes them. Leave them
in place unless the user asks you to tidy up.
