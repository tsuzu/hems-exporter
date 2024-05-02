package fetcher

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"time"

	smartmeter "github.com/hnw/go-smartmeter"
	"github.com/tsuzu/hems-exporter/metrics"
)

type Fetcher struct {
	device  *smartmeter.Device
	metrics metrics.Exporter
}

func NewFetcher(device *smartmeter.Device, metrics metrics.Exporter) *Fetcher {
	return &Fetcher{
		device:  device,
		metrics: metrics,
	}
}

func (f *Fetcher) Prepare(ctx context.Context) error {
	if f.device.Channel == "" {
		err := f.device.Scan(
			// In my environment, it takes about 40~50 seconds to scan.
			smartmeter.Timeout(60 * time.Second),
		)
		if err != nil {
			return fmt.Errorf("failed to scan: %w", err)
		}
	}
	if f.device.Channel == "" {
		// neighbor?
		ipAddr, err := f.device.GetNeibourIP()
		if err != nil {
			return fmt.Errorf("failed to get neighbor ip: %w", err)
		}

		f.device.IPAddr = ipAddr
	}

	return nil
}

func (f *Fetcher) Run(ctx context.Context) (err error) {
	defer func() {
		if err != nil {
			f.metrics.ReportFailure()
		}
	}()

	if err := f.Prepare(ctx); err != nil {
		return fmt.Errorf("failed to prepare: %w", err)
	}

	request := smartmeter.NewFrame(smartmeter.LvSmartElectricEnergyMeter, smartmeter.Get, []*smartmeter.Property{
		smartmeter.NewProperty(smartmeter.LvSmartElectricEnergyMeter_InstantaneousElectricPower, nil),
		smartmeter.NewProperty(smartmeter.LvSmartElectricEnergyMeter_InstantaneousCurrent, nil),
	})

	var response *smartmeter.Frame

	var counter int
	for {
		response, err = f.device.QueryEchonetLite(request, smartmeter.Retry(3))
		if err == nil {
			break
		}
		slog.Error("failed to query echonet lite", "error", err)

		if counter >= 2 {
			return fmt.Errorf("failed to query echonet lite: %w", err)
		}
		counter++

		err = f.device.Authenticate()
		if err != nil {
			return fmt.Errorf("failed to authenticate: %w", err)
		}
	}

	if len(response.Properties) == 0 {
		return fmt.Errorf("no property in response")
	}

	var power, rCurrent, tCurrent float64
	for _, p := range response.Properties {
		switch p.EPC {
		case smartmeter.LvSmartElectricEnergyMeter_InstantaneousElectricPower:
			// 瞬時電力計測値
			power = float64(int32(binary.BigEndian.Uint32(p.EDT)))
		case smartmeter.LvSmartElectricEnergyMeter_InstantaneousCurrent:
			// 瞬時電流計測値
			rCurrent = float64(int16(binary.BigEndian.Uint16(p.EDT[:2]))) / 10.0
			tCurrent = float64(int16(binary.BigEndian.Uint16(p.EDT[2:]))) / 10.0
		}
	}

	slog.Info("fetched power consumption", "power", power, "rCurrent", rCurrent, "tCurrent", tCurrent)

	f.metrics.ReportSuccess(power, rCurrent, tCurrent)

	return nil
}
