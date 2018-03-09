package kkformat

import (
	"encoding/binary"
)

var firstZeroLookup = [255]uint8{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 4, 4, 5, 5, 5, 5, 6, 6, 7,
}

var bitManipulationClear = [64]uint64{
	0x7fffffffffffffff, 0xbfffffffffffffff, 0xdfffffffffffffff, 0xefffffffffffffff,
	0xf7ffffffffffffff, 0xfbffffffffffffff, 0xfdffffffffffffff, 0xfeffffffffffffff,
	0xff7fffffffffffff, 0xffbfffffffffffff, 0xffdfffffffffffff, 0xffefffffffffffff,
	0xfff7ffffffffffff, 0xfffbffffffffffff, 0xfffdffffffffffff, 0xfffeffffffffffff,
	0xffff7fffffffffff, 0xffffbfffffffffff, 0xffffdfffffffffff, 0xffffefffffffffff,
	0xfffff7ffffffffff, 0xfffffbffffffffff, 0xfffffdffffffffff, 0xfffffeffffffffff,
	0xffffff7fffffffff, 0xffffffbfffffffff, 0xffffffdfffffffff, 0xffffffefffffffff,
	0xfffffff7ffffffff, 0xfffffffbffffffff, 0xfffffffdffffffff, 0xfffffffeffffffff,
	0xffffffff7fffffff, 0xffffffffbfffffff, 0xffffffffdfffffff, 0xffffffffefffffff,
	0xfffffffff7ffffff, 0xfffffffffbffffff, 0xfffffffffdffffff, 0xfffffffffeffffff,
	0xffffffffff7fffff, 0xffffffffffbfffff, 0xffffffffffdfffff, 0xffffffffffefffff,
	0xfffffffffff7ffff, 0xfffffffffffbffff, 0xfffffffffffdffff, 0xfffffffffffeffff,
	0xffffffffffff7fff, 0xffffffffffffbfff, 0xffffffffffffdfff, 0xffffffffffffefff,
	0xfffffffffffff7ff, 0xfffffffffffffbff, 0xfffffffffffffdff, 0xfffffffffffffeff,
	0xffffffffffffff7f, 0xffffffffffffffbf, 0xffffffffffffffdf, 0xffffffffffffffef,
	0xfffffffffffffff7, 0xfffffffffffffffb, 0xfffffffffffffffd, 0xfffffffffffffffe,
}

const (
	uint64Max = 0xffffffffffffffff
)

func firstZero64(v uint64) int {
	if v == uint64Max {
		return -1
	}

	var bits uint64 = 64
	var h uint8

	for {
		bits -= 8
		h = uint8(v >> bits)
		if h != 255 {
			break
		}
	}

	return 56 - int(bits) + int(firstZeroLookup[h])
}

func setBit64(v uint64, bit int, value int) uint64 {
	if value == 0 {
		return v & bitManipulationClear[bit]
	}
	return v | ^bitManipulationClear[bit]
}

type block [520]byte

func (b *block) setMaster(m int, v int) {
	master := binary.BigEndian.Uint64(b[512:])
	master = setBit64(master, m, v)
	binary.BigEndian.PutUint64(b[512:], master)
}

func (b *block) mark(index int) {
	if index < 0 || index > 4096 {
		panic(index)
	}

	m := index / 64
	s := index - m*64
	v := binary.BigEndian.Uint64(b[m*8:])
	if v == uint64Max {
		return
	}

	v = setBit64(v, s, 1)
	binary.BigEndian.PutUint64(b[m*8:], v)
	if v == uint64Max {
		b.setMaster(m, 1)
	}
}

func (b *block) unmark(index int) {
	if index < 0 || index > 4096 {
		panic(index)
	}

	m := index / 64
	s := index - m*64

	v := binary.BigEndian.Uint64(b[m*8:])
	if v == 0 {
		return
	}

	v = setBit64(v, s, 0)
	binary.BigEndian.PutUint64(b[m*8:], v)

	if v == 0 {
		b.setMaster(m, 0)
	}
}

func (b *block) getFirstUnmarked() int {
	master := binary.BigEndian.Uint64(b[512:])
	if master == uint64Max {
		return -1
	}

	m := firstZero64(master)
	v := binary.BigEndian.Uint64(b[m*8:])
	s := firstZero64(v)
	return m*64 + s
}

func (b *block) clear() {
	for i := 0; i < len(b); i++ {
		b[i] = 0
	}
}
