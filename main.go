package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"github.com/mickamy/tapbox/internal/config"
	grpcproxy "github.com/mickamy/tapbox/internal/proxy/grpc"
	httpproxy "github.com/mickamy/tapbox/internal/proxy/http"
	sqlproxy "github.com/mickamy/tapbox/internal/proxy/sql"
	"github.com/mickamy/tapbox/internal/trace"
	"github.com/mickamy/tapbox/internal/ui"
)

var version = "dev"

const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 5 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "tapbox: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--version") || strings.HasPrefix(arg, "-version") || arg == "-v" {
			fmt.Println("tapbox " + version)
			return nil
		}
	}

	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	store := trace.NewMemStore(cfg.MaxTraces)
	collector := trace.NewCollector(store)
	correlator := sqlproxy.NewCorrelator(0)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// HTTP proxy
	httpProxy, err := httpproxy.NewProxy(cfg.HTTPTarget, collector, cfg.MaxBodySize)
	if err != nil {
		return fmt.Errorf("http proxy: %w", err)
	}
	httpProxy.OnSpan = func(traceID, spanID string) {
		correlator.SetActive(traceID, spanID)
	}
	httpProxy.OnSpanEnd = func(traceID, spanID string) {
		correlator.ClearActive(traceID, spanID)
	}

	g.Go(func() error {
		srv := &http.Server{
			Addr:              cfg.HTTPListen,
			Handler:           httpProxy,
			ReadHeaderTimeout: readHeaderTimeout,
		}
		go func() {
			<-ctx.Done()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer shutdownCancel()
			if closeErr := srv.Shutdown(shutdownCtx); closeErr != nil { //nolint:contextcheck // fresh context for shutdown
				log.Printf("http proxy shutdown: %v", closeErr)
			}
		}()
		log.Printf("HTTP proxy listening on %s -> %s", cfg.HTTPListen, cfg.HTTPTarget)
		if srvErr := srv.ListenAndServe(); srvErr != nil && !errors.Is(srvErr, http.ErrServerClosed) {
			return fmt.Errorf("http proxy: %w", srvErr)
		}
		return nil
	})

	// gRPC proxy
	if cfg.EnableGRPC {
		gp, gpErr := grpcproxy.NewProxy(cfg.GRPCTarget, collector)
		if gpErr != nil {
			return fmt.Errorf("grpc proxy: %w", gpErr)
		}
		gp.OnSpan = func(traceID, spanID string) {
			correlator.SetActive(traceID, spanID)
		}
		gp.OnSpanEnd = func(traceID, spanID string) {
			correlator.ClearActive(traceID, spanID)
		}
		grpcServer := grpc.NewServer(grpc.UnknownServiceHandler(gp.UnknownHandler()))

		g.Go(func() error {
			var lc net.ListenConfig
			ln, lisErr := lc.Listen(ctx, "tcp", cfg.GRPCListen)
			if lisErr != nil {
				return fmt.Errorf("grpc listen: %w", lisErr)
			}
			go func() {
				<-ctx.Done()
				grpcServer.GracefulStop()
				if closeErr := gp.Close(); closeErr != nil {
					log.Printf("grpc proxy close: %v", closeErr)
				}
			}()
			log.Printf("gRPC proxy listening on %s -> %s", cfg.GRPCListen, cfg.GRPCTarget)
			return grpcServer.Serve(ln)
		})
	}

	// SQL proxy
	if cfg.EnableSQL {
		sp := sqlproxy.NewProxy(cfg.SQLTarget, collector, correlator)
		if lisErr := sp.Listen(ctx, cfg.SQLListen); lisErr != nil {
			return fmt.Errorf("sql proxy: %w", lisErr)
		}
		g.Go(func() error {
			go func() {
				<-ctx.Done()
				if closeErr := sp.Close(); closeErr != nil {
					log.Printf("sql proxy close: %v", closeErr)
				}
			}()
			log.Printf("SQL proxy listening on %s -> %s", cfg.SQLListen, cfg.SQLTarget)
			return sp.Serve()
		})
	}

	// UI server
	uiServer := ui.NewServer(store, cfg.ExplainDSN)
	uiServer.Start(ctx)

	g.Go(func() error {
		srv := &http.Server{
			Addr:              cfg.UIListen,
			Handler:           uiServer.Handler(),
			ReadHeaderTimeout: readHeaderTimeout,
		}
		go func() {
			<-ctx.Done()
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer shutdownCancel()
			if closeErr := srv.Shutdown(shutdownCtx); closeErr != nil { //nolint:contextcheck // fresh context for shutdown
				log.Printf("ui server shutdown: %v", closeErr)
			}
		}()
		if strings.HasPrefix(cfg.UIListen, ":") {
			log.Printf("UI available at http://localhost%s", cfg.UIListen)
		} else {
			log.Printf("UI available at http://%s", cfg.UIListen)
		}
		if srvErr := srv.ListenAndServe(); srvErr != nil && !errors.Is(srvErr, http.ErrServerClosed) {
			return fmt.Errorf("ui server: %w", srvErr)
		}
		return nil
	})

	log.Printf("tapbox %s started", version)

	if waitErr := g.Wait(); waitErr != nil {
		return fmt.Errorf("server error: %w", waitErr)
	}
	return nil
}
