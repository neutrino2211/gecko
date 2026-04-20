package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"go.lsp.dev/jsonrpc2"
)

func main() {
	logFile, err := os.OpenFile(filepath.Join(os.TempDir(), "gecko-lsp.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}
	log.SetFlags(log.Lshortfile | log.Ltime)
	log.Println("Gecko LSP server starting...")

	ctx := context.Background()

	server := NewServer()

	stream := jsonrpc2.NewStream(&readWriteCloser{
		reader: os.Stdin,
		writer: os.Stdout,
	})

	conn := jsonrpc2.NewConn(stream)
	server.conn = conn

	conn.Go(ctx, server.Handle)
	<-conn.Done()

	if err := conn.Err(); err != nil {
		log.Printf("Connection error: %v", err)
		os.Exit(1)
	}
}

type readWriteCloser struct {
	reader *os.File
	writer *os.File
}

func (rwc *readWriteCloser) Read(p []byte) (int, error) {
	return rwc.reader.Read(p)
}

func (rwc *readWriteCloser) Write(p []byte) (int, error) {
	return rwc.writer.Write(p)
}

func (rwc *readWriteCloser) Close() error {
	if err := rwc.reader.Close(); err != nil {
		return err
	}
	return rwc.writer.Close()
}
