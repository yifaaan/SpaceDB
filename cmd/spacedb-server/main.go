package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"spacedb/engine"
	"spacedb/executor"
	"spacedb/storage"
)

const (
	defaultAddress = "127.0.0.1:8080"
	defaultDBPath  = "spacedb.log"

	maxRequestSize = 1024 * 1024
)

type Server struct {
	engine engine.Engine
	logger *slog.Logger

	clients sync.WaitGroup
}

func NewServer(db engine.Engine, logger *slog.Logger) *Server {
	return &Server{
		engine: db,
		logger: logger,
	}
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	stop := context.AfterFunc(ctx, func() {
		_ = listener.Close()
	})
	defer stop()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				s.clients.Wait()
				return nil
			}
			return fmt.Errorf("server: accepting connection: %w", err)
		}

		s.clients.Go(func() {
			defer conn.Close()

			stopConn := context.AfterFunc(ctx, func() {
				_ = conn.Close()
			})
			defer stopConn()

			if err := s.handleConnection(ctx, conn); err != nil &&
				ctx.Err() == nil {
				s.logger.Error(
					"connection failed",
					"remote", conn.RemoteAddr(),
					"error", err,
				)
			}
		})
	}
}

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) error {
	session := engine.NewSession(s.engine)

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), maxRequestSize)

	writer := bufio.NewWriter(conn)

	for scanner.Scan() {
		sql := strings.TrimSpace(scanner.Text())
		if sql == "" {
			continue
		}

		result, err := session.Execute(sql)

		var response string
		if err != nil {
			response = "ERROR: " + err.Error()

			s.logger.Warn(
				"SQL execution failed",
				"remote", conn.RemoteAddr(),
				"sql", sql,
				"error", err,
			)
		} else {
			response = executor.FormatResult(result)

			s.logger.Info(
				"SQL executed",
				"remote", conn.RemoteAddr(),
				"sql", sql,
			)
		}

		if _, err := writer.WriteString(response + "\n"); err != nil {
			return fmt.Errorf("server: writing response: %w", err)
		}
		if err := writer.Flush(); err != nil {
			return fmt.Errorf("server: flushing response: %w", err)
		}
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("server: reading request: %w", err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	address := defaultAddress
	databasePath := defaultDBPath

	if len(os.Args) >= 2 {
		address = os.Args[1]
	}
	if len(os.Args) >= 3 {
		databasePath = os.Args[2]
	}
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		return fmt.Errorf("server: creating database directory: %w", err)
	}

	disk, err := storage.NewDiskEngine(databasePath)
	if err != nil {
		return fmt.Errorf("server: opening database: %w", err)
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return errors.Join(fmt.Errorf("server: listening on %s: %w", address, err), disk.Close())
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	logger.Info(
		"SpaceDB server started",
		"address", listener.Addr(),
		"database", databasePath,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := NewServer(engine.NewKVEngine(disk), logger)
	serveErr := server.Serve(ctx, listener)

	return errors.Join(serveErr, disk.Close())
}
