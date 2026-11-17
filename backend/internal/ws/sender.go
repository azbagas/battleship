package ws

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	defaultWriteTimeout = 5 * time.Second
	defaultOutboxSize   = 64
)

type outboundFrame struct {
	msgType string
	payload any
}

// wsSender serializes outgoing WebSocket writes for a single client connection.
// Pre-handshake frames write synchronously with a write timeout; post-handshake
// frames enqueue onto a bounded outbox so slow consumers cannot block broadcasts.
type wsSender struct {
	mu     sync.Mutex
	conn   *websocket.Conn
	ctx    context.Context
	outbox chan outboundFrame
	async  bool
	closed bool
	wg     sync.WaitGroup
}

// StartAsync transitions the sender to a bounded non-blocking outbox drained by a
// dedicated writer goroutine tied to ctx.
func (s *wsSender) StartAsync(ctx context.Context) {
	s.mu.Lock()
	if s.closed || s.async {
		s.mu.Unlock()
		return
	}
	s.async = true
	s.outbox = make(chan outboundFrame, defaultOutboxSize)
	s.wg.Add(1)
	s.mu.Unlock()

	go s.writeLoop(ctx)
}

func (s *wsSender) writeLoop(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-s.outbox:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, defaultWriteTimeout)
			err := writeJSON(writeCtx, s.conn, OutboundMessage{
				Type:    MessageType(frame.msgType),
				Payload: frame.payload,
			})
			cancel()
			if err != nil {
				s.closeOnSlowOrBroken("write failed")
				return
			}
		}
	}
}

func (s *wsSender) closeOutbox() {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		if s.async && s.outbox != nil {
			close(s.outbox)
		}
	}
	s.mu.Unlock()
}

func (s *wsSender) closeOnSlowOrBroken(reason string) {
	s.closeOutbox()
	_ = s.conn.Close(websocket.StatusGoingAway, reason)
}

// Stop closes the outbox (if active) and waits for the writer goroutine to exit.
func (s *wsSender) Stop() {
	s.closeOutbox()
	s.wg.Wait()
}

// Send formats and transmits an outbound message envelope.
func (s *wsSender) Send(msgType string, payload any) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("websocket sender closed")
	}
	if !s.async {
		defer s.mu.Unlock()
		writeCtx, cancel := context.WithTimeout(s.ctx, defaultWriteTimeout)
		defer cancel()
		return writeJSON(writeCtx, s.conn, OutboundMessage{
			Type:    MessageType(msgType),
			Payload: payload,
		})
	}

	select {
	case s.outbox <- outboundFrame{msgType: msgType, payload: payload}:
		s.mu.Unlock()
		return nil
	default:
		// Outbox is full: client is a slow consumer; detach it without blocking callers.
		s.closed = true
		close(s.outbox)
		s.mu.Unlock()
		_ = s.conn.Close(websocket.StatusGoingAway, "slow consumer")
		return errors.New("outbound buffer full: slow consumer")
	}
}
