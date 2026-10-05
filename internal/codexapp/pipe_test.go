package codexapp

import (
	"io"
)

// pipeTransport is a Transport backed by io.Pipe pairs for tests.
// The reader/writer halves correspond to what the *Client* sees:
//
//	client.Reader() -> serverWrite (server writes here)
//	client.Writer() -> serverRead  (server reads here)
type pipeTransport struct {
	clientRead  *io.PipeReader
	clientWrite *io.PipeWriter
}

func (p *pipeTransport) Reader() io.Reader { return p.clientRead }
func (p *pipeTransport) Writer() io.Writer { return p.clientWrite }

func (p *pipeTransport) Close() error {
	_ = p.clientWrite.Close()
	_ = p.clientRead.Close()
	return nil
}

// newPipePair returns a Client-side transport plus the server-side
// reader/writer it should use to respond.
func newPipePair() (transport *pipeTransport, serverRead *io.PipeReader, serverWrite *io.PipeWriter) {
	cr, sw := io.Pipe() // server -> client
	sr, cw := io.Pipe() // client -> server
	return &pipeTransport{clientRead: cr, clientWrite: cw}, sr, sw
}
