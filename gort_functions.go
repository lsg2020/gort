package gort

import (
	"debug/dwarf"
	"fmt"
	"reflect"
	"unsafe"

	"github.com/go-delve/delve/pkg/proc"
)

func (d *Dwarf) findFunc(name string) (*proc.Function, error) {
	for i := len(d.bi.Functions) - 1; i >= 0; i-- {
		if d.bi.Functions[i].Name == name {
			if d.bi.Functions[i].Entry != 0 {
				return &(d.bi.Functions[i]), nil
			}
			return nil, ErrNotFound
		}
	}
	return nil, ErrNotFound
}

func (d *Dwarf) getFunctionArgTypes(f *proc.Function) ([]reflect.Type, []reflect.Type, []string, []string, error) {
	rOffset := reflect.ValueOf(f).Elem().FieldByName("offset")
	rCU := reflect.ValueOf(f).Elem().FieldByName("cu")
	if !rOffset.IsValid() || !rCU.IsValid() {
		return nil, nil, nil, nil, ErrNotSupport
	}
	rImage := rCU.Elem().FieldByName("image")
	if !rImage.IsValid() {
		return nil, nil, nil, nil, ErrNotSupport
	}
	rDwarf := rImage.Elem().FieldByName("dwarf")
	if !rDwarf.IsValid() {
		return nil, nil, nil, nil, ErrNotSupport
	}
	image := reflect.NewAt(rImage.Type().Elem(), unsafe.Pointer(rImage.Elem().UnsafeAddr())).Interface().(*proc.Image)
	dwarfData := reflect.NewAt(rDwarf.Type().Elem(), unsafe.Pointer(rDwarf.Elem().UnsafeAddr())).Interface().(*dwarf.Data)
	reader := image.DwarfReader()
	reader.Seek(dwarf.Offset(rOffset.Uint()))
	entry, err := reader.Next()
	if err != nil || entry == nil || entry.Tag != dwarf.TagSubprogram {
		return nil, nil, nil, nil, fmt.Errorf("get function arg types not found %s", f.Name)
	}
	name, ok := entry.Val(dwarf.AttrName).(string)
	if !ok || f.Name != name {
		return nil, nil, nil, nil, fmt.Errorf("get function arg types name err %s:%s", f.Name, name)
	}

	var inTyps []reflect.Type
	var outTyps []reflect.Type
	var inNames []string
	var outNames []string

	for {
		child, err := reader.Next()
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("get function arg types reader err %s:%s", f.Name, err.Error())
		}
		if child == nil || child.Tag == 0 {
			break
		}
		if child.Tag != dwarf.TagFormalParameter {
			continue
		}

		dtyp, err := entryType(dwarfData, child)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("get function arg types type err %s:%s", f.Name, err.Error())
		}
		dname := dwarfTypeName(dtyp)
		rtyp, err := d.FindType(dname)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("get function arg types type err %s:%s", f.Name, err.Error())
		}

		isret, _ := child.Val(dwarf.AttrVarParam).(bool)
		if isret {
			outTyps = append(outTyps, rtyp)
			outNames = append(outNames, dname)
		} else {
			inTyps = append(inTyps, rtyp)
			inNames = append(inNames, dname)
		}
	}
	return inTyps, outTyps, inNames, outNames, nil
}

func (d *Dwarf) ForeachFunc(f func(name string, pc uint64)) error {
	if err := d.check(); err != nil {
		return err
	}

	for _, function := range d.bi.Functions {
		if function.Entry != 0 {
			f(function.Name, function.Entry)
		}
	}
	return nil
}

func (d *Dwarf) FindFuncEntry(name string) (*proc.Function, error) {
	if err := d.check(); err != nil {
		return nil, err
	}

	f, err := d.findFunc(name)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (d *Dwarf) FindFuncPc(name string) (uint64, error) {
	if err := d.check(); err != nil {
		return 0, err
	}

	f, err := d.findFunc(name)
	if err != nil {
		return 0, err
	}
	return f.Entry, nil
}

func (d *Dwarf) FindFuncType(name string, variadic bool) (reflect.Type, error) {
	if err := d.check(); err != nil {
		return nil, err
	}

	f, err := d.findFunc(name)
	if err != nil {
		return nil, err
	}
	inTyps, outTyps, _, _, err := d.getFunctionArgTypes(f)
	if err != nil {
		return nil, err
	}

	ftyp := reflect.FuncOf(inTyps, outTyps, variadic)
	return ftyp, nil
}

func (d *Dwarf) FindFunc(name string, variadic bool) (reflect.Value, error) {
	pc, err := d.FindFuncPc(name)
	if err != nil {
		return reflect.Value{}, err
	}
	ftyp, err := d.FindFuncType(name, variadic)
	if err != nil {
		return reflect.Value{}, err
	}

	newFunc := reflect.MakeFunc(ftyp, nil)
	funcPtrVal := reflect.ValueOf(newFunc).FieldByName("ptr").Pointer()
	funcPtr := (*Func)(unsafe.Pointer(funcPtrVal))
	funcPtr.codePtr = uintptr(pc)

	return newFunc, nil
}

func (d *Dwarf) CallFunc(name string, variadic bool, args []reflect.Value) ([]reflect.Value, error) {
	if err := d.check(); err != nil {
		return nil, err
	}
	f, err := d.findFunc(name)
	if err != nil {
		return nil, err
	}

	inTyps, outTyps, inNames, _, err := d.getFunctionArgTypes(f)
	if err != nil {
		return nil, err
	}

	ftyp := reflect.FuncOf(inTyps, outTyps, variadic)
	newFunc := reflect.MakeFunc(ftyp, nil)
	funcPtrVal := reflect.ValueOf(newFunc).FieldByName("ptr").Pointer()
	funcPtr := (*Func)(unsafe.Pointer(funcPtrVal))
	funcPtr.codePtr = uintptr(f.Entry)

	getInTyp := func(i int) (reflect.Type, string) {
		if len(inTyps) <= 0 {
			return nil, ""
		}
		if i < len(inTyps) {
			return inTyps[i], inNames[i]
		}
		if variadic {
			return inTyps[len(inTyps)-1], inNames[len(inNames)-1]
		}
		return nil, ""
	}

	for i, arg := range args {
		inTyp, inName := getInTyp(i)
		if inTyp == nil {
			return nil, fmt.Errorf("len mismatch %d", i)
		}

		if !arg.Type().AssignableTo(inTyp) {
			return nil, fmt.Errorf("type mismatch %d:%s", i, inName)
		}
	}

	out := newFunc.Call(args)
	return out, nil
}
