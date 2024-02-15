// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package neuron

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/relabel"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
)

const (
	caFile             = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	collectionInterval = 60 * time.Second
	jobName            = "containerInsightsNeuronMonitorScraper"
)

type NeuronMonitorScraper struct {
	ctx                 context.Context
	settings            component.TelemetrySettings
	host                component.Host
	hostInfoProvider    hostInfoProvider
	podNameInfoProvider podNameInfoProvider
	prometheusReceiver  receiver.Metrics
	running             bool
}

type NeuronMonitorScraperOpts struct {
	Ctx                 context.Context
	TelemetrySettings   component.TelemetrySettings
	Consumer            consumer.Metrics
	Host                component.Host
	HostInfoProvider    hostInfoProvider
	PodNameInfoProvider podNameInfoProvider
	BearerToken         string
}

type hostInfoProvider interface {
	GetClusterName() string
	GetInstanceID() string
}

// lets assume new provider will provide PodName
type podNameInfoProvider interface {
	GetPodName() string
	GetNamespace() string
}

func NewNeuronMonitorScraper(opts NeuronMonitorScraperOpts) (*NeuronMonitorScraper, error) {
	if opts.Consumer == nil {
		return nil, errors.New("consumer cannot be nil")
	}
	if opts.Host == nil {
		return nil, errors.New("host cannot be nil")
	}
	if opts.HostInfoProvider == nil {
		return nil, errors.New("cluster name provider cannot be nil")
	}
	if opts.PodNameInfoProvider == nil {
		return nil, errors.New("pod name provider cannot be nil")
	}

	scrapeConfig := &config.ScrapeConfig{
		// TLS needs to be enabled between pods communication
		// It can further get restricted by adding authentication mechanism to limit the data
		//HTTPClientConfig: configutil.HTTPClientConfig{
		//	BasicAuth: &configutil.BasicAuth{
		//		Username: "",
		//		Password: "",
		//	},
		//	Authorization: &configutil.Authorization{
		//		Type: "basic_auth",
		//	},
		//	TLSConfig: configutil.TLSConfig{
		//		CAFile:             caFile,
		//		InsecureSkipVerify: false,
		//	},
		//},
		ScrapeInterval: model.Duration(collectionInterval),
		ScrapeTimeout:  model.Duration(collectionInterval),
		JobName:        jobName,
		Scheme:         "http",
		MetricsPath:    "/metrics",
		ServiceDiscoveryConfigs: discovery.Configs{
			&kubernetes.SDConfig{
				Role: kubernetes.RoleEndpoint,
				NamespaceDiscovery: kubernetes.NamespaceDiscovery{
					IncludeOwnNamespace: true,
				},
				Selectors: []kubernetes.SelectorConfig{
					{
						Role:  kubernetes.RoleEndpoint,
						Label: "k8s-app=dcgm-exporter-service", // TODO: Find the correct name
					},
				},
				AttachMetadata: kubernetes.AttachMetadataConfig{
					Node: true,
				},
			},
		},
		RelabelConfigs: []*relabel.Config{
			{
				SourceLabels: model.LabelNames{"__address__"},
				Regex:        relabel.MustNewRegexp("([^:]+)(?::\\d+)?"),
				Replacement:  "${1}:9400",
				TargetLabel:  "__address__",
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"__meta_kubernetes_pod_node_name"},
				TargetLabel:  "NodeName",
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  "$1",
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"__meta_kubernetes_service_name"},
				TargetLabel:  "Service",
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  "$1",
				Action:       relabel.Replace,
			},
		},
		MetricRelabelConfigs: []*relabel.Config{
			{
				SourceLabels: model.LabelNames{"__name__"},
				Regex:        relabel.MustNewRegexp("neuron.*"),
				Action:       relabel.Keep,
			},
			// hacky way to inject static values (clusterName & instanceId) to label set without additional processor
			// relabel looks up an existing label then creates another label with given key (TargetLabel) and value (static)
			{
				SourceLabels: model.LabelNames{"__name__"},
				TargetLabel:  "Namespace",
				Regex:        relabel.MustNewRegexp("neuron.*"),
				Replacement:  opts.PodNameInfoProvider.GetNamespace(),
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"__name__"},
				TargetLabel:  "ClusterName",
				Regex:        relabel.MustNewRegexp("neuron.*"),
				Replacement:  opts.HostInfoProvider.GetClusterName(),
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"__name__"},
				TargetLabel:  "InstanceId",
				Regex:        relabel.MustNewRegexp("neuron.*"),
				Replacement:  opts.HostInfoProvider.GetInstanceID(),
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"namespace"},
				TargetLabel:  "PodName",
				Regex:        relabel.MustNewRegexp("neuron.*"),
				Replacement:  opts.PodNameInfoProvider.GetPodName(),
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"neuroncore"},
				TargetLabel:  "DeviceId",
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  "${1}",
				Action:       relabel.Replace,
			},
		},
	}

	promConfig := prometheusreceiver.Config{
		PrometheusConfig: &config.Config{
			ScrapeConfigs: []*config.ScrapeConfig{scrapeConfig},
		},
	}

	params := receiver.CreateSettings{
		TelemetrySettings: opts.TelemetrySettings,
	}

	promFactory := prometheusreceiver.NewFactory()
	promReceiver, err := promFactory.CreateMetricsReceiver(opts.Ctx, params, &promConfig, opts.Consumer)
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus receiver: %w", err)
	}

	return &NeuronMonitorScraper{
		ctx:                 opts.Ctx,
		settings:            opts.TelemetrySettings,
		host:                opts.Host,
		hostInfoProvider:    opts.HostInfoProvider,
		podNameInfoProvider: opts.PodNameInfoProvider,
		prometheusReceiver:  promReceiver,
	}, nil
}

func (ds *NeuronMonitorScraper) GetMetrics() []pmetric.Metrics {
	// This method will never return metrics because the metrics are collected by the scraper.
	// This method will ensure the scraper is running
	if !ds.running {
		ds.settings.Logger.Info("The scraper is not running, starting up the scraper")
		err := ds.prometheusReceiver.Start(ds.ctx, ds.host)
		if err != nil {
			ds.settings.Logger.Error("Unable to start PrometheusReceiver", zap.Error(err))
		}
		ds.running = err == nil
	}
	return nil
}

func (ds *NeuronMonitorScraper) Shutdown() {
	if ds.running {
		err := ds.prometheusReceiver.Shutdown(ds.ctx)
		if err != nil {
			ds.settings.Logger.Error("Unable to shutdown PrometheusReceiver", zap.Error(err))
		}
		ds.running = false
	}
}
