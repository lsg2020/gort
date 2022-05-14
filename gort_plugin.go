package gort

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/go-delve/delve/pkg/proc"
)

const (
	maxNumLibraries      = 1000000 // maximum number of loaded libraries, to avoid loading forever on corrupted memory
	maxLibraryPathLength = 1000000 // maximum length for the path of a library, to avoid loading forever on corrupted memory
)

const (
	_DT_NULL  = 0  // DT_NULL as defined by SysV ABI specification
	_DT_DEBUG = 21 // DT_DEBUG as defined by SysV ABI specification
)

func (d *Dwarf) SearchPluginByName(name string) (string, uint64, error) {
	libs, addr, err := d.SearchPlugins()
	if err != nil {
		return "", 0, err
	}
	for i := 0; i < len(libs); i++ {
		if strings.LastIndex(libs[i], name) >= 0 {
			return libs[i], addr[i], nil
		}
	}
	return "", 0, ErrNotFound
}

func (d *Dwarf) SearchPlugins() ([]string, []uint64, error) {
	if err := d.check(); err != nil {
		return nil, nil, err
	}
	bi := d.bi

	if bi.ElfDynamicSection.Addr == 0 {
		// no dynamic section, therefore nothing to do here
		return nil, nil, nil
	}
	debugAddr, err := dynamicSearchDebug(bi)
	if err != nil {
		return nil, nil, err
	}
	if debugAddr == 0 {
		// no DT_DEBUG entry
		return nil, nil, nil
	}

	// Offsets of the fields of the r_debug and link_map structs,
	// see /usr/include/elf/link.h for a full description of those structs.
	debugMapOffset := uint64(bi.Arch.PtrSize())

	r_map, err := readPtr(bi, debugAddr+debugMapOffset)
	if err != nil {
		return nil, nil, err
	}

	var libs []string
	var addr []uint64

	for {
		if r_map == 0 {
			break
		}
		if len(libs) > maxNumLibraries {
			return nil, nil, ErrTooManyLibraries
		}
		lm, err := readLinkMapNode(bi, r_map)
		if err != nil {
			return nil, nil, err
		}

		libs = append(libs, lm.name)
		addr = append(addr, lm.addr)
		r_map = lm.next
	}

	return libs, addr, nil
}

func readPtr(bi *proc.BinaryInfo, addr uint64) (uint64, error) {
	ptrbuf := entryAddress(uintptr(addr), bi.Arch.PtrSize())
	return readUintRaw(bytes.NewReader(ptrbuf), binary.LittleEndian, bi.Arch.PtrSize())
}

// readUintRaw reads an integer of ptrSize bytes, with the specified byte order, from reader.
func readUintRaw(reader io.Reader, order binary.ByteOrder, ptrSize int) (uint64, error) {
	switch ptrSize {
	case 4:
		var n uint32
		if err := binary.Read(reader, order, &n); err != nil {
			return 0, err
		}
		return uint64(n), nil
	case 8:
		var n uint64
		if err := binary.Read(reader, order, &n); err != nil {
			return 0, err
		}
		return n, nil
	}
	return 0, fmt.Errorf("not supprted ptr size %d", ptrSize)
}

// dynamicSearchDebug searches for the DT_DEBUG entry in the .dynamic section
func dynamicSearchDebug(bi *proc.BinaryInfo) (uint64, error) {
	dynbuf := entryAddress(uintptr(bi.ElfDynamicSection.Addr), int(bi.ElfDynamicSection.Size))
	rd := bytes.NewReader(dynbuf)

	for {
		var tag, val uint64
		var err error
		if tag, err = readUintRaw(rd, binary.LittleEndian, bi.Arch.PtrSize()); err != nil {
			return 0, err
		}
		if val, err = readUintRaw(rd, binary.LittleEndian, bi.Arch.PtrSize()); err != nil {
			return 0, err
		}
		switch tag {
		case _DT_NULL:
			return 0, nil
		case _DT_DEBUG:
			return val, nil
		}
	}
}

type linkMap struct {
	addr       uint64
	name       string
	ld         uint64
	next, prev uint64
}

func readLinkMapNode(bi *proc.BinaryInfo, r_map uint64) (*linkMap, error) {
	var lm linkMap
	var ptrs [5]uint64
	for i := range ptrs {
		var err error
		ptrs[i], err = readPtr(bi, r_map+uint64(bi.Arch.PtrSize()*i))
		if err != nil {
			return nil, err
		}
	}
	lm.addr = ptrs[0]
	var err error
	lm.name, err = readCString(ptrs[1])
	if err != nil {
		return nil, err
	}
	lm.ld = ptrs[2]
	lm.next = ptrs[3]
	lm.prev = ptrs[4]
	return &lm, nil
}

func readCString(addr uint64) (string, error) {
	if addr == 0 {
		return "", nil
	}
	r := []byte{}
	for {
		if len(r) > maxLibraryPathLength {
			return "", fmt.Errorf("error reading libraries: string too long (%d)", len(r))
		}
		buf := entryAddress(uintptr(addr), 1)
		if buf[0] == 0 {
			break
		}
		r = append(r, buf[0])
		addr++
	}
	return string(r), nil
}
