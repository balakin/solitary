package lima

import (
	_ "embed"
	"io"
)

//go:embed files.py
var fileHelper string

func GuestMkdir(instance, home, dir string) error {
	return ExecStream(instance, nil, io.Discard, "python3", "-c", fileHelper, "mkdir", home, dir, "")
}

func GuestWrite(instance, home, dir, name, mode string, source io.Reader) error {
	return ExecStream(instance, source, io.Discard, "python3", "-c", fileHelper, "write", home, dir, name, mode)
}

func GuestRead(instance, home, dir, name string, target io.Writer) error {
	return ExecStream(instance, nil, target, "python3", "-c", fileHelper, "read", home, dir, name)
}

func GuestList(instance, home, dir string, target io.Writer) error {
	return ExecStream(instance, nil, target, "python3", "-c", fileHelper, "list", home, dir, "")
}
