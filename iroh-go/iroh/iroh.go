package iroh

/*
#cgo windows LDFLAGS: -L${SRCDIR} -liroh
#cgo linux LDFLAGS: -L${SRCDIR} -liroh -lm -Wl,-unresolved-symbols=ignore-all
#cgo darwin LDFLAGS: -L${SRCDIR} -liroh -Wl,-undefined,dynamic_lookup
#include "./iroh.h"
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type RustBuffer = C.RustBuffer

type RustBufferI interface {
	AsReader() *bytes.Reader
	Free()
	ToGoBytes() []byte
	Data() unsafe.Pointer
	Len() int
	Capacity() int
}

func RustBufferFromExternal(b RustBufferI) RustBuffer {
	return RustBuffer{
		capacity: C.int(b.Capacity()),
		len:      C.int(b.Len()),
		data:     (*C.uchar)(b.Data()),
	}
}

func (cb RustBuffer) Capacity() int {
	return int(cb.capacity)
}

func (cb RustBuffer) Len() int {
	return int(cb.len)
}

func (cb RustBuffer) Data() unsafe.Pointer {
	return unsafe.Pointer(cb.data)
}

func (cb RustBuffer) AsReader() *bytes.Reader {
	b := unsafe.Slice((*byte)(cb.data), C.int(cb.len))
	return bytes.NewReader(b)
}

func (cb RustBuffer) Free() {
	rustCall(func(status *C.RustCallStatus) bool {
		C.ffi_iroh_rustbuffer_free(cb, status)
		return false
	})
}

func (cb RustBuffer) ToGoBytes() []byte {
	return C.GoBytes(unsafe.Pointer(cb.data), C.int(cb.len))
}

func stringToRustBuffer(str string) RustBuffer {
	return bytesToRustBuffer([]byte(str))
}

func bytesToRustBuffer(b []byte) RustBuffer {
	if len(b) == 0 {
		return RustBuffer{}
	}
	// We can pass the pointer along here, as it is pinned
	// for the duration of this call
	foreign := C.ForeignBytes{
		len:  C.int(len(b)),
		data: (*C.uchar)(unsafe.Pointer(&b[0])),
	}

	return rustCall(func(status *C.RustCallStatus) RustBuffer {
		return C.ffi_iroh_rustbuffer_from_bytes(foreign, status)
	})
}

type BufLifter[GoType any] interface {
	Lift(value RustBufferI) GoType
}

type BufLowerer[GoType any] interface {
	Lower(value GoType) RustBuffer
}

type FfiConverter[GoType any, FfiType any] interface {
	Lift(value FfiType) GoType
	Lower(value GoType) FfiType
}

type BufReader[GoType any] interface {
	Read(reader io.Reader) GoType
}

type BufWriter[GoType any] interface {
	Write(writer io.Writer, value GoType)
}

type FfiRustBufConverter[GoType any, FfiType any] interface {
	FfiConverter[GoType, FfiType]
	BufReader[GoType]
}

func LowerIntoRustBuffer[GoType any](bufWriter BufWriter[GoType], value GoType) RustBuffer {
	// This might be not the most efficient way but it does not require knowing allocation size
	// beforehand
	var buffer bytes.Buffer
	bufWriter.Write(&buffer, value)

	bytes, err := io.ReadAll(&buffer)
	if err != nil {
		panic(fmt.Errorf("reading written data: %w", err))
	}
	return bytesToRustBuffer(bytes)
}

func LiftFromRustBuffer[GoType any](bufReader BufReader[GoType], rbuf RustBufferI) GoType {
	defer rbuf.Free()
	reader := rbuf.AsReader()
	item := bufReader.Read(reader)
	if reader.Len() > 0 {
		// TODO: Remove this
		leftover, _ := io.ReadAll(reader)
		panic(fmt.Errorf("Junk remaining in buffer after lifting: %s", string(leftover)))
	}
	return item
}

func rustCallWithError[U any](converter BufLifter[error], callback func(*C.RustCallStatus) U) (U, error) {
	var status C.RustCallStatus
	returnValue := callback(&status)
	err := checkCallStatus(converter, status)

	return returnValue, err
}

func checkCallStatus(converter BufLifter[error], status C.RustCallStatus) error {
	switch status.code {
	case 0:
		return nil
	case 1:
		return converter.Lift(status.errorBuf)
	case 2:
		// when the rust code sees a panic, it tries to construct a rustbuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(status.errorBuf)))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		return fmt.Errorf("unknown status code: %d", status.code)
	}
}

func checkCallStatusUnknown(status C.RustCallStatus) error {
	switch status.code {
	case 0:
		return nil
	case 1:
		panic(fmt.Errorf("function not returning an error returned an error"))
	case 2:
		// when the rust code sees a panic, it tries to construct a rustbuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(status.errorBuf)))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		return fmt.Errorf("unknown status code: %d", status.code)
	}
}

func rustCall[U any](callback func(*C.RustCallStatus) U) U {
	returnValue, err := rustCallWithError(nil, callback)
	if err != nil {
		panic(err)
	}
	return returnValue
}

func writeInt8(writer io.Writer, value int8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint8(writer io.Writer, value uint8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt16(writer io.Writer, value int16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint16(writer io.Writer, value uint16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt32(writer io.Writer, value int32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint32(writer io.Writer, value uint32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt64(writer io.Writer, value int64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint64(writer io.Writer, value uint64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat32(writer io.Writer, value float32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat64(writer io.Writer, value float64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func readInt8(reader io.Reader) int8 {
	var result int8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint8(reader io.Reader) uint8 {
	var result uint8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt16(reader io.Reader) int16 {
	var result int16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint16(reader io.Reader) uint16 {
	var result uint16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt32(reader io.Reader) int32 {
	var result int32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint32(reader io.Reader) uint32 {
	var result uint32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt64(reader io.Reader) int64 {
	var result int64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint64(reader io.Reader) uint64 {
	var result uint64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat32(reader io.Reader) float32 {
	var result float32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat64(reader io.Reader) float64 {
	var result float64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func init() {

	(&FfiConverterCallbackInterfaceAddCallback{}).register()
	(&FfiConverterCallbackInterfaceDocExportFileCallback{}).register()
	(&FfiConverterCallbackInterfaceDocImportFileCallback{}).register()
	(&FfiConverterCallbackInterfaceDownloadCallback{}).register()
	(&FfiConverterCallbackInterfaceSubscribeCallback{}).register()
	uniffiCheckChecksums()
}

func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 24
	// Get the scaffolding contract version by calling the into the dylib
	scaffoldingContractVersion := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.ffi_iroh_uniffi_contract_version(uniffiStatus)
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("iroh: UniFFI contract version mismatch")
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_func_key_to_path(uniffiStatus)
		})
		if checksum != 1201 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_func_key_to_path: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_func_path_to_key(uniffiStatus)
		})
		if checksum != 27769 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_func_path_to_key: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_func_set_log_level(uniffiStatus)
		})
		if checksum != 52296 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_func_set_log_level: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_func_start_metrics_collection(uniffiStatus)
		})
		if checksum != 17691 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_func_start_metrics_collection: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_addprogress_as_abort(uniffiStatus)
		})
		if checksum != 64540 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_addprogress_as_abort: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_addprogress_as_all_done(uniffiStatus)
		})
		if checksum != 24629 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_addprogress_as_all_done: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_addprogress_as_done(uniffiStatus)
		})
		if checksum != 65369 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_addprogress_as_done: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_addprogress_as_found(uniffiStatus)
		})
		if checksum != 14508 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_addprogress_as_found: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_addprogress_as_progress(uniffiStatus)
		})
		if checksum != 54075 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_addprogress_as_progress: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_addprogress_type(uniffiStatus)
		})
		if checksum != 63416 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_addprogress_type: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_authorid_equal(uniffiStatus)
		})
		if checksum != 33867 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_authorid_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_authorid_to_string(uniffiStatus)
		})
		if checksum != 42389 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_authorid_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_connectiontype_as_direct(uniffiStatus)
		})
		if checksum != 41690 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_connectiontype_as_direct: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_connectiontype_as_mixed(uniffiStatus)
		})
		if checksum != 41300 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_connectiontype_as_mixed: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_connectiontype_as_relay(uniffiStatus)
		})
		if checksum != 54439 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_connectiontype_as_relay: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_connectiontype_type(uniffiStatus)
		})
		if checksum != 1057 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_connectiontype_type: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_directaddrinfo_addr(uniffiStatus)
		})
		if checksum != 49936 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_directaddrinfo_addr: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_directaddrinfo_last_control(uniffiStatus)
		})
		if checksum != 46706 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_directaddrinfo_last_control: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_directaddrinfo_last_payload(uniffiStatus)
		})
		if checksum != 16797 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_directaddrinfo_last_payload: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_directaddrinfo_latency(uniffiStatus)
		})
		if checksum != 62303 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_directaddrinfo_latency: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_close(uniffiStatus)
		})
		if checksum != 23013 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_close: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_del(uniffiStatus)
		})
		if checksum != 22285 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_del: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_export_file(uniffiStatus)
		})
		if checksum != 34185 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_export_file: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_get_download_policy(uniffiStatus)
		})
		if checksum != 13666 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_get_download_policy: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_get_exact(uniffiStatus)
		})
		if checksum != 48441 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_get_exact: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_get_many(uniffiStatus)
		})
		if checksum != 58857 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_get_many: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_get_one(uniffiStatus)
		})
		if checksum != 25151 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_get_one: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_id(uniffiStatus)
		})
		if checksum != 34677 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_import_file(uniffiStatus)
		})
		if checksum != 33349 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_import_file: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_leave(uniffiStatus)
		})
		if checksum != 55816 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_leave: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_set_bytes(uniffiStatus)
		})
		if checksum != 15024 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_set_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_set_download_policy(uniffiStatus)
		})
		if checksum != 13428 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_set_download_policy: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_set_hash(uniffiStatus)
		})
		if checksum != 20311 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_set_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_share(uniffiStatus)
		})
		if checksum != 28913 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_share: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_start_sync(uniffiStatus)
		})
		if checksum != 54158 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_start_sync: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_status(uniffiStatus)
		})
		if checksum != 59550 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_status: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_doc_subscribe(uniffiStatus)
		})
		if checksum != 2866 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_doc_subscribe: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docexportprogress_as_abort(uniffiStatus)
		})
		if checksum != 39226 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docexportprogress_as_abort: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docexportprogress_as_found(uniffiStatus)
		})
		if checksum != 11254 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docexportprogress_as_found: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docexportprogress_as_progress(uniffiStatus)
		})
		if checksum != 8859 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docexportprogress_as_progress: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docexportprogress_type(uniffiStatus)
		})
		if checksum != 43844 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docexportprogress_type: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docimportprogress_as_abort(uniffiStatus)
		})
		if checksum != 45779 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docimportprogress_as_abort: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docimportprogress_as_all_done(uniffiStatus)
		})
		if checksum != 7478 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docimportprogress_as_all_done: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docimportprogress_as_found(uniffiStatus)
		})
		if checksum != 55008 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docimportprogress_as_found: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docimportprogress_as_ingest_done(uniffiStatus)
		})
		if checksum != 37186 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docimportprogress_as_ingest_done: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docimportprogress_as_progress(uniffiStatus)
		})
		if checksum != 35401 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docimportprogress_as_progress: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docimportprogress_type(uniffiStatus)
		})
		if checksum != 49227 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docimportprogress_type: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docticket_equal(uniffiStatus)
		})
		if checksum != 14909 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docticket_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docticket_to_string(uniffiStatus)
		})
		if checksum != 22814 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docticket_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_downloadprogress_as_abort(uniffiStatus)
		})
		if checksum != 13741 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_downloadprogress_as_abort: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_downloadprogress_as_done(uniffiStatus)
		})
		if checksum != 54270 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_downloadprogress_as_done: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_downloadprogress_as_export(uniffiStatus)
		})
		if checksum != 48739 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_downloadprogress_as_export: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_downloadprogress_as_export_progress(uniffiStatus)
		})
		if checksum != 42097 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_downloadprogress_as_export_progress: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_downloadprogress_as_found(uniffiStatus)
		})
		if checksum != 13482 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_downloadprogress_as_found: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_downloadprogress_as_found_hash_seq(uniffiStatus)
		})
		if checksum != 64232 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_downloadprogress_as_found_hash_seq: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_downloadprogress_as_network_done(uniffiStatus)
		})
		if checksum != 49397 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_downloadprogress_as_network_done: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_downloadprogress_as_progress(uniffiStatus)
		})
		if checksum != 7204 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_downloadprogress_as_progress: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_downloadprogress_type(uniffiStatus)
		})
		if checksum != 8349 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_downloadprogress_type: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_entry_author(uniffiStatus)
		})
		if checksum != 26124 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_entry_author: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_entry_content_bytes(uniffiStatus)
		})
		if checksum != 26896 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_entry_content_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_entry_content_hash(uniffiStatus)
		})
		if checksum != 39306 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_entry_content_hash: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_entry_content_len(uniffiStatus)
		})
		if checksum != 60107 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_entry_content_len: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_entry_key(uniffiStatus)
		})
		if checksum != 19122 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_entry_key: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_entry_namespace(uniffiStatus)
		})
		if checksum != 41306 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_entry_namespace: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_filterkind_matches(uniffiStatus)
		})
		if checksum != 35187 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_filterkind_matches: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_hash_equal(uniffiStatus)
		})
		if checksum != 65301 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_hash_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_hash_to_bytes(uniffiStatus)
		})
		if checksum != 29465 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_hash_to_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_hash_to_hex(uniffiStatus)
		})
		if checksum != 27622 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_hash_to_hex: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_hash_to_string(uniffiStatus)
		})
		if checksum != 61408 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_hash_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_ipv4addr_equal(uniffiStatus)
		})
		if checksum != 51523 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_ipv4addr_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_ipv4addr_octets(uniffiStatus)
		})
		if checksum != 17752 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_ipv4addr_octets: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_ipv4addr_to_string(uniffiStatus)
		})
		if checksum != 5658 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_ipv4addr_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_ipv6addr_equal(uniffiStatus)
		})
		if checksum != 26037 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_ipv6addr_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_ipv6addr_segments(uniffiStatus)
		})
		if checksum != 41182 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_ipv6addr_segments: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_ipv6addr_to_string(uniffiStatus)
		})
		if checksum != 46637 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_ipv6addr_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_author_create(uniffiStatus)
		})
		if checksum != 31148 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_author_create: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_author_list(uniffiStatus)
		})
		if checksum != 12499 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_author_list: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_blobs_add_bytes(uniffiStatus)
		})
		if checksum != 20668 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_blobs_add_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_blobs_add_from_path(uniffiStatus)
		})
		if checksum != 38440 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_blobs_add_from_path: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_blobs_delete_blob(uniffiStatus)
		})
		if checksum != 24766 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_blobs_delete_blob: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_blobs_download(uniffiStatus)
		})
		if checksum != 50921 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_blobs_download: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_blobs_list(uniffiStatus)
		})
		if checksum != 49039 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_blobs_list: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_blobs_list_collections(uniffiStatus)
		})
		if checksum != 28497 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_blobs_list_collections: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_blobs_list_incomplete(uniffiStatus)
		})
		if checksum != 39285 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_blobs_list_incomplete: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_blobs_read_to_bytes(uniffiStatus)
		})
		if checksum != 6512 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_blobs_read_to_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_blobs_size(uniffiStatus)
		})
		if checksum != 52941 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_blobs_size: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_blobs_write_to_path(uniffiStatus)
		})
		if checksum != 9029 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_blobs_write_to_path: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_connection_info(uniffiStatus)
		})
		if checksum != 39895 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_connection_info: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_connections(uniffiStatus)
		})
		if checksum != 37352 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_connections: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_doc_create(uniffiStatus)
		})
		if checksum != 64213 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_doc_create: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_doc_drop(uniffiStatus)
		})
		if checksum != 64324 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_doc_drop: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_doc_join(uniffiStatus)
		})
		if checksum != 30773 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_doc_join: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_doc_list(uniffiStatus)
		})
		if checksum != 44252 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_doc_list: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_doc_open(uniffiStatus)
		})
		if checksum != 8490 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_doc_open: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_node_id(uniffiStatus)
		})
		if checksum != 31962 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_node_id: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_stats(uniffiStatus)
		})
		if checksum != 16158 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_stats: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_status(uniffiStatus)
		})
		if checksum != 32660 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_status: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_tags_delete(uniffiStatus)
		})
		if checksum != 21632 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_tags_delete: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_irohnode_tags_list(uniffiStatus)
		})
		if checksum != 6726 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_irohnode_tags_list: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_liveevent_as_content_ready(uniffiStatus)
		})
		if checksum != 15237 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_liveevent_as_content_ready: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_liveevent_as_insert_local(uniffiStatus)
		})
		if checksum != 431 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_liveevent_as_insert_local: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_liveevent_as_insert_remote(uniffiStatus)
		})
		if checksum != 17302 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_liveevent_as_insert_remote: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_liveevent_as_neighbor_down(uniffiStatus)
		})
		if checksum != 154 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_liveevent_as_neighbor_down: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_liveevent_as_neighbor_up(uniffiStatus)
		})
		if checksum != 25727 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_liveevent_as_neighbor_up: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_liveevent_as_sync_finished(uniffiStatus)
		})
		if checksum != 14329 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_liveevent_as_sync_finished: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_liveevent_type(uniffiStatus)
		})
		if checksum != 35533 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_liveevent_type: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_namespaceid_equal(uniffiStatus)
		})
		if checksum != 18805 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_namespaceid_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_namespaceid_to_string(uniffiStatus)
		})
		if checksum != 63715 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_namespaceid_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_nodeaddr_derp_url(uniffiStatus)
		})
		if checksum != 1517 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_nodeaddr_derp_url: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_nodeaddr_direct_addresses(uniffiStatus)
		})
		if checksum != 20857 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_nodeaddr_direct_addresses: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_nodeaddr_equal(uniffiStatus)
		})
		if checksum != 45841 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_nodeaddr_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_nodestatusresponse_listen_addrs(uniffiStatus)
		})
		if checksum != 44280 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_nodestatusresponse_listen_addrs: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_nodestatusresponse_node_addr(uniffiStatus)
		})
		if checksum != 37017 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_nodestatusresponse_node_addr: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_nodestatusresponse_version(uniffiStatus)
		})
		if checksum != 50257 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_nodestatusresponse_version: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_publickey_equal(uniffiStatus)
		})
		if checksum != 10645 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_publickey_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_publickey_fmt_short(uniffiStatus)
		})
		if checksum != 33947 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_publickey_fmt_short: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_publickey_to_bytes(uniffiStatus)
		})
		if checksum != 54334 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_publickey_to_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_publickey_to_string(uniffiStatus)
		})
		if checksum != 48998 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_publickey_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_query_limit(uniffiStatus)
		})
		if checksum != 6405 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_query_limit: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_query_offset(uniffiStatus)
		})
		if checksum != 5309 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_query_offset: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_rangespec_is_all(uniffiStatus)
		})
		if checksum != 17079 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_rangespec_is_all: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_rangespec_is_empty(uniffiStatus)
		})
		if checksum != 55537 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_rangespec_is_empty: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_socketaddr_as_ipv4(uniffiStatus)
		})
		if checksum != 50860 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_socketaddr_as_ipv4: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_socketaddr_as_ipv6(uniffiStatus)
		})
		if checksum != 40970 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_socketaddr_as_ipv6: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_socketaddr_equal(uniffiStatus)
		})
		if checksum != 1891 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_socketaddr_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_socketaddr_type(uniffiStatus)
		})
		if checksum != 50972 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_socketaddr_type: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_socketaddrv4_equal(uniffiStatus)
		})
		if checksum != 51550 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_socketaddrv4_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_socketaddrv4_ip(uniffiStatus)
		})
		if checksum != 54004 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_socketaddrv4_ip: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_socketaddrv4_port(uniffiStatus)
		})
		if checksum != 34504 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_socketaddrv4_port: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_socketaddrv4_to_string(uniffiStatus)
		})
		if checksum != 43672 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_socketaddrv4_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_socketaddrv6_equal(uniffiStatus)
		})
		if checksum != 37651 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_socketaddrv6_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_socketaddrv6_ip(uniffiStatus)
		})
		if checksum != 49803 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_socketaddrv6_ip: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_socketaddrv6_port(uniffiStatus)
		})
		if checksum != 39562 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_socketaddrv6_port: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_socketaddrv6_to_string(uniffiStatus)
		})
		if checksum != 14154 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_socketaddrv6_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_tag_equal(uniffiStatus)
		})
		if checksum != 62383 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_tag_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_tag_to_bytes(uniffiStatus)
		})
		if checksum != 33917 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_tag_to_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_tag_to_string(uniffiStatus)
		})
		if checksum != 65488 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_tag_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_url_equal(uniffiStatus)
		})
		if checksum != 65501 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_url_equal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_url_to_string(uniffiStatus)
		})
		if checksum != 43798 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_url_to_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_authorid_from_string(uniffiStatus)
		})
		if checksum != 14210 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_authorid_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_blobdownloadrequest_new(uniffiStatus)
		})
		if checksum != 5113 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_blobdownloadrequest_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_docticket_from_string(uniffiStatus)
		})
		if checksum != 40262 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_docticket_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_downloadlocation_external(uniffiStatus)
		})
		if checksum != 45372 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_downloadlocation_external: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_downloadlocation_internal(uniffiStatus)
		})
		if checksum != 751 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_downloadlocation_internal: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_downloadpolicy_everything(uniffiStatus)
		})
		if checksum != 38497 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_downloadpolicy_everything: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_downloadpolicy_everything_except(uniffiStatus)
		})
		if checksum != 43304 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_downloadpolicy_everything_except: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_downloadpolicy_nothing(uniffiStatus)
		})
		if checksum != 1427 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_downloadpolicy_nothing: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_downloadpolicy_nothing_except(uniffiStatus)
		})
		if checksum != 28298 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_downloadpolicy_nothing_except: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_filterkind_exact(uniffiStatus)
		})
		if checksum != 52030 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_filterkind_exact: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_filterkind_prefix(uniffiStatus)
		})
		if checksum != 40434 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_filterkind_prefix: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_hash_from_bytes(uniffiStatus)
		})
		if checksum != 19134 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_hash_from_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_hash_from_string(uniffiStatus)
		})
		if checksum != 30790 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_hash_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_hash_new(uniffiStatus)
		})
		if checksum != 22809 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_hash_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_ipv4addr_from_string(uniffiStatus)
		})
		if checksum != 60777 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_ipv4addr_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_ipv4addr_new(uniffiStatus)
		})
		if checksum != 51336 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_ipv4addr_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_ipv6addr_from_string(uniffiStatus)
		})
		if checksum != 24533 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_ipv6addr_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_ipv6addr_new(uniffiStatus)
		})
		if checksum != 18364 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_ipv6addr_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_irohnode_new(uniffiStatus)
		})
		if checksum != 22562 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_irohnode_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_namespaceid_from_string(uniffiStatus)
		})
		if checksum != 47535 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_namespaceid_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_nodeaddr_new(uniffiStatus)
		})
		if checksum != 37391 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_nodeaddr_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_publickey_from_bytes(uniffiStatus)
		})
		if checksum != 65104 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_publickey_from_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_publickey_from_string(uniffiStatus)
		})
		if checksum != 18975 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_publickey_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_query_all(uniffiStatus)
		})
		if checksum != 18362 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_query_all: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_query_author(uniffiStatus)
		})
		if checksum != 6757 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_query_author: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_query_author_key_exact(uniffiStatus)
		})
		if checksum != 21618 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_query_author_key_exact: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_query_author_key_prefix(uniffiStatus)
		})
		if checksum != 63753 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_query_author_key_prefix: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_query_key_exact(uniffiStatus)
		})
		if checksum != 32100 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_query_key_exact: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_query_key_prefix(uniffiStatus)
		})
		if checksum != 44412 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_query_key_prefix: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_query_single_latest_per_key(uniffiStatus)
		})
		if checksum != 42778 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_query_single_latest_per_key: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_settagoption_auto(uniffiStatus)
		})
		if checksum != 13040 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_settagoption_auto: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_settagoption_named(uniffiStatus)
		})
		if checksum != 24631 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_settagoption_named: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_socketaddr_from_ipv4(uniffiStatus)
		})
		if checksum != 48670 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_socketaddr_from_ipv4: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_socketaddr_from_ipv6(uniffiStatus)
		})
		if checksum != 45955 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_socketaddr_from_ipv6: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_socketaddrv4_from_string(uniffiStatus)
		})
		if checksum != 16157 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_socketaddrv4_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_socketaddrv4_new(uniffiStatus)
		})
		if checksum != 12651 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_socketaddrv4_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_socketaddrv6_from_string(uniffiStatus)
		})
		if checksum != 22443 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_socketaddrv6_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_socketaddrv6_new(uniffiStatus)
		})
		if checksum != 46347 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_socketaddrv6_new: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_tag_from_bytes(uniffiStatus)
		})
		if checksum != 48807 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_tag_from_bytes: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_tag_from_string(uniffiStatus)
		})
		if checksum != 40751 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_tag_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_url_from_string(uniffiStatus)
		})
		if checksum != 50979 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_url_from_string: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_wrapoption_no_wrap(uniffiStatus)
		})
		if checksum != 60952 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_wrapoption_no_wrap: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_constructor_wrapoption_wrap(uniffiStatus)
		})
		if checksum != 59295 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_constructor_wrapoption_wrap: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_addcallback_progress(uniffiStatus)
		})
		if checksum != 42266 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_addcallback_progress: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docexportfilecallback_progress(uniffiStatus)
		})
		if checksum != 20951 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docexportfilecallback_progress: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_docimportfilecallback_progress(uniffiStatus)
		})
		if checksum != 18783 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_docimportfilecallback_progress: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_downloadcallback_progress(uniffiStatus)
		})
		if checksum != 64403 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_downloadcallback_progress: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_iroh_checksum_method_subscribecallback_event(uniffiStatus)
		})
		if checksum != 18725 {
			// If this happens try cleaning and rebuilding your project
			panic("iroh: uniffi_iroh_checksum_method_subscribecallback_event: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterUint8 struct{}

var FfiConverterUint8INSTANCE = FfiConverterUint8{}

func (FfiConverterUint8) Lower(value uint8) C.uint8_t {
	return C.uint8_t(value)
}

func (FfiConverterUint8) Write(writer io.Writer, value uint8) {
	writeUint8(writer, value)
}

func (FfiConverterUint8) Lift(value C.uint8_t) uint8 {
	return uint8(value)
}

func (FfiConverterUint8) Read(reader io.Reader) uint8 {
	return readUint8(reader)
}

type FfiDestroyerUint8 struct{}

func (FfiDestroyerUint8) Destroy(_ uint8) {}

type FfiConverterUint16 struct{}

var FfiConverterUint16INSTANCE = FfiConverterUint16{}

func (FfiConverterUint16) Lower(value uint16) C.uint16_t {
	return C.uint16_t(value)
}

func (FfiConverterUint16) Write(writer io.Writer, value uint16) {
	writeUint16(writer, value)
}

func (FfiConverterUint16) Lift(value C.uint16_t) uint16 {
	return uint16(value)
}

func (FfiConverterUint16) Read(reader io.Reader) uint16 {
	return readUint16(reader)
}

type FfiDestroyerUint16 struct{}

func (FfiDestroyerUint16) Destroy(_ uint16) {}

type FfiConverterUint64 struct{}

var FfiConverterUint64INSTANCE = FfiConverterUint64{}

func (FfiConverterUint64) Lower(value uint64) C.uint64_t {
	return C.uint64_t(value)
}

func (FfiConverterUint64) Write(writer io.Writer, value uint64) {
	writeUint64(writer, value)
}

func (FfiConverterUint64) Lift(value C.uint64_t) uint64 {
	return uint64(value)
}

func (FfiConverterUint64) Read(reader io.Reader) uint64 {
	return readUint64(reader)
}

type FfiDestroyerUint64 struct{}

func (FfiDestroyerUint64) Destroy(_ uint64) {}

type FfiConverterBool struct{}

var FfiConverterBoolINSTANCE = FfiConverterBool{}

func (FfiConverterBool) Lower(value bool) C.int8_t {
	if value {
		return C.int8_t(1)
	}
	return C.int8_t(0)
}

func (FfiConverterBool) Write(writer io.Writer, value bool) {
	if value {
		writeInt8(writer, 1)
	} else {
		writeInt8(writer, 0)
	}
}

func (FfiConverterBool) Lift(value C.int8_t) bool {
	return value != 0
}

func (FfiConverterBool) Read(reader io.Reader) bool {
	return readInt8(reader) != 0
}

type FfiDestroyerBool struct{}

func (FfiDestroyerBool) Destroy(_ bool) {}

type FfiConverterString struct{}

var FfiConverterStringINSTANCE = FfiConverterString{}

func (FfiConverterString) Lift(rb RustBufferI) string {
	defer rb.Free()
	reader := rb.AsReader()
	b, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("reading reader: %w", err))
	}
	return string(b)
}

func (FfiConverterString) Read(reader io.Reader) string {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading string, expected %d, read %d", length, read_length))
	}
	return string(buffer)
}

func (FfiConverterString) Lower(value string) RustBuffer {
	return stringToRustBuffer(value)
}

func (FfiConverterString) Write(writer io.Writer, value string) {
	if len(value) > math.MaxInt32 {
		panic("String is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := io.WriteString(writer, value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing string, expected %d, written %d", len(value), write_length))
	}
}

type FfiDestroyerString struct{}

func (FfiDestroyerString) Destroy(_ string) {}

type FfiConverterBytes struct{}

var FfiConverterBytesINSTANCE = FfiConverterBytes{}

func (c FfiConverterBytes) Lower(value []byte) RustBuffer {
	return LowerIntoRustBuffer[[]byte](c, value)
}

func (c FfiConverterBytes) Write(writer io.Writer, value []byte) {
	if len(value) > math.MaxInt32 {
		panic("[]byte is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := writer.Write(value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing []byte, expected %d, written %d", len(value), write_length))
	}
}

func (c FfiConverterBytes) Lift(rb RustBufferI) []byte {
	return LiftFromRustBuffer[[]byte](c, rb)
}

func (c FfiConverterBytes) Read(reader io.Reader) []byte {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading []byte, expected %d, read %d", length, read_length))
	}
	return buffer
}

type FfiDestroyerBytes struct{}

func (FfiDestroyerBytes) Destroy(_ []byte) {}

type FfiConverterTimestamp struct{}

var FfiConverterTimestampINSTANCE = FfiConverterTimestamp{}

func (c FfiConverterTimestamp) Lift(rb RustBufferI) time.Time {
	return LiftFromRustBuffer[time.Time](c, rb)
}

func (c FfiConverterTimestamp) Read(reader io.Reader) time.Time {
	sec := readInt64(reader)
	nsec := readUint32(reader)

	var sign int64 = 1
	if sec < 0 {
		sign = -1
	}

	return time.Unix(sec, int64(nsec)*sign)
}

func (c FfiConverterTimestamp) Lower(value time.Time) RustBuffer {
	return LowerIntoRustBuffer[time.Time](c, value)
}

func (c FfiConverterTimestamp) Write(writer io.Writer, value time.Time) {
	sec := value.Unix()
	nsec := uint32(value.Nanosecond())
	if value.Unix() < 0 {
		nsec = 1_000_000_000 - nsec
		sec += 1
	}

	writeInt64(writer, sec)
	writeUint32(writer, nsec)
}

type FfiDestroyerTimestamp struct{}

func (FfiDestroyerTimestamp) Destroy(_ time.Time) {}

// FfiConverterDuration converts between uniffi duration and Go duration.
type FfiConverterDuration struct{}

var FfiConverterDurationINSTANCE = FfiConverterDuration{}

func (c FfiConverterDuration) Lift(rb RustBufferI) time.Duration {
	return LiftFromRustBuffer[time.Duration](c, rb)
}

func (c FfiConverterDuration) Read(reader io.Reader) time.Duration {
	sec := readUint64(reader)
	nsec := readUint32(reader)
	return time.Duration(sec*1_000_000_000 + uint64(nsec))
}

func (c FfiConverterDuration) Lower(value time.Duration) RustBuffer {
	return LowerIntoRustBuffer[time.Duration](c, value)
}

func (c FfiConverterDuration) Write(writer io.Writer, value time.Duration) {
	if value.Nanoseconds() < 0 {
		// Rust does not support negative durations:
		// https://www.reddit.com/r/rust/comments/ljl55u/why_rusts_duration_not_supporting_negative_values/
		// This panic is very bad, because it depends on user input, and in Go user input related
		// error are supposed to be returned as errors, and not cause panics. However, with the
		// current architecture, its not possible to return an error from here, so panic is used as
		// the only other option to signal an error.
		panic("negative duration is not allowed")
	}

	writeUint64(writer, uint64(value)/1_000_000_000)
	writeUint32(writer, uint32(uint64(value)%1_000_000_000))
}

type FfiDestroyerDuration struct{}

func (FfiDestroyerDuration) Destroy(_ time.Duration) {}

// Below is an implementation of synchronization requirements outlined in the link.
// https://github.com/mozilla/uniffi-rs/blob/0dc031132d9493ca812c3af6e7dd60ad2ea95bf0/uniffi_bindgen/src/bindings/kotlin/templates/ObjectRuntime.kt#L31

type FfiObject struct {
	pointer      unsafe.Pointer
	callCounter  atomic.Int64
	freeFunction func(unsafe.Pointer, *C.RustCallStatus)
	destroyed    atomic.Bool
}

func newFfiObject(pointer unsafe.Pointer, freeFunction func(unsafe.Pointer, *C.RustCallStatus)) FfiObject {
	return FfiObject{
		pointer:      pointer,
		freeFunction: freeFunction,
	}
}

func (ffiObject *FfiObject) incrementPointer(debugName string) unsafe.Pointer {
	for {
		counter := ffiObject.callCounter.Load()
		if counter <= -1 {
			panic(fmt.Errorf("%v object has already been destroyed", debugName))
		}
		if counter == math.MaxInt64 {
			panic(fmt.Errorf("%v object call counter would overflow", debugName))
		}
		if ffiObject.callCounter.CompareAndSwap(counter, counter+1) {
			break
		}
	}

	return ffiObject.pointer
}

func (ffiObject *FfiObject) decrementPointer() {
	if ffiObject.callCounter.Add(-1) == -1 {
		ffiObject.freeRustArcPtr()
	}
}

func (ffiObject *FfiObject) destroy() {
	if ffiObject.destroyed.CompareAndSwap(false, true) {
		if ffiObject.callCounter.Add(-1) == -1 {
			ffiObject.freeRustArcPtr()
		}
	}
}

func (ffiObject *FfiObject) freeRustArcPtr() {
	rustCall(func(status *C.RustCallStatus) int32 {
		ffiObject.freeFunction(ffiObject.pointer, status)
		return 0
	})
}

type AddProgress struct {
	ffiObject FfiObject
}

func (_self *AddProgress) AsAbort() AddProgressAbort {
	_pointer := _self.ffiObject.incrementPointer("*AddProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeAddProgressAbortINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_addprogress_as_abort(
			_pointer, _uniffiStatus)
	}))
}

func (_self *AddProgress) AsAllDone() AddProgressAllDone {
	_pointer := _self.ffiObject.incrementPointer("*AddProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeAddProgressAllDoneINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_addprogress_as_all_done(
			_pointer, _uniffiStatus)
	}))
}

func (_self *AddProgress) AsDone() AddProgressDone {
	_pointer := _self.ffiObject.incrementPointer("*AddProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeAddProgressDoneINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_addprogress_as_done(
			_pointer, _uniffiStatus)
	}))
}

func (_self *AddProgress) AsFound() AddProgressFound {
	_pointer := _self.ffiObject.incrementPointer("*AddProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeAddProgressFoundINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_addprogress_as_found(
			_pointer, _uniffiStatus)
	}))
}

func (_self *AddProgress) AsProgress() AddProgressProgress {
	_pointer := _self.ffiObject.incrementPointer("*AddProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeAddProgressProgressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_addprogress_as_progress(
			_pointer, _uniffiStatus)
	}))
}

func (_self *AddProgress) Type() AddProgressType {
	_pointer := _self.ffiObject.incrementPointer("*AddProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeAddProgressTypeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_addprogress_type(
			_pointer, _uniffiStatus)
	}))
}

func (object *AddProgress) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAddProgress struct{}

var FfiConverterAddProgressINSTANCE = FfiConverterAddProgress{}

func (c FfiConverterAddProgress) Lift(pointer unsafe.Pointer) *AddProgress {
	result := &AddProgress{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_addprogress(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*AddProgress).Destroy)
	return result
}

func (c FfiConverterAddProgress) Read(reader io.Reader) *AddProgress {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAddProgress) Lower(value *AddProgress) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*AddProgress")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterAddProgress) Write(writer io.Writer, value *AddProgress) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAddProgress struct{}

func (_ FfiDestroyerAddProgress) Destroy(value *AddProgress) {
	value.Destroy()
}

type AuthorId struct {
	ffiObject FfiObject
}

func AuthorIdFromString(str string) (*AuthorId, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_authorid_from_string(FfiConverterStringINSTANCE.Lower(str), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *AuthorId
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAuthorIdINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *AuthorId) Equal(other *AuthorId) bool {
	_pointer := _self.ffiObject.incrementPointer("*AuthorId")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_authorid_equal(
			_pointer, FfiConverterAuthorIdINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *AuthorId) ToString() string {
	_pointer := _self.ffiObject.incrementPointer("*AuthorId")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_authorid_to_string(
			_pointer, _uniffiStatus)
	}))
}

func (object *AuthorId) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterAuthorId struct{}

var FfiConverterAuthorIdINSTANCE = FfiConverterAuthorId{}

func (c FfiConverterAuthorId) Lift(pointer unsafe.Pointer) *AuthorId {
	result := &AuthorId{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_authorid(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*AuthorId).Destroy)
	return result
}

func (c FfiConverterAuthorId) Read(reader io.Reader) *AuthorId {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterAuthorId) Lower(value *AuthorId) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*AuthorId")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterAuthorId) Write(writer io.Writer, value *AuthorId) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerAuthorId struct{}

func (_ FfiDestroyerAuthorId) Destroy(value *AuthorId) {
	value.Destroy()
}

type BlobDownloadRequest struct {
	ffiObject FfiObject
}

func NewBlobDownloadRequest(hash *Hash, format BlobFormat, node *NodeAddr, tag *SetTagOption, out *DownloadLocation) *BlobDownloadRequest {
	return FfiConverterBlobDownloadRequestINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_blobdownloadrequest_new(FfiConverterHashINSTANCE.Lower(hash), FfiConverterTypeBlobFormatINSTANCE.Lower(format), FfiConverterNodeAddrINSTANCE.Lower(node), FfiConverterSetTagOptionINSTANCE.Lower(tag), FfiConverterDownloadLocationINSTANCE.Lower(out), _uniffiStatus)
	}))
}

func (object *BlobDownloadRequest) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterBlobDownloadRequest struct{}

var FfiConverterBlobDownloadRequestINSTANCE = FfiConverterBlobDownloadRequest{}

func (c FfiConverterBlobDownloadRequest) Lift(pointer unsafe.Pointer) *BlobDownloadRequest {
	result := &BlobDownloadRequest{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_blobdownloadrequest(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*BlobDownloadRequest).Destroy)
	return result
}

func (c FfiConverterBlobDownloadRequest) Read(reader io.Reader) *BlobDownloadRequest {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterBlobDownloadRequest) Lower(value *BlobDownloadRequest) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*BlobDownloadRequest")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterBlobDownloadRequest) Write(writer io.Writer, value *BlobDownloadRequest) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerBlobDownloadRequest struct{}

func (_ FfiDestroyerBlobDownloadRequest) Destroy(value *BlobDownloadRequest) {
	value.Destroy()
}

type ConnectionType struct {
	ffiObject FfiObject
}

func (_self *ConnectionType) AsDirect() *SocketAddr {
	_pointer := _self.ffiObject.incrementPointer("*ConnectionType")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSocketAddrINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_connectiontype_as_direct(
			_pointer, _uniffiStatus)
	}))
}

func (_self *ConnectionType) AsMixed() ConnectionTypeMixed {
	_pointer := _self.ffiObject.incrementPointer("*ConnectionType")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeConnectionTypeMixedINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_connectiontype_as_mixed(
			_pointer, _uniffiStatus)
	}))
}

func (_self *ConnectionType) AsRelay() *Url {
	_pointer := _self.ffiObject.incrementPointer("*ConnectionType")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUrlINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_connectiontype_as_relay(
			_pointer, _uniffiStatus)
	}))
}

func (_self *ConnectionType) Type() ConnType {
	_pointer := _self.ffiObject.incrementPointer("*ConnectionType")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeConnTypeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_connectiontype_type(
			_pointer, _uniffiStatus)
	}))
}

func (object *ConnectionType) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterConnectionType struct{}

var FfiConverterConnectionTypeINSTANCE = FfiConverterConnectionType{}

func (c FfiConverterConnectionType) Lift(pointer unsafe.Pointer) *ConnectionType {
	result := &ConnectionType{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_connectiontype(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*ConnectionType).Destroy)
	return result
}

func (c FfiConverterConnectionType) Read(reader io.Reader) *ConnectionType {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterConnectionType) Lower(value *ConnectionType) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*ConnectionType")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterConnectionType) Write(writer io.Writer, value *ConnectionType) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerConnectionType struct{}

func (_ FfiDestroyerConnectionType) Destroy(value *ConnectionType) {
	value.Destroy()
}

type DirectAddrInfo struct {
	ffiObject FfiObject
}

func (_self *DirectAddrInfo) Addr() *SocketAddr {
	_pointer := _self.ffiObject.incrementPointer("*DirectAddrInfo")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSocketAddrINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_directaddrinfo_addr(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DirectAddrInfo) LastControl() *LatencyAndControlMsg {
	_pointer := _self.ffiObject.incrementPointer("*DirectAddrInfo")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalTypeLatencyAndControlMsgINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_directaddrinfo_last_control(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DirectAddrInfo) LastPayload() *time.Duration {
	_pointer := _self.ffiObject.incrementPointer("*DirectAddrInfo")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalDurationINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_directaddrinfo_last_payload(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DirectAddrInfo) Latency() *time.Duration {
	_pointer := _self.ffiObject.incrementPointer("*DirectAddrInfo")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalDurationINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_directaddrinfo_latency(
			_pointer, _uniffiStatus)
	}))
}

func (object *DirectAddrInfo) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterDirectAddrInfo struct{}

var FfiConverterDirectAddrInfoINSTANCE = FfiConverterDirectAddrInfo{}

func (c FfiConverterDirectAddrInfo) Lift(pointer unsafe.Pointer) *DirectAddrInfo {
	result := &DirectAddrInfo{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_directaddrinfo(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*DirectAddrInfo).Destroy)
	return result
}

func (c FfiConverterDirectAddrInfo) Read(reader io.Reader) *DirectAddrInfo {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterDirectAddrInfo) Lower(value *DirectAddrInfo) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*DirectAddrInfo")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterDirectAddrInfo) Write(writer io.Writer, value *DirectAddrInfo) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerDirectAddrInfo struct{}

func (_ FfiDestroyerDirectAddrInfo) Destroy(value *DirectAddrInfo) {
	value.Destroy()
}

type Doc struct {
	ffiObject FfiObject
}

func (_self *Doc) Close() error {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_doc_close(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (_self *Doc) Del(authorId *AuthorId, prefix []byte) (uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_iroh_fn_method_doc_del(
			_pointer, FfiConverterAuthorIdINSTANCE.Lower(authorId), FfiConverterBytesINSTANCE.Lower(prefix), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUint64INSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *Doc) ExportFile(entry *Entry, path string, cb *DocExportFileCallback) error {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_doc_export_file(
			_pointer, FfiConverterEntryINSTANCE.Lower(entry), FfiConverterStringINSTANCE.Lower(path), FfiConverterOptionalCallbackInterfaceDocExportFileCallbackINSTANCE.Lower(cb), _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (_self *Doc) GetDownloadPolicy() (*DownloadPolicy, error) {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_doc_get_download_policy(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *DownloadPolicy
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterDownloadPolicyINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *Doc) GetExact(author *AuthorId, key []byte, includeEmpty bool) (**Entry, error) {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_doc_get_exact(
			_pointer, FfiConverterAuthorIdINSTANCE.Lower(author), FfiConverterBytesINSTANCE.Lower(key), FfiConverterBoolINSTANCE.Lower(includeEmpty), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue **Entry
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalEntryINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *Doc) GetMany(query *Query) ([]*Entry, error) {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_doc_get_many(
			_pointer, FfiConverterQueryINSTANCE.Lower(query), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []*Entry
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceEntryINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *Doc) GetOne(query *Query) (**Entry, error) {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_doc_get_one(
			_pointer, FfiConverterQueryINSTANCE.Lower(query), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue **Entry
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalEntryINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *Doc) Id() *NamespaceId {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterNamespaceIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_doc_id(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Doc) ImportFile(author *AuthorId, key []byte, path string, inPlace bool, cb *DocImportFileCallback) error {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_doc_import_file(
			_pointer, FfiConverterAuthorIdINSTANCE.Lower(author), FfiConverterBytesINSTANCE.Lower(key), FfiConverterStringINSTANCE.Lower(path), FfiConverterBoolINSTANCE.Lower(inPlace), FfiConverterOptionalCallbackInterfaceDocImportFileCallbackINSTANCE.Lower(cb), _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (_self *Doc) Leave() error {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_doc_leave(
			_pointer, _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (_self *Doc) SetBytes(author *AuthorId, key []byte, value []byte) (*Hash, error) {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_doc_set_bytes(
			_pointer, FfiConverterAuthorIdINSTANCE.Lower(author), FfiConverterBytesINSTANCE.Lower(key), FfiConverterBytesINSTANCE.Lower(value), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Hash
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterHashINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *Doc) SetDownloadPolicy(policy *DownloadPolicy) error {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_doc_set_download_policy(
			_pointer, FfiConverterDownloadPolicyINSTANCE.Lower(policy), _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (_self *Doc) SetHash(author *AuthorId, key []byte, hash *Hash, size uint64) error {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_doc_set_hash(
			_pointer, FfiConverterAuthorIdINSTANCE.Lower(author), FfiConverterBytesINSTANCE.Lower(key), FfiConverterHashINSTANCE.Lower(hash), FfiConverterUint64INSTANCE.Lower(size), _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (_self *Doc) Share(mode ShareMode) (*DocTicket, error) {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_doc_share(
			_pointer, FfiConverterTypeShareModeINSTANCE.Lower(mode), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *DocTicket
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterDocTicketINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *Doc) StartSync(peers []*NodeAddr) error {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_doc_start_sync(
			_pointer, FfiConverterSequenceNodeAddrINSTANCE.Lower(peers), _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (_self *Doc) Status() (OpenState, error) {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_doc_status(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue OpenState
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTypeOpenStateINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *Doc) Subscribe(cb SubscribeCallback) error {
	_pointer := _self.ffiObject.incrementPointer("*Doc")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_doc_subscribe(
			_pointer, FfiConverterCallbackInterfaceSubscribeCallbackINSTANCE.Lower(cb), _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (object *Doc) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterDoc struct{}

var FfiConverterDocINSTANCE = FfiConverterDoc{}

func (c FfiConverterDoc) Lift(pointer unsafe.Pointer) *Doc {
	result := &Doc{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_doc(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*Doc).Destroy)
	return result
}

func (c FfiConverterDoc) Read(reader io.Reader) *Doc {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterDoc) Lower(value *Doc) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Doc")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterDoc) Write(writer io.Writer, value *Doc) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerDoc struct{}

func (_ FfiDestroyerDoc) Destroy(value *Doc) {
	value.Destroy()
}

type DocExportProgress struct {
	ffiObject FfiObject
}

func (_self *DocExportProgress) AsAbort() DocExportProgressAbort {
	_pointer := _self.ffiObject.incrementPointer("*DocExportProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDocExportProgressAbortINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_docexportprogress_as_abort(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DocExportProgress) AsFound() DocExportProgressFound {
	_pointer := _self.ffiObject.incrementPointer("*DocExportProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDocExportProgressFoundINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_docexportprogress_as_found(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DocExportProgress) AsProgress() DocExportProgressProgress {
	_pointer := _self.ffiObject.incrementPointer("*DocExportProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDocExportProgressProgressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_docexportprogress_as_progress(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DocExportProgress) Type() DocExportProgressType {
	_pointer := _self.ffiObject.incrementPointer("*DocExportProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDocExportProgressTypeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_docexportprogress_type(
			_pointer, _uniffiStatus)
	}))
}

func (object *DocExportProgress) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterDocExportProgress struct{}

var FfiConverterDocExportProgressINSTANCE = FfiConverterDocExportProgress{}

func (c FfiConverterDocExportProgress) Lift(pointer unsafe.Pointer) *DocExportProgress {
	result := &DocExportProgress{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_docexportprogress(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*DocExportProgress).Destroy)
	return result
}

func (c FfiConverterDocExportProgress) Read(reader io.Reader) *DocExportProgress {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterDocExportProgress) Lower(value *DocExportProgress) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*DocExportProgress")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterDocExportProgress) Write(writer io.Writer, value *DocExportProgress) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerDocExportProgress struct{}

func (_ FfiDestroyerDocExportProgress) Destroy(value *DocExportProgress) {
	value.Destroy()
}

type DocImportProgress struct {
	ffiObject FfiObject
}

func (_self *DocImportProgress) AsAbort() DocImportProgressAbort {
	_pointer := _self.ffiObject.incrementPointer("*DocImportProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDocImportProgressAbortINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_docimportprogress_as_abort(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DocImportProgress) AsAllDone() DocImportProgressAllDone {
	_pointer := _self.ffiObject.incrementPointer("*DocImportProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDocImportProgressAllDoneINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_docimportprogress_as_all_done(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DocImportProgress) AsFound() DocImportProgressFound {
	_pointer := _self.ffiObject.incrementPointer("*DocImportProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDocImportProgressFoundINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_docimportprogress_as_found(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DocImportProgress) AsIngestDone() DocImportProgressIngestDone {
	_pointer := _self.ffiObject.incrementPointer("*DocImportProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDocImportProgressIngestDoneINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_docimportprogress_as_ingest_done(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DocImportProgress) AsProgress() DocImportProgressProgress {
	_pointer := _self.ffiObject.incrementPointer("*DocImportProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDocImportProgressProgressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_docimportprogress_as_progress(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DocImportProgress) Type() DocImportProgressType {
	_pointer := _self.ffiObject.incrementPointer("*DocImportProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDocImportProgressTypeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_docimportprogress_type(
			_pointer, _uniffiStatus)
	}))
}

func (object *DocImportProgress) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterDocImportProgress struct{}

var FfiConverterDocImportProgressINSTANCE = FfiConverterDocImportProgress{}

func (c FfiConverterDocImportProgress) Lift(pointer unsafe.Pointer) *DocImportProgress {
	result := &DocImportProgress{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_docimportprogress(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*DocImportProgress).Destroy)
	return result
}

func (c FfiConverterDocImportProgress) Read(reader io.Reader) *DocImportProgress {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterDocImportProgress) Lower(value *DocImportProgress) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*DocImportProgress")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterDocImportProgress) Write(writer io.Writer, value *DocImportProgress) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerDocImportProgress struct{}

func (_ FfiDestroyerDocImportProgress) Destroy(value *DocImportProgress) {
	value.Destroy()
}

type DocTicket struct {
	ffiObject FfiObject
}

func DocTicketFromString(content string) (*DocTicket, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_docticket_from_string(FfiConverterStringINSTANCE.Lower(content), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *DocTicket
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterDocTicketINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *DocTicket) Equal(other *DocTicket) bool {
	_pointer := _self.ffiObject.incrementPointer("*DocTicket")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_docticket_equal(
			_pointer, FfiConverterDocTicketINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *DocTicket) ToString() string {
	_pointer := _self.ffiObject.incrementPointer("*DocTicket")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_docticket_to_string(
			_pointer, _uniffiStatus)
	}))
}

func (object *DocTicket) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterDocTicket struct{}

var FfiConverterDocTicketINSTANCE = FfiConverterDocTicket{}

func (c FfiConverterDocTicket) Lift(pointer unsafe.Pointer) *DocTicket {
	result := &DocTicket{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_docticket(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*DocTicket).Destroy)
	return result
}

func (c FfiConverterDocTicket) Read(reader io.Reader) *DocTicket {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterDocTicket) Lower(value *DocTicket) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*DocTicket")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterDocTicket) Write(writer io.Writer, value *DocTicket) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerDocTicket struct{}

func (_ FfiDestroyerDocTicket) Destroy(value *DocTicket) {
	value.Destroy()
}

type DownloadLocation struct {
	ffiObject FfiObject
}

func DownloadLocationExternal(path string, inPlace bool) *DownloadLocation {
	return FfiConverterDownloadLocationINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_downloadlocation_external(FfiConverterStringINSTANCE.Lower(path), FfiConverterBoolINSTANCE.Lower(inPlace), _uniffiStatus)
	}))
}
func DownloadLocationInternal() *DownloadLocation {
	return FfiConverterDownloadLocationINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_downloadlocation_internal(_uniffiStatus)
	}))
}

func (object *DownloadLocation) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterDownloadLocation struct{}

var FfiConverterDownloadLocationINSTANCE = FfiConverterDownloadLocation{}

func (c FfiConverterDownloadLocation) Lift(pointer unsafe.Pointer) *DownloadLocation {
	result := &DownloadLocation{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_downloadlocation(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*DownloadLocation).Destroy)
	return result
}

func (c FfiConverterDownloadLocation) Read(reader io.Reader) *DownloadLocation {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterDownloadLocation) Lower(value *DownloadLocation) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*DownloadLocation")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterDownloadLocation) Write(writer io.Writer, value *DownloadLocation) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerDownloadLocation struct{}

func (_ FfiDestroyerDownloadLocation) Destroy(value *DownloadLocation) {
	value.Destroy()
}

type DownloadPolicy struct {
	ffiObject FfiObject
}

func DownloadPolicyEverything() *DownloadPolicy {
	return FfiConverterDownloadPolicyINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_downloadpolicy_everything(_uniffiStatus)
	}))
}
func DownloadPolicyEverythingExcept(filters []*FilterKind) *DownloadPolicy {
	return FfiConverterDownloadPolicyINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_downloadpolicy_everything_except(FfiConverterSequenceFilterKindINSTANCE.Lower(filters), _uniffiStatus)
	}))
}
func DownloadPolicyNothing() *DownloadPolicy {
	return FfiConverterDownloadPolicyINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_downloadpolicy_nothing(_uniffiStatus)
	}))
}
func DownloadPolicyNothingExcept(filters []*FilterKind) *DownloadPolicy {
	return FfiConverterDownloadPolicyINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_downloadpolicy_nothing_except(FfiConverterSequenceFilterKindINSTANCE.Lower(filters), _uniffiStatus)
	}))
}

func (object *DownloadPolicy) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterDownloadPolicy struct{}

var FfiConverterDownloadPolicyINSTANCE = FfiConverterDownloadPolicy{}

func (c FfiConverterDownloadPolicy) Lift(pointer unsafe.Pointer) *DownloadPolicy {
	result := &DownloadPolicy{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_downloadpolicy(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*DownloadPolicy).Destroy)
	return result
}

func (c FfiConverterDownloadPolicy) Read(reader io.Reader) *DownloadPolicy {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterDownloadPolicy) Lower(value *DownloadPolicy) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*DownloadPolicy")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterDownloadPolicy) Write(writer io.Writer, value *DownloadPolicy) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerDownloadPolicy struct{}

func (_ FfiDestroyerDownloadPolicy) Destroy(value *DownloadPolicy) {
	value.Destroy()
}

type DownloadProgress struct {
	ffiObject FfiObject
}

func (_self *DownloadProgress) AsAbort() DownloadProgressAbort {
	_pointer := _self.ffiObject.incrementPointer("*DownloadProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDownloadProgressAbortINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_downloadprogress_as_abort(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DownloadProgress) AsDone() DownloadProgressDone {
	_pointer := _self.ffiObject.incrementPointer("*DownloadProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDownloadProgressDoneINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_downloadprogress_as_done(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DownloadProgress) AsExport() DownloadProgressExport {
	_pointer := _self.ffiObject.incrementPointer("*DownloadProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDownloadProgressExportINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_downloadprogress_as_export(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DownloadProgress) AsExportProgress() DownloadProgressExportProgress {
	_pointer := _self.ffiObject.incrementPointer("*DownloadProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDownloadProgressExportProgressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_downloadprogress_as_export_progress(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DownloadProgress) AsFound() DownloadProgressFound {
	_pointer := _self.ffiObject.incrementPointer("*DownloadProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDownloadProgressFoundINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_downloadprogress_as_found(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DownloadProgress) AsFoundHashSeq() DownloadProgressFoundHashSeq {
	_pointer := _self.ffiObject.incrementPointer("*DownloadProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDownloadProgressFoundHashSeqINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_downloadprogress_as_found_hash_seq(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DownloadProgress) AsNetworkDone() DownloadProgressNetworkDone {
	_pointer := _self.ffiObject.incrementPointer("*DownloadProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDownloadProgressNetworkDoneINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_downloadprogress_as_network_done(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DownloadProgress) AsProgress() DownloadProgressProgress {
	_pointer := _self.ffiObject.incrementPointer("*DownloadProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDownloadProgressProgressINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_downloadprogress_as_progress(
			_pointer, _uniffiStatus)
	}))
}

func (_self *DownloadProgress) Type() DownloadProgressType {
	_pointer := _self.ffiObject.incrementPointer("*DownloadProgress")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeDownloadProgressTypeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_downloadprogress_type(
			_pointer, _uniffiStatus)
	}))
}

func (object *DownloadProgress) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterDownloadProgress struct{}

var FfiConverterDownloadProgressINSTANCE = FfiConverterDownloadProgress{}

func (c FfiConverterDownloadProgress) Lift(pointer unsafe.Pointer) *DownloadProgress {
	result := &DownloadProgress{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_downloadprogress(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*DownloadProgress).Destroy)
	return result
}

func (c FfiConverterDownloadProgress) Read(reader io.Reader) *DownloadProgress {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterDownloadProgress) Lower(value *DownloadProgress) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*DownloadProgress")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterDownloadProgress) Write(writer io.Writer, value *DownloadProgress) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerDownloadProgress struct{}

func (_ FfiDestroyerDownloadProgress) Destroy(value *DownloadProgress) {
	value.Destroy()
}

type Entry struct {
	ffiObject FfiObject
}

func (_self *Entry) Author() *AuthorId {
	_pointer := _self.ffiObject.incrementPointer("*Entry")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterAuthorIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_entry_author(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Entry) ContentBytes(doc *Doc) ([]byte, error) {
	_pointer := _self.ffiObject.incrementPointer("*Entry")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_entry_content_bytes(
			_pointer, FfiConverterDocINSTANCE.Lower(doc), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *Entry) ContentHash() *Hash {
	_pointer := _self.ffiObject.incrementPointer("*Entry")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterHashINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_entry_content_hash(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Entry) ContentLen() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Entry")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_iroh_fn_method_entry_content_len(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Entry) Key() []byte {
	_pointer := _self.ffiObject.incrementPointer("*Entry")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_entry_key(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Entry) Namespace() *NamespaceId {
	_pointer := _self.ffiObject.incrementPointer("*Entry")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterNamespaceIdINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_entry_namespace(
			_pointer, _uniffiStatus)
	}))
}

func (object *Entry) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterEntry struct{}

var FfiConverterEntryINSTANCE = FfiConverterEntry{}

func (c FfiConverterEntry) Lift(pointer unsafe.Pointer) *Entry {
	result := &Entry{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_entry(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*Entry).Destroy)
	return result
}

func (c FfiConverterEntry) Read(reader io.Reader) *Entry {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterEntry) Lower(value *Entry) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Entry")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterEntry) Write(writer io.Writer, value *Entry) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerEntry struct{}

func (_ FfiDestroyerEntry) Destroy(value *Entry) {
	value.Destroy()
}

type FilterKind struct {
	ffiObject FfiObject
}

func FilterKindExact(key []byte) *FilterKind {
	return FfiConverterFilterKindINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_filterkind_exact(FfiConverterBytesINSTANCE.Lower(key), _uniffiStatus)
	}))
}
func FilterKindPrefix(prefix []byte) *FilterKind {
	return FfiConverterFilterKindINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_filterkind_prefix(FfiConverterBytesINSTANCE.Lower(prefix), _uniffiStatus)
	}))
}

func (_self *FilterKind) Matches(key []byte) bool {
	_pointer := _self.ffiObject.incrementPointer("*FilterKind")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_filterkind_matches(
			_pointer, FfiConverterBytesINSTANCE.Lower(key), _uniffiStatus)
	}))
}

func (object *FilterKind) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterFilterKind struct{}

var FfiConverterFilterKindINSTANCE = FfiConverterFilterKind{}

func (c FfiConverterFilterKind) Lift(pointer unsafe.Pointer) *FilterKind {
	result := &FilterKind{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_filterkind(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*FilterKind).Destroy)
	return result
}

func (c FfiConverterFilterKind) Read(reader io.Reader) *FilterKind {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterFilterKind) Lower(value *FilterKind) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*FilterKind")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterFilterKind) Write(writer io.Writer, value *FilterKind) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerFilterKind struct{}

func (_ FfiDestroyerFilterKind) Destroy(value *FilterKind) {
	value.Destroy()
}

type Hash struct {
	ffiObject FfiObject
}

func NewHash(buf []byte) *Hash {
	return FfiConverterHashINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_hash_new(FfiConverterBytesINSTANCE.Lower(buf), _uniffiStatus)
	}))
}

func HashFromBytes(bytes []byte) (*Hash, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_hash_from_bytes(FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Hash
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterHashINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}
func HashFromString(s string) (*Hash, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_hash_from_string(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Hash
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterHashINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *Hash) Equal(other *Hash) bool {
	_pointer := _self.ffiObject.incrementPointer("*Hash")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_hash_equal(
			_pointer, FfiConverterHashINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *Hash) ToBytes() []byte {
	_pointer := _self.ffiObject.incrementPointer("*Hash")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_hash_to_bytes(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Hash) ToHex() string {
	_pointer := _self.ffiObject.incrementPointer("*Hash")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_hash_to_hex(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Hash) ToString() string {
	_pointer := _self.ffiObject.incrementPointer("*Hash")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_hash_to_string(
			_pointer, _uniffiStatus)
	}))
}

func (object *Hash) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterHash struct{}

var FfiConverterHashINSTANCE = FfiConverterHash{}

func (c FfiConverterHash) Lift(pointer unsafe.Pointer) *Hash {
	result := &Hash{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_hash(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*Hash).Destroy)
	return result
}

func (c FfiConverterHash) Read(reader io.Reader) *Hash {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterHash) Lower(value *Hash) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Hash")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterHash) Write(writer io.Writer, value *Hash) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerHash struct{}

func (_ FfiDestroyerHash) Destroy(value *Hash) {
	value.Destroy()
}

type Ipv4Addr struct {
	ffiObject FfiObject
}

func NewIpv4Addr(a uint8, b uint8, c uint8, d uint8) *Ipv4Addr {
	return FfiConverterIpv4AddrINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_ipv4addr_new(FfiConverterUint8INSTANCE.Lower(a), FfiConverterUint8INSTANCE.Lower(b), FfiConverterUint8INSTANCE.Lower(c), FfiConverterUint8INSTANCE.Lower(d), _uniffiStatus)
	}))
}

func Ipv4AddrFromString(str string) (*Ipv4Addr, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_ipv4addr_from_string(FfiConverterStringINSTANCE.Lower(str), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Ipv4Addr
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterIpv4AddrINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *Ipv4Addr) Equal(other *Ipv4Addr) bool {
	_pointer := _self.ffiObject.incrementPointer("*Ipv4Addr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_ipv4addr_equal(
			_pointer, FfiConverterIpv4AddrINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *Ipv4Addr) Octets() []uint8 {
	_pointer := _self.ffiObject.incrementPointer("*Ipv4Addr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint8INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_ipv4addr_octets(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Ipv4Addr) ToString() string {
	_pointer := _self.ffiObject.incrementPointer("*Ipv4Addr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_ipv4addr_to_string(
			_pointer, _uniffiStatus)
	}))
}

func (object *Ipv4Addr) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterIpv4Addr struct{}

var FfiConverterIpv4AddrINSTANCE = FfiConverterIpv4Addr{}

func (c FfiConverterIpv4Addr) Lift(pointer unsafe.Pointer) *Ipv4Addr {
	result := &Ipv4Addr{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_ipv4addr(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*Ipv4Addr).Destroy)
	return result
}

func (c FfiConverterIpv4Addr) Read(reader io.Reader) *Ipv4Addr {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterIpv4Addr) Lower(value *Ipv4Addr) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Ipv4Addr")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterIpv4Addr) Write(writer io.Writer, value *Ipv4Addr) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerIpv4Addr struct{}

func (_ FfiDestroyerIpv4Addr) Destroy(value *Ipv4Addr) {
	value.Destroy()
}

type Ipv6Addr struct {
	ffiObject FfiObject
}

func NewIpv6Addr(a uint16, b uint16, c uint16, d uint16, e uint16, f uint16, g uint16, h uint16) *Ipv6Addr {
	return FfiConverterIpv6AddrINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_ipv6addr_new(FfiConverterUint16INSTANCE.Lower(a), FfiConverterUint16INSTANCE.Lower(b), FfiConverterUint16INSTANCE.Lower(c), FfiConverterUint16INSTANCE.Lower(d), FfiConverterUint16INSTANCE.Lower(e), FfiConverterUint16INSTANCE.Lower(f), FfiConverterUint16INSTANCE.Lower(g), FfiConverterUint16INSTANCE.Lower(h), _uniffiStatus)
	}))
}

func Ipv6AddrFromString(str string) (*Ipv6Addr, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_ipv6addr_from_string(FfiConverterStringINSTANCE.Lower(str), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Ipv6Addr
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterIpv6AddrINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *Ipv6Addr) Equal(other *Ipv6Addr) bool {
	_pointer := _self.ffiObject.incrementPointer("*Ipv6Addr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_ipv6addr_equal(
			_pointer, FfiConverterIpv6AddrINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *Ipv6Addr) Segments() []uint16 {
	_pointer := _self.ffiObject.incrementPointer("*Ipv6Addr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceUint16INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_ipv6addr_segments(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Ipv6Addr) ToString() string {
	_pointer := _self.ffiObject.incrementPointer("*Ipv6Addr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_ipv6addr_to_string(
			_pointer, _uniffiStatus)
	}))
}

func (object *Ipv6Addr) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterIpv6Addr struct{}

var FfiConverterIpv6AddrINSTANCE = FfiConverterIpv6Addr{}

func (c FfiConverterIpv6Addr) Lift(pointer unsafe.Pointer) *Ipv6Addr {
	result := &Ipv6Addr{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_ipv6addr(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*Ipv6Addr).Destroy)
	return result
}

func (c FfiConverterIpv6Addr) Read(reader io.Reader) *Ipv6Addr {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterIpv6Addr) Lower(value *Ipv6Addr) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Ipv6Addr")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterIpv6Addr) Write(writer io.Writer, value *Ipv6Addr) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerIpv6Addr struct{}

func (_ FfiDestroyerIpv6Addr) Destroy(value *Ipv6Addr) {
	value.Destroy()
}

type IrohNode struct {
	ffiObject FfiObject
}

func NewIrohNode(path string) (*IrohNode, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_irohnode_new(FfiConverterStringINSTANCE.Lower(path), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *IrohNode
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterIrohNodeINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) AuthorCreate() (*AuthorId, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_irohnode_author_create(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *AuthorId
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterAuthorIdINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) AuthorList() ([]*AuthorId, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_irohnode_author_list(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []*AuthorId
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceAuthorIdINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) BlobsAddBytes(bytes []byte, tag *SetTagOption) (BlobAddOutcome, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_irohnode_blobs_add_bytes(
			_pointer, FfiConverterBytesINSTANCE.Lower(bytes), FfiConverterSetTagOptionINSTANCE.Lower(tag), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue BlobAddOutcome
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTypeBlobAddOutcomeINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) BlobsAddFromPath(path string, inPlace bool, tag *SetTagOption, wrap *WrapOption, cb AddCallback) error {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_irohnode_blobs_add_from_path(
			_pointer, FfiConverterStringINSTANCE.Lower(path), FfiConverterBoolINSTANCE.Lower(inPlace), FfiConverterSetTagOptionINSTANCE.Lower(tag), FfiConverterWrapOptionINSTANCE.Lower(wrap), FfiConverterCallbackInterfaceAddCallbackINSTANCE.Lower(cb), _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (_self *IrohNode) BlobsDeleteBlob(hash *Hash) error {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_irohnode_blobs_delete_blob(
			_pointer, FfiConverterHashINSTANCE.Lower(hash), _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (_self *IrohNode) BlobsDownload(req *BlobDownloadRequest, cb DownloadCallback) error {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_irohnode_blobs_download(
			_pointer, FfiConverterBlobDownloadRequestINSTANCE.Lower(req), FfiConverterCallbackInterfaceDownloadCallbackINSTANCE.Lower(cb), _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (_self *IrohNode) BlobsList() ([]*Hash, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_irohnode_blobs_list(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []*Hash
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceHashINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) BlobsListCollections() ([]BlobListCollectionsResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_irohnode_blobs_list_collections(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []BlobListCollectionsResponse
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceTypeBlobListCollectionsResponseINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) BlobsListIncomplete() ([]BlobListIncompleteResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_irohnode_blobs_list_incomplete(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []BlobListIncompleteResponse
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceTypeBlobListIncompleteResponseINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) BlobsReadToBytes(hash *Hash) ([]byte, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_irohnode_blobs_read_to_bytes(
			_pointer, FfiConverterHashINSTANCE.Lower(hash), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) BlobsSize(hash *Hash) (uint64, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_iroh_fn_method_irohnode_blobs_size(
			_pointer, FfiConverterHashINSTANCE.Lower(hash), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue uint64
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUint64INSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) BlobsWriteToPath(hash *Hash, path string) error {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_irohnode_blobs_write_to_path(
			_pointer, FfiConverterHashINSTANCE.Lower(hash), FfiConverterStringINSTANCE.Lower(path), _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (_self *IrohNode) ConnectionInfo(nodeId *PublicKey) (*ConnectionInfo, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_irohnode_connection_info(
			_pointer, FfiConverterPublicKeyINSTANCE.Lower(nodeId), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *ConnectionInfo
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalTypeConnectionInfoINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) Connections() ([]ConnectionInfo, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_irohnode_connections(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []ConnectionInfo
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceTypeConnectionInfoINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) DocCreate() (*Doc, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_irohnode_doc_create(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Doc
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterDocINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) DocDrop(docId *NamespaceId) error {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_irohnode_doc_drop(
			_pointer, FfiConverterNamespaceIdINSTANCE.Lower(docId), _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (_self *IrohNode) DocJoin(ticket *DocTicket) (*Doc, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_irohnode_doc_join(
			_pointer, FfiConverterDocTicketINSTANCE.Lower(ticket), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Doc
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterDocINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) DocList() ([]NamespaceAndCapability, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_irohnode_doc_list(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []NamespaceAndCapability
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceTypeNamespaceAndCapabilityINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) DocOpen(id *NamespaceId) (**Doc, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_irohnode_doc_open(
			_pointer, FfiConverterNamespaceIdINSTANCE.Lower(id), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue **Doc
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterOptionalDocINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) NodeId() string {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_irohnode_node_id(
			_pointer, _uniffiStatus)
	}))
}

func (_self *IrohNode) Stats() (map[string]CounterStats, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_irohnode_stats(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue map[string]CounterStats
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterMapStringTypeCounterStatsINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) Status() (*NodeStatusResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_irohnode_status(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *NodeStatusResponse
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterNodeStatusResponseINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *IrohNode) TagsDelete(name *Tag) error {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_method_irohnode_tags_delete(
			_pointer, FfiConverterTagINSTANCE.Lower(name), _uniffiStatus)
		return false
	})
	return _uniffiErr
}

func (_self *IrohNode) TagsList() ([]ListTagsResponse, error) {
	_pointer := _self.ffiObject.incrementPointer("*IrohNode")
	defer _self.ffiObject.decrementPointer()
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_irohnode_tags_list(
			_pointer, _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []ListTagsResponse
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSequenceTypeListTagsResponseINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (object *IrohNode) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterIrohNode struct{}

var FfiConverterIrohNodeINSTANCE = FfiConverterIrohNode{}

func (c FfiConverterIrohNode) Lift(pointer unsafe.Pointer) *IrohNode {
	result := &IrohNode{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_irohnode(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*IrohNode).Destroy)
	return result
}

func (c FfiConverterIrohNode) Read(reader io.Reader) *IrohNode {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterIrohNode) Lower(value *IrohNode) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*IrohNode")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterIrohNode) Write(writer io.Writer, value *IrohNode) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerIrohNode struct{}

func (_ FfiDestroyerIrohNode) Destroy(value *IrohNode) {
	value.Destroy()
}

type LiveEvent struct {
	ffiObject FfiObject
}

func (_self *LiveEvent) AsContentReady() *Hash {
	_pointer := _self.ffiObject.incrementPointer("*LiveEvent")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterHashINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_liveevent_as_content_ready(
			_pointer, _uniffiStatus)
	}))
}

func (_self *LiveEvent) AsInsertLocal() *Entry {
	_pointer := _self.ffiObject.incrementPointer("*LiveEvent")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterEntryINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_liveevent_as_insert_local(
			_pointer, _uniffiStatus)
	}))
}

func (_self *LiveEvent) AsInsertRemote() InsertRemoteEvent {
	_pointer := _self.ffiObject.incrementPointer("*LiveEvent")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeInsertRemoteEventINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_liveevent_as_insert_remote(
			_pointer, _uniffiStatus)
	}))
}

func (_self *LiveEvent) AsNeighborDown() *PublicKey {
	_pointer := _self.ffiObject.incrementPointer("*LiveEvent")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterPublicKeyINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_liveevent_as_neighbor_down(
			_pointer, _uniffiStatus)
	}))
}

func (_self *LiveEvent) AsNeighborUp() *PublicKey {
	_pointer := _self.ffiObject.incrementPointer("*LiveEvent")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterPublicKeyINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_liveevent_as_neighbor_up(
			_pointer, _uniffiStatus)
	}))
}

func (_self *LiveEvent) AsSyncFinished() SyncEvent {
	_pointer := _self.ffiObject.incrementPointer("*LiveEvent")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeSyncEventINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_liveevent_as_sync_finished(
			_pointer, _uniffiStatus)
	}))
}

func (_self *LiveEvent) Type() LiveEventType {
	_pointer := _self.ffiObject.incrementPointer("*LiveEvent")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeLiveEventTypeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_liveevent_type(
			_pointer, _uniffiStatus)
	}))
}

func (object *LiveEvent) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterLiveEvent struct{}

var FfiConverterLiveEventINSTANCE = FfiConverterLiveEvent{}

func (c FfiConverterLiveEvent) Lift(pointer unsafe.Pointer) *LiveEvent {
	result := &LiveEvent{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_liveevent(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*LiveEvent).Destroy)
	return result
}

func (c FfiConverterLiveEvent) Read(reader io.Reader) *LiveEvent {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterLiveEvent) Lower(value *LiveEvent) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*LiveEvent")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterLiveEvent) Write(writer io.Writer, value *LiveEvent) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerLiveEvent struct{}

func (_ FfiDestroyerLiveEvent) Destroy(value *LiveEvent) {
	value.Destroy()
}

type NamespaceId struct {
	ffiObject FfiObject
}

func NamespaceIdFromString(str string) (*NamespaceId, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_namespaceid_from_string(FfiConverterStringINSTANCE.Lower(str), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *NamespaceId
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterNamespaceIdINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *NamespaceId) Equal(other *NamespaceId) bool {
	_pointer := _self.ffiObject.incrementPointer("*NamespaceId")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_namespaceid_equal(
			_pointer, FfiConverterNamespaceIdINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *NamespaceId) ToString() string {
	_pointer := _self.ffiObject.incrementPointer("*NamespaceId")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_namespaceid_to_string(
			_pointer, _uniffiStatus)
	}))
}

func (object *NamespaceId) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterNamespaceId struct{}

var FfiConverterNamespaceIdINSTANCE = FfiConverterNamespaceId{}

func (c FfiConverterNamespaceId) Lift(pointer unsafe.Pointer) *NamespaceId {
	result := &NamespaceId{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_namespaceid(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*NamespaceId).Destroy)
	return result
}

func (c FfiConverterNamespaceId) Read(reader io.Reader) *NamespaceId {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterNamespaceId) Lower(value *NamespaceId) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*NamespaceId")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterNamespaceId) Write(writer io.Writer, value *NamespaceId) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerNamespaceId struct{}

func (_ FfiDestroyerNamespaceId) Destroy(value *NamespaceId) {
	value.Destroy()
}

type NodeAddr struct {
	ffiObject FfiObject
}

func NewNodeAddr(nodeId *PublicKey, derpUrl **Url, addresses []*SocketAddr) *NodeAddr {
	return FfiConverterNodeAddrINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_nodeaddr_new(FfiConverterPublicKeyINSTANCE.Lower(nodeId), FfiConverterOptionalUrlINSTANCE.Lower(derpUrl), FfiConverterSequenceSocketAddrINSTANCE.Lower(addresses), _uniffiStatus)
	}))
}

func (_self *NodeAddr) DerpUrl() **Url {
	_pointer := _self.ffiObject.incrementPointer("*NodeAddr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUrlINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_nodeaddr_derp_url(
			_pointer, _uniffiStatus)
	}))
}

func (_self *NodeAddr) DirectAddresses() []*SocketAddr {
	_pointer := _self.ffiObject.incrementPointer("*NodeAddr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceSocketAddrINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_nodeaddr_direct_addresses(
			_pointer, _uniffiStatus)
	}))
}

func (_self *NodeAddr) Equal(other *NodeAddr) bool {
	_pointer := _self.ffiObject.incrementPointer("*NodeAddr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_nodeaddr_equal(
			_pointer, FfiConverterNodeAddrINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (object *NodeAddr) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterNodeAddr struct{}

var FfiConverterNodeAddrINSTANCE = FfiConverterNodeAddr{}

func (c FfiConverterNodeAddr) Lift(pointer unsafe.Pointer) *NodeAddr {
	result := &NodeAddr{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_nodeaddr(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*NodeAddr).Destroy)
	return result
}

func (c FfiConverterNodeAddr) Read(reader io.Reader) *NodeAddr {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterNodeAddr) Lower(value *NodeAddr) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*NodeAddr")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterNodeAddr) Write(writer io.Writer, value *NodeAddr) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerNodeAddr struct{}

func (_ FfiDestroyerNodeAddr) Destroy(value *NodeAddr) {
	value.Destroy()
}

type NodeStatusResponse struct {
	ffiObject FfiObject
}

func (_self *NodeStatusResponse) ListenAddrs() []*SocketAddr {
	_pointer := _self.ffiObject.incrementPointer("*NodeStatusResponse")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSequenceSocketAddrINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_nodestatusresponse_listen_addrs(
			_pointer, _uniffiStatus)
	}))
}

func (_self *NodeStatusResponse) NodeAddr() *NodeAddr {
	_pointer := _self.ffiObject.incrementPointer("*NodeStatusResponse")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterNodeAddrINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_nodestatusresponse_node_addr(
			_pointer, _uniffiStatus)
	}))
}

func (_self *NodeStatusResponse) Version() string {
	_pointer := _self.ffiObject.incrementPointer("*NodeStatusResponse")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_nodestatusresponse_version(
			_pointer, _uniffiStatus)
	}))
}

func (object *NodeStatusResponse) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterNodeStatusResponse struct{}

var FfiConverterNodeStatusResponseINSTANCE = FfiConverterNodeStatusResponse{}

func (c FfiConverterNodeStatusResponse) Lift(pointer unsafe.Pointer) *NodeStatusResponse {
	result := &NodeStatusResponse{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_nodestatusresponse(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*NodeStatusResponse).Destroy)
	return result
}

func (c FfiConverterNodeStatusResponse) Read(reader io.Reader) *NodeStatusResponse {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterNodeStatusResponse) Lower(value *NodeStatusResponse) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*NodeStatusResponse")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterNodeStatusResponse) Write(writer io.Writer, value *NodeStatusResponse) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerNodeStatusResponse struct{}

func (_ FfiDestroyerNodeStatusResponse) Destroy(value *NodeStatusResponse) {
	value.Destroy()
}

type PublicKey struct {
	ffiObject FfiObject
}

func PublicKeyFromBytes(bytes []byte) (*PublicKey, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_publickey_from_bytes(FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *PublicKey
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPublicKeyINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}
func PublicKeyFromString(s string) (*PublicKey, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_publickey_from_string(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *PublicKey
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterPublicKeyINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *PublicKey) Equal(other *PublicKey) bool {
	_pointer := _self.ffiObject.incrementPointer("*PublicKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_publickey_equal(
			_pointer, FfiConverterPublicKeyINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *PublicKey) FmtShort() string {
	_pointer := _self.ffiObject.incrementPointer("*PublicKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_publickey_fmt_short(
			_pointer, _uniffiStatus)
	}))
}

func (_self *PublicKey) ToBytes() []byte {
	_pointer := _self.ffiObject.incrementPointer("*PublicKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_publickey_to_bytes(
			_pointer, _uniffiStatus)
	}))
}

func (_self *PublicKey) ToString() string {
	_pointer := _self.ffiObject.incrementPointer("*PublicKey")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_publickey_to_string(
			_pointer, _uniffiStatus)
	}))
}

func (object *PublicKey) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterPublicKey struct{}

var FfiConverterPublicKeyINSTANCE = FfiConverterPublicKey{}

func (c FfiConverterPublicKey) Lift(pointer unsafe.Pointer) *PublicKey {
	result := &PublicKey{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_publickey(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*PublicKey).Destroy)
	return result
}

func (c FfiConverterPublicKey) Read(reader io.Reader) *PublicKey {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterPublicKey) Lower(value *PublicKey) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*PublicKey")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterPublicKey) Write(writer io.Writer, value *PublicKey) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerPublicKey struct{}

func (_ FfiDestroyerPublicKey) Destroy(value *PublicKey) {
	value.Destroy()
}

type Query struct {
	ffiObject FfiObject
}

func QueryAll(opts *QueryOptions) *Query {
	return FfiConverterQueryINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_query_all(FfiConverterOptionalTypeQueryOptionsINSTANCE.Lower(opts), _uniffiStatus)
	}))
}
func QueryAuthor(author *AuthorId, opts *QueryOptions) *Query {
	return FfiConverterQueryINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_query_author(FfiConverterAuthorIdINSTANCE.Lower(author), FfiConverterOptionalTypeQueryOptionsINSTANCE.Lower(opts), _uniffiStatus)
	}))
}
func QueryAuthorKeyExact(author *AuthorId, key []byte) *Query {
	return FfiConverterQueryINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_query_author_key_exact(FfiConverterAuthorIdINSTANCE.Lower(author), FfiConverterBytesINSTANCE.Lower(key), _uniffiStatus)
	}))
}
func QueryAuthorKeyPrefix(author *AuthorId, prefix []byte, opts *QueryOptions) *Query {
	return FfiConverterQueryINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_query_author_key_prefix(FfiConverterAuthorIdINSTANCE.Lower(author), FfiConverterBytesINSTANCE.Lower(prefix), FfiConverterOptionalTypeQueryOptionsINSTANCE.Lower(opts), _uniffiStatus)
	}))
}
func QueryKeyExact(key []byte, opts *QueryOptions) *Query {
	return FfiConverterQueryINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_query_key_exact(FfiConverterBytesINSTANCE.Lower(key), FfiConverterOptionalTypeQueryOptionsINSTANCE.Lower(opts), _uniffiStatus)
	}))
}
func QueryKeyPrefix(prefix []byte, opts *QueryOptions) *Query {
	return FfiConverterQueryINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_query_key_prefix(FfiConverterBytesINSTANCE.Lower(prefix), FfiConverterOptionalTypeQueryOptionsINSTANCE.Lower(opts), _uniffiStatus)
	}))
}
func QuerySingleLatestPerKey(opts *QueryOptions) *Query {
	return FfiConverterQueryINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_query_single_latest_per_key(FfiConverterOptionalTypeQueryOptionsINSTANCE.Lower(opts), _uniffiStatus)
	}))
}

func (_self *Query) Limit() *uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Query")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterOptionalUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_query_limit(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Query) Offset() uint64 {
	_pointer := _self.ffiObject.incrementPointer("*Query")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint64INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint64_t {
		return C.uniffi_iroh_fn_method_query_offset(
			_pointer, _uniffiStatus)
	}))
}

func (object *Query) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterQuery struct{}

var FfiConverterQueryINSTANCE = FfiConverterQuery{}

func (c FfiConverterQuery) Lift(pointer unsafe.Pointer) *Query {
	result := &Query{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_query(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*Query).Destroy)
	return result
}

func (c FfiConverterQuery) Read(reader io.Reader) *Query {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterQuery) Lower(value *Query) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Query")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterQuery) Write(writer io.Writer, value *Query) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerQuery struct{}

func (_ FfiDestroyerQuery) Destroy(value *Query) {
	value.Destroy()
}

type RangeSpec struct {
	ffiObject FfiObject
}

func (_self *RangeSpec) IsAll() bool {
	_pointer := _self.ffiObject.incrementPointer("*RangeSpec")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_rangespec_is_all(
			_pointer, _uniffiStatus)
	}))
}

func (_self *RangeSpec) IsEmpty() bool {
	_pointer := _self.ffiObject.incrementPointer("*RangeSpec")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_rangespec_is_empty(
			_pointer, _uniffiStatus)
	}))
}

func (object *RangeSpec) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterRangeSpec struct{}

var FfiConverterRangeSpecINSTANCE = FfiConverterRangeSpec{}

func (c FfiConverterRangeSpec) Lift(pointer unsafe.Pointer) *RangeSpec {
	result := &RangeSpec{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_rangespec(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*RangeSpec).Destroy)
	return result
}

func (c FfiConverterRangeSpec) Read(reader io.Reader) *RangeSpec {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterRangeSpec) Lower(value *RangeSpec) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*RangeSpec")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterRangeSpec) Write(writer io.Writer, value *RangeSpec) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerRangeSpec struct{}

func (_ FfiDestroyerRangeSpec) Destroy(value *RangeSpec) {
	value.Destroy()
}

type SetTagOption struct {
	ffiObject FfiObject
}

func SetTagOptionAuto() *SetTagOption {
	return FfiConverterSetTagOptionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_settagoption_auto(_uniffiStatus)
	}))
}
func SetTagOptionNamed(tag *Tag) *SetTagOption {
	return FfiConverterSetTagOptionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_settagoption_named(FfiConverterTagINSTANCE.Lower(tag), _uniffiStatus)
	}))
}

func (object *SetTagOption) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSetTagOption struct{}

var FfiConverterSetTagOptionINSTANCE = FfiConverterSetTagOption{}

func (c FfiConverterSetTagOption) Lift(pointer unsafe.Pointer) *SetTagOption {
	result := &SetTagOption{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_settagoption(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*SetTagOption).Destroy)
	return result
}

func (c FfiConverterSetTagOption) Read(reader io.Reader) *SetTagOption {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSetTagOption) Lower(value *SetTagOption) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*SetTagOption")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterSetTagOption) Write(writer io.Writer, value *SetTagOption) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSetTagOption struct{}

func (_ FfiDestroyerSetTagOption) Destroy(value *SetTagOption) {
	value.Destroy()
}

type SocketAddr struct {
	ffiObject FfiObject
}

func SocketAddrFromIpv4(ipv4 *Ipv4Addr, port uint16) *SocketAddr {
	return FfiConverterSocketAddrINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_socketaddr_from_ipv4(FfiConverterIpv4AddrINSTANCE.Lower(ipv4), FfiConverterUint16INSTANCE.Lower(port), _uniffiStatus)
	}))
}
func SocketAddrFromIpv6(ipv6 *Ipv6Addr, port uint16) *SocketAddr {
	return FfiConverterSocketAddrINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_socketaddr_from_ipv6(FfiConverterIpv6AddrINSTANCE.Lower(ipv6), FfiConverterUint16INSTANCE.Lower(port), _uniffiStatus)
	}))
}

func (_self *SocketAddr) AsIpv4() *SocketAddrV4 {
	_pointer := _self.ffiObject.incrementPointer("*SocketAddr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSocketAddrV4INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_socketaddr_as_ipv4(
			_pointer, _uniffiStatus)
	}))
}

func (_self *SocketAddr) AsIpv6() *SocketAddrV6 {
	_pointer := _self.ffiObject.incrementPointer("*SocketAddr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterSocketAddrV6INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_socketaddr_as_ipv6(
			_pointer, _uniffiStatus)
	}))
}

func (_self *SocketAddr) Equal(other *SocketAddr) bool {
	_pointer := _self.ffiObject.incrementPointer("*SocketAddr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_socketaddr_equal(
			_pointer, FfiConverterSocketAddrINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *SocketAddr) Type() SocketAddrType {
	_pointer := _self.ffiObject.incrementPointer("*SocketAddr")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterTypeSocketAddrTypeINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_socketaddr_type(
			_pointer, _uniffiStatus)
	}))
}

func (object *SocketAddr) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSocketAddr struct{}

var FfiConverterSocketAddrINSTANCE = FfiConverterSocketAddr{}

func (c FfiConverterSocketAddr) Lift(pointer unsafe.Pointer) *SocketAddr {
	result := &SocketAddr{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_socketaddr(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*SocketAddr).Destroy)
	return result
}

func (c FfiConverterSocketAddr) Read(reader io.Reader) *SocketAddr {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSocketAddr) Lower(value *SocketAddr) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*SocketAddr")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterSocketAddr) Write(writer io.Writer, value *SocketAddr) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSocketAddr struct{}

func (_ FfiDestroyerSocketAddr) Destroy(value *SocketAddr) {
	value.Destroy()
}

type SocketAddrV4 struct {
	ffiObject FfiObject
}

func NewSocketAddrV4(ipv4 *Ipv4Addr, port uint16) *SocketAddrV4 {
	return FfiConverterSocketAddrV4INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_socketaddrv4_new(FfiConverterIpv4AddrINSTANCE.Lower(ipv4), FfiConverterUint16INSTANCE.Lower(port), _uniffiStatus)
	}))
}

func SocketAddrV4FromString(str string) (*SocketAddrV4, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_socketaddrv4_from_string(FfiConverterStringINSTANCE.Lower(str), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *SocketAddrV4
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSocketAddrV4INSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *SocketAddrV4) Equal(other *SocketAddrV4) bool {
	_pointer := _self.ffiObject.incrementPointer("*SocketAddrV4")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_socketaddrv4_equal(
			_pointer, FfiConverterSocketAddrV4INSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *SocketAddrV4) Ip() *Ipv4Addr {
	_pointer := _self.ffiObject.incrementPointer("*SocketAddrV4")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterIpv4AddrINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_socketaddrv4_ip(
			_pointer, _uniffiStatus)
	}))
}

func (_self *SocketAddrV4) Port() uint16 {
	_pointer := _self.ffiObject.incrementPointer("*SocketAddrV4")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint16INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_iroh_fn_method_socketaddrv4_port(
			_pointer, _uniffiStatus)
	}))
}

func (_self *SocketAddrV4) ToString() string {
	_pointer := _self.ffiObject.incrementPointer("*SocketAddrV4")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_socketaddrv4_to_string(
			_pointer, _uniffiStatus)
	}))
}

func (object *SocketAddrV4) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSocketAddrV4 struct{}

var FfiConverterSocketAddrV4INSTANCE = FfiConverterSocketAddrV4{}

func (c FfiConverterSocketAddrV4) Lift(pointer unsafe.Pointer) *SocketAddrV4 {
	result := &SocketAddrV4{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_socketaddrv4(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*SocketAddrV4).Destroy)
	return result
}

func (c FfiConverterSocketAddrV4) Read(reader io.Reader) *SocketAddrV4 {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSocketAddrV4) Lower(value *SocketAddrV4) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*SocketAddrV4")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterSocketAddrV4) Write(writer io.Writer, value *SocketAddrV4) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSocketAddrV4 struct{}

func (_ FfiDestroyerSocketAddrV4) Destroy(value *SocketAddrV4) {
	value.Destroy()
}

type SocketAddrV6 struct {
	ffiObject FfiObject
}

func NewSocketAddrV6(ipv6 *Ipv6Addr, port uint16) *SocketAddrV6 {
	return FfiConverterSocketAddrV6INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_socketaddrv6_new(FfiConverterIpv6AddrINSTANCE.Lower(ipv6), FfiConverterUint16INSTANCE.Lower(port), _uniffiStatus)
	}))
}

func SocketAddrV6FromString(str string) (*SocketAddrV6, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_socketaddrv6_from_string(FfiConverterStringINSTANCE.Lower(str), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *SocketAddrV6
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterSocketAddrV6INSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *SocketAddrV6) Equal(other *SocketAddrV6) bool {
	_pointer := _self.ffiObject.incrementPointer("*SocketAddrV6")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_socketaddrv6_equal(
			_pointer, FfiConverterSocketAddrV6INSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *SocketAddrV6) Ip() *Ipv6Addr {
	_pointer := _self.ffiObject.incrementPointer("*SocketAddrV6")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterIpv6AddrINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_method_socketaddrv6_ip(
			_pointer, _uniffiStatus)
	}))
}

func (_self *SocketAddrV6) Port() uint16 {
	_pointer := _self.ffiObject.incrementPointer("*SocketAddrV6")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterUint16INSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.uint16_t {
		return C.uniffi_iroh_fn_method_socketaddrv6_port(
			_pointer, _uniffiStatus)
	}))
}

func (_self *SocketAddrV6) ToString() string {
	_pointer := _self.ffiObject.incrementPointer("*SocketAddrV6")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_socketaddrv6_to_string(
			_pointer, _uniffiStatus)
	}))
}

func (object *SocketAddrV6) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterSocketAddrV6 struct{}

var FfiConverterSocketAddrV6INSTANCE = FfiConverterSocketAddrV6{}

func (c FfiConverterSocketAddrV6) Lift(pointer unsafe.Pointer) *SocketAddrV6 {
	result := &SocketAddrV6{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_socketaddrv6(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*SocketAddrV6).Destroy)
	return result
}

func (c FfiConverterSocketAddrV6) Read(reader io.Reader) *SocketAddrV6 {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterSocketAddrV6) Lower(value *SocketAddrV6) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*SocketAddrV6")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterSocketAddrV6) Write(writer io.Writer, value *SocketAddrV6) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerSocketAddrV6 struct{}

func (_ FfiDestroyerSocketAddrV6) Destroy(value *SocketAddrV6) {
	value.Destroy()
}

type Tag struct {
	ffiObject FfiObject
}

func TagFromBytes(bytes []byte) *Tag {
	return FfiConverterTagINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_tag_from_bytes(FfiConverterBytesINSTANCE.Lower(bytes), _uniffiStatus)
	}))
}
func TagFromString(s string) *Tag {
	return FfiConverterTagINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_tag_from_string(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	}))
}

func (_self *Tag) Equal(other *Tag) bool {
	_pointer := _self.ffiObject.incrementPointer("*Tag")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_tag_equal(
			_pointer, FfiConverterTagINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *Tag) ToBytes() []byte {
	_pointer := _self.ffiObject.incrementPointer("*Tag")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBytesINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_tag_to_bytes(
			_pointer, _uniffiStatus)
	}))
}

func (_self *Tag) ToString() string {
	_pointer := _self.ffiObject.incrementPointer("*Tag")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_tag_to_string(
			_pointer, _uniffiStatus)
	}))
}

func (object *Tag) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterTag struct{}

var FfiConverterTagINSTANCE = FfiConverterTag{}

func (c FfiConverterTag) Lift(pointer unsafe.Pointer) *Tag {
	result := &Tag{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_tag(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*Tag).Destroy)
	return result
}

func (c FfiConverterTag) Read(reader io.Reader) *Tag {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterTag) Lower(value *Tag) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Tag")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterTag) Write(writer io.Writer, value *Tag) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerTag struct{}

func (_ FfiDestroyerTag) Destroy(value *Tag) {
	value.Destroy()
}

type Url struct {
	ffiObject FfiObject
}

func UrlFromString(s string) (*Url, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_url_from_string(FfiConverterStringINSTANCE.Lower(s), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue *Url
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterUrlINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func (_self *Url) Equal(other *Url) bool {
	_pointer := _self.ffiObject.incrementPointer("*Url")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterBoolINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) C.int8_t {
		return C.uniffi_iroh_fn_method_url_equal(
			_pointer, FfiConverterUrlINSTANCE.Lower(other), _uniffiStatus)
	}))
}

func (_self *Url) ToString() string {
	_pointer := _self.ffiObject.incrementPointer("*Url")
	defer _self.ffiObject.decrementPointer()
	return FfiConverterStringINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_method_url_to_string(
			_pointer, _uniffiStatus)
	}))
}

func (object *Url) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterUrl struct{}

var FfiConverterUrlINSTANCE = FfiConverterUrl{}

func (c FfiConverterUrl) Lift(pointer unsafe.Pointer) *Url {
	result := &Url{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_url(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*Url).Destroy)
	return result
}

func (c FfiConverterUrl) Read(reader io.Reader) *Url {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterUrl) Lower(value *Url) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*Url")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterUrl) Write(writer io.Writer, value *Url) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerUrl struct{}

func (_ FfiDestroyerUrl) Destroy(value *Url) {
	value.Destroy()
}

type WrapOption struct {
	ffiObject FfiObject
}

func WrapOptionNoWrap() *WrapOption {
	return FfiConverterWrapOptionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_wrapoption_no_wrap(_uniffiStatus)
	}))
}
func WrapOptionWrap(name *string) *WrapOption {
	return FfiConverterWrapOptionINSTANCE.Lift(rustCall(func(_uniffiStatus *C.RustCallStatus) unsafe.Pointer {
		return C.uniffi_iroh_fn_constructor_wrapoption_wrap(FfiConverterOptionalStringINSTANCE.Lower(name), _uniffiStatus)
	}))
}

func (object *WrapOption) Destroy() {
	runtime.SetFinalizer(object, nil)
	object.ffiObject.destroy()
}

type FfiConverterWrapOption struct{}

var FfiConverterWrapOptionINSTANCE = FfiConverterWrapOption{}

func (c FfiConverterWrapOption) Lift(pointer unsafe.Pointer) *WrapOption {
	result := &WrapOption{
		newFfiObject(
			pointer,
			func(pointer unsafe.Pointer, status *C.RustCallStatus) {
				C.uniffi_iroh_fn_free_wrapoption(pointer, status)
			}),
	}
	runtime.SetFinalizer(result, (*WrapOption).Destroy)
	return result
}

func (c FfiConverterWrapOption) Read(reader io.Reader) *WrapOption {
	return c.Lift(unsafe.Pointer(uintptr(readUint64(reader))))
}

func (c FfiConverterWrapOption) Lower(value *WrapOption) unsafe.Pointer {
	// TODO: this is bad - all synchronization from ObjectRuntime.go is discarded here,
	// because the pointer will be decremented immediately after this function returns,
	// and someone will be left holding onto a non-locked pointer.
	pointer := value.ffiObject.incrementPointer("*WrapOption")
	defer value.ffiObject.decrementPointer()
	return pointer
}

func (c FfiConverterWrapOption) Write(writer io.Writer, value *WrapOption) {
	writeUint64(writer, uint64(uintptr(c.Lower(value))))
}

type FfiDestroyerWrapOption struct{}

func (_ FfiDestroyerWrapOption) Destroy(value *WrapOption) {
	value.Destroy()
}

type AddProgressAbort struct {
	Error string
}

func (r *AddProgressAbort) Destroy() {
	FfiDestroyerString{}.Destroy(r.Error)
}

type FfiConverterTypeAddProgressAbort struct{}

var FfiConverterTypeAddProgressAbortINSTANCE = FfiConverterTypeAddProgressAbort{}

func (c FfiConverterTypeAddProgressAbort) Lift(rb RustBufferI) AddProgressAbort {
	return LiftFromRustBuffer[AddProgressAbort](c, rb)
}

func (c FfiConverterTypeAddProgressAbort) Read(reader io.Reader) AddProgressAbort {
	return AddProgressAbort{
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeAddProgressAbort) Lower(value AddProgressAbort) RustBuffer {
	return LowerIntoRustBuffer[AddProgressAbort](c, value)
}

func (c FfiConverterTypeAddProgressAbort) Write(writer io.Writer, value AddProgressAbort) {
	FfiConverterStringINSTANCE.Write(writer, value.Error)
}

type FfiDestroyerTypeAddProgressAbort struct{}

func (_ FfiDestroyerTypeAddProgressAbort) Destroy(value AddProgressAbort) {
	value.Destroy()
}

type AddProgressAllDone struct {
	Hash   *Hash
	Format BlobFormat
	Tag    *Tag
}

func (r *AddProgressAllDone) Destroy() {
	FfiDestroyerHash{}.Destroy(r.Hash)
	FfiDestroyerTypeBlobFormat{}.Destroy(r.Format)
	FfiDestroyerTag{}.Destroy(r.Tag)
}

type FfiConverterTypeAddProgressAllDone struct{}

var FfiConverterTypeAddProgressAllDoneINSTANCE = FfiConverterTypeAddProgressAllDone{}

func (c FfiConverterTypeAddProgressAllDone) Lift(rb RustBufferI) AddProgressAllDone {
	return LiftFromRustBuffer[AddProgressAllDone](c, rb)
}

func (c FfiConverterTypeAddProgressAllDone) Read(reader io.Reader) AddProgressAllDone {
	return AddProgressAllDone{
		FfiConverterHashINSTANCE.Read(reader),
		FfiConverterTypeBlobFormatINSTANCE.Read(reader),
		FfiConverterTagINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeAddProgressAllDone) Lower(value AddProgressAllDone) RustBuffer {
	return LowerIntoRustBuffer[AddProgressAllDone](c, value)
}

func (c FfiConverterTypeAddProgressAllDone) Write(writer io.Writer, value AddProgressAllDone) {
	FfiConverterHashINSTANCE.Write(writer, value.Hash)
	FfiConverterTypeBlobFormatINSTANCE.Write(writer, value.Format)
	FfiConverterTagINSTANCE.Write(writer, value.Tag)
}

type FfiDestroyerTypeAddProgressAllDone struct{}

func (_ FfiDestroyerTypeAddProgressAllDone) Destroy(value AddProgressAllDone) {
	value.Destroy()
}

type AddProgressDone struct {
	Id   uint64
	Hash *Hash
}

func (r *AddProgressDone) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
	FfiDestroyerHash{}.Destroy(r.Hash)
}

type FfiConverterTypeAddProgressDone struct{}

var FfiConverterTypeAddProgressDoneINSTANCE = FfiConverterTypeAddProgressDone{}

func (c FfiConverterTypeAddProgressDone) Lift(rb RustBufferI) AddProgressDone {
	return LiftFromRustBuffer[AddProgressDone](c, rb)
}

func (c FfiConverterTypeAddProgressDone) Read(reader io.Reader) AddProgressDone {
	return AddProgressDone{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterHashINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeAddProgressDone) Lower(value AddProgressDone) RustBuffer {
	return LowerIntoRustBuffer[AddProgressDone](c, value)
}

func (c FfiConverterTypeAddProgressDone) Write(writer io.Writer, value AddProgressDone) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
	FfiConverterHashINSTANCE.Write(writer, value.Hash)
}

type FfiDestroyerTypeAddProgressDone struct{}

func (_ FfiDestroyerTypeAddProgressDone) Destroy(value AddProgressDone) {
	value.Destroy()
}

type AddProgressFound struct {
	Id   uint64
	Name string
	Size uint64
}

func (r *AddProgressFound) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
	FfiDestroyerString{}.Destroy(r.Name)
	FfiDestroyerUint64{}.Destroy(r.Size)
}

type FfiConverterTypeAddProgressFound struct{}

var FfiConverterTypeAddProgressFoundINSTANCE = FfiConverterTypeAddProgressFound{}

func (c FfiConverterTypeAddProgressFound) Lift(rb RustBufferI) AddProgressFound {
	return LiftFromRustBuffer[AddProgressFound](c, rb)
}

func (c FfiConverterTypeAddProgressFound) Read(reader io.Reader) AddProgressFound {
	return AddProgressFound{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeAddProgressFound) Lower(value AddProgressFound) RustBuffer {
	return LowerIntoRustBuffer[AddProgressFound](c, value)
}

func (c FfiConverterTypeAddProgressFound) Write(writer io.Writer, value AddProgressFound) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
	FfiConverterStringINSTANCE.Write(writer, value.Name)
	FfiConverterUint64INSTANCE.Write(writer, value.Size)
}

type FfiDestroyerTypeAddProgressFound struct{}

func (_ FfiDestroyerTypeAddProgressFound) Destroy(value AddProgressFound) {
	value.Destroy()
}

type AddProgressProgress struct {
	Id     uint64
	Offset uint64
}

func (r *AddProgressProgress) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
	FfiDestroyerUint64{}.Destroy(r.Offset)
}

type FfiConverterTypeAddProgressProgress struct{}

var FfiConverterTypeAddProgressProgressINSTANCE = FfiConverterTypeAddProgressProgress{}

func (c FfiConverterTypeAddProgressProgress) Lift(rb RustBufferI) AddProgressProgress {
	return LiftFromRustBuffer[AddProgressProgress](c, rb)
}

func (c FfiConverterTypeAddProgressProgress) Read(reader io.Reader) AddProgressProgress {
	return AddProgressProgress{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeAddProgressProgress) Lower(value AddProgressProgress) RustBuffer {
	return LowerIntoRustBuffer[AddProgressProgress](c, value)
}

func (c FfiConverterTypeAddProgressProgress) Write(writer io.Writer, value AddProgressProgress) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
	FfiConverterUint64INSTANCE.Write(writer, value.Offset)
}

type FfiDestroyerTypeAddProgressProgress struct{}

func (_ FfiDestroyerTypeAddProgressProgress) Destroy(value AddProgressProgress) {
	value.Destroy()
}

type BlobAddOutcome struct {
	Hash   *Hash
	Format BlobFormat
	Size   uint64
	Tag    *Tag
}

func (r *BlobAddOutcome) Destroy() {
	FfiDestroyerHash{}.Destroy(r.Hash)
	FfiDestroyerTypeBlobFormat{}.Destroy(r.Format)
	FfiDestroyerUint64{}.Destroy(r.Size)
	FfiDestroyerTag{}.Destroy(r.Tag)
}

type FfiConverterTypeBlobAddOutcome struct{}

var FfiConverterTypeBlobAddOutcomeINSTANCE = FfiConverterTypeBlobAddOutcome{}

func (c FfiConverterTypeBlobAddOutcome) Lift(rb RustBufferI) BlobAddOutcome {
	return LiftFromRustBuffer[BlobAddOutcome](c, rb)
}

func (c FfiConverterTypeBlobAddOutcome) Read(reader io.Reader) BlobAddOutcome {
	return BlobAddOutcome{
		FfiConverterHashINSTANCE.Read(reader),
		FfiConverterTypeBlobFormatINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterTagINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeBlobAddOutcome) Lower(value BlobAddOutcome) RustBuffer {
	return LowerIntoRustBuffer[BlobAddOutcome](c, value)
}

func (c FfiConverterTypeBlobAddOutcome) Write(writer io.Writer, value BlobAddOutcome) {
	FfiConverterHashINSTANCE.Write(writer, value.Hash)
	FfiConverterTypeBlobFormatINSTANCE.Write(writer, value.Format)
	FfiConverterUint64INSTANCE.Write(writer, value.Size)
	FfiConverterTagINSTANCE.Write(writer, value.Tag)
}

type FfiDestroyerTypeBlobAddOutcome struct{}

func (_ FfiDestroyerTypeBlobAddOutcome) Destroy(value BlobAddOutcome) {
	value.Destroy()
}

type BlobListCollectionsResponse struct {
	Tag             *Tag
	Hash            *Hash
	TotalBlobsCount *uint64
	TotalBlobsSize  *uint64
}

func (r *BlobListCollectionsResponse) Destroy() {
	FfiDestroyerTag{}.Destroy(r.Tag)
	FfiDestroyerHash{}.Destroy(r.Hash)
	FfiDestroyerOptionalUint64{}.Destroy(r.TotalBlobsCount)
	FfiDestroyerOptionalUint64{}.Destroy(r.TotalBlobsSize)
}

type FfiConverterTypeBlobListCollectionsResponse struct{}

var FfiConverterTypeBlobListCollectionsResponseINSTANCE = FfiConverterTypeBlobListCollectionsResponse{}

func (c FfiConverterTypeBlobListCollectionsResponse) Lift(rb RustBufferI) BlobListCollectionsResponse {
	return LiftFromRustBuffer[BlobListCollectionsResponse](c, rb)
}

func (c FfiConverterTypeBlobListCollectionsResponse) Read(reader io.Reader) BlobListCollectionsResponse {
	return BlobListCollectionsResponse{
		FfiConverterTagINSTANCE.Read(reader),
		FfiConverterHashINSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
		FfiConverterOptionalUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeBlobListCollectionsResponse) Lower(value BlobListCollectionsResponse) RustBuffer {
	return LowerIntoRustBuffer[BlobListCollectionsResponse](c, value)
}

func (c FfiConverterTypeBlobListCollectionsResponse) Write(writer io.Writer, value BlobListCollectionsResponse) {
	FfiConverterTagINSTANCE.Write(writer, value.Tag)
	FfiConverterHashINSTANCE.Write(writer, value.Hash)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.TotalBlobsCount)
	FfiConverterOptionalUint64INSTANCE.Write(writer, value.TotalBlobsSize)
}

type FfiDestroyerTypeBlobListCollectionsResponse struct{}

func (_ FfiDestroyerTypeBlobListCollectionsResponse) Destroy(value BlobListCollectionsResponse) {
	value.Destroy()
}

type BlobListIncompleteResponse struct {
	Size         uint64
	ExpectedSize uint64
	Hash         *Hash
}

func (r *BlobListIncompleteResponse) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Size)
	FfiDestroyerUint64{}.Destroy(r.ExpectedSize)
	FfiDestroyerHash{}.Destroy(r.Hash)
}

type FfiConverterTypeBlobListIncompleteResponse struct{}

var FfiConverterTypeBlobListIncompleteResponseINSTANCE = FfiConverterTypeBlobListIncompleteResponse{}

func (c FfiConverterTypeBlobListIncompleteResponse) Lift(rb RustBufferI) BlobListIncompleteResponse {
	return LiftFromRustBuffer[BlobListIncompleteResponse](c, rb)
}

func (c FfiConverterTypeBlobListIncompleteResponse) Read(reader io.Reader) BlobListIncompleteResponse {
	return BlobListIncompleteResponse{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterHashINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeBlobListIncompleteResponse) Lower(value BlobListIncompleteResponse) RustBuffer {
	return LowerIntoRustBuffer[BlobListIncompleteResponse](c, value)
}

func (c FfiConverterTypeBlobListIncompleteResponse) Write(writer io.Writer, value BlobListIncompleteResponse) {
	FfiConverterUint64INSTANCE.Write(writer, value.Size)
	FfiConverterUint64INSTANCE.Write(writer, value.ExpectedSize)
	FfiConverterHashINSTANCE.Write(writer, value.Hash)
}

type FfiDestroyerTypeBlobListIncompleteResponse struct{}

func (_ FfiDestroyerTypeBlobListIncompleteResponse) Destroy(value BlobListIncompleteResponse) {
	value.Destroy()
}

type BlobListResponse struct {
	Path string
	Hash *Hash
	Size uint64
}

func (r *BlobListResponse) Destroy() {
	FfiDestroyerString{}.Destroy(r.Path)
	FfiDestroyerHash{}.Destroy(r.Hash)
	FfiDestroyerUint64{}.Destroy(r.Size)
}

type FfiConverterTypeBlobListResponse struct{}

var FfiConverterTypeBlobListResponseINSTANCE = FfiConverterTypeBlobListResponse{}

func (c FfiConverterTypeBlobListResponse) Lift(rb RustBufferI) BlobListResponse {
	return LiftFromRustBuffer[BlobListResponse](c, rb)
}

func (c FfiConverterTypeBlobListResponse) Read(reader io.Reader) BlobListResponse {
	return BlobListResponse{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterHashINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeBlobListResponse) Lower(value BlobListResponse) RustBuffer {
	return LowerIntoRustBuffer[BlobListResponse](c, value)
}

func (c FfiConverterTypeBlobListResponse) Write(writer io.Writer, value BlobListResponse) {
	FfiConverterStringINSTANCE.Write(writer, value.Path)
	FfiConverterHashINSTANCE.Write(writer, value.Hash)
	FfiConverterUint64INSTANCE.Write(writer, value.Size)
}

type FfiDestroyerTypeBlobListResponse struct{}

func (_ FfiDestroyerTypeBlobListResponse) Destroy(value BlobListResponse) {
	value.Destroy()
}

type ConnectionInfo struct {
	PublicKey *PublicKey
	DerpUrl   **Url
	Addrs     []*DirectAddrInfo
	ConnType  *ConnectionType
	Latency   *time.Duration
	LastUsed  *time.Duration
}

func (r *ConnectionInfo) Destroy() {
	FfiDestroyerPublicKey{}.Destroy(r.PublicKey)
	FfiDestroyerOptionalUrl{}.Destroy(r.DerpUrl)
	FfiDestroyerSequenceDirectAddrInfo{}.Destroy(r.Addrs)
	FfiDestroyerConnectionType{}.Destroy(r.ConnType)
	FfiDestroyerOptionalDuration{}.Destroy(r.Latency)
	FfiDestroyerOptionalDuration{}.Destroy(r.LastUsed)
}

type FfiConverterTypeConnectionInfo struct{}

var FfiConverterTypeConnectionInfoINSTANCE = FfiConverterTypeConnectionInfo{}

func (c FfiConverterTypeConnectionInfo) Lift(rb RustBufferI) ConnectionInfo {
	return LiftFromRustBuffer[ConnectionInfo](c, rb)
}

func (c FfiConverterTypeConnectionInfo) Read(reader io.Reader) ConnectionInfo {
	return ConnectionInfo{
		FfiConverterPublicKeyINSTANCE.Read(reader),
		FfiConverterOptionalUrlINSTANCE.Read(reader),
		FfiConverterSequenceDirectAddrInfoINSTANCE.Read(reader),
		FfiConverterConnectionTypeINSTANCE.Read(reader),
		FfiConverterOptionalDurationINSTANCE.Read(reader),
		FfiConverterOptionalDurationINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeConnectionInfo) Lower(value ConnectionInfo) RustBuffer {
	return LowerIntoRustBuffer[ConnectionInfo](c, value)
}

func (c FfiConverterTypeConnectionInfo) Write(writer io.Writer, value ConnectionInfo) {
	FfiConverterPublicKeyINSTANCE.Write(writer, value.PublicKey)
	FfiConverterOptionalUrlINSTANCE.Write(writer, value.DerpUrl)
	FfiConverterSequenceDirectAddrInfoINSTANCE.Write(writer, value.Addrs)
	FfiConverterConnectionTypeINSTANCE.Write(writer, value.ConnType)
	FfiConverterOptionalDurationINSTANCE.Write(writer, value.Latency)
	FfiConverterOptionalDurationINSTANCE.Write(writer, value.LastUsed)
}

type FfiDestroyerTypeConnectionInfo struct{}

func (_ FfiDestroyerTypeConnectionInfo) Destroy(value ConnectionInfo) {
	value.Destroy()
}

type ConnectionTypeMixed struct {
	Addr    *SocketAddr
	DerpUrl *Url
}

func (r *ConnectionTypeMixed) Destroy() {
	FfiDestroyerSocketAddr{}.Destroy(r.Addr)
	FfiDestroyerUrl{}.Destroy(r.DerpUrl)
}

type FfiConverterTypeConnectionTypeMixed struct{}

var FfiConverterTypeConnectionTypeMixedINSTANCE = FfiConverterTypeConnectionTypeMixed{}

func (c FfiConverterTypeConnectionTypeMixed) Lift(rb RustBufferI) ConnectionTypeMixed {
	return LiftFromRustBuffer[ConnectionTypeMixed](c, rb)
}

func (c FfiConverterTypeConnectionTypeMixed) Read(reader io.Reader) ConnectionTypeMixed {
	return ConnectionTypeMixed{
		FfiConverterSocketAddrINSTANCE.Read(reader),
		FfiConverterUrlINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeConnectionTypeMixed) Lower(value ConnectionTypeMixed) RustBuffer {
	return LowerIntoRustBuffer[ConnectionTypeMixed](c, value)
}

func (c FfiConverterTypeConnectionTypeMixed) Write(writer io.Writer, value ConnectionTypeMixed) {
	FfiConverterSocketAddrINSTANCE.Write(writer, value.Addr)
	FfiConverterUrlINSTANCE.Write(writer, value.DerpUrl)
}

type FfiDestroyerTypeConnectionTypeMixed struct{}

func (_ FfiDestroyerTypeConnectionTypeMixed) Destroy(value ConnectionTypeMixed) {
	value.Destroy()
}

type CounterStats struct {
	Value       uint64
	Description string
}

func (r *CounterStats) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Value)
	FfiDestroyerString{}.Destroy(r.Description)
}

type FfiConverterTypeCounterStats struct{}

var FfiConverterTypeCounterStatsINSTANCE = FfiConverterTypeCounterStats{}

func (c FfiConverterTypeCounterStats) Lift(rb RustBufferI) CounterStats {
	return LiftFromRustBuffer[CounterStats](c, rb)
}

func (c FfiConverterTypeCounterStats) Read(reader io.Reader) CounterStats {
	return CounterStats{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeCounterStats) Lower(value CounterStats) RustBuffer {
	return LowerIntoRustBuffer[CounterStats](c, value)
}

func (c FfiConverterTypeCounterStats) Write(writer io.Writer, value CounterStats) {
	FfiConverterUint64INSTANCE.Write(writer, value.Value)
	FfiConverterStringINSTANCE.Write(writer, value.Description)
}

type FfiDestroyerTypeCounterStats struct{}

func (_ FfiDestroyerTypeCounterStats) Destroy(value CounterStats) {
	value.Destroy()
}

type DocExportProgressAbort struct {
	Error string
}

func (r *DocExportProgressAbort) Destroy() {
	FfiDestroyerString{}.Destroy(r.Error)
}

type FfiConverterTypeDocExportProgressAbort struct{}

var FfiConverterTypeDocExportProgressAbortINSTANCE = FfiConverterTypeDocExportProgressAbort{}

func (c FfiConverterTypeDocExportProgressAbort) Lift(rb RustBufferI) DocExportProgressAbort {
	return LiftFromRustBuffer[DocExportProgressAbort](c, rb)
}

func (c FfiConverterTypeDocExportProgressAbort) Read(reader io.Reader) DocExportProgressAbort {
	return DocExportProgressAbort{
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDocExportProgressAbort) Lower(value DocExportProgressAbort) RustBuffer {
	return LowerIntoRustBuffer[DocExportProgressAbort](c, value)
}

func (c FfiConverterTypeDocExportProgressAbort) Write(writer io.Writer, value DocExportProgressAbort) {
	FfiConverterStringINSTANCE.Write(writer, value.Error)
}

type FfiDestroyerTypeDocExportProgressAbort struct{}

func (_ FfiDestroyerTypeDocExportProgressAbort) Destroy(value DocExportProgressAbort) {
	value.Destroy()
}

type DocExportProgressFound struct {
	Id      uint64
	Hash    *Hash
	Key     []byte
	Size    uint64
	Outpath string
}

func (r *DocExportProgressFound) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
	FfiDestroyerHash{}.Destroy(r.Hash)
	FfiDestroyerBytes{}.Destroy(r.Key)
	FfiDestroyerUint64{}.Destroy(r.Size)
	FfiDestroyerString{}.Destroy(r.Outpath)
}

type FfiConverterTypeDocExportProgressFound struct{}

var FfiConverterTypeDocExportProgressFoundINSTANCE = FfiConverterTypeDocExportProgressFound{}

func (c FfiConverterTypeDocExportProgressFound) Lift(rb RustBufferI) DocExportProgressFound {
	return LiftFromRustBuffer[DocExportProgressFound](c, rb)
}

func (c FfiConverterTypeDocExportProgressFound) Read(reader io.Reader) DocExportProgressFound {
	return DocExportProgressFound{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterHashINSTANCE.Read(reader),
		FfiConverterBytesINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDocExportProgressFound) Lower(value DocExportProgressFound) RustBuffer {
	return LowerIntoRustBuffer[DocExportProgressFound](c, value)
}

func (c FfiConverterTypeDocExportProgressFound) Write(writer io.Writer, value DocExportProgressFound) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
	FfiConverterHashINSTANCE.Write(writer, value.Hash)
	FfiConverterBytesINSTANCE.Write(writer, value.Key)
	FfiConverterUint64INSTANCE.Write(writer, value.Size)
	FfiConverterStringINSTANCE.Write(writer, value.Outpath)
}

type FfiDestroyerTypeDocExportProgressFound struct{}

func (_ FfiDestroyerTypeDocExportProgressFound) Destroy(value DocExportProgressFound) {
	value.Destroy()
}

type DocExportProgressProgress struct {
	Id     uint64
	Offset uint64
}

func (r *DocExportProgressProgress) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
	FfiDestroyerUint64{}.Destroy(r.Offset)
}

type FfiConverterTypeDocExportProgressProgress struct{}

var FfiConverterTypeDocExportProgressProgressINSTANCE = FfiConverterTypeDocExportProgressProgress{}

func (c FfiConverterTypeDocExportProgressProgress) Lift(rb RustBufferI) DocExportProgressProgress {
	return LiftFromRustBuffer[DocExportProgressProgress](c, rb)
}

func (c FfiConverterTypeDocExportProgressProgress) Read(reader io.Reader) DocExportProgressProgress {
	return DocExportProgressProgress{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDocExportProgressProgress) Lower(value DocExportProgressProgress) RustBuffer {
	return LowerIntoRustBuffer[DocExportProgressProgress](c, value)
}

func (c FfiConverterTypeDocExportProgressProgress) Write(writer io.Writer, value DocExportProgressProgress) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
	FfiConverterUint64INSTANCE.Write(writer, value.Offset)
}

type FfiDestroyerTypeDocExportProgressProgress struct{}

func (_ FfiDestroyerTypeDocExportProgressProgress) Destroy(value DocExportProgressProgress) {
	value.Destroy()
}

type DocImportProgressAbort struct {
	Error string
}

func (r *DocImportProgressAbort) Destroy() {
	FfiDestroyerString{}.Destroy(r.Error)
}

type FfiConverterTypeDocImportProgressAbort struct{}

var FfiConverterTypeDocImportProgressAbortINSTANCE = FfiConverterTypeDocImportProgressAbort{}

func (c FfiConverterTypeDocImportProgressAbort) Lift(rb RustBufferI) DocImportProgressAbort {
	return LiftFromRustBuffer[DocImportProgressAbort](c, rb)
}

func (c FfiConverterTypeDocImportProgressAbort) Read(reader io.Reader) DocImportProgressAbort {
	return DocImportProgressAbort{
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDocImportProgressAbort) Lower(value DocImportProgressAbort) RustBuffer {
	return LowerIntoRustBuffer[DocImportProgressAbort](c, value)
}

func (c FfiConverterTypeDocImportProgressAbort) Write(writer io.Writer, value DocImportProgressAbort) {
	FfiConverterStringINSTANCE.Write(writer, value.Error)
}

type FfiDestroyerTypeDocImportProgressAbort struct{}

func (_ FfiDestroyerTypeDocImportProgressAbort) Destroy(value DocImportProgressAbort) {
	value.Destroy()
}

type DocImportProgressAllDone struct {
	Key []byte
}

func (r *DocImportProgressAllDone) Destroy() {
	FfiDestroyerBytes{}.Destroy(r.Key)
}

type FfiConverterTypeDocImportProgressAllDone struct{}

var FfiConverterTypeDocImportProgressAllDoneINSTANCE = FfiConverterTypeDocImportProgressAllDone{}

func (c FfiConverterTypeDocImportProgressAllDone) Lift(rb RustBufferI) DocImportProgressAllDone {
	return LiftFromRustBuffer[DocImportProgressAllDone](c, rb)
}

func (c FfiConverterTypeDocImportProgressAllDone) Read(reader io.Reader) DocImportProgressAllDone {
	return DocImportProgressAllDone{
		FfiConverterBytesINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDocImportProgressAllDone) Lower(value DocImportProgressAllDone) RustBuffer {
	return LowerIntoRustBuffer[DocImportProgressAllDone](c, value)
}

func (c FfiConverterTypeDocImportProgressAllDone) Write(writer io.Writer, value DocImportProgressAllDone) {
	FfiConverterBytesINSTANCE.Write(writer, value.Key)
}

type FfiDestroyerTypeDocImportProgressAllDone struct{}

func (_ FfiDestroyerTypeDocImportProgressAllDone) Destroy(value DocImportProgressAllDone) {
	value.Destroy()
}

type DocImportProgressFound struct {
	Id   uint64
	Name string
	Size uint64
}

func (r *DocImportProgressFound) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
	FfiDestroyerString{}.Destroy(r.Name)
	FfiDestroyerUint64{}.Destroy(r.Size)
}

type FfiConverterTypeDocImportProgressFound struct{}

var FfiConverterTypeDocImportProgressFoundINSTANCE = FfiConverterTypeDocImportProgressFound{}

func (c FfiConverterTypeDocImportProgressFound) Lift(rb RustBufferI) DocImportProgressFound {
	return LiftFromRustBuffer[DocImportProgressFound](c, rb)
}

func (c FfiConverterTypeDocImportProgressFound) Read(reader io.Reader) DocImportProgressFound {
	return DocImportProgressFound{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDocImportProgressFound) Lower(value DocImportProgressFound) RustBuffer {
	return LowerIntoRustBuffer[DocImportProgressFound](c, value)
}

func (c FfiConverterTypeDocImportProgressFound) Write(writer io.Writer, value DocImportProgressFound) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
	FfiConverterStringINSTANCE.Write(writer, value.Name)
	FfiConverterUint64INSTANCE.Write(writer, value.Size)
}

type FfiDestroyerTypeDocImportProgressFound struct{}

func (_ FfiDestroyerTypeDocImportProgressFound) Destroy(value DocImportProgressFound) {
	value.Destroy()
}

type DocImportProgressIngestDone struct {
	Id   uint64
	Hash *Hash
}

func (r *DocImportProgressIngestDone) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
	FfiDestroyerHash{}.Destroy(r.Hash)
}

type FfiConverterTypeDocImportProgressIngestDone struct{}

var FfiConverterTypeDocImportProgressIngestDoneINSTANCE = FfiConverterTypeDocImportProgressIngestDone{}

func (c FfiConverterTypeDocImportProgressIngestDone) Lift(rb RustBufferI) DocImportProgressIngestDone {
	return LiftFromRustBuffer[DocImportProgressIngestDone](c, rb)
}

func (c FfiConverterTypeDocImportProgressIngestDone) Read(reader io.Reader) DocImportProgressIngestDone {
	return DocImportProgressIngestDone{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterHashINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDocImportProgressIngestDone) Lower(value DocImportProgressIngestDone) RustBuffer {
	return LowerIntoRustBuffer[DocImportProgressIngestDone](c, value)
}

func (c FfiConverterTypeDocImportProgressIngestDone) Write(writer io.Writer, value DocImportProgressIngestDone) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
	FfiConverterHashINSTANCE.Write(writer, value.Hash)
}

type FfiDestroyerTypeDocImportProgressIngestDone struct{}

func (_ FfiDestroyerTypeDocImportProgressIngestDone) Destroy(value DocImportProgressIngestDone) {
	value.Destroy()
}

type DocImportProgressProgress struct {
	Id     uint64
	Offset uint64
}

func (r *DocImportProgressProgress) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
	FfiDestroyerUint64{}.Destroy(r.Offset)
}

type FfiConverterTypeDocImportProgressProgress struct{}

var FfiConverterTypeDocImportProgressProgressINSTANCE = FfiConverterTypeDocImportProgressProgress{}

func (c FfiConverterTypeDocImportProgressProgress) Lift(rb RustBufferI) DocImportProgressProgress {
	return LiftFromRustBuffer[DocImportProgressProgress](c, rb)
}

func (c FfiConverterTypeDocImportProgressProgress) Read(reader io.Reader) DocImportProgressProgress {
	return DocImportProgressProgress{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDocImportProgressProgress) Lower(value DocImportProgressProgress) RustBuffer {
	return LowerIntoRustBuffer[DocImportProgressProgress](c, value)
}

func (c FfiConverterTypeDocImportProgressProgress) Write(writer io.Writer, value DocImportProgressProgress) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
	FfiConverterUint64INSTANCE.Write(writer, value.Offset)
}

type FfiDestroyerTypeDocImportProgressProgress struct{}

func (_ FfiDestroyerTypeDocImportProgressProgress) Destroy(value DocImportProgressProgress) {
	value.Destroy()
}

type DownloadProgressAbort struct {
	Error string
}

func (r *DownloadProgressAbort) Destroy() {
	FfiDestroyerString{}.Destroy(r.Error)
}

type FfiConverterTypeDownloadProgressAbort struct{}

var FfiConverterTypeDownloadProgressAbortINSTANCE = FfiConverterTypeDownloadProgressAbort{}

func (c FfiConverterTypeDownloadProgressAbort) Lift(rb RustBufferI) DownloadProgressAbort {
	return LiftFromRustBuffer[DownloadProgressAbort](c, rb)
}

func (c FfiConverterTypeDownloadProgressAbort) Read(reader io.Reader) DownloadProgressAbort {
	return DownloadProgressAbort{
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDownloadProgressAbort) Lower(value DownloadProgressAbort) RustBuffer {
	return LowerIntoRustBuffer[DownloadProgressAbort](c, value)
}

func (c FfiConverterTypeDownloadProgressAbort) Write(writer io.Writer, value DownloadProgressAbort) {
	FfiConverterStringINSTANCE.Write(writer, value.Error)
}

type FfiDestroyerTypeDownloadProgressAbort struct{}

func (_ FfiDestroyerTypeDownloadProgressAbort) Destroy(value DownloadProgressAbort) {
	value.Destroy()
}

type DownloadProgressDone struct {
	Id uint64
}

func (r *DownloadProgressDone) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
}

type FfiConverterTypeDownloadProgressDone struct{}

var FfiConverterTypeDownloadProgressDoneINSTANCE = FfiConverterTypeDownloadProgressDone{}

func (c FfiConverterTypeDownloadProgressDone) Lift(rb RustBufferI) DownloadProgressDone {
	return LiftFromRustBuffer[DownloadProgressDone](c, rb)
}

func (c FfiConverterTypeDownloadProgressDone) Read(reader io.Reader) DownloadProgressDone {
	return DownloadProgressDone{
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDownloadProgressDone) Lower(value DownloadProgressDone) RustBuffer {
	return LowerIntoRustBuffer[DownloadProgressDone](c, value)
}

func (c FfiConverterTypeDownloadProgressDone) Write(writer io.Writer, value DownloadProgressDone) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
}

type FfiDestroyerTypeDownloadProgressDone struct{}

func (_ FfiDestroyerTypeDownloadProgressDone) Destroy(value DownloadProgressDone) {
	value.Destroy()
}

type DownloadProgressExport struct {
	Id     uint64
	Hash   *Hash
	Size   uint64
	Target string
}

func (r *DownloadProgressExport) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
	FfiDestroyerHash{}.Destroy(r.Hash)
	FfiDestroyerUint64{}.Destroy(r.Size)
	FfiDestroyerString{}.Destroy(r.Target)
}

type FfiConverterTypeDownloadProgressExport struct{}

var FfiConverterTypeDownloadProgressExportINSTANCE = FfiConverterTypeDownloadProgressExport{}

func (c FfiConverterTypeDownloadProgressExport) Lift(rb RustBufferI) DownloadProgressExport {
	return LiftFromRustBuffer[DownloadProgressExport](c, rb)
}

func (c FfiConverterTypeDownloadProgressExport) Read(reader io.Reader) DownloadProgressExport {
	return DownloadProgressExport{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterHashINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDownloadProgressExport) Lower(value DownloadProgressExport) RustBuffer {
	return LowerIntoRustBuffer[DownloadProgressExport](c, value)
}

func (c FfiConverterTypeDownloadProgressExport) Write(writer io.Writer, value DownloadProgressExport) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
	FfiConverterHashINSTANCE.Write(writer, value.Hash)
	FfiConverterUint64INSTANCE.Write(writer, value.Size)
	FfiConverterStringINSTANCE.Write(writer, value.Target)
}

type FfiDestroyerTypeDownloadProgressExport struct{}

func (_ FfiDestroyerTypeDownloadProgressExport) Destroy(value DownloadProgressExport) {
	value.Destroy()
}

type DownloadProgressExportProgress struct {
	Id     uint64
	Offset uint64
}

func (r *DownloadProgressExportProgress) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
	FfiDestroyerUint64{}.Destroy(r.Offset)
}

type FfiConverterTypeDownloadProgressExportProgress struct{}

var FfiConverterTypeDownloadProgressExportProgressINSTANCE = FfiConverterTypeDownloadProgressExportProgress{}

func (c FfiConverterTypeDownloadProgressExportProgress) Lift(rb RustBufferI) DownloadProgressExportProgress {
	return LiftFromRustBuffer[DownloadProgressExportProgress](c, rb)
}

func (c FfiConverterTypeDownloadProgressExportProgress) Read(reader io.Reader) DownloadProgressExportProgress {
	return DownloadProgressExportProgress{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDownloadProgressExportProgress) Lower(value DownloadProgressExportProgress) RustBuffer {
	return LowerIntoRustBuffer[DownloadProgressExportProgress](c, value)
}

func (c FfiConverterTypeDownloadProgressExportProgress) Write(writer io.Writer, value DownloadProgressExportProgress) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
	FfiConverterUint64INSTANCE.Write(writer, value.Offset)
}

type FfiDestroyerTypeDownloadProgressExportProgress struct{}

func (_ FfiDestroyerTypeDownloadProgressExportProgress) Destroy(value DownloadProgressExportProgress) {
	value.Destroy()
}

type DownloadProgressFound struct {
	Id    uint64
	Child uint64
	Hash  *Hash
	Size  uint64
}

func (r *DownloadProgressFound) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
	FfiDestroyerUint64{}.Destroy(r.Child)
	FfiDestroyerHash{}.Destroy(r.Hash)
	FfiDestroyerUint64{}.Destroy(r.Size)
}

type FfiConverterTypeDownloadProgressFound struct{}

var FfiConverterTypeDownloadProgressFoundINSTANCE = FfiConverterTypeDownloadProgressFound{}

func (c FfiConverterTypeDownloadProgressFound) Lift(rb RustBufferI) DownloadProgressFound {
	return LiftFromRustBuffer[DownloadProgressFound](c, rb)
}

func (c FfiConverterTypeDownloadProgressFound) Read(reader io.Reader) DownloadProgressFound {
	return DownloadProgressFound{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterHashINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDownloadProgressFound) Lower(value DownloadProgressFound) RustBuffer {
	return LowerIntoRustBuffer[DownloadProgressFound](c, value)
}

func (c FfiConverterTypeDownloadProgressFound) Write(writer io.Writer, value DownloadProgressFound) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
	FfiConverterUint64INSTANCE.Write(writer, value.Child)
	FfiConverterHashINSTANCE.Write(writer, value.Hash)
	FfiConverterUint64INSTANCE.Write(writer, value.Size)
}

type FfiDestroyerTypeDownloadProgressFound struct{}

func (_ FfiDestroyerTypeDownloadProgressFound) Destroy(value DownloadProgressFound) {
	value.Destroy()
}

type DownloadProgressFoundHashSeq struct {
	Children uint64
	Hash     *Hash
}

func (r *DownloadProgressFoundHashSeq) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Children)
	FfiDestroyerHash{}.Destroy(r.Hash)
}

type FfiConverterTypeDownloadProgressFoundHashSeq struct{}

var FfiConverterTypeDownloadProgressFoundHashSeqINSTANCE = FfiConverterTypeDownloadProgressFoundHashSeq{}

func (c FfiConverterTypeDownloadProgressFoundHashSeq) Lift(rb RustBufferI) DownloadProgressFoundHashSeq {
	return LiftFromRustBuffer[DownloadProgressFoundHashSeq](c, rb)
}

func (c FfiConverterTypeDownloadProgressFoundHashSeq) Read(reader io.Reader) DownloadProgressFoundHashSeq {
	return DownloadProgressFoundHashSeq{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterHashINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDownloadProgressFoundHashSeq) Lower(value DownloadProgressFoundHashSeq) RustBuffer {
	return LowerIntoRustBuffer[DownloadProgressFoundHashSeq](c, value)
}

func (c FfiConverterTypeDownloadProgressFoundHashSeq) Write(writer io.Writer, value DownloadProgressFoundHashSeq) {
	FfiConverterUint64INSTANCE.Write(writer, value.Children)
	FfiConverterHashINSTANCE.Write(writer, value.Hash)
}

type FfiDestroyerTypeDownloadProgressFoundHashSeq struct{}

func (_ FfiDestroyerTypeDownloadProgressFoundHashSeq) Destroy(value DownloadProgressFoundHashSeq) {
	value.Destroy()
}

type DownloadProgressFoundLocal struct {
	Child       uint64
	Hash        *Hash
	Size        uint64
	ValidRanges *RangeSpec
}

func (r *DownloadProgressFoundLocal) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Child)
	FfiDestroyerHash{}.Destroy(r.Hash)
	FfiDestroyerUint64{}.Destroy(r.Size)
	FfiDestroyerRangeSpec{}.Destroy(r.ValidRanges)
}

type FfiConverterTypeDownloadProgressFoundLocal struct{}

var FfiConverterTypeDownloadProgressFoundLocalINSTANCE = FfiConverterTypeDownloadProgressFoundLocal{}

func (c FfiConverterTypeDownloadProgressFoundLocal) Lift(rb RustBufferI) DownloadProgressFoundLocal {
	return LiftFromRustBuffer[DownloadProgressFoundLocal](c, rb)
}

func (c FfiConverterTypeDownloadProgressFoundLocal) Read(reader io.Reader) DownloadProgressFoundLocal {
	return DownloadProgressFoundLocal{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterHashINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterRangeSpecINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDownloadProgressFoundLocal) Lower(value DownloadProgressFoundLocal) RustBuffer {
	return LowerIntoRustBuffer[DownloadProgressFoundLocal](c, value)
}

func (c FfiConverterTypeDownloadProgressFoundLocal) Write(writer io.Writer, value DownloadProgressFoundLocal) {
	FfiConverterUint64INSTANCE.Write(writer, value.Child)
	FfiConverterHashINSTANCE.Write(writer, value.Hash)
	FfiConverterUint64INSTANCE.Write(writer, value.Size)
	FfiConverterRangeSpecINSTANCE.Write(writer, value.ValidRanges)
}

type FfiDestroyerTypeDownloadProgressFoundLocal struct{}

func (_ FfiDestroyerTypeDownloadProgressFoundLocal) Destroy(value DownloadProgressFoundLocal) {
	value.Destroy()
}

type DownloadProgressNetworkDone struct {
	BytesWritten uint64
	BytesRead    uint64
	Elapsed      time.Duration
}

func (r *DownloadProgressNetworkDone) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.BytesWritten)
	FfiDestroyerUint64{}.Destroy(r.BytesRead)
	FfiDestroyerDuration{}.Destroy(r.Elapsed)
}

type FfiConverterTypeDownloadProgressNetworkDone struct{}

var FfiConverterTypeDownloadProgressNetworkDoneINSTANCE = FfiConverterTypeDownloadProgressNetworkDone{}

func (c FfiConverterTypeDownloadProgressNetworkDone) Lift(rb RustBufferI) DownloadProgressNetworkDone {
	return LiftFromRustBuffer[DownloadProgressNetworkDone](c, rb)
}

func (c FfiConverterTypeDownloadProgressNetworkDone) Read(reader io.Reader) DownloadProgressNetworkDone {
	return DownloadProgressNetworkDone{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterDurationINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDownloadProgressNetworkDone) Lower(value DownloadProgressNetworkDone) RustBuffer {
	return LowerIntoRustBuffer[DownloadProgressNetworkDone](c, value)
}

func (c FfiConverterTypeDownloadProgressNetworkDone) Write(writer io.Writer, value DownloadProgressNetworkDone) {
	FfiConverterUint64INSTANCE.Write(writer, value.BytesWritten)
	FfiConverterUint64INSTANCE.Write(writer, value.BytesRead)
	FfiConverterDurationINSTANCE.Write(writer, value.Elapsed)
}

type FfiDestroyerTypeDownloadProgressNetworkDone struct{}

func (_ FfiDestroyerTypeDownloadProgressNetworkDone) Destroy(value DownloadProgressNetworkDone) {
	value.Destroy()
}

type DownloadProgressProgress struct {
	Id     uint64
	Offset uint64
}

func (r *DownloadProgressProgress) Destroy() {
	FfiDestroyerUint64{}.Destroy(r.Id)
	FfiDestroyerUint64{}.Destroy(r.Offset)
}

type FfiConverterTypeDownloadProgressProgress struct{}

var FfiConverterTypeDownloadProgressProgressINSTANCE = FfiConverterTypeDownloadProgressProgress{}

func (c FfiConverterTypeDownloadProgressProgress) Lift(rb RustBufferI) DownloadProgressProgress {
	return LiftFromRustBuffer[DownloadProgressProgress](c, rb)
}

func (c FfiConverterTypeDownloadProgressProgress) Read(reader io.Reader) DownloadProgressProgress {
	return DownloadProgressProgress{
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeDownloadProgressProgress) Lower(value DownloadProgressProgress) RustBuffer {
	return LowerIntoRustBuffer[DownloadProgressProgress](c, value)
}

func (c FfiConverterTypeDownloadProgressProgress) Write(writer io.Writer, value DownloadProgressProgress) {
	FfiConverterUint64INSTANCE.Write(writer, value.Id)
	FfiConverterUint64INSTANCE.Write(writer, value.Offset)
}

type FfiDestroyerTypeDownloadProgressProgress struct{}

func (_ FfiDestroyerTypeDownloadProgressProgress) Destroy(value DownloadProgressProgress) {
	value.Destroy()
}

type InsertRemoteEvent struct {
	From          *PublicKey
	Entry         *Entry
	ContentStatus ContentStatus
}

func (r *InsertRemoteEvent) Destroy() {
	FfiDestroyerPublicKey{}.Destroy(r.From)
	FfiDestroyerEntry{}.Destroy(r.Entry)
	FfiDestroyerTypeContentStatus{}.Destroy(r.ContentStatus)
}

type FfiConverterTypeInsertRemoteEvent struct{}

var FfiConverterTypeInsertRemoteEventINSTANCE = FfiConverterTypeInsertRemoteEvent{}

func (c FfiConverterTypeInsertRemoteEvent) Lift(rb RustBufferI) InsertRemoteEvent {
	return LiftFromRustBuffer[InsertRemoteEvent](c, rb)
}

func (c FfiConverterTypeInsertRemoteEvent) Read(reader io.Reader) InsertRemoteEvent {
	return InsertRemoteEvent{
		FfiConverterPublicKeyINSTANCE.Read(reader),
		FfiConverterEntryINSTANCE.Read(reader),
		FfiConverterTypeContentStatusINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeInsertRemoteEvent) Lower(value InsertRemoteEvent) RustBuffer {
	return LowerIntoRustBuffer[InsertRemoteEvent](c, value)
}

func (c FfiConverterTypeInsertRemoteEvent) Write(writer io.Writer, value InsertRemoteEvent) {
	FfiConverterPublicKeyINSTANCE.Write(writer, value.From)
	FfiConverterEntryINSTANCE.Write(writer, value.Entry)
	FfiConverterTypeContentStatusINSTANCE.Write(writer, value.ContentStatus)
}

type FfiDestroyerTypeInsertRemoteEvent struct{}

func (_ FfiDestroyerTypeInsertRemoteEvent) Destroy(value InsertRemoteEvent) {
	value.Destroy()
}

type LatencyAndControlMsg struct {
	Latency    time.Duration
	ControlMsg string
}

func (r *LatencyAndControlMsg) Destroy() {
	FfiDestroyerDuration{}.Destroy(r.Latency)
	FfiDestroyerString{}.Destroy(r.ControlMsg)
}

type FfiConverterTypeLatencyAndControlMsg struct{}

var FfiConverterTypeLatencyAndControlMsgINSTANCE = FfiConverterTypeLatencyAndControlMsg{}

func (c FfiConverterTypeLatencyAndControlMsg) Lift(rb RustBufferI) LatencyAndControlMsg {
	return LiftFromRustBuffer[LatencyAndControlMsg](c, rb)
}

func (c FfiConverterTypeLatencyAndControlMsg) Read(reader io.Reader) LatencyAndControlMsg {
	return LatencyAndControlMsg{
		FfiConverterDurationINSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeLatencyAndControlMsg) Lower(value LatencyAndControlMsg) RustBuffer {
	return LowerIntoRustBuffer[LatencyAndControlMsg](c, value)
}

func (c FfiConverterTypeLatencyAndControlMsg) Write(writer io.Writer, value LatencyAndControlMsg) {
	FfiConverterDurationINSTANCE.Write(writer, value.Latency)
	FfiConverterStringINSTANCE.Write(writer, value.ControlMsg)
}

type FfiDestroyerTypeLatencyAndControlMsg struct{}

func (_ FfiDestroyerTypeLatencyAndControlMsg) Destroy(value LatencyAndControlMsg) {
	value.Destroy()
}

type ListTagsResponse struct {
	Name   *Tag
	Format BlobFormat
	Hash   *Hash
}

func (r *ListTagsResponse) Destroy() {
	FfiDestroyerTag{}.Destroy(r.Name)
	FfiDestroyerTypeBlobFormat{}.Destroy(r.Format)
	FfiDestroyerHash{}.Destroy(r.Hash)
}

type FfiConverterTypeListTagsResponse struct{}

var FfiConverterTypeListTagsResponseINSTANCE = FfiConverterTypeListTagsResponse{}

func (c FfiConverterTypeListTagsResponse) Lift(rb RustBufferI) ListTagsResponse {
	return LiftFromRustBuffer[ListTagsResponse](c, rb)
}

func (c FfiConverterTypeListTagsResponse) Read(reader io.Reader) ListTagsResponse {
	return ListTagsResponse{
		FfiConverterTagINSTANCE.Read(reader),
		FfiConverterTypeBlobFormatINSTANCE.Read(reader),
		FfiConverterHashINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeListTagsResponse) Lower(value ListTagsResponse) RustBuffer {
	return LowerIntoRustBuffer[ListTagsResponse](c, value)
}

func (c FfiConverterTypeListTagsResponse) Write(writer io.Writer, value ListTagsResponse) {
	FfiConverterTagINSTANCE.Write(writer, value.Name)
	FfiConverterTypeBlobFormatINSTANCE.Write(writer, value.Format)
	FfiConverterHashINSTANCE.Write(writer, value.Hash)
}

type FfiDestroyerTypeListTagsResponse struct{}

func (_ FfiDestroyerTypeListTagsResponse) Destroy(value ListTagsResponse) {
	value.Destroy()
}

type NamespaceAndCapability struct {
	Namespace  *NamespaceId
	Capability CapabilityKind
}

func (r *NamespaceAndCapability) Destroy() {
	FfiDestroyerNamespaceId{}.Destroy(r.Namespace)
	FfiDestroyerTypeCapabilityKind{}.Destroy(r.Capability)
}

type FfiConverterTypeNamespaceAndCapability struct{}

var FfiConverterTypeNamespaceAndCapabilityINSTANCE = FfiConverterTypeNamespaceAndCapability{}

func (c FfiConverterTypeNamespaceAndCapability) Lift(rb RustBufferI) NamespaceAndCapability {
	return LiftFromRustBuffer[NamespaceAndCapability](c, rb)
}

func (c FfiConverterTypeNamespaceAndCapability) Read(reader io.Reader) NamespaceAndCapability {
	return NamespaceAndCapability{
		FfiConverterNamespaceIdINSTANCE.Read(reader),
		FfiConverterTypeCapabilityKindINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeNamespaceAndCapability) Lower(value NamespaceAndCapability) RustBuffer {
	return LowerIntoRustBuffer[NamespaceAndCapability](c, value)
}

func (c FfiConverterTypeNamespaceAndCapability) Write(writer io.Writer, value NamespaceAndCapability) {
	FfiConverterNamespaceIdINSTANCE.Write(writer, value.Namespace)
	FfiConverterTypeCapabilityKindINSTANCE.Write(writer, value.Capability)
}

type FfiDestroyerTypeNamespaceAndCapability struct{}

func (_ FfiDestroyerTypeNamespaceAndCapability) Destroy(value NamespaceAndCapability) {
	value.Destroy()
}

type OpenState struct {
	Sync        bool
	Subscribers uint64
	Handles     uint64
}

func (r *OpenState) Destroy() {
	FfiDestroyerBool{}.Destroy(r.Sync)
	FfiDestroyerUint64{}.Destroy(r.Subscribers)
	FfiDestroyerUint64{}.Destroy(r.Handles)
}

type FfiConverterTypeOpenState struct{}

var FfiConverterTypeOpenStateINSTANCE = FfiConverterTypeOpenState{}

func (c FfiConverterTypeOpenState) Lift(rb RustBufferI) OpenState {
	return LiftFromRustBuffer[OpenState](c, rb)
}

func (c FfiConverterTypeOpenState) Read(reader io.Reader) OpenState {
	return OpenState{
		FfiConverterBoolINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeOpenState) Lower(value OpenState) RustBuffer {
	return LowerIntoRustBuffer[OpenState](c, value)
}

func (c FfiConverterTypeOpenState) Write(writer io.Writer, value OpenState) {
	FfiConverterBoolINSTANCE.Write(writer, value.Sync)
	FfiConverterUint64INSTANCE.Write(writer, value.Subscribers)
	FfiConverterUint64INSTANCE.Write(writer, value.Handles)
}

type FfiDestroyerTypeOpenState struct{}

func (_ FfiDestroyerTypeOpenState) Destroy(value OpenState) {
	value.Destroy()
}

type QueryOptions struct {
	SortBy    SortBy
	Direction SortDirection
	Offset    uint64
	Limit     uint64
}

func (r *QueryOptions) Destroy() {
	FfiDestroyerTypeSortBy{}.Destroy(r.SortBy)
	FfiDestroyerTypeSortDirection{}.Destroy(r.Direction)
	FfiDestroyerUint64{}.Destroy(r.Offset)
	FfiDestroyerUint64{}.Destroy(r.Limit)
}

type FfiConverterTypeQueryOptions struct{}

var FfiConverterTypeQueryOptionsINSTANCE = FfiConverterTypeQueryOptions{}

func (c FfiConverterTypeQueryOptions) Lift(rb RustBufferI) QueryOptions {
	return LiftFromRustBuffer[QueryOptions](c, rb)
}

func (c FfiConverterTypeQueryOptions) Read(reader io.Reader) QueryOptions {
	return QueryOptions{
		FfiConverterTypeSortByINSTANCE.Read(reader),
		FfiConverterTypeSortDirectionINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeQueryOptions) Lower(value QueryOptions) RustBuffer {
	return LowerIntoRustBuffer[QueryOptions](c, value)
}

func (c FfiConverterTypeQueryOptions) Write(writer io.Writer, value QueryOptions) {
	FfiConverterTypeSortByINSTANCE.Write(writer, value.SortBy)
	FfiConverterTypeSortDirectionINSTANCE.Write(writer, value.Direction)
	FfiConverterUint64INSTANCE.Write(writer, value.Offset)
	FfiConverterUint64INSTANCE.Write(writer, value.Limit)
}

type FfiDestroyerTypeQueryOptions struct{}

func (_ FfiDestroyerTypeQueryOptions) Destroy(value QueryOptions) {
	value.Destroy()
}

type SyncEvent struct {
	Peer     *PublicKey
	Origin   Origin
	Started  time.Time
	Finished time.Time
	Result   *string
}

func (r *SyncEvent) Destroy() {
	FfiDestroyerPublicKey{}.Destroy(r.Peer)
	FfiDestroyerTypeOrigin{}.Destroy(r.Origin)
	FfiDestroyerTimestamp{}.Destroy(r.Started)
	FfiDestroyerTimestamp{}.Destroy(r.Finished)
	FfiDestroyerOptionalString{}.Destroy(r.Result)
}

type FfiConverterTypeSyncEvent struct{}

var FfiConverterTypeSyncEventINSTANCE = FfiConverterTypeSyncEvent{}

func (c FfiConverterTypeSyncEvent) Lift(rb RustBufferI) SyncEvent {
	return LiftFromRustBuffer[SyncEvent](c, rb)
}

func (c FfiConverterTypeSyncEvent) Read(reader io.Reader) SyncEvent {
	return SyncEvent{
		FfiConverterPublicKeyINSTANCE.Read(reader),
		FfiConverterTypeOriginINSTANCE.Read(reader),
		FfiConverterTimestampINSTANCE.Read(reader),
		FfiConverterTimestampINSTANCE.Read(reader),
		FfiConverterOptionalStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeSyncEvent) Lower(value SyncEvent) RustBuffer {
	return LowerIntoRustBuffer[SyncEvent](c, value)
}

func (c FfiConverterTypeSyncEvent) Write(writer io.Writer, value SyncEvent) {
	FfiConverterPublicKeyINSTANCE.Write(writer, value.Peer)
	FfiConverterTypeOriginINSTANCE.Write(writer, value.Origin)
	FfiConverterTimestampINSTANCE.Write(writer, value.Started)
	FfiConverterTimestampINSTANCE.Write(writer, value.Finished)
	FfiConverterOptionalStringINSTANCE.Write(writer, value.Result)
}

type FfiDestroyerTypeSyncEvent struct{}

func (_ FfiDestroyerTypeSyncEvent) Destroy(value SyncEvent) {
	value.Destroy()
}

type AddProgressType uint

const (
	AddProgressTypeFound    AddProgressType = 1
	AddProgressTypeProgress AddProgressType = 2
	AddProgressTypeDone     AddProgressType = 3
	AddProgressTypeAllDone  AddProgressType = 4
	AddProgressTypeAbort    AddProgressType = 5
)

type FfiConverterTypeAddProgressType struct{}

var FfiConverterTypeAddProgressTypeINSTANCE = FfiConverterTypeAddProgressType{}

func (c FfiConverterTypeAddProgressType) Lift(rb RustBufferI) AddProgressType {
	return LiftFromRustBuffer[AddProgressType](c, rb)
}

func (c FfiConverterTypeAddProgressType) Lower(value AddProgressType) RustBuffer {
	return LowerIntoRustBuffer[AddProgressType](c, value)
}
func (FfiConverterTypeAddProgressType) Read(reader io.Reader) AddProgressType {
	id := readInt32(reader)
	return AddProgressType(id)
}

func (FfiConverterTypeAddProgressType) Write(writer io.Writer, value AddProgressType) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeAddProgressType struct{}

func (_ FfiDestroyerTypeAddProgressType) Destroy(value AddProgressType) {
}

type BlobFormat uint

const (
	BlobFormatRaw     BlobFormat = 1
	BlobFormatHashSeq BlobFormat = 2
)

type FfiConverterTypeBlobFormat struct{}

var FfiConverterTypeBlobFormatINSTANCE = FfiConverterTypeBlobFormat{}

func (c FfiConverterTypeBlobFormat) Lift(rb RustBufferI) BlobFormat {
	return LiftFromRustBuffer[BlobFormat](c, rb)
}

func (c FfiConverterTypeBlobFormat) Lower(value BlobFormat) RustBuffer {
	return LowerIntoRustBuffer[BlobFormat](c, value)
}
func (FfiConverterTypeBlobFormat) Read(reader io.Reader) BlobFormat {
	id := readInt32(reader)
	return BlobFormat(id)
}

func (FfiConverterTypeBlobFormat) Write(writer io.Writer, value BlobFormat) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeBlobFormat struct{}

func (_ FfiDestroyerTypeBlobFormat) Destroy(value BlobFormat) {
}

type CapabilityKind uint

const (
	CapabilityKindWrite CapabilityKind = 1
	CapabilityKindRead  CapabilityKind = 2
)

type FfiConverterTypeCapabilityKind struct{}

var FfiConverterTypeCapabilityKindINSTANCE = FfiConverterTypeCapabilityKind{}

func (c FfiConverterTypeCapabilityKind) Lift(rb RustBufferI) CapabilityKind {
	return LiftFromRustBuffer[CapabilityKind](c, rb)
}

func (c FfiConverterTypeCapabilityKind) Lower(value CapabilityKind) RustBuffer {
	return LowerIntoRustBuffer[CapabilityKind](c, value)
}
func (FfiConverterTypeCapabilityKind) Read(reader io.Reader) CapabilityKind {
	id := readInt32(reader)
	return CapabilityKind(id)
}

func (FfiConverterTypeCapabilityKind) Write(writer io.Writer, value CapabilityKind) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeCapabilityKind struct{}

func (_ FfiDestroyerTypeCapabilityKind) Destroy(value CapabilityKind) {
}

type ConnType uint

const (
	ConnTypeDirect ConnType = 1
	ConnTypeRelay  ConnType = 2
	ConnTypeMixed  ConnType = 3
	ConnTypeNone   ConnType = 4
)

type FfiConverterTypeConnType struct{}

var FfiConverterTypeConnTypeINSTANCE = FfiConverterTypeConnType{}

func (c FfiConverterTypeConnType) Lift(rb RustBufferI) ConnType {
	return LiftFromRustBuffer[ConnType](c, rb)
}

func (c FfiConverterTypeConnType) Lower(value ConnType) RustBuffer {
	return LowerIntoRustBuffer[ConnType](c, value)
}
func (FfiConverterTypeConnType) Read(reader io.Reader) ConnType {
	id := readInt32(reader)
	return ConnType(id)
}

func (FfiConverterTypeConnType) Write(writer io.Writer, value ConnType) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeConnType struct{}

func (_ FfiDestroyerTypeConnType) Destroy(value ConnType) {
}

type ContentStatus uint

const (
	ContentStatusComplete   ContentStatus = 1
	ContentStatusIncomplete ContentStatus = 2
	ContentStatusMissing    ContentStatus = 3
)

type FfiConverterTypeContentStatus struct{}

var FfiConverterTypeContentStatusINSTANCE = FfiConverterTypeContentStatus{}

func (c FfiConverterTypeContentStatus) Lift(rb RustBufferI) ContentStatus {
	return LiftFromRustBuffer[ContentStatus](c, rb)
}

func (c FfiConverterTypeContentStatus) Lower(value ContentStatus) RustBuffer {
	return LowerIntoRustBuffer[ContentStatus](c, value)
}
func (FfiConverterTypeContentStatus) Read(reader io.Reader) ContentStatus {
	id := readInt32(reader)
	return ContentStatus(id)
}

func (FfiConverterTypeContentStatus) Write(writer io.Writer, value ContentStatus) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeContentStatus struct{}

func (_ FfiDestroyerTypeContentStatus) Destroy(value ContentStatus) {
}

type DocExportProgressType uint

const (
	DocExportProgressTypeFound    DocExportProgressType = 1
	DocExportProgressTypeProgress DocExportProgressType = 2
	DocExportProgressTypeAllDone  DocExportProgressType = 3
	DocExportProgressTypeAbort    DocExportProgressType = 4
)

type FfiConverterTypeDocExportProgressType struct{}

var FfiConverterTypeDocExportProgressTypeINSTANCE = FfiConverterTypeDocExportProgressType{}

func (c FfiConverterTypeDocExportProgressType) Lift(rb RustBufferI) DocExportProgressType {
	return LiftFromRustBuffer[DocExportProgressType](c, rb)
}

func (c FfiConverterTypeDocExportProgressType) Lower(value DocExportProgressType) RustBuffer {
	return LowerIntoRustBuffer[DocExportProgressType](c, value)
}
func (FfiConverterTypeDocExportProgressType) Read(reader io.Reader) DocExportProgressType {
	id := readInt32(reader)
	return DocExportProgressType(id)
}

func (FfiConverterTypeDocExportProgressType) Write(writer io.Writer, value DocExportProgressType) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeDocExportProgressType struct{}

func (_ FfiDestroyerTypeDocExportProgressType) Destroy(value DocExportProgressType) {
}

type DocImportProgressType uint

const (
	DocImportProgressTypeFound      DocImportProgressType = 1
	DocImportProgressTypeProgress   DocImportProgressType = 2
	DocImportProgressTypeIngestDone DocImportProgressType = 3
	DocImportProgressTypeAllDone    DocImportProgressType = 4
	DocImportProgressTypeAbort      DocImportProgressType = 5
)

type FfiConverterTypeDocImportProgressType struct{}

var FfiConverterTypeDocImportProgressTypeINSTANCE = FfiConverterTypeDocImportProgressType{}

func (c FfiConverterTypeDocImportProgressType) Lift(rb RustBufferI) DocImportProgressType {
	return LiftFromRustBuffer[DocImportProgressType](c, rb)
}

func (c FfiConverterTypeDocImportProgressType) Lower(value DocImportProgressType) RustBuffer {
	return LowerIntoRustBuffer[DocImportProgressType](c, value)
}
func (FfiConverterTypeDocImportProgressType) Read(reader io.Reader) DocImportProgressType {
	id := readInt32(reader)
	return DocImportProgressType(id)
}

func (FfiConverterTypeDocImportProgressType) Write(writer io.Writer, value DocImportProgressType) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeDocImportProgressType struct{}

func (_ FfiDestroyerTypeDocImportProgressType) Destroy(value DocImportProgressType) {
}

type DownloadProgressType uint

const (
	DownloadProgressTypeFoundLocal     DownloadProgressType = 1
	DownloadProgressTypeConnected      DownloadProgressType = 2
	DownloadProgressTypeFound          DownloadProgressType = 3
	DownloadProgressTypeFoundHashSeq   DownloadProgressType = 4
	DownloadProgressTypeProgress       DownloadProgressType = 5
	DownloadProgressTypeDone           DownloadProgressType = 6
	DownloadProgressTypeNetworkDone    DownloadProgressType = 7
	DownloadProgressTypeExport         DownloadProgressType = 8
	DownloadProgressTypeExportProgress DownloadProgressType = 9
	DownloadProgressTypeAllDone        DownloadProgressType = 10
	DownloadProgressTypeAbort          DownloadProgressType = 11
)

type FfiConverterTypeDownloadProgressType struct{}

var FfiConverterTypeDownloadProgressTypeINSTANCE = FfiConverterTypeDownloadProgressType{}

func (c FfiConverterTypeDownloadProgressType) Lift(rb RustBufferI) DownloadProgressType {
	return LiftFromRustBuffer[DownloadProgressType](c, rb)
}

func (c FfiConverterTypeDownloadProgressType) Lower(value DownloadProgressType) RustBuffer {
	return LowerIntoRustBuffer[DownloadProgressType](c, value)
}
func (FfiConverterTypeDownloadProgressType) Read(reader io.Reader) DownloadProgressType {
	id := readInt32(reader)
	return DownloadProgressType(id)
}

func (FfiConverterTypeDownloadProgressType) Write(writer io.Writer, value DownloadProgressType) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeDownloadProgressType struct{}

func (_ FfiDestroyerTypeDownloadProgressType) Destroy(value DownloadProgressType) {
}

type IrohError struct {
	err error
}

func (err IrohError) Error() string {
	return fmt.Sprintf("IrohError: %s", err.err.Error())
}

func (err IrohError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrIrohErrorRuntime = fmt.Errorf("IrohErrorRuntime")
var ErrIrohErrorNodeCreate = fmt.Errorf("IrohErrorNodeCreate")
var ErrIrohErrorDoc = fmt.Errorf("IrohErrorDoc")
var ErrIrohErrorAuthor = fmt.Errorf("IrohErrorAuthor")
var ErrIrohErrorNamespace = fmt.Errorf("IrohErrorNamespace")
var ErrIrohErrorDocTicket = fmt.Errorf("IrohErrorDocTicket")
var ErrIrohErrorUniffi = fmt.Errorf("IrohErrorUniffi")
var ErrIrohErrorConnection = fmt.Errorf("IrohErrorConnection")
var ErrIrohErrorBlobs = fmt.Errorf("IrohErrorBlobs")
var ErrIrohErrorIpv4Addr = fmt.Errorf("IrohErrorIpv4Addr")
var ErrIrohErrorIpv6Addr = fmt.Errorf("IrohErrorIpv6Addr")
var ErrIrohErrorSocketAddrV4 = fmt.Errorf("IrohErrorSocketAddrV4")
var ErrIrohErrorSocketAddrV6 = fmt.Errorf("IrohErrorSocketAddrV6")
var ErrIrohErrorPublicKey = fmt.Errorf("IrohErrorPublicKey")
var ErrIrohErrorNodeAddr = fmt.Errorf("IrohErrorNodeAddr")
var ErrIrohErrorHash = fmt.Errorf("IrohErrorHash")
var ErrIrohErrorFsUtil = fmt.Errorf("IrohErrorFsUtil")
var ErrIrohErrorTags = fmt.Errorf("IrohErrorTags")
var ErrIrohErrorUrl = fmt.Errorf("IrohErrorUrl")
var ErrIrohErrorEntry = fmt.Errorf("IrohErrorEntry")

// Variant structs
type IrohErrorRuntime struct {
	Description string
}

func NewIrohErrorRuntime(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorRuntime{
			Description: description,
		},
	}
}

func (err IrohErrorRuntime) Error() string {
	return fmt.Sprint("Runtime",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorRuntime) Is(target error) bool {
	return target == ErrIrohErrorRuntime
}

type IrohErrorNodeCreate struct {
	Description string
}

func NewIrohErrorNodeCreate(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorNodeCreate{
			Description: description,
		},
	}
}

func (err IrohErrorNodeCreate) Error() string {
	return fmt.Sprint("NodeCreate",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorNodeCreate) Is(target error) bool {
	return target == ErrIrohErrorNodeCreate
}

type IrohErrorDoc struct {
	Description string
}

func NewIrohErrorDoc(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorDoc{
			Description: description,
		},
	}
}

func (err IrohErrorDoc) Error() string {
	return fmt.Sprint("Doc",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorDoc) Is(target error) bool {
	return target == ErrIrohErrorDoc
}

type IrohErrorAuthor struct {
	Description string
}

func NewIrohErrorAuthor(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorAuthor{
			Description: description,
		},
	}
}

func (err IrohErrorAuthor) Error() string {
	return fmt.Sprint("Author",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorAuthor) Is(target error) bool {
	return target == ErrIrohErrorAuthor
}

type IrohErrorNamespace struct {
	Description string
}

func NewIrohErrorNamespace(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorNamespace{
			Description: description,
		},
	}
}

func (err IrohErrorNamespace) Error() string {
	return fmt.Sprint("Namespace",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorNamespace) Is(target error) bool {
	return target == ErrIrohErrorNamespace
}

type IrohErrorDocTicket struct {
	Description string
}

func NewIrohErrorDocTicket(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorDocTicket{
			Description: description,
		},
	}
}

func (err IrohErrorDocTicket) Error() string {
	return fmt.Sprint("DocTicket",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorDocTicket) Is(target error) bool {
	return target == ErrIrohErrorDocTicket
}

type IrohErrorUniffi struct {
	Description string
}

func NewIrohErrorUniffi(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorUniffi{
			Description: description,
		},
	}
}

func (err IrohErrorUniffi) Error() string {
	return fmt.Sprint("Uniffi",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorUniffi) Is(target error) bool {
	return target == ErrIrohErrorUniffi
}

type IrohErrorConnection struct {
	Description string
}

func NewIrohErrorConnection(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorConnection{
			Description: description,
		},
	}
}

func (err IrohErrorConnection) Error() string {
	return fmt.Sprint("Connection",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorConnection) Is(target error) bool {
	return target == ErrIrohErrorConnection
}

type IrohErrorBlobs struct {
	Description string
}

func NewIrohErrorBlobs(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorBlobs{
			Description: description,
		},
	}
}

func (err IrohErrorBlobs) Error() string {
	return fmt.Sprint("Blobs",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorBlobs) Is(target error) bool {
	return target == ErrIrohErrorBlobs
}

type IrohErrorIpv4Addr struct {
	Description string
}

func NewIrohErrorIpv4Addr(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorIpv4Addr{
			Description: description,
		},
	}
}

func (err IrohErrorIpv4Addr) Error() string {
	return fmt.Sprint("Ipv4Addr",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorIpv4Addr) Is(target error) bool {
	return target == ErrIrohErrorIpv4Addr
}

type IrohErrorIpv6Addr struct {
	Description string
}

func NewIrohErrorIpv6Addr(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorIpv6Addr{
			Description: description,
		},
	}
}

func (err IrohErrorIpv6Addr) Error() string {
	return fmt.Sprint("Ipv6Addr",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorIpv6Addr) Is(target error) bool {
	return target == ErrIrohErrorIpv6Addr
}

type IrohErrorSocketAddrV4 struct {
	Description string
}

func NewIrohErrorSocketAddrV4(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorSocketAddrV4{
			Description: description,
		},
	}
}

func (err IrohErrorSocketAddrV4) Error() string {
	return fmt.Sprint("SocketAddrV4",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorSocketAddrV4) Is(target error) bool {
	return target == ErrIrohErrorSocketAddrV4
}

type IrohErrorSocketAddrV6 struct {
	Description string
}

func NewIrohErrorSocketAddrV6(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorSocketAddrV6{
			Description: description,
		},
	}
}

func (err IrohErrorSocketAddrV6) Error() string {
	return fmt.Sprint("SocketAddrV6",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorSocketAddrV6) Is(target error) bool {
	return target == ErrIrohErrorSocketAddrV6
}

type IrohErrorPublicKey struct {
	Description string
}

func NewIrohErrorPublicKey(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorPublicKey{
			Description: description,
		},
	}
}

func (err IrohErrorPublicKey) Error() string {
	return fmt.Sprint("PublicKey",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorPublicKey) Is(target error) bool {
	return target == ErrIrohErrorPublicKey
}

type IrohErrorNodeAddr struct {
	Description string
}

func NewIrohErrorNodeAddr(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorNodeAddr{
			Description: description,
		},
	}
}

func (err IrohErrorNodeAddr) Error() string {
	return fmt.Sprint("NodeAddr",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorNodeAddr) Is(target error) bool {
	return target == ErrIrohErrorNodeAddr
}

type IrohErrorHash struct {
	Description string
}

func NewIrohErrorHash(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorHash{
			Description: description,
		},
	}
}

func (err IrohErrorHash) Error() string {
	return fmt.Sprint("Hash",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorHash) Is(target error) bool {
	return target == ErrIrohErrorHash
}

type IrohErrorFsUtil struct {
	Description string
}

func NewIrohErrorFsUtil(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorFsUtil{
			Description: description,
		},
	}
}

func (err IrohErrorFsUtil) Error() string {
	return fmt.Sprint("FsUtil",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorFsUtil) Is(target error) bool {
	return target == ErrIrohErrorFsUtil
}

type IrohErrorTags struct {
	Description string
}

func NewIrohErrorTags(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorTags{
			Description: description,
		},
	}
}

func (err IrohErrorTags) Error() string {
	return fmt.Sprint("Tags",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorTags) Is(target error) bool {
	return target == ErrIrohErrorTags
}

type IrohErrorUrl struct {
	Description string
}

func NewIrohErrorUrl(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorUrl{
			Description: description,
		},
	}
}

func (err IrohErrorUrl) Error() string {
	return fmt.Sprint("Url",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorUrl) Is(target error) bool {
	return target == ErrIrohErrorUrl
}

type IrohErrorEntry struct {
	Description string
}

func NewIrohErrorEntry(
	description string,
) *IrohError {
	return &IrohError{
		err: &IrohErrorEntry{
			Description: description,
		},
	}
}

func (err IrohErrorEntry) Error() string {
	return fmt.Sprint("Entry",
		": ",

		"Description=",
		err.Description,
	)
}

func (self IrohErrorEntry) Is(target error) bool {
	return target == ErrIrohErrorEntry
}

type FfiConverterTypeIrohError struct{}

var FfiConverterTypeIrohErrorINSTANCE = FfiConverterTypeIrohError{}

func (c FfiConverterTypeIrohError) Lift(eb RustBufferI) error {
	return LiftFromRustBuffer[error](c, eb)
}

func (c FfiConverterTypeIrohError) Lower(value *IrohError) RustBuffer {
	return LowerIntoRustBuffer[*IrohError](c, value)
}

func (c FfiConverterTypeIrohError) Read(reader io.Reader) error {
	errorID := readUint32(reader)

	switch errorID {
	case 1:
		return &IrohError{&IrohErrorRuntime{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 2:
		return &IrohError{&IrohErrorNodeCreate{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 3:
		return &IrohError{&IrohErrorDoc{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 4:
		return &IrohError{&IrohErrorAuthor{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 5:
		return &IrohError{&IrohErrorNamespace{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 6:
		return &IrohError{&IrohErrorDocTicket{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 7:
		return &IrohError{&IrohErrorUniffi{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 8:
		return &IrohError{&IrohErrorConnection{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 9:
		return &IrohError{&IrohErrorBlobs{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 10:
		return &IrohError{&IrohErrorIpv4Addr{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 11:
		return &IrohError{&IrohErrorIpv6Addr{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 12:
		return &IrohError{&IrohErrorSocketAddrV4{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 13:
		return &IrohError{&IrohErrorSocketAddrV6{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 14:
		return &IrohError{&IrohErrorPublicKey{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 15:
		return &IrohError{&IrohErrorNodeAddr{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 16:
		return &IrohError{&IrohErrorHash{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 17:
		return &IrohError{&IrohErrorFsUtil{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 18:
		return &IrohError{&IrohErrorTags{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 19:
		return &IrohError{&IrohErrorUrl{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	case 20:
		return &IrohError{&IrohErrorEntry{
			Description: FfiConverterStringINSTANCE.Read(reader),
		}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterTypeIrohError.Read()", errorID))
	}
}

func (c FfiConverterTypeIrohError) Write(writer io.Writer, value *IrohError) {
	switch variantValue := value.err.(type) {
	case *IrohErrorRuntime:
		writeInt32(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorNodeCreate:
		writeInt32(writer, 2)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorDoc:
		writeInt32(writer, 3)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorAuthor:
		writeInt32(writer, 4)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorNamespace:
		writeInt32(writer, 5)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorDocTicket:
		writeInt32(writer, 6)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorUniffi:
		writeInt32(writer, 7)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorConnection:
		writeInt32(writer, 8)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorBlobs:
		writeInt32(writer, 9)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorIpv4Addr:
		writeInt32(writer, 10)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorIpv6Addr:
		writeInt32(writer, 11)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorSocketAddrV4:
		writeInt32(writer, 12)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorSocketAddrV6:
		writeInt32(writer, 13)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorPublicKey:
		writeInt32(writer, 14)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorNodeAddr:
		writeInt32(writer, 15)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorHash:
		writeInt32(writer, 16)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorFsUtil:
		writeInt32(writer, 17)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorTags:
		writeInt32(writer, 18)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorUrl:
		writeInt32(writer, 19)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	case *IrohErrorEntry:
		writeInt32(writer, 20)
		FfiConverterStringINSTANCE.Write(writer, variantValue.Description)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterTypeIrohError.Write", value))
	}
}

type LiveEventType uint

const (
	LiveEventTypeInsertLocal  LiveEventType = 1
	LiveEventTypeInsertRemote LiveEventType = 2
	LiveEventTypeContentReady LiveEventType = 3
	LiveEventTypeNeighborUp   LiveEventType = 4
	LiveEventTypeNeighborDown LiveEventType = 5
	LiveEventTypeSyncFinished LiveEventType = 6
)

type FfiConverterTypeLiveEventType struct{}

var FfiConverterTypeLiveEventTypeINSTANCE = FfiConverterTypeLiveEventType{}

func (c FfiConverterTypeLiveEventType) Lift(rb RustBufferI) LiveEventType {
	return LiftFromRustBuffer[LiveEventType](c, rb)
}

func (c FfiConverterTypeLiveEventType) Lower(value LiveEventType) RustBuffer {
	return LowerIntoRustBuffer[LiveEventType](c, value)
}
func (FfiConverterTypeLiveEventType) Read(reader io.Reader) LiveEventType {
	id := readInt32(reader)
	return LiveEventType(id)
}

func (FfiConverterTypeLiveEventType) Write(writer io.Writer, value LiveEventType) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeLiveEventType struct{}

func (_ FfiDestroyerTypeLiveEventType) Destroy(value LiveEventType) {
}

type LogLevel uint

const (
	LogLevelTrace LogLevel = 1
	LogLevelDebug LogLevel = 2
	LogLevelInfo  LogLevel = 3
	LogLevelWarn  LogLevel = 4
	LogLevelError LogLevel = 5
	LogLevelOff   LogLevel = 6
)

type FfiConverterTypeLogLevel struct{}

var FfiConverterTypeLogLevelINSTANCE = FfiConverterTypeLogLevel{}

func (c FfiConverterTypeLogLevel) Lift(rb RustBufferI) LogLevel {
	return LiftFromRustBuffer[LogLevel](c, rb)
}

func (c FfiConverterTypeLogLevel) Lower(value LogLevel) RustBuffer {
	return LowerIntoRustBuffer[LogLevel](c, value)
}
func (FfiConverterTypeLogLevel) Read(reader io.Reader) LogLevel {
	id := readInt32(reader)
	return LogLevel(id)
}

func (FfiConverterTypeLogLevel) Write(writer io.Writer, value LogLevel) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeLogLevel struct{}

func (_ FfiDestroyerTypeLogLevel) Destroy(value LogLevel) {
}

type Origin interface {
	Destroy()
}
type OriginConnect struct {
	Reason SyncReason
}

func (e OriginConnect) Destroy() {
	FfiDestroyerTypeSyncReason{}.Destroy(e.Reason)
}

type OriginAccept struct {
}

func (e OriginAccept) Destroy() {
}

type FfiConverterTypeOrigin struct{}

var FfiConverterTypeOriginINSTANCE = FfiConverterTypeOrigin{}

func (c FfiConverterTypeOrigin) Lift(rb RustBufferI) Origin {
	return LiftFromRustBuffer[Origin](c, rb)
}

func (c FfiConverterTypeOrigin) Lower(value Origin) RustBuffer {
	return LowerIntoRustBuffer[Origin](c, value)
}
func (FfiConverterTypeOrigin) Read(reader io.Reader) Origin {
	id := readInt32(reader)
	switch id {
	case 1:
		return OriginConnect{
			FfiConverterTypeSyncReasonINSTANCE.Read(reader),
		}
	case 2:
		return OriginAccept{}
	default:
		panic(fmt.Sprintf("invalid enum value %v in FfiConverterTypeOrigin.Read()", id))
	}
}

func (FfiConverterTypeOrigin) Write(writer io.Writer, value Origin) {
	switch variant_value := value.(type) {
	case OriginConnect:
		writeInt32(writer, 1)
		FfiConverterTypeSyncReasonINSTANCE.Write(writer, variant_value.Reason)
	case OriginAccept:
		writeInt32(writer, 2)
	default:
		_ = variant_value
		panic(fmt.Sprintf("invalid enum value `%v` in FfiConverterTypeOrigin.Write", value))
	}
}

type FfiDestroyerTypeOrigin struct{}

func (_ FfiDestroyerTypeOrigin) Destroy(value Origin) {
	value.Destroy()
}

type ShareMode uint

const (
	ShareModeRead  ShareMode = 1
	ShareModeWrite ShareMode = 2
)

type FfiConverterTypeShareMode struct{}

var FfiConverterTypeShareModeINSTANCE = FfiConverterTypeShareMode{}

func (c FfiConverterTypeShareMode) Lift(rb RustBufferI) ShareMode {
	return LiftFromRustBuffer[ShareMode](c, rb)
}

func (c FfiConverterTypeShareMode) Lower(value ShareMode) RustBuffer {
	return LowerIntoRustBuffer[ShareMode](c, value)
}
func (FfiConverterTypeShareMode) Read(reader io.Reader) ShareMode {
	id := readInt32(reader)
	return ShareMode(id)
}

func (FfiConverterTypeShareMode) Write(writer io.Writer, value ShareMode) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeShareMode struct{}

func (_ FfiDestroyerTypeShareMode) Destroy(value ShareMode) {
}

type SocketAddrType uint

const (
	SocketAddrTypeV4 SocketAddrType = 1
	SocketAddrTypeV6 SocketAddrType = 2
)

type FfiConverterTypeSocketAddrType struct{}

var FfiConverterTypeSocketAddrTypeINSTANCE = FfiConverterTypeSocketAddrType{}

func (c FfiConverterTypeSocketAddrType) Lift(rb RustBufferI) SocketAddrType {
	return LiftFromRustBuffer[SocketAddrType](c, rb)
}

func (c FfiConverterTypeSocketAddrType) Lower(value SocketAddrType) RustBuffer {
	return LowerIntoRustBuffer[SocketAddrType](c, value)
}
func (FfiConverterTypeSocketAddrType) Read(reader io.Reader) SocketAddrType {
	id := readInt32(reader)
	return SocketAddrType(id)
}

func (FfiConverterTypeSocketAddrType) Write(writer io.Writer, value SocketAddrType) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeSocketAddrType struct{}

func (_ FfiDestroyerTypeSocketAddrType) Destroy(value SocketAddrType) {
}

type SortBy uint

const (
	SortByKeyAuthor SortBy = 1
	SortByAuthorKey SortBy = 2
)

type FfiConverterTypeSortBy struct{}

var FfiConverterTypeSortByINSTANCE = FfiConverterTypeSortBy{}

func (c FfiConverterTypeSortBy) Lift(rb RustBufferI) SortBy {
	return LiftFromRustBuffer[SortBy](c, rb)
}

func (c FfiConverterTypeSortBy) Lower(value SortBy) RustBuffer {
	return LowerIntoRustBuffer[SortBy](c, value)
}
func (FfiConverterTypeSortBy) Read(reader io.Reader) SortBy {
	id := readInt32(reader)
	return SortBy(id)
}

func (FfiConverterTypeSortBy) Write(writer io.Writer, value SortBy) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeSortBy struct{}

func (_ FfiDestroyerTypeSortBy) Destroy(value SortBy) {
}

type SortDirection uint

const (
	SortDirectionAsc  SortDirection = 1
	SortDirectionDesc SortDirection = 2
)

type FfiConverterTypeSortDirection struct{}

var FfiConverterTypeSortDirectionINSTANCE = FfiConverterTypeSortDirection{}

func (c FfiConverterTypeSortDirection) Lift(rb RustBufferI) SortDirection {
	return LiftFromRustBuffer[SortDirection](c, rb)
}

func (c FfiConverterTypeSortDirection) Lower(value SortDirection) RustBuffer {
	return LowerIntoRustBuffer[SortDirection](c, value)
}
func (FfiConverterTypeSortDirection) Read(reader io.Reader) SortDirection {
	id := readInt32(reader)
	return SortDirection(id)
}

func (FfiConverterTypeSortDirection) Write(writer io.Writer, value SortDirection) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeSortDirection struct{}

func (_ FfiDestroyerTypeSortDirection) Destroy(value SortDirection) {
}

type SyncReason uint

const (
	SyncReasonDirectJoin  SyncReason = 1
	SyncReasonNewNeighbor SyncReason = 2
	SyncReasonSyncReport  SyncReason = 3
	SyncReasonResync      SyncReason = 4
)

type FfiConverterTypeSyncReason struct{}

var FfiConverterTypeSyncReasonINSTANCE = FfiConverterTypeSyncReason{}

func (c FfiConverterTypeSyncReason) Lift(rb RustBufferI) SyncReason {
	return LiftFromRustBuffer[SyncReason](c, rb)
}

func (c FfiConverterTypeSyncReason) Lower(value SyncReason) RustBuffer {
	return LowerIntoRustBuffer[SyncReason](c, value)
}
func (FfiConverterTypeSyncReason) Read(reader io.Reader) SyncReason {
	id := readInt32(reader)
	return SyncReason(id)
}

func (FfiConverterTypeSyncReason) Write(writer io.Writer, value SyncReason) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeSyncReason struct{}

func (_ FfiDestroyerTypeSyncReason) Destroy(value SyncReason) {
}

type uniffiCallbackResult C.int32_t

const (
	uniffiIdxCallbackFree               uniffiCallbackResult = 0
	uniffiCallbackResultSuccess         uniffiCallbackResult = 0
	uniffiCallbackResultError           uniffiCallbackResult = 1
	uniffiCallbackUnexpectedResultError uniffiCallbackResult = 2
	uniffiCallbackCancelled             uniffiCallbackResult = 3
)

type concurrentHandleMap[T any] struct {
	leftMap       map[uint64]*T
	rightMap      map[*T]uint64
	currentHandle uint64
	lock          sync.RWMutex
}

func newConcurrentHandleMap[T any]() *concurrentHandleMap[T] {
	return &concurrentHandleMap[T]{
		leftMap:  map[uint64]*T{},
		rightMap: map[*T]uint64{},
	}
}

func (cm *concurrentHandleMap[T]) insert(obj *T) uint64 {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	if existingHandle, ok := cm.rightMap[obj]; ok {
		return existingHandle
	}
	cm.currentHandle = cm.currentHandle + 1
	cm.leftMap[cm.currentHandle] = obj
	cm.rightMap[obj] = cm.currentHandle
	return cm.currentHandle
}

func (cm *concurrentHandleMap[T]) remove(handle uint64) bool {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	if val, ok := cm.leftMap[handle]; ok {
		delete(cm.leftMap, handle)
		delete(cm.rightMap, val)
	}
	return false
}

func (cm *concurrentHandleMap[T]) tryGet(handle uint64) (*T, bool) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()

	val, ok := cm.leftMap[handle]
	return val, ok
}

type FfiConverterCallbackInterface[CallbackInterface any] struct {
	handleMap *concurrentHandleMap[CallbackInterface]
}

func (c *FfiConverterCallbackInterface[CallbackInterface]) drop(handle uint64) RustBuffer {
	c.handleMap.remove(handle)
	return RustBuffer{}
}

func (c *FfiConverterCallbackInterface[CallbackInterface]) Lift(handle uint64) CallbackInterface {
	val, ok := c.handleMap.tryGet(handle)
	if !ok {
		panic(fmt.Errorf("no callback in handle map: %d", handle))
	}
	return *val
}

func (c *FfiConverterCallbackInterface[CallbackInterface]) Read(reader io.Reader) CallbackInterface {
	return c.Lift(readUint64(reader))
}

func (c *FfiConverterCallbackInterface[CallbackInterface]) Lower(value CallbackInterface) C.uint64_t {
	return C.uint64_t(c.handleMap.insert(&value))
}

func (c *FfiConverterCallbackInterface[CallbackInterface]) Write(writer io.Writer, value CallbackInterface) {
	writeUint64(writer, uint64(c.Lower(value)))
}

// Declaration and FfiConverters for AddCallback Callback Interface
type AddCallback interface {
	Progress(progress *AddProgress) *IrohError
}

// foreignCallbackCallbackInterfaceAddCallback cannot be callable be a compiled function at a same time
type foreignCallbackCallbackInterfaceAddCallback struct{}

//export iroh_cgo_AddCallback
func iroh_cgo_AddCallback(handle C.uint64_t, method C.int32_t, argsPtr *C.uint8_t, argsLen C.int32_t, outBuf *C.RustBuffer) C.int32_t {
	cb := FfiConverterCallbackInterfaceAddCallbackINSTANCE.Lift(uint64(handle))
	switch method {
	case 0:
		// 0 means Rust is done with the callback, and the callback
		// can be dropped by the foreign language.
		*outBuf = FfiConverterCallbackInterfaceAddCallbackINSTANCE.drop(uint64(handle))
		// See docs of ForeignCallback in `uniffi/src/ffi/foreigncallbacks.rs`
		return C.int32_t(uniffiIdxCallbackFree)

	case 1:
		var result uniffiCallbackResult
		args := unsafe.Slice((*byte)(argsPtr), argsLen)
		result = foreignCallbackCallbackInterfaceAddCallback{}.InvokeProgress(cb, args, outBuf)
		return C.int32_t(result)

	default:
		// This should never happen, because an out of bounds method index won't
		// ever be used. Once we can catch errors, we should return an InternalException.
		// https://github.com/mozilla/uniffi-rs/issues/351
		return C.int32_t(uniffiCallbackUnexpectedResultError)
	}
}

func (foreignCallbackCallbackInterfaceAddCallback) InvokeProgress(callback AddCallback, args []byte, outBuf *C.RustBuffer) uniffiCallbackResult {
	reader := bytes.NewReader(args)
	err := callback.Progress(FfiConverterAddProgressINSTANCE.Read(reader))

	if err != nil {
		// The only way to bypass an unexpected error is to bypass pointer to an empty
		// instance of the error
		if err.err == nil {
			return uniffiCallbackUnexpectedResultError
		}
		*outBuf = LowerIntoRustBuffer[*IrohError](FfiConverterTypeIrohErrorINSTANCE, err)
		return uniffiCallbackResultError
	}
	return uniffiCallbackResultSuccess
}

type FfiConverterCallbackInterfaceAddCallback struct {
	FfiConverterCallbackInterface[AddCallback]
}

var FfiConverterCallbackInterfaceAddCallbackINSTANCE = &FfiConverterCallbackInterfaceAddCallback{
	FfiConverterCallbackInterface: FfiConverterCallbackInterface[AddCallback]{
		handleMap: newConcurrentHandleMap[AddCallback](),
	},
}

// This is a static function because only 1 instance is supported for registering
func (c *FfiConverterCallbackInterfaceAddCallback) register() {
	rustCall(func(status *C.RustCallStatus) int32 {
		C.uniffi_iroh_fn_init_callback_addcallback(C.ForeignCallback(C.iroh_cgo_AddCallback), status)
		return 0
	})
}

type FfiDestroyerCallbackInterfaceAddCallback struct{}

func (FfiDestroyerCallbackInterfaceAddCallback) Destroy(value AddCallback) {
}

// Declaration and FfiConverters for DocExportFileCallback Callback Interface
type DocExportFileCallback interface {
	Progress(progress *DocExportProgress) *IrohError
}

// foreignCallbackCallbackInterfaceDocExportFileCallback cannot be callable be a compiled function at a same time
type foreignCallbackCallbackInterfaceDocExportFileCallback struct{}

//export iroh_cgo_DocExportFileCallback
func iroh_cgo_DocExportFileCallback(handle C.uint64_t, method C.int32_t, argsPtr *C.uint8_t, argsLen C.int32_t, outBuf *C.RustBuffer) C.int32_t {
	cb := FfiConverterCallbackInterfaceDocExportFileCallbackINSTANCE.Lift(uint64(handle))
	switch method {
	case 0:
		// 0 means Rust is done with the callback, and the callback
		// can be dropped by the foreign language.
		*outBuf = FfiConverterCallbackInterfaceDocExportFileCallbackINSTANCE.drop(uint64(handle))
		// See docs of ForeignCallback in `uniffi/src/ffi/foreigncallbacks.rs`
		return C.int32_t(uniffiIdxCallbackFree)

	case 1:
		var result uniffiCallbackResult
		args := unsafe.Slice((*byte)(argsPtr), argsLen)
		result = foreignCallbackCallbackInterfaceDocExportFileCallback{}.InvokeProgress(cb, args, outBuf)
		return C.int32_t(result)

	default:
		// This should never happen, because an out of bounds method index won't
		// ever be used. Once we can catch errors, we should return an InternalException.
		// https://github.com/mozilla/uniffi-rs/issues/351
		return C.int32_t(uniffiCallbackUnexpectedResultError)
	}
}

func (foreignCallbackCallbackInterfaceDocExportFileCallback) InvokeProgress(callback DocExportFileCallback, args []byte, outBuf *C.RustBuffer) uniffiCallbackResult {
	reader := bytes.NewReader(args)
	err := callback.Progress(FfiConverterDocExportProgressINSTANCE.Read(reader))

	if err != nil {
		// The only way to bypass an unexpected error is to bypass pointer to an empty
		// instance of the error
		if err.err == nil {
			return uniffiCallbackUnexpectedResultError
		}
		*outBuf = LowerIntoRustBuffer[*IrohError](FfiConverterTypeIrohErrorINSTANCE, err)
		return uniffiCallbackResultError
	}
	return uniffiCallbackResultSuccess
}

type FfiConverterCallbackInterfaceDocExportFileCallback struct {
	FfiConverterCallbackInterface[DocExportFileCallback]
}

var FfiConverterCallbackInterfaceDocExportFileCallbackINSTANCE = &FfiConverterCallbackInterfaceDocExportFileCallback{
	FfiConverterCallbackInterface: FfiConverterCallbackInterface[DocExportFileCallback]{
		handleMap: newConcurrentHandleMap[DocExportFileCallback](),
	},
}

// This is a static function because only 1 instance is supported for registering
func (c *FfiConverterCallbackInterfaceDocExportFileCallback) register() {
	rustCall(func(status *C.RustCallStatus) int32 {
		C.uniffi_iroh_fn_init_callback_docexportfilecallback(C.ForeignCallback(C.iroh_cgo_DocExportFileCallback), status)
		return 0
	})
}

type FfiDestroyerCallbackInterfaceDocExportFileCallback struct{}

func (FfiDestroyerCallbackInterfaceDocExportFileCallback) Destroy(value DocExportFileCallback) {
}

// Declaration and FfiConverters for DocImportFileCallback Callback Interface
type DocImportFileCallback interface {
	Progress(progress *DocImportProgress) *IrohError
}

// foreignCallbackCallbackInterfaceDocImportFileCallback cannot be callable be a compiled function at a same time
type foreignCallbackCallbackInterfaceDocImportFileCallback struct{}

//export iroh_cgo_DocImportFileCallback
func iroh_cgo_DocImportFileCallback(handle C.uint64_t, method C.int32_t, argsPtr *C.uint8_t, argsLen C.int32_t, outBuf *C.RustBuffer) C.int32_t {
	cb := FfiConverterCallbackInterfaceDocImportFileCallbackINSTANCE.Lift(uint64(handle))
	switch method {
	case 0:
		// 0 means Rust is done with the callback, and the callback
		// can be dropped by the foreign language.
		*outBuf = FfiConverterCallbackInterfaceDocImportFileCallbackINSTANCE.drop(uint64(handle))
		// See docs of ForeignCallback in `uniffi/src/ffi/foreigncallbacks.rs`
		return C.int32_t(uniffiIdxCallbackFree)

	case 1:
		var result uniffiCallbackResult
		args := unsafe.Slice((*byte)(argsPtr), argsLen)
		result = foreignCallbackCallbackInterfaceDocImportFileCallback{}.InvokeProgress(cb, args, outBuf)
		return C.int32_t(result)

	default:
		// This should never happen, because an out of bounds method index won't
		// ever be used. Once we can catch errors, we should return an InternalException.
		// https://github.com/mozilla/uniffi-rs/issues/351
		return C.int32_t(uniffiCallbackUnexpectedResultError)
	}
}

func (foreignCallbackCallbackInterfaceDocImportFileCallback) InvokeProgress(callback DocImportFileCallback, args []byte, outBuf *C.RustBuffer) uniffiCallbackResult {
	reader := bytes.NewReader(args)
	err := callback.Progress(FfiConverterDocImportProgressINSTANCE.Read(reader))

	if err != nil {
		// The only way to bypass an unexpected error is to bypass pointer to an empty
		// instance of the error
		if err.err == nil {
			return uniffiCallbackUnexpectedResultError
		}
		*outBuf = LowerIntoRustBuffer[*IrohError](FfiConverterTypeIrohErrorINSTANCE, err)
		return uniffiCallbackResultError
	}
	return uniffiCallbackResultSuccess
}

type FfiConverterCallbackInterfaceDocImportFileCallback struct {
	FfiConverterCallbackInterface[DocImportFileCallback]
}

var FfiConverterCallbackInterfaceDocImportFileCallbackINSTANCE = &FfiConverterCallbackInterfaceDocImportFileCallback{
	FfiConverterCallbackInterface: FfiConverterCallbackInterface[DocImportFileCallback]{
		handleMap: newConcurrentHandleMap[DocImportFileCallback](),
	},
}

// This is a static function because only 1 instance is supported for registering
func (c *FfiConverterCallbackInterfaceDocImportFileCallback) register() {
	rustCall(func(status *C.RustCallStatus) int32 {
		C.uniffi_iroh_fn_init_callback_docimportfilecallback(C.ForeignCallback(C.iroh_cgo_DocImportFileCallback), status)
		return 0
	})
}

type FfiDestroyerCallbackInterfaceDocImportFileCallback struct{}

func (FfiDestroyerCallbackInterfaceDocImportFileCallback) Destroy(value DocImportFileCallback) {
}

// Declaration and FfiConverters for DownloadCallback Callback Interface
type DownloadCallback interface {
	Progress(progress *DownloadProgress) *IrohError
}

// foreignCallbackCallbackInterfaceDownloadCallback cannot be callable be a compiled function at a same time
type foreignCallbackCallbackInterfaceDownloadCallback struct{}

//export iroh_cgo_DownloadCallback
func iroh_cgo_DownloadCallback(handle C.uint64_t, method C.int32_t, argsPtr *C.uint8_t, argsLen C.int32_t, outBuf *C.RustBuffer) C.int32_t {
	cb := FfiConverterCallbackInterfaceDownloadCallbackINSTANCE.Lift(uint64(handle))
	switch method {
	case 0:
		// 0 means Rust is done with the callback, and the callback
		// can be dropped by the foreign language.
		*outBuf = FfiConverterCallbackInterfaceDownloadCallbackINSTANCE.drop(uint64(handle))
		// See docs of ForeignCallback in `uniffi/src/ffi/foreigncallbacks.rs`
		return C.int32_t(uniffiIdxCallbackFree)

	case 1:
		var result uniffiCallbackResult
		args := unsafe.Slice((*byte)(argsPtr), argsLen)
		result = foreignCallbackCallbackInterfaceDownloadCallback{}.InvokeProgress(cb, args, outBuf)
		return C.int32_t(result)

	default:
		// This should never happen, because an out of bounds method index won't
		// ever be used. Once we can catch errors, we should return an InternalException.
		// https://github.com/mozilla/uniffi-rs/issues/351
		return C.int32_t(uniffiCallbackUnexpectedResultError)
	}
}

func (foreignCallbackCallbackInterfaceDownloadCallback) InvokeProgress(callback DownloadCallback, args []byte, outBuf *C.RustBuffer) uniffiCallbackResult {
	reader := bytes.NewReader(args)
	err := callback.Progress(FfiConverterDownloadProgressINSTANCE.Read(reader))

	if err != nil {
		// The only way to bypass an unexpected error is to bypass pointer to an empty
		// instance of the error
		if err.err == nil {
			return uniffiCallbackUnexpectedResultError
		}
		*outBuf = LowerIntoRustBuffer[*IrohError](FfiConverterTypeIrohErrorINSTANCE, err)
		return uniffiCallbackResultError
	}
	return uniffiCallbackResultSuccess
}

type FfiConverterCallbackInterfaceDownloadCallback struct {
	FfiConverterCallbackInterface[DownloadCallback]
}

var FfiConverterCallbackInterfaceDownloadCallbackINSTANCE = &FfiConverterCallbackInterfaceDownloadCallback{
	FfiConverterCallbackInterface: FfiConverterCallbackInterface[DownloadCallback]{
		handleMap: newConcurrentHandleMap[DownloadCallback](),
	},
}

// This is a static function because only 1 instance is supported for registering
func (c *FfiConverterCallbackInterfaceDownloadCallback) register() {
	rustCall(func(status *C.RustCallStatus) int32 {
		C.uniffi_iroh_fn_init_callback_downloadcallback(C.ForeignCallback(C.iroh_cgo_DownloadCallback), status)
		return 0
	})
}

type FfiDestroyerCallbackInterfaceDownloadCallback struct{}

func (FfiDestroyerCallbackInterfaceDownloadCallback) Destroy(value DownloadCallback) {
}

// Declaration and FfiConverters for SubscribeCallback Callback Interface
type SubscribeCallback interface {
	Event(event *LiveEvent) *IrohError
}

// foreignCallbackCallbackInterfaceSubscribeCallback cannot be callable be a compiled function at a same time
type foreignCallbackCallbackInterfaceSubscribeCallback struct{}

//export iroh_cgo_SubscribeCallback
func iroh_cgo_SubscribeCallback(handle C.uint64_t, method C.int32_t, argsPtr *C.uint8_t, argsLen C.int32_t, outBuf *C.RustBuffer) C.int32_t {
	cb := FfiConverterCallbackInterfaceSubscribeCallbackINSTANCE.Lift(uint64(handle))
	switch method {
	case 0:
		// 0 means Rust is done with the callback, and the callback
		// can be dropped by the foreign language.
		*outBuf = FfiConverterCallbackInterfaceSubscribeCallbackINSTANCE.drop(uint64(handle))
		// See docs of ForeignCallback in `uniffi/src/ffi/foreigncallbacks.rs`
		return C.int32_t(uniffiIdxCallbackFree)

	case 1:
		var result uniffiCallbackResult
		args := unsafe.Slice((*byte)(argsPtr), argsLen)
		result = foreignCallbackCallbackInterfaceSubscribeCallback{}.InvokeEvent(cb, args, outBuf)
		return C.int32_t(result)

	default:
		// This should never happen, because an out of bounds method index won't
		// ever be used. Once we can catch errors, we should return an InternalException.
		// https://github.com/mozilla/uniffi-rs/issues/351
		return C.int32_t(uniffiCallbackUnexpectedResultError)
	}
}

func (foreignCallbackCallbackInterfaceSubscribeCallback) InvokeEvent(callback SubscribeCallback, args []byte, outBuf *C.RustBuffer) uniffiCallbackResult {
	reader := bytes.NewReader(args)
	err := callback.Event(FfiConverterLiveEventINSTANCE.Read(reader))

	if err != nil {
		// The only way to bypass an unexpected error is to bypass pointer to an empty
		// instance of the error
		if err.err == nil {
			return uniffiCallbackUnexpectedResultError
		}
		*outBuf = LowerIntoRustBuffer[*IrohError](FfiConverterTypeIrohErrorINSTANCE, err)
		return uniffiCallbackResultError
	}
	return uniffiCallbackResultSuccess
}

type FfiConverterCallbackInterfaceSubscribeCallback struct {
	FfiConverterCallbackInterface[SubscribeCallback]
}

var FfiConverterCallbackInterfaceSubscribeCallbackINSTANCE = &FfiConverterCallbackInterfaceSubscribeCallback{
	FfiConverterCallbackInterface: FfiConverterCallbackInterface[SubscribeCallback]{
		handleMap: newConcurrentHandleMap[SubscribeCallback](),
	},
}

// This is a static function because only 1 instance is supported for registering
func (c *FfiConverterCallbackInterfaceSubscribeCallback) register() {
	rustCall(func(status *C.RustCallStatus) int32 {
		C.uniffi_iroh_fn_init_callback_subscribecallback(C.ForeignCallback(C.iroh_cgo_SubscribeCallback), status)
		return 0
	})
}

type FfiDestroyerCallbackInterfaceSubscribeCallback struct{}

func (FfiDestroyerCallbackInterfaceSubscribeCallback) Destroy(value SubscribeCallback) {
}

type FfiConverterOptionalUint64 struct{}

var FfiConverterOptionalUint64INSTANCE = FfiConverterOptionalUint64{}

func (c FfiConverterOptionalUint64) Lift(rb RustBufferI) *uint64 {
	return LiftFromRustBuffer[*uint64](c, rb)
}

func (_ FfiConverterOptionalUint64) Read(reader io.Reader) *uint64 {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUint64INSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUint64) Lower(value *uint64) RustBuffer {
	return LowerIntoRustBuffer[*uint64](c, value)
}

func (_ FfiConverterOptionalUint64) Write(writer io.Writer, value *uint64) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUint64INSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUint64 struct{}

func (_ FfiDestroyerOptionalUint64) Destroy(value *uint64) {
	if value != nil {
		FfiDestroyerUint64{}.Destroy(*value)
	}
}

type FfiConverterOptionalString struct{}

var FfiConverterOptionalStringINSTANCE = FfiConverterOptionalString{}

func (c FfiConverterOptionalString) Lift(rb RustBufferI) *string {
	return LiftFromRustBuffer[*string](c, rb)
}

func (_ FfiConverterOptionalString) Read(reader io.Reader) *string {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterStringINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalString) Lower(value *string) RustBuffer {
	return LowerIntoRustBuffer[*string](c, value)
}

func (_ FfiConverterOptionalString) Write(writer io.Writer, value *string) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterStringINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalString struct{}

func (_ FfiDestroyerOptionalString) Destroy(value *string) {
	if value != nil {
		FfiDestroyerString{}.Destroy(*value)
	}
}

type FfiConverterOptionalDuration struct{}

var FfiConverterOptionalDurationINSTANCE = FfiConverterOptionalDuration{}

func (c FfiConverterOptionalDuration) Lift(rb RustBufferI) *time.Duration {
	return LiftFromRustBuffer[*time.Duration](c, rb)
}

func (_ FfiConverterOptionalDuration) Read(reader io.Reader) *time.Duration {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterDurationINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalDuration) Lower(value *time.Duration) RustBuffer {
	return LowerIntoRustBuffer[*time.Duration](c, value)
}

func (_ FfiConverterOptionalDuration) Write(writer io.Writer, value *time.Duration) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterDurationINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalDuration struct{}

func (_ FfiDestroyerOptionalDuration) Destroy(value *time.Duration) {
	if value != nil {
		FfiDestroyerDuration{}.Destroy(*value)
	}
}

type FfiConverterOptionalDoc struct{}

var FfiConverterOptionalDocINSTANCE = FfiConverterOptionalDoc{}

func (c FfiConverterOptionalDoc) Lift(rb RustBufferI) **Doc {
	return LiftFromRustBuffer[**Doc](c, rb)
}

func (_ FfiConverterOptionalDoc) Read(reader io.Reader) **Doc {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterDocINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalDoc) Lower(value **Doc) RustBuffer {
	return LowerIntoRustBuffer[**Doc](c, value)
}

func (_ FfiConverterOptionalDoc) Write(writer io.Writer, value **Doc) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterDocINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalDoc struct{}

func (_ FfiDestroyerOptionalDoc) Destroy(value **Doc) {
	if value != nil {
		FfiDestroyerDoc{}.Destroy(*value)
	}
}

type FfiConverterOptionalEntry struct{}

var FfiConverterOptionalEntryINSTANCE = FfiConverterOptionalEntry{}

func (c FfiConverterOptionalEntry) Lift(rb RustBufferI) **Entry {
	return LiftFromRustBuffer[**Entry](c, rb)
}

func (_ FfiConverterOptionalEntry) Read(reader io.Reader) **Entry {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterEntryINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalEntry) Lower(value **Entry) RustBuffer {
	return LowerIntoRustBuffer[**Entry](c, value)
}

func (_ FfiConverterOptionalEntry) Write(writer io.Writer, value **Entry) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterEntryINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalEntry struct{}

func (_ FfiDestroyerOptionalEntry) Destroy(value **Entry) {
	if value != nil {
		FfiDestroyerEntry{}.Destroy(*value)
	}
}

type FfiConverterOptionalUrl struct{}

var FfiConverterOptionalUrlINSTANCE = FfiConverterOptionalUrl{}

func (c FfiConverterOptionalUrl) Lift(rb RustBufferI) **Url {
	return LiftFromRustBuffer[**Url](c, rb)
}

func (_ FfiConverterOptionalUrl) Read(reader io.Reader) **Url {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterUrlINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalUrl) Lower(value **Url) RustBuffer {
	return LowerIntoRustBuffer[**Url](c, value)
}

func (_ FfiConverterOptionalUrl) Write(writer io.Writer, value **Url) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterUrlINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalUrl struct{}

func (_ FfiDestroyerOptionalUrl) Destroy(value **Url) {
	if value != nil {
		FfiDestroyerUrl{}.Destroy(*value)
	}
}

type FfiConverterOptionalTypeConnectionInfo struct{}

var FfiConverterOptionalTypeConnectionInfoINSTANCE = FfiConverterOptionalTypeConnectionInfo{}

func (c FfiConverterOptionalTypeConnectionInfo) Lift(rb RustBufferI) *ConnectionInfo {
	return LiftFromRustBuffer[*ConnectionInfo](c, rb)
}

func (_ FfiConverterOptionalTypeConnectionInfo) Read(reader io.Reader) *ConnectionInfo {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterTypeConnectionInfoINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalTypeConnectionInfo) Lower(value *ConnectionInfo) RustBuffer {
	return LowerIntoRustBuffer[*ConnectionInfo](c, value)
}

func (_ FfiConverterOptionalTypeConnectionInfo) Write(writer io.Writer, value *ConnectionInfo) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterTypeConnectionInfoINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalTypeConnectionInfo struct{}

func (_ FfiDestroyerOptionalTypeConnectionInfo) Destroy(value *ConnectionInfo) {
	if value != nil {
		FfiDestroyerTypeConnectionInfo{}.Destroy(*value)
	}
}

type FfiConverterOptionalTypeLatencyAndControlMsg struct{}

var FfiConverterOptionalTypeLatencyAndControlMsgINSTANCE = FfiConverterOptionalTypeLatencyAndControlMsg{}

func (c FfiConverterOptionalTypeLatencyAndControlMsg) Lift(rb RustBufferI) *LatencyAndControlMsg {
	return LiftFromRustBuffer[*LatencyAndControlMsg](c, rb)
}

func (_ FfiConverterOptionalTypeLatencyAndControlMsg) Read(reader io.Reader) *LatencyAndControlMsg {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterTypeLatencyAndControlMsgINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalTypeLatencyAndControlMsg) Lower(value *LatencyAndControlMsg) RustBuffer {
	return LowerIntoRustBuffer[*LatencyAndControlMsg](c, value)
}

func (_ FfiConverterOptionalTypeLatencyAndControlMsg) Write(writer io.Writer, value *LatencyAndControlMsg) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterTypeLatencyAndControlMsgINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalTypeLatencyAndControlMsg struct{}

func (_ FfiDestroyerOptionalTypeLatencyAndControlMsg) Destroy(value *LatencyAndControlMsg) {
	if value != nil {
		FfiDestroyerTypeLatencyAndControlMsg{}.Destroy(*value)
	}
}

type FfiConverterOptionalTypeQueryOptions struct{}

var FfiConverterOptionalTypeQueryOptionsINSTANCE = FfiConverterOptionalTypeQueryOptions{}

func (c FfiConverterOptionalTypeQueryOptions) Lift(rb RustBufferI) *QueryOptions {
	return LiftFromRustBuffer[*QueryOptions](c, rb)
}

func (_ FfiConverterOptionalTypeQueryOptions) Read(reader io.Reader) *QueryOptions {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterTypeQueryOptionsINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalTypeQueryOptions) Lower(value *QueryOptions) RustBuffer {
	return LowerIntoRustBuffer[*QueryOptions](c, value)
}

func (_ FfiConverterOptionalTypeQueryOptions) Write(writer io.Writer, value *QueryOptions) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterTypeQueryOptionsINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalTypeQueryOptions struct{}

func (_ FfiDestroyerOptionalTypeQueryOptions) Destroy(value *QueryOptions) {
	if value != nil {
		FfiDestroyerTypeQueryOptions{}.Destroy(*value)
	}
}

type FfiConverterOptionalCallbackInterfaceDocExportFileCallback struct{}

var FfiConverterOptionalCallbackInterfaceDocExportFileCallbackINSTANCE = FfiConverterOptionalCallbackInterfaceDocExportFileCallback{}

func (c FfiConverterOptionalCallbackInterfaceDocExportFileCallback) Lift(rb RustBufferI) *DocExportFileCallback {
	return LiftFromRustBuffer[*DocExportFileCallback](c, rb)
}

func (_ FfiConverterOptionalCallbackInterfaceDocExportFileCallback) Read(reader io.Reader) *DocExportFileCallback {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterCallbackInterfaceDocExportFileCallbackINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalCallbackInterfaceDocExportFileCallback) Lower(value *DocExportFileCallback) RustBuffer {
	return LowerIntoRustBuffer[*DocExportFileCallback](c, value)
}

func (_ FfiConverterOptionalCallbackInterfaceDocExportFileCallback) Write(writer io.Writer, value *DocExportFileCallback) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterCallbackInterfaceDocExportFileCallbackINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalCallbackInterfaceDocExportFileCallback struct{}

func (_ FfiDestroyerOptionalCallbackInterfaceDocExportFileCallback) Destroy(value *DocExportFileCallback) {
	if value != nil {
		FfiDestroyerCallbackInterfaceDocExportFileCallback{}.Destroy(*value)
	}
}

type FfiConverterOptionalCallbackInterfaceDocImportFileCallback struct{}

var FfiConverterOptionalCallbackInterfaceDocImportFileCallbackINSTANCE = FfiConverterOptionalCallbackInterfaceDocImportFileCallback{}

func (c FfiConverterOptionalCallbackInterfaceDocImportFileCallback) Lift(rb RustBufferI) *DocImportFileCallback {
	return LiftFromRustBuffer[*DocImportFileCallback](c, rb)
}

func (_ FfiConverterOptionalCallbackInterfaceDocImportFileCallback) Read(reader io.Reader) *DocImportFileCallback {
	if readInt8(reader) == 0 {
		return nil
	}
	temp := FfiConverterCallbackInterfaceDocImportFileCallbackINSTANCE.Read(reader)
	return &temp
}

func (c FfiConverterOptionalCallbackInterfaceDocImportFileCallback) Lower(value *DocImportFileCallback) RustBuffer {
	return LowerIntoRustBuffer[*DocImportFileCallback](c, value)
}

func (_ FfiConverterOptionalCallbackInterfaceDocImportFileCallback) Write(writer io.Writer, value *DocImportFileCallback) {
	if value == nil {
		writeInt8(writer, 0)
	} else {
		writeInt8(writer, 1)
		FfiConverterCallbackInterfaceDocImportFileCallbackINSTANCE.Write(writer, *value)
	}
}

type FfiDestroyerOptionalCallbackInterfaceDocImportFileCallback struct{}

func (_ FfiDestroyerOptionalCallbackInterfaceDocImportFileCallback) Destroy(value *DocImportFileCallback) {
	if value != nil {
		FfiDestroyerCallbackInterfaceDocImportFileCallback{}.Destroy(*value)
	}
}

type FfiConverterSequenceUint8 struct{}

var FfiConverterSequenceUint8INSTANCE = FfiConverterSequenceUint8{}

func (c FfiConverterSequenceUint8) Lift(rb RustBufferI) []uint8 {
	return LiftFromRustBuffer[[]uint8](c, rb)
}

func (c FfiConverterSequenceUint8) Read(reader io.Reader) []uint8 {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]uint8, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterUint8INSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceUint8) Lower(value []uint8) RustBuffer {
	return LowerIntoRustBuffer[[]uint8](c, value)
}

func (c FfiConverterSequenceUint8) Write(writer io.Writer, value []uint8) {
	if len(value) > math.MaxInt32 {
		panic("[]uint8 is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterUint8INSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceUint8 struct{}

func (FfiDestroyerSequenceUint8) Destroy(sequence []uint8) {
	for _, value := range sequence {
		FfiDestroyerUint8{}.Destroy(value)
	}
}

type FfiConverterSequenceUint16 struct{}

var FfiConverterSequenceUint16INSTANCE = FfiConverterSequenceUint16{}

func (c FfiConverterSequenceUint16) Lift(rb RustBufferI) []uint16 {
	return LiftFromRustBuffer[[]uint16](c, rb)
}

func (c FfiConverterSequenceUint16) Read(reader io.Reader) []uint16 {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]uint16, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterUint16INSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceUint16) Lower(value []uint16) RustBuffer {
	return LowerIntoRustBuffer[[]uint16](c, value)
}

func (c FfiConverterSequenceUint16) Write(writer io.Writer, value []uint16) {
	if len(value) > math.MaxInt32 {
		panic("[]uint16 is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterUint16INSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceUint16 struct{}

func (FfiDestroyerSequenceUint16) Destroy(sequence []uint16) {
	for _, value := range sequence {
		FfiDestroyerUint16{}.Destroy(value)
	}
}

type FfiConverterSequenceAuthorId struct{}

var FfiConverterSequenceAuthorIdINSTANCE = FfiConverterSequenceAuthorId{}

func (c FfiConverterSequenceAuthorId) Lift(rb RustBufferI) []*AuthorId {
	return LiftFromRustBuffer[[]*AuthorId](c, rb)
}

func (c FfiConverterSequenceAuthorId) Read(reader io.Reader) []*AuthorId {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*AuthorId, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterAuthorIdINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceAuthorId) Lower(value []*AuthorId) RustBuffer {
	return LowerIntoRustBuffer[[]*AuthorId](c, value)
}

func (c FfiConverterSequenceAuthorId) Write(writer io.Writer, value []*AuthorId) {
	if len(value) > math.MaxInt32 {
		panic("[]*AuthorId is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterAuthorIdINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceAuthorId struct{}

func (FfiDestroyerSequenceAuthorId) Destroy(sequence []*AuthorId) {
	for _, value := range sequence {
		FfiDestroyerAuthorId{}.Destroy(value)
	}
}

type FfiConverterSequenceDirectAddrInfo struct{}

var FfiConverterSequenceDirectAddrInfoINSTANCE = FfiConverterSequenceDirectAddrInfo{}

func (c FfiConverterSequenceDirectAddrInfo) Lift(rb RustBufferI) []*DirectAddrInfo {
	return LiftFromRustBuffer[[]*DirectAddrInfo](c, rb)
}

func (c FfiConverterSequenceDirectAddrInfo) Read(reader io.Reader) []*DirectAddrInfo {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*DirectAddrInfo, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterDirectAddrInfoINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceDirectAddrInfo) Lower(value []*DirectAddrInfo) RustBuffer {
	return LowerIntoRustBuffer[[]*DirectAddrInfo](c, value)
}

func (c FfiConverterSequenceDirectAddrInfo) Write(writer io.Writer, value []*DirectAddrInfo) {
	if len(value) > math.MaxInt32 {
		panic("[]*DirectAddrInfo is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterDirectAddrInfoINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceDirectAddrInfo struct{}

func (FfiDestroyerSequenceDirectAddrInfo) Destroy(sequence []*DirectAddrInfo) {
	for _, value := range sequence {
		FfiDestroyerDirectAddrInfo{}.Destroy(value)
	}
}

type FfiConverterSequenceEntry struct{}

var FfiConverterSequenceEntryINSTANCE = FfiConverterSequenceEntry{}

func (c FfiConverterSequenceEntry) Lift(rb RustBufferI) []*Entry {
	return LiftFromRustBuffer[[]*Entry](c, rb)
}

func (c FfiConverterSequenceEntry) Read(reader io.Reader) []*Entry {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*Entry, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterEntryINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceEntry) Lower(value []*Entry) RustBuffer {
	return LowerIntoRustBuffer[[]*Entry](c, value)
}

func (c FfiConverterSequenceEntry) Write(writer io.Writer, value []*Entry) {
	if len(value) > math.MaxInt32 {
		panic("[]*Entry is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterEntryINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceEntry struct{}

func (FfiDestroyerSequenceEntry) Destroy(sequence []*Entry) {
	for _, value := range sequence {
		FfiDestroyerEntry{}.Destroy(value)
	}
}

type FfiConverterSequenceFilterKind struct{}

var FfiConverterSequenceFilterKindINSTANCE = FfiConverterSequenceFilterKind{}

func (c FfiConverterSequenceFilterKind) Lift(rb RustBufferI) []*FilterKind {
	return LiftFromRustBuffer[[]*FilterKind](c, rb)
}

func (c FfiConverterSequenceFilterKind) Read(reader io.Reader) []*FilterKind {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*FilterKind, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterFilterKindINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceFilterKind) Lower(value []*FilterKind) RustBuffer {
	return LowerIntoRustBuffer[[]*FilterKind](c, value)
}

func (c FfiConverterSequenceFilterKind) Write(writer io.Writer, value []*FilterKind) {
	if len(value) > math.MaxInt32 {
		panic("[]*FilterKind is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterFilterKindINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceFilterKind struct{}

func (FfiDestroyerSequenceFilterKind) Destroy(sequence []*FilterKind) {
	for _, value := range sequence {
		FfiDestroyerFilterKind{}.Destroy(value)
	}
}

type FfiConverterSequenceHash struct{}

var FfiConverterSequenceHashINSTANCE = FfiConverterSequenceHash{}

func (c FfiConverterSequenceHash) Lift(rb RustBufferI) []*Hash {
	return LiftFromRustBuffer[[]*Hash](c, rb)
}

func (c FfiConverterSequenceHash) Read(reader io.Reader) []*Hash {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*Hash, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterHashINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceHash) Lower(value []*Hash) RustBuffer {
	return LowerIntoRustBuffer[[]*Hash](c, value)
}

func (c FfiConverterSequenceHash) Write(writer io.Writer, value []*Hash) {
	if len(value) > math.MaxInt32 {
		panic("[]*Hash is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterHashINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceHash struct{}

func (FfiDestroyerSequenceHash) Destroy(sequence []*Hash) {
	for _, value := range sequence {
		FfiDestroyerHash{}.Destroy(value)
	}
}

type FfiConverterSequenceNodeAddr struct{}

var FfiConverterSequenceNodeAddrINSTANCE = FfiConverterSequenceNodeAddr{}

func (c FfiConverterSequenceNodeAddr) Lift(rb RustBufferI) []*NodeAddr {
	return LiftFromRustBuffer[[]*NodeAddr](c, rb)
}

func (c FfiConverterSequenceNodeAddr) Read(reader io.Reader) []*NodeAddr {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*NodeAddr, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterNodeAddrINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceNodeAddr) Lower(value []*NodeAddr) RustBuffer {
	return LowerIntoRustBuffer[[]*NodeAddr](c, value)
}

func (c FfiConverterSequenceNodeAddr) Write(writer io.Writer, value []*NodeAddr) {
	if len(value) > math.MaxInt32 {
		panic("[]*NodeAddr is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterNodeAddrINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceNodeAddr struct{}

func (FfiDestroyerSequenceNodeAddr) Destroy(sequence []*NodeAddr) {
	for _, value := range sequence {
		FfiDestroyerNodeAddr{}.Destroy(value)
	}
}

type FfiConverterSequenceSocketAddr struct{}

var FfiConverterSequenceSocketAddrINSTANCE = FfiConverterSequenceSocketAddr{}

func (c FfiConverterSequenceSocketAddr) Lift(rb RustBufferI) []*SocketAddr {
	return LiftFromRustBuffer[[]*SocketAddr](c, rb)
}

func (c FfiConverterSequenceSocketAddr) Read(reader io.Reader) []*SocketAddr {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]*SocketAddr, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterSocketAddrINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceSocketAddr) Lower(value []*SocketAddr) RustBuffer {
	return LowerIntoRustBuffer[[]*SocketAddr](c, value)
}

func (c FfiConverterSequenceSocketAddr) Write(writer io.Writer, value []*SocketAddr) {
	if len(value) > math.MaxInt32 {
		panic("[]*SocketAddr is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterSocketAddrINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceSocketAddr struct{}

func (FfiDestroyerSequenceSocketAddr) Destroy(sequence []*SocketAddr) {
	for _, value := range sequence {
		FfiDestroyerSocketAddr{}.Destroy(value)
	}
}

type FfiConverterSequenceTypeBlobListCollectionsResponse struct{}

var FfiConverterSequenceTypeBlobListCollectionsResponseINSTANCE = FfiConverterSequenceTypeBlobListCollectionsResponse{}

func (c FfiConverterSequenceTypeBlobListCollectionsResponse) Lift(rb RustBufferI) []BlobListCollectionsResponse {
	return LiftFromRustBuffer[[]BlobListCollectionsResponse](c, rb)
}

func (c FfiConverterSequenceTypeBlobListCollectionsResponse) Read(reader io.Reader) []BlobListCollectionsResponse {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]BlobListCollectionsResponse, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTypeBlobListCollectionsResponseINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTypeBlobListCollectionsResponse) Lower(value []BlobListCollectionsResponse) RustBuffer {
	return LowerIntoRustBuffer[[]BlobListCollectionsResponse](c, value)
}

func (c FfiConverterSequenceTypeBlobListCollectionsResponse) Write(writer io.Writer, value []BlobListCollectionsResponse) {
	if len(value) > math.MaxInt32 {
		panic("[]BlobListCollectionsResponse is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTypeBlobListCollectionsResponseINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTypeBlobListCollectionsResponse struct{}

func (FfiDestroyerSequenceTypeBlobListCollectionsResponse) Destroy(sequence []BlobListCollectionsResponse) {
	for _, value := range sequence {
		FfiDestroyerTypeBlobListCollectionsResponse{}.Destroy(value)
	}
}

type FfiConverterSequenceTypeBlobListIncompleteResponse struct{}

var FfiConverterSequenceTypeBlobListIncompleteResponseINSTANCE = FfiConverterSequenceTypeBlobListIncompleteResponse{}

func (c FfiConverterSequenceTypeBlobListIncompleteResponse) Lift(rb RustBufferI) []BlobListIncompleteResponse {
	return LiftFromRustBuffer[[]BlobListIncompleteResponse](c, rb)
}

func (c FfiConverterSequenceTypeBlobListIncompleteResponse) Read(reader io.Reader) []BlobListIncompleteResponse {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]BlobListIncompleteResponse, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTypeBlobListIncompleteResponseINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTypeBlobListIncompleteResponse) Lower(value []BlobListIncompleteResponse) RustBuffer {
	return LowerIntoRustBuffer[[]BlobListIncompleteResponse](c, value)
}

func (c FfiConverterSequenceTypeBlobListIncompleteResponse) Write(writer io.Writer, value []BlobListIncompleteResponse) {
	if len(value) > math.MaxInt32 {
		panic("[]BlobListIncompleteResponse is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTypeBlobListIncompleteResponseINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTypeBlobListIncompleteResponse struct{}

func (FfiDestroyerSequenceTypeBlobListIncompleteResponse) Destroy(sequence []BlobListIncompleteResponse) {
	for _, value := range sequence {
		FfiDestroyerTypeBlobListIncompleteResponse{}.Destroy(value)
	}
}

type FfiConverterSequenceTypeConnectionInfo struct{}

var FfiConverterSequenceTypeConnectionInfoINSTANCE = FfiConverterSequenceTypeConnectionInfo{}

func (c FfiConverterSequenceTypeConnectionInfo) Lift(rb RustBufferI) []ConnectionInfo {
	return LiftFromRustBuffer[[]ConnectionInfo](c, rb)
}

func (c FfiConverterSequenceTypeConnectionInfo) Read(reader io.Reader) []ConnectionInfo {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]ConnectionInfo, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTypeConnectionInfoINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTypeConnectionInfo) Lower(value []ConnectionInfo) RustBuffer {
	return LowerIntoRustBuffer[[]ConnectionInfo](c, value)
}

func (c FfiConverterSequenceTypeConnectionInfo) Write(writer io.Writer, value []ConnectionInfo) {
	if len(value) > math.MaxInt32 {
		panic("[]ConnectionInfo is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTypeConnectionInfoINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTypeConnectionInfo struct{}

func (FfiDestroyerSequenceTypeConnectionInfo) Destroy(sequence []ConnectionInfo) {
	for _, value := range sequence {
		FfiDestroyerTypeConnectionInfo{}.Destroy(value)
	}
}

type FfiConverterSequenceTypeListTagsResponse struct{}

var FfiConverterSequenceTypeListTagsResponseINSTANCE = FfiConverterSequenceTypeListTagsResponse{}

func (c FfiConverterSequenceTypeListTagsResponse) Lift(rb RustBufferI) []ListTagsResponse {
	return LiftFromRustBuffer[[]ListTagsResponse](c, rb)
}

func (c FfiConverterSequenceTypeListTagsResponse) Read(reader io.Reader) []ListTagsResponse {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]ListTagsResponse, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTypeListTagsResponseINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTypeListTagsResponse) Lower(value []ListTagsResponse) RustBuffer {
	return LowerIntoRustBuffer[[]ListTagsResponse](c, value)
}

func (c FfiConverterSequenceTypeListTagsResponse) Write(writer io.Writer, value []ListTagsResponse) {
	if len(value) > math.MaxInt32 {
		panic("[]ListTagsResponse is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTypeListTagsResponseINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTypeListTagsResponse struct{}

func (FfiDestroyerSequenceTypeListTagsResponse) Destroy(sequence []ListTagsResponse) {
	for _, value := range sequence {
		FfiDestroyerTypeListTagsResponse{}.Destroy(value)
	}
}

type FfiConverterSequenceTypeNamespaceAndCapability struct{}

var FfiConverterSequenceTypeNamespaceAndCapabilityINSTANCE = FfiConverterSequenceTypeNamespaceAndCapability{}

func (c FfiConverterSequenceTypeNamespaceAndCapability) Lift(rb RustBufferI) []NamespaceAndCapability {
	return LiftFromRustBuffer[[]NamespaceAndCapability](c, rb)
}

func (c FfiConverterSequenceTypeNamespaceAndCapability) Read(reader io.Reader) []NamespaceAndCapability {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]NamespaceAndCapability, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTypeNamespaceAndCapabilityINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTypeNamespaceAndCapability) Lower(value []NamespaceAndCapability) RustBuffer {
	return LowerIntoRustBuffer[[]NamespaceAndCapability](c, value)
}

func (c FfiConverterSequenceTypeNamespaceAndCapability) Write(writer io.Writer, value []NamespaceAndCapability) {
	if len(value) > math.MaxInt32 {
		panic("[]NamespaceAndCapability is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTypeNamespaceAndCapabilityINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTypeNamespaceAndCapability struct{}

func (FfiDestroyerSequenceTypeNamespaceAndCapability) Destroy(sequence []NamespaceAndCapability) {
	for _, value := range sequence {
		FfiDestroyerTypeNamespaceAndCapability{}.Destroy(value)
	}
}

type FfiConverterMapStringTypeCounterStats struct{}

var FfiConverterMapStringTypeCounterStatsINSTANCE = FfiConverterMapStringTypeCounterStats{}

func (c FfiConverterMapStringTypeCounterStats) Lift(rb RustBufferI) map[string]CounterStats {
	return LiftFromRustBuffer[map[string]CounterStats](c, rb)
}

func (_ FfiConverterMapStringTypeCounterStats) Read(reader io.Reader) map[string]CounterStats {
	result := make(map[string]CounterStats)
	length := readInt32(reader)
	for i := int32(0); i < length; i++ {
		key := FfiConverterStringINSTANCE.Read(reader)
		value := FfiConverterTypeCounterStatsINSTANCE.Read(reader)
		result[key] = value
	}
	return result
}

func (c FfiConverterMapStringTypeCounterStats) Lower(value map[string]CounterStats) RustBuffer {
	return LowerIntoRustBuffer[map[string]CounterStats](c, value)
}

func (_ FfiConverterMapStringTypeCounterStats) Write(writer io.Writer, mapValue map[string]CounterStats) {
	if len(mapValue) > math.MaxInt32 {
		panic("map[string]CounterStats is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(mapValue)))
	for key, value := range mapValue {
		FfiConverterStringINSTANCE.Write(writer, key)
		FfiConverterTypeCounterStatsINSTANCE.Write(writer, value)
	}
}

type FfiDestroyerMapStringTypeCounterStats struct{}

func (_ FfiDestroyerMapStringTypeCounterStats) Destroy(mapValue map[string]CounterStats) {
	for key, value := range mapValue {
		FfiDestroyerString{}.Destroy(key)
		FfiDestroyerTypeCounterStats{}.Destroy(value)
	}
}

func KeyToPath(key []byte, prefix *string, root *string) (string, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_func_key_to_path(FfiConverterBytesINSTANCE.Lower(key), FfiConverterOptionalStringINSTANCE.Lower(prefix), FfiConverterOptionalStringINSTANCE.Lower(root), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func PathToKey(path string, prefix *string, root *string) ([]byte, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_iroh_fn_func_path_to_key(FfiConverterStringINSTANCE.Lower(path), FfiConverterOptionalStringINSTANCE.Lower(prefix), FfiConverterOptionalStringINSTANCE.Lower(root), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func SetLogLevel(level LogLevel) {
	rustCall(func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_func_set_log_level(FfiConverterTypeLogLevelINSTANCE.Lower(level), _uniffiStatus)
		return false
	})
}

func StartMetricsCollection() error {
	_, _uniffiErr := rustCallWithError(FfiConverterTypeIrohError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_iroh_fn_func_start_metrics_collection(_uniffiStatus)
		return false
	})
	return _uniffiErr
}
