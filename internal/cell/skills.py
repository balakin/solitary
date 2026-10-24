"""Host-invoked upkeep of solitary's skills in a cell's home; never trust cell paths.

Every path is relative to the home and walked one component at a time with
O_NOFOLLOW, so nothing the cell put in its home can turn a write here into a
write somewhere else on the machine. Only entries named solitary-* are ever
replaced or removed.
"""
import base64
import json
import os
import secrets
import shutil
import stat
import sys

PREFIX = "solitary-"
STAGING = ".solitary-skill-"
DIRECTORY = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW


def parts(path):
    split = path.split("/")
    for part in split:
        if not part or part in (".", ".."):
            raise ValueError("invalid path: " + path)
    return split


def walk(home, path, create, created):
    """Opens path under home, making what is missing when create is set.

    Returns the open directory, or None when something other than a directory
    is in the way, or when it is missing and create is not set."""
    fd = os.open(home, DIRECTORY)
    done = []
    try:
        for part in parts(path):
            done.append(part)
            try:
                child = os.open(part, DIRECTORY, dir_fd=fd)
            except FileNotFoundError:
                if not create:
                    os.close(fd)
                    return None
                os.mkdir(part, 0o755, dir_fd=fd)
                created.append("/".join(done))
                child = os.open(part, DIRECTORY, dir_fd=fd)
            except OSError:
                # A symlink (ELOOP under O_NOFOLLOW) or a file in the way.
                os.close(fd)
                return None
            os.close(fd)
            fd = child
    except BaseException:
        os.close(fd)
        raise
    return fd


def remove(fd, name):
    info = os.stat(name, dir_fd=fd, follow_symlinks=False)
    if stat.S_ISDIR(info.st_mode):
        shutil.rmtree(name, dir_fd=fd)
    else:
        os.unlink(name, dir_fd=fd)


def write_tree(fd, files):
    for path, entry in sorted(files.items()):
        *dirs, name = parts(path)
        current = os.dup(fd)
        try:
            for part in dirs:
                try:
                    os.mkdir(part, 0o755, dir_fd=current)
                except FileExistsError:
                    pass
                child = os.open(part, DIRECTORY, dir_fd=current)
                os.close(current)
                current = child
            target = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, int(entry["mode"], 8), dir_fd=current)
            with os.fdopen(target, "wb") as stream:
                stream.write(base64.b64decode(entry["data"]))
        finally:
            os.close(current)


def install(fd, skills):
    for name in os.listdir(fd):
        if name.startswith(STAGING):
            remove(fd, name)

    for name, files in skills.items():
        staged = STAGING + secrets.token_hex(8)
        os.mkdir(staged, 0o755, dir_fd=fd)
        staged_fd = os.open(staged, DIRECTORY, dir_fd=fd)
        try:
            write_tree(staged_fd, files)
        finally:
            os.close(staged_fd)

        # Two renames, since one cannot replace a directory that has anything
        # in it; a reader sees the old skill or the new one, never half.
        old = None
        try:
            os.stat(name, dir_fd=fd, follow_symlinks=False)
            old = STAGING + secrets.token_hex(8)
            os.rename(name, old, src_dir_fd=fd, dst_dir_fd=fd)
        except FileNotFoundError:
            pass
        os.rename(staged, name, src_dir_fd=fd, dst_dir_fd=fd)
        if old:
            remove(fd, old)

    for name in os.listdir(fd):
        if name.startswith(PREFIX) and name not in skills:
            remove(fd, name)


def clear(fd):
    for name in os.listdir(fd):
        if name.startswith(PREFIX) or name.startswith(STAGING):
            remove(fd, name)


def ours(fd, prefix, created):
    """Whether everything under fd is solitary's: directories it made, and
    solitary-* entries in them."""
    for name in os.listdir(fd):
        if name.startswith(PREFIX) or name.startswith(STAGING):
            continue
        path = prefix + "/" + name
        if path not in created:
            return False
        try:
            child = os.open(name, DIRECTORY, dir_fd=fd)
        except OSError:
            return False
        try:
            if not ours(child, path, created):
                return False
        finally:
            os.close(child)
    return True


def footprint(home, request):
    created = set(request["created"])
    found = []
    for marker in request["markers"]:
        if marker not in created:
            continue
        fd = walk(home, marker, False, [])
        if fd is None:
            continue
        try:
            if ours(fd, marker, created):
                found.append(marker)
        finally:
            os.close(fd)
    return found


def sync(home, request):
    created, written, skipped = [], [], []
    skills = request["skills"]

    for target in request["targets"]:
        fd = walk(home, target, True, created)
        if fd is None:
            skipped.append(target)
            continue
        try:
            install(fd, skills)
            written.append(target)
        finally:
            os.close(fd)

    for previous in request["previous"]:
        if previous in request["targets"]:
            continue
        fd = walk(home, previous, False, [])
        if fd is None:
            continue
        try:
            clear(fd)
        finally:
            os.close(fd)

    # What was made before and is gone since is forgotten, so the record only
    # ever describes the home as it is.
    kept = []
    for path in request["created"]:
        fd = walk(home, path, False, [])
        if fd is not None:
            os.close(fd)
            kept.append(path)

    return {"written": written, "skipped": skipped, "created": kept + [c for c in created if c not in kept]}


action, home = sys.argv[1:3]
request = json.load(sys.stdin)
if action == "footprint":
    print(json.dumps(footprint(home, request)))
elif action == "sync":
    print(json.dumps(sync(home, request)))
else:
    raise ValueError("unknown operation")
