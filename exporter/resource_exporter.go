package exporter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"
	"github.com/vaerh/mikrotik-prom-exporter/mikrotik"
)

const DefaultMetricsCollectionInterval = 30 * time.Second

type ResourceExporter struct {
	ctx                context.Context
	schema             *ResourceSchema
	promMertics        map[string]any
	globalVars         map[string]string
	collectionInterval time.Duration
}

func (r *ResourceExporter) GetCollectInterval() time.Duration {
	return r.collectionInterval
}

func (r *ResourceExporter) SetCollectInterval(t time.Duration) {
	r.collectionInterval = t
}

func NewResourceExporter(ctx context.Context, schema *ResourceSchema, constLabels prometheus.Labels, reg *prom.Registry) *ResourceExporter {
	var exporter = &ResourceExporter{
		ctx:                ctx,
		schema:             schema,
		promMertics:        make(map[string]any),
		collectionInterval: DefaultMetricsCollectionInterval,
	}

	for _, metric := range schema.Metrics {
		var cl = make(prom.Labels, len(metric.constLabels)+len(constLabels))
		for k, v := range metric.constLabels {
			cl[k] = v
		}
		for k, v := range constLabels {
			cl[k] = v
		}

		switch metric.PromMetricType {
		case CounterVec:
			counter := promauto.NewCounterVec(prom.CounterOpts{
				Namespace:   schema.PromNamespace,
				Subsystem:   schema.PromSubsystem,
				Name:        metric.PromMetricName,
				Help:        metric.PromMetricHelp,
				ConstLabels: cl,
			}, metric.GetLabels())

			reg.MustRegister(counter)
			exporter.promMertics[metric.PromMetricName] = counter

		case GaugeVec:
			gauge := promauto.NewGaugeVec(prom.GaugeOpts{
				Namespace:   schema.PromNamespace,
				Subsystem:   schema.PromSubsystem,
				Name:        metric.PromMetricName,
				Help:        metric.PromMetricHelp,
				ConstLabels: cl,
			}, metric.GetLabels())

			reg.MustRegister(gauge)
			exporter.promMertics[metric.PromMetricName] = gauge
		}

		// FIXME
		// spew.Dump(metric.GetLabels())
	}

	return exporter
}

func (r *ResourceExporter) ExportMetrics(ctx context.Context) error {
	timer := time.NewTicker(r.collectionInterval)

	firstRun := make(chan struct{}, 1)
	firstRun <- struct{}{}

	for done := false; !done; {
		select {
		case <-firstRun:
			if err := r.exportMetrics(ctx); err != nil {
				return fmt.Errorf("exporting metrics: %w", err)
			}
		case <-timer.C:
			if err := r.exportMetrics(ctx); err != nil {
				return fmt.Errorf("exporting metrics: %w", err)
			}
		case <-ctx.Done():
			zerolog.Ctx(ctx).Debug().Msg("terminating exporter")
			done = true
		}
	}

	return nil
}

func (r *ResourceExporter) exportMetrics(ctx context.Context) error {
	logger := zerolog.Ctx(ctx)
	logger.Debug().Msg("exporting resources")

	mikrotikResource, err := r.ReadResource()
	if err != nil {
		return fmt.Errorf("reading resource: %w", err)
	}

	// Zeroize
	for _, metric := range r.schema.Metrics {
		if metric.PromResetGaugeEveryTime {
			if g, ok := r.promMertics[metric.PromMetricName].(*prom.GaugeVec); ok {
				g.Reset()
			}
		}
	}

	for _, instanceJSON := range mikrotikResource {
		// collect metrics & labels
		for _, metric := range r.schema.Metrics {
			var res any
			var err error
			// Parse value
			inVal := instanceJSON[metric.MtFieldName]
			switch strings.ToLower(metric.MtFieldType) {
			case Int:
				res, err = strconv.ParseFloat(inVal, 64)
				if err != nil {
					logger.Warn().Fields(map[string]any{metric.MtFieldName: inVal}).Err(err).Msg("extracting value from resource")
					continue
				}
			case Time:
				d, err := mikrotik.ParseDuration(inVal)
				if err != nil {
					logger.Warn().Fields(map[string]any{metric.MtFieldName: inVal}).Err(err).Msg("extracting value from resource")
					continue
				}
				res = d.Seconds()
			case Const:
				res = float64(1.0)
			case Bool:
				res = mikrotik.BoolFromMikrotikJSONToFloat(inVal)
			}

			var labels = make(prom.Labels, len(metric.labels))
			for labelName, mtFieldName := range metric.labels {
				labels[labelName] = instanceJSON[mtFieldName]
				if v, ok := r.globalVars[mtFieldName]; ok {
					labels[labelName] = v
				}
			}

			switch m := r.promMertics[metric.PromMetricName].(type) {
			case *prom.CounterVec:
				if metric.PromMetricOperation == OperAdd {
					m.With(labels).Add(res.(float64))
				} else {
					m.With(labels).Inc()
				}
			case *prom.GaugeVec:
				switch metric.PromMetricOperation {
				case OperInc:
					m.With(labels).Inc()
				case OperDec:
					m.With(labels).Dec()
				case OperAdd:
					m.With(labels).Add(res.(float64))
				case OperSub:
					m.With(labels).Sub(res.(float64))
				case OperCurrTime:
					m.With(labels).SetToCurrentTime()
				case OperSet:
					fallthrough
				default:
					m.With(labels).Set(res.(float64))
				}
			}
		}
	}
	return err
}

func (r *ResourceExporter) ReadResource() ([]mikrotik.MikrotikItem, error) {
	if len(r.schema.ResourceFilter) == 0 {
		return mikrotik.Read(r.schema.MikrotikResourcePath, mikrotik.Ctx(r.ctx), nil)
	}
	var filter []string
	for k, v := range r.schema.ResourceFilter {
		filter = append(filter, k+"="+v)
	}
	return mikrotik.ReadFiltered(filter, r.schema.MikrotikResourcePath, mikrotik.Ctx(r.ctx), nil)
}

func (r *ResourceExporter) SetGlobalVars(m map[string]string) {
	r.globalVars = m
}
