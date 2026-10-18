"""Host-invoked I/O under an opened guest directory; never trust cell paths."""
import json
import os
import secrets
import shutil
import stat
import sys

action, home, directory, name = sys.argv[1:5]
flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
fd = os.open(home, flags)
try:
    for part in directory.split("/"):
        if not part or part in (".", ".."):
            raise ValueError("invalid directory")
        if action in ("write", "mkdir"):
            try:
                os.mkdir(part, 0o700, dir_fd=fd)
            except FileExistsError:
                pass
        child = os.open(part, flags, dir_fd=fd)
        os.close(fd)
        fd = child
    if action == "mkdir":
        pass
    elif action == "list":
        entries = []
        for filename in os.listdir(fd):
            info = os.stat(filename, dir_fd=fd, follow_symlinks=False)
            if stat.S_ISREG(info.st_mode):
                entries.append({"Name": filename, "Size": info.st_size})
        print(json.dumps(entries))
    else:
        if not name or name in (".", "..") or "/" in name or "\\" in name:
            raise ValueError("invalid filename")
        if action == "read":
            source = os.open(name, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fd)
            with os.fdopen(source, "rb") as stream:
                if not stat.S_ISREG(os.fstat(stream.fileno()).st_mode):
                    raise ValueError("source must be a regular file")
                shutil.copyfileobj(stream, sys.stdout.buffer)
        elif action == "write":
            try:
                info = os.stat(name, dir_fd=fd, follow_symlinks=False)
            except FileNotFoundError:
                info = None
            if info is not None and not stat.S_ISREG(info.st_mode):
                raise ValueError("destination must be a regular file")
            temporary = ".solitary-" + secrets.token_hex(16)
            target = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd)
            try:
                with os.fdopen(target, "wb") as stream:
                    shutil.copyfileobj(sys.stdin.buffer, stream)
                    os.fchmod(stream.fileno(), int(sys.argv[5], 8))
                    os.fsync(stream.fileno())
                os.rename(temporary, name, src_dir_fd=fd, dst_dir_fd=fd)
            finally:
                try:
                    os.unlink(temporary, dir_fd=fd)
                except FileNotFoundError:
                    pass
        else:
            raise ValueError("unknown operation")
finally:
    os.close(fd)
