package gort

import (
	"debug/dwarf"
	"reflect"
	"unsafe"

	"github.com/go-delve/delve/pkg/proc"
)

func (d *Dwarf) loadGlobals() {
	d.globals = make(map[string]reflect.Value)

	packageVars := reflect.ValueOf(d.bi).Elem().FieldByName("packageVars")
	if packageVars.IsValid() {
		for i := 0; i < packageVars.Len(); i++ {
			rv := packageVars.Index(i)
			rName := rv.FieldByName("name")
			rAddr := rv.FieldByName("addr")
			rOffset := rv.FieldByName("offset")
			rCU := rv.FieldByName("cu")
			if !rName.IsValid() || !rAddr.IsValid() || !rCU.IsValid() || !rOffset.IsValid() {
				continue
			}
			rImage := rCU.Elem().FieldByName("image")
			if !rImage.IsValid() {
				continue
			}
			rDwarf := rImage.Elem().FieldByName("dwarf")
			if !rDwarf.IsValid() {
				continue
			}
			image := reflect.NewAt(rImage.Type().Elem(), unsafe.Pointer(rImage.Elem().UnsafeAddr())).Interface().(*proc.Image)
			dwarfData := reflect.NewAt(rDwarf.Type().Elem(), unsafe.Pointer(rDwarf.Elem().UnsafeAddr())).Interface().(*dwarf.Data)
			reader := image.DwarfReader()
			reader.Seek(dwarf.Offset(rOffset.Uint()))
			entry, err := reader.Next()
			if err != nil || entry == nil || entry.Tag != dwarf.TagVariable {
				continue
			}
			name, ok := entry.Val(dwarf.AttrName).(string)
			if !ok || rName.String() != name {
				continue
			}

			dtyp, err := entryType(dwarfData, entry)
			if err != nil {
				continue
			}
			dname := dwarfTypeName(dtyp)
			if dname == "<unspecified>" || dname == "" {
				continue
			}

			rtyp, err := d.FindType(dname)
			if err != nil || rtyp == nil {
				continue
			}
			d.globals[name] = reflect.NewAt(rtyp, unsafe.Pointer(uintptr(rAddr.Uint()))).Elem()
		}
	}
}

func (d *Dwarf) ForeachGlobal(f func(name string, v reflect.Value)) error {
	if err := d.check(); err != nil {
		return err
	}
	if d.globals == nil {
		d.loadGlobals()
	}

	for name, v := range d.globals {
		f(name, v)
	}
	return nil
}

func (d *Dwarf) FindGlobal(name string) (reflect.Value, error) {
	if err := d.check(); err != nil {
		return reflect.Value{}, err
	}
	if d.globals == nil {
		d.loadGlobals()
	}

	v, ok := d.globals[name]
	if !ok {
		return reflect.Value{}, ErrNotFound
	}
	return v, nil
}
