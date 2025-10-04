package main

import (
	"context"
	"immo-lux/internal/config"
	"immo-lux/internal/server"
	"immo-lux/internal/server_error"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	serverConfig, err := parseServerConfig()
	if err != nil {
		return server_error.Wrap("MAIN", "failed to get configuration", err)
	}

	httpServer, err := server.New(serverConfig)
	if err != nil {
		return server_error.Wrap("MAIN", "failed to create server", err)
	}
	defer func() {
		if closeErrors := httpServer.Close(); len(closeErrors) > 0 {
			for _, closeErr := range closeErrors {
				slog.Error("Failed to close server resources", "error", closeErr)
			}
		}
	}()
	httpServer.RegisterRoutes()

	listeningErrorChannel := make(chan error, 1)
	go func() {
		if listenErr := httpServer.Listen(); listenErr != nil {
			listeningErrorChannel <- server_error.Wrap("MAIN", "server failed to start", listenErr)
		}
		close(listeningErrorChannel)
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signalCh)

	select {
	case listenErr := <-listeningErrorChannel:
		if listenErr != nil {
			return listenErr
		}
	case <-signalCh:
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Minute)
	defer shutdownCancel()

	if err = httpServer.Shutdown(shutdownCtx); err != nil {
		return server_error.Wrap("MAIN", "failed to shutdown server gracefully", err)
	}

	slog.Info("Server shutdown successfully")

	return nil
}

func parseServerConfig() (*config.ServerConfig, error) {
	if len(os.Args) < 2 {
		return nil, server_error.New("INIT", "missing configuration file path as 1st argument")
	}
	file, err := os.ReadFile(os.Args[1])
	if err != nil {
		return nil, server_error.Wrap("INIT", "error when reading config file", err)
	}
	return config.GetServerConfig(file)
}
