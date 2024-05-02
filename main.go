package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	smartmeter "github.com/hnw/go-smartmeter"
	"github.com/tsuzu/hems-exporter/fetcher"
	"github.com/tsuzu/hems-exporter/metrics"
)

var (
	bRouteID       string
	bRoutePassword string
	device         string
	port           string
	interval       time.Duration
	disableDSE     bool
)

func init() {
	bRouteID = os.Getenv("B_ROUTE_ID")
	bRoutePassword = os.Getenv("B_ROUTE_PASSWORD")
	device = os.Getenv("DEVICE")
	port = os.Getenv("PORT")
	interval, _ = time.ParseDuration(os.Getenv("INTERVAL"))
	disableDSE, _ = strconv.ParseBool(os.Getenv("DISABLE_DSE"))

	if port == "" {
		port = "8080"
	}
	if interval == 0 {
		interval = 30 * time.Second
	}

	flag.StringVar(&bRouteID, "b-route-id", bRouteID, "B-route ID")
	flag.StringVar(&bRoutePassword, "b-route-password", bRoutePassword, "B-route password")
	flag.StringVar(&device, "device", device, "device")
	flag.StringVar(&port, "port", port, "port")
	flag.DurationVar(&interval, "interval", interval, "interval")
	flag.BoolVar(&disableDSE, "disable-dse", disableDSE, "disable DSE")
	flag.Parse()

	if bRouteID == "" {
		slog.Error("B-route ID is required")
		os.Exit(1)
	}
	if bRoutePassword == "" {
		slog.Error("B-route password is required")
		os.Exit(1)
	}
	if device == "" {
		slog.Error("device is required")
		os.Exit(1)
	}
}

func main() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{})
	logger := slog.New(handler)
	logLogger := slog.NewLogLogger(handler, slog.LevelInfo)

	slog.SetDefault(logger)

	logger.Info("initializing")

	sm, err := smartmeter.Open(
		device,
		smartmeter.ID(bRouteID),
		smartmeter.Password(bRoutePassword),
		smartmeter.DualStackSK(!disableDSE),
		smartmeter.Logger(logLogger),
	)
	if err != nil {
		slog.Error("failed to open smartmeter", "error", err)
		os.Exit(1)
	}
	// BUG: Close() is not supported on smartmeter.Device

	exporter := metrics.NewExporter()
	fetcher := fetcher.NewFetcher(sm, exporter)

	for {
		err := fetcher.Prepare(context.Background())

		if err == nil {
			break
		}

		slog.Error("failed to prepare", "error", err)
	}

	http.Handle("/metrics", exporter)
	go http.ListenAndServe(":"+port, nil)

	slog.Info("smartmeter is opened")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	for {
		if err := fetcher.Run(ctx); err != nil {
			slog.Error("failed to fetch", "error", err)
		}

		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()

			slog.Info("shutting down")
			return
		}
	}
}
