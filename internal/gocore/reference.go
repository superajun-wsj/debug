// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package gocore

import (
	"github.com/goretk/gore"
	"golang.org/x/debug/internal/core"
)

type spanClass uint8

const (
	_MaxSmallSize   = 32768
	smallSizeDiv    = 8
	smallSizeMax    = 1024
	largeSizeDiv    = 128
	_NumSizeClasses = 68
	// Tiny allocator parameters, see "Tiny allocator" comment in malloc.go.
	_TinySize      = 16
	_TinySizeClass = int8(2)
)
const (
	maxTinySize    = _TinySize
	tinySizeClass  = _TinySizeClass
	maxSmallSize   = _MaxSmallSize
	numSpanClasses = _NumSizeClasses << 1
	tinySpanClass  = spanClass(tinySizeClass<<1 | 1)
)

var class_to_size = [_NumSizeClasses]uint16{0, 8, 16, 24, 32, 48, 64, 80, 96, 112, 128, 144, 160, 176, 192, 208, 224, 240, 256, 288, 320, 352, 384, 416, 448, 480, 512, 576, 640, 704, 768, 896, 1024, 1152, 1280, 1408, 1536, 1792, 2048, 2304, 2688, 3072, 3200, 3456, 4096, 4864, 5376, 6144, 6528, 6784, 6912, 8192, 9472, 9728, 10240, 10880, 12288, 13568, 14336, 16384, 18432, 19072, 20480, 21760, 24576, 27264, 28672, 32768}
var size_to_class8 = [smallSizeMax/smallSizeDiv + 1]uint8{0, 1, 2, 3, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10, 10, 11, 11, 12, 12, 13, 13, 14, 14, 15, 15, 16, 16, 17, 17, 18, 18, 19, 19, 19, 19, 20, 20, 20, 20, 21, 21, 21, 21, 22, 22, 22, 22, 23, 23, 23, 23, 24, 24, 24, 24, 25, 25, 25, 25, 26, 26, 26, 26, 27, 27, 27, 27, 27, 27, 27, 27, 28, 28, 28, 28, 28, 28, 28, 28, 29, 29, 29, 29, 29, 29, 29, 29, 30, 30, 30, 30, 30, 30, 30, 30, 31, 31, 31, 31, 31, 31, 31, 31, 31, 31, 31, 31, 31, 31, 31, 31, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32, 32}
var size_to_class128 = [(_MaxSmallSize-smallSizeMax)/largeSizeDiv + 1]uint8{32, 33, 34, 35, 36, 37, 37, 38, 38, 39, 39, 40, 40, 40, 41, 41, 41, 42, 43, 43, 44, 44, 44, 44, 44, 45, 45, 45, 45, 45, 45, 46, 46, 46, 46, 47, 47, 47, 47, 47, 47, 48, 48, 48, 49, 49, 50, 51, 51, 51, 51, 51, 51, 51, 51, 51, 51, 52, 52, 52, 52, 52, 52, 52, 52, 52, 52, 53, 53, 54, 54, 54, 54, 55, 55, 55, 55, 55, 56, 56, 56, 56, 56, 56, 56, 56, 56, 56, 56, 57, 57, 57, 57, 57, 57, 57, 57, 57, 57, 58, 58, 58, 58, 58, 58, 59, 59, 59, 59, 59, 59, 59, 59, 59, 59, 59, 59, 59, 59, 59, 59, 60, 60, 60, 60, 60, 60, 60, 60, 60, 60, 60, 60, 60, 60, 60, 60, 61, 61, 61, 61, 61, 62, 62, 62, 62, 62, 62, 62, 62, 62, 62, 62, 63, 63, 63, 63, 63, 63, 63, 63, 63, 63, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 64, 65, 65, 65, 65, 65, 65, 65, 65, 65, 65, 65, 65, 65, 65, 65, 65, 65, 65, 65, 65, 65, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67, 67}

type matchType struct {
	single []*gore.GoType
	array  []*gore.GoType
}

// obj and _type must be in the same spanClass.
// obj must be in heap memory area.
// sTypeMatchByBitmap match a single type by its bitmap info.
func (p *Process) sTypeMatchByBitmap(obj Object, _type *gore.GoType) bool {
	ptrSize := p.proc.PtrSize()
	spanInfo := p.findHeapInfo(core.Address(obj))
	spanSize := spanInfo.size
	typeSize := _type.Size
	// check size class.
	if spanSize != int64(class_to_size[uint8(p.calSizeClass(int(typeSize)))]) {
		return false
	}
	// noscan means no bitmap info, always view it as no-matched.
	if spanInfo.noscan || _type.PtrBytes == 0x0 {
		return false
	}
	// check bitmap match.
	if (!spanInfo.noscan) && _type.PtrBytes > 0x0 {
		addr := core.Address(obj)
		for index := 0x0; index < int(spanInfo.size)/int(ptrSize); index++ {
			if p.findHeapInfo(addr).IsPtr(addr, ptrSize) != _type.IsPtr(index, ptrSize) {
				return false
			}
			addr = addr.Add(ptrSize)
		}
	}
	return true
}

// obj must be in heap memory area.
// aTypeMatchByBitmap match a type's array bitmap info, for example: [20]_type
func (p *Process) aTypeMatchByBitmap(obj Object, _type *gore.GoType) bool {
	ptrSize := p.proc.PtrSize()
	spanInfo := p.findHeapInfo(core.Address(obj))
	spanSize := spanInfo.size
	typeSize := _type.Size
	// view [1]_type case as _type here.
	if spanSize <= 2*int64(typeSize) {
		return false
	}
	// noscan means no bitmap info, always view it as no-matched.
	if spanInfo.noscan || _type.PtrBytes == 0x0 {
		return false
	}
	// check size class consistency.
	num := spanSize / int64(typeSize)
	mod := spanSize % int64(typeSize)
	// array type should not store in this span class.
	if mod != 0x0 && p.calSizeClass(int(spanSize)) != p.calSizeClass(int(num)*int(typeSize)) {
		return false
	}
	// check bitmap match.
	// assume that struct in an array is compacted.
	addr := core.Address(obj)
	for i := 0x0; i < int(num); i++ {
		for index := 0x0; index < int(typeSize)/int(ptrSize); index++ {
			if p.findHeapInfo(addr).IsPtr(addr, ptrSize) != _type.IsPtr(index, ptrSize) {
				return false
			}
			addr = addr.Add(ptrSize)
		}
	}
	// check the tail bitmap is not ptr if it has.
	for i := 0; i < int(mod); i++ {
		if p.findHeapInfo(addr).IsPtr(addr, ptrSize) {
			return false
		}
		addr = addr.Add(ptrSize)
	}
	return true
}

// typeMatchCheck try to match all types which bitmap match.
func (p *Process) typeMatchCheck(a core.Address) (result *matchType) {
	// get the head address with offset = 0
	obj, off0 := p.FindObject(a)
	// address a is not a heap address.
	if obj == 0 && off0 == 0 {
		return
	}
	// calculate heap info by head address.
	spanInfo := p.findHeapInfo(core.Address(obj))
	// noscan means no bitmap info, always view it as no-matched.
	noscan := spanInfo.noscan
	if noscan {
		return
	}
	// match possible single type case and array type case.
	spansize := spanInfo.size
	spanclass := p.calSpanClass(int(spansize), noscan)
	result = &matchType{single: make([]*gore.GoType, 0), array: make([]*gore.GoType, 0)}
	if p.spanClassModuleType[spanclass] != nil {
		for _, _type := range p.spanClassModuleType[spanclass] {
			if p.sTypeMatchByBitmap(obj, _type) {
				result.single = append(result.single, _type)
			}
			if p.aTypeMatchByBitmap(obj, _type) {
				result.array = append(result.array, _type)
			}
		}
	}
	return
}
