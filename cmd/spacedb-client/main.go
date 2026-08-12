package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
)

const defaultAddress = "127.0.0.1:8080"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	address := defaultAddress
	if len(os.Args) >= 2 {
		address = os.Args[1]
	}

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("client: connecting to %s: %w", address, err)
	}
	defer conn.Close()

	tcpConn := conn.(*net.TCPConn)

	responseDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(os.Stdout, tcpConn)
		if err != nil {
			err = fmt.Errorf("client: reading response: %w", err)
		}
		responseDone <- err
	}()

	_, requestErr := io.Copy(tcpConn, os.Stdin)
	if requestErr != nil {
		requestErr = fmt.Errorf("client: sending request: %w", requestErr)
	}

	closeWriteErr := tcpConn.CloseWrite()
	if closeWriteErr != nil {
		closeWriteErr = fmt.Errorf("client: closing request stream: %w", closeWriteErr)
	}

	responseErr := <-responseDone

	return errors.Join(requestErr, closeWriteErr, responseErr)
}
