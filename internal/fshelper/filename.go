package fshelper

import (
	"io/fs"
	"log/slog"
)

type Filename struct {
	fsys fs.FS
	name string
}

func NewFilename(fsys fs.FS, name string) Filename {
	return Filename{fsys: fsys, name: name}
}

func (fn Filename) LogValue() slog.Value {
	if fn.IsEmpty() {
		return slog.Value{}
	}
	return slog.StringValue(fn.FullName())
}

func (fn Filename) FS() fs.FS {
	return fn.fsys
}

func (fn Filename) Name() string {
	return fn.name
}

func (fn Filename) FullName() string {
	fsys := fn.fsys
	if fsys, ok := fsys.(interface{ Name() string }); ok {
		return fsys.Name() + ":" + fn.name
	}
	return fn.name
}

func (fn Filename) Open() (fs.File, error) {
	if fn.fsys == nil {
		return nil, fs.ErrNotExist
	}
	return fn.fsys.Open(fn.name)
}

func (fn Filename) Stat() (fs.FileInfo, error) {
	if fn.fsys == nil {
		return nil, fs.ErrNotExist
	}
	return fs.Stat(fn.fsys, fn.name)
}

func (fn Filename) IsEmpty() bool {
	return fn.fsys == nil && fn.name == ""
}
