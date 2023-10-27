package quic

import (
	"sync"

	"github.com/quic-go/quic-go/internal/protocol"
)

type packetBuffer struct {
	Data []byte

	// refCount counts how many packets Data is used in.
	// It doesn't support concurrent use.
	// It is > 1 when used for coalesced packet.
	refCount int

	packetSize protocol.ByteCount
}

// Split increases the refCount.
// It must be called when a packet buffer is used for more than one packet,
// e.g. when splitting coalesced packets.
func (b *packetBuffer) Split() {
	b.refCount++
}

// Decrement decrements the reference counter.
// It doesn't put the buffer back into the pool.
func (b *packetBuffer) Decrement() {
	b.refCount--
	if b.refCount < 0 {
		panic("negative packetBuffer refCount")
	}
}

// MaybeRelease puts the packet buffer back into the pool,
// if the reference counter already reached 0.
func (b *packetBuffer) MaybeRelease() {
	// only put the packetBuffer back if it's not used any more
	if b.refCount == 0 {
		b.putBack()
	}
}

// Release puts back the packet buffer into the pool.
// It should be called when processing is definitely finished.
func (b *packetBuffer) Release() {
	b.Decrement()
	if b.refCount != 0 {
		panic("packetBuffer refCount not zero")
	}
	b.putBack()
}

// Len returns the length of Data
func (b *packetBuffer) Len() protocol.ByteCount { return protocol.ByteCount(len(b.Data)) }
func (b *packetBuffer) Cap() protocol.ByteCount { return protocol.ByteCount(cap(b.Data)) }

func (b *packetBuffer) putBack() {
	if cap(b.Data) == int(b.packetSize) {
		bufferPool.Put(b)
		return
	}
	if cap(b.Data) == protocol.MaxLargePacketBufferSize {
		largeBufferPool.Put(b)
		return
	}
	panic("putPacketBuffer called with packet of wrong size!")
}

var bufferPool, largeBufferPool sync.Pool
var getBufferOnce, getLargeBufferOnce sync.Once

func getPacketBuffer(size protocol.ByteCount) *packetBuffer {
	// initialize packetBuffer constructor only once
	getBufferOnce.Do(func() {
		bufferPool.New = func() any {
			return &packetBuffer{packetSize: size, Data: make([]byte, 0, size)}
		}
	})
	buf := bufferPool.Get().(*packetBuffer)
	buf.refCount = 1
	buf.Data = buf.Data[:0]
	return buf
}

func getLargePacketBuffer(size protocol.ByteCount) *packetBuffer {
	// initialize getLargeBuffer constructor only once
	getLargeBufferOnce.Do(func() {
		largeBufferPool.New = func() any {
			return &packetBuffer{packetSize: size, Data: make([]byte, 0, size)}
		}
	})
	buf := largeBufferPool.Get().(*packetBuffer)
	buf.refCount = 1
	buf.Data = buf.Data[:0]
	return buf
}
