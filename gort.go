package gort

import (
	"errors"
	"os"
	"reflect"
	"runtime"

	"github.com/go-delve/delve/pkg/proc"
)

var (
	ErrNeedInit         = errors.New("need init")
	ErrNotFound         = errors.New("not found")
	ErrNotSupport       = errors.New("not support")
	ErrTooManyLibraries = errors.New("number of loaded libraries exceeds maximum")
)

func NewDwarf(path string) (*Dwarf, error) {
	return (&Dwarf{}).init(path)
}

type Dwarf struct {
	bi *proc.BinaryInfo

	mds             []moduleData
	globals         map[string]reflect.Value
	imageCacheTypes map[*proc.Image]map[string]uint64
}

func (d *Dwarf) init(path string) (*Dwarf, error) {
	var err error
	if path == "" {
		if path, err = os.Executable(); err != nil {
			return nil, err
		}
	}

	bi := proc.NewBinaryInfo(runtime.GOOS, runtime.GOARCH)
	err = bi.LoadBinaryInfo(path, 0, nil)
	if err != nil {
		return nil, err
	}
	d.bi = bi

	if err = d.refreshModule(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Dwarf) AddImage(path string, addr uint64) error {
	if err := d.check(); err != nil {
		return err
	}

	if err := d.bi.AddImage(path, addr); err != nil {
		return err
	}
	return d.refreshModule()
}

func (d *Dwarf) refreshModule() error {
	mds, err := loadModuleData(d.bi, new(localMemory))
	if err != nil {
		return err
	}
	d.mds = mds
	d.globals = nil
	return nil
}

func (d *Dwarf) check() error {
	if d.bi == nil {
		return ErrNeedInit
	}
	return nil
}

func (d *Dwarf) BI() *proc.BinaryInfo {
	return d.bi
}
