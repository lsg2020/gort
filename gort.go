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

func NewDwarfRT(path string) (*DwarfRT, error) {
	return (&DwarfRT{}).init(path)
}

type DwarfRT struct {
	bi *proc.BinaryInfo

	mds             []moduleData
	globals         map[string]reflect.Value
	imageCacheTypes map[*proc.Image]map[string]uint64
}

func (d *DwarfRT) init(path string) (*DwarfRT, error) {
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

func (d *DwarfRT) AddImage(path string, addr uint64) error {
	if err := d.check(); err != nil {
		return err
	}

	if err := d.bi.AddImage(path, addr); err != nil {
		return err
	}
	return d.refreshModule()
}

func (d *DwarfRT) refreshModule() error {
	mds, err := loadModuleData(d.bi, new(localMemory))
	if err != nil {
		return err
	}
	d.mds = mds
	d.globals = nil
	return nil
}

func (d *DwarfRT) check() error {
	if d.bi == nil {
		return ErrNeedInit
	}
	return nil
}

func (d *DwarfRT) BI() *proc.BinaryInfo {
	return d.bi
}
