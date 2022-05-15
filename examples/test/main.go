package main

import (
	"debug/dwarf"
	"fmt"
	"log"
	"os"
	"reflect"
	"runtime"
	"unsafe"

	"github.com/go-delve/delve/pkg/dwarf/godwarf"
	"github.com/go-delve/delve/pkg/proc"
)

func main() {
	path, err := os.Executable()
	if err != nil {
		log.Fatalln(err)
	}

	bi := proc.NewBinaryInfo(runtime.GOOS, runtime.GOARCH)
	err = bi.LoadBinaryInfo(path, 0, nil)
	if err != nil {
		log.Fatalln(err)
	}
	mds, err := loadModuleData(bi, new(localMemory))
	if err != nil {
		log.Fatalln(err)
	}

	types, err := bi.Types()
	if err != nil {
		log.Fatalln(err)
	}

	for _, name := range types {
		dwarfType, err := findType(bi, name)
		if err != nil {
			continue
		}

		typeAddr, err := dwarfToRuntimeType(bi, mds, dwarfType, name)
		if err != nil {
			continue
		}

		typ := reflect.TypeOf(*(*interface{})(unsafe.Pointer(&typeAddr)))
		log.Printf("load type name:%s type:%s\n", name, typ)
	}
}

// delve counterpart to runtime.moduledata
type moduleData struct {
	text, etext   uint64
	types, etypes uint64
	typemapVar    *proc.Variable
}

//go:linkname findType github.com/go-delve/delve/pkg/proc.(*BinaryInfo).findType
func findType(bi *proc.BinaryInfo, name string) (godwarf.Type, error)

//go:linkname loadModuleData github.com/go-delve/delve/pkg/proc.loadModuleData
func loadModuleData(bi *proc.BinaryInfo, mem proc.MemoryReadWriter) ([]moduleData, error)

//go:linkname imageToModuleData github.com/go-delve/delve/pkg/proc.(*BinaryInfo).imageToModuleData
func imageToModuleData(bi *proc.BinaryInfo, image *proc.Image, mds []moduleData) *moduleData

type localMemory int

func (mem *localMemory) ReadMemory(data []byte, addr uint64) (int, error) {
	buf := *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{Data: uintptr(addr), Len: len(data), Cap: len(data)}))
	copy(data, buf)
	return len(data), nil
}

func (mem *localMemory) WriteMemory(addr uint64, data []byte) (int, error) {
	return 0, fmt.Errorf("not support")
}

func dwarfToRuntimeType(bi *proc.BinaryInfo, mds []moduleData, typ godwarf.Type, name string) (typeAddr uint64, err error) {
	if typ.Common().Index >= len(bi.Images) {
		return 0, fmt.Errorf("could not find image for type %s", name)
	}
	img := bi.Images[typ.Common().Index]
	rdr := img.DwarfReader()
	rdr.Seek(typ.Common().Offset)
	e, err := rdr.Next()
	if err != nil {
		return 0, fmt.Errorf("could not find dwarf entry for type:%s err:%s", name, err)
	}
	entryName, ok := e.Val(dwarf.AttrName).(string)
	if !ok || entryName != name {
		return 0, fmt.Errorf("could not find name for type:%s entry:%s", name, entryName)
	}
	off, ok := e.Val(godwarf.AttrGoRuntimeType).(uint64)
	if !ok || off == 0 {
		return 0, fmt.Errorf("could not find runtime type for type:%s", name)
	}

	md := imageToModuleData(bi, img, mds)
	if md == nil {
		return 0, fmt.Errorf("could not find module data for type %s", name)
	}

	typeAddr = md.types + off
	if typeAddr < md.types || typeAddr >= md.etypes {
		return off, nil
	}
	return typeAddr, nil
}
