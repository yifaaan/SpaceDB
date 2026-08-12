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
	"spacedb/parser"
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

			if err := s.handleConnection(ctx, conn); err != nil && ctx.Err() != nil {
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
	// 每个连接拥有自己的 Session。
	//
	// Session 本身不长期持有事务。每次 Execute 都会：
	// Begin -> Execute -> Commit/Rollback。
	session := engine.NewSession(s.engine)
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), maxRequestSize)

	for scanner.Scan() {
		sql := strings.TrimSpace(scanner.Text())
		if sql == "" {
			continue
		}

		result, err := session.Execute(sql)
		if err != nil {
			s.logger.Warn(
				"SQL execution failed",
				"remote", conn.RemoteAddr(),
				"sql", sql,
				"error", err,
			)
			continue
		}

		// 当前历史阶段只在服务端观察结果。
		// 下一步实现 Client 时，再定义结果编码和写回协议。
		s.logger.Info(
			"SQL executed",
			"remote", conn.RemoteAddr(),
			"sql", sql,
			"result", result,
		)
	}

	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("server: reading request: %w", err)
	}

	return nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: spacedb <sql>")
		os.Exit(2)
	}

	statement, err := parser.Parse(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%T: %v\n", statement, statement)
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
