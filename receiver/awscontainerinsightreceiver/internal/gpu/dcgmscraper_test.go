// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gpu

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/mocks"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver"
	configutil "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

const renameMetric = `
# HELP DCGM_FI_DEV_GPU_TEMP GPU temperature (in C).
# TYPE DCGM_FI_DEV_GPU_TEMP gauge
DCGM_FI_DEV_GPU_TEMP{gpu="0",UUID="uuid",device="nvidia0",modelName="NVIDIA A10G",Hostname="hostname",DCGM_FI_DRIVER_VERSION="470.182.03",container="main",namespace="kube-system",pod="fullname-hash"} 65
# HELP DCGM_FI_DEV_GPU_UTIL GPU utilization (in %).
# TYPE DCGM_FI_DEV_GPU_UTIL gauge
DCGM_FI_DEV_GPU_UTIL{gpu="0",UUID="uuid",device="nvidia0",modelName="NVIDIA A10G",Hostname="hostname",DCGM_FI_DRIVER_VERSION="470.182.03",container="main",namespace="kube-system",pod="fullname-hash"} 100
`

const dummyInstanceId = "i-0000000000"

type mockHostInfoProvider struct {
}

func (m mockHostInfoProvider) GetClusterName() string {
	return "cluster-name"
}

func (m mockHostInfoProvider) GetInstanceID() string {
	return dummyInstanceId
}

type mockConsumer struct {
	t             *testing.T
	up            *bool
	gpuTemp       *bool
	gpuUtil       *bool
	relabeled     *bool
	podNameParsed *bool
	instanceId    *bool
}

func (m mockConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (m mockConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	assert.Equal(m.t, 1, md.ResourceMetrics().Len())

	scopeMetrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for i := 0; i < scopeMetrics.Len(); i++ {
		metric := scopeMetrics.At(i)
		if metric.Name() == "DCGM_FI_DEV_GPU_UTIL" {
			assert.Equal(m.t, float64(100), metric.Gauge().DataPoints().At(0).DoubleValue())
			*m.gpuUtil = true
			instanceId, _ := metric.Gauge().DataPoints().At(0).Attributes().Get("InstanceId")
			*m.instanceId = instanceId.Str() == dummyInstanceId
		}
		if metric.Name() == "DCGM_FI_DEV_GPU_TEMP" {
			*m.gpuTemp = true
			fullPodName, relabeled := metric.Gauge().DataPoints().At(0).Attributes().Get("FullPodName")
			splits := strings.Split(fullPodName.Str(), "-")
			podName, _ := metric.Gauge().DataPoints().At(0).Attributes().Get("PodName")
			*m.podNameParsed = podName.Str() == splits[0]
			*m.relabeled = relabeled
		}
		if metric.Name() == "up" {
			assert.Equal(m.t, float64(1), metric.Gauge().DataPoints().At(0).DoubleValue())
			*m.up = true
		}
	}

	return nil
}

func TestNewDcgmScraperBadInputs(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	tests := []DcgmScraperOpts{
		{
			Ctx:               context.TODO(),
			TelemetrySettings: settings,
			Consumer:          nil,
			Host:              componenttest.NewNopHost(),
			HostInfoProvider:  mockHostInfoProvider{},
		},
		{
			Ctx:               context.TODO(),
			TelemetrySettings: settings,
			Consumer:          mockConsumer{},
			Host:              nil,
			HostInfoProvider:  mockHostInfoProvider{},
		},
		{
			Ctx:               context.TODO(),
			TelemetrySettings: settings,
			Consumer:          mockConsumer{},
			Host:              componenttest.NewNopHost(),
			HostInfoProvider:  nil,
		},
	}

	for _, tt := range tests {
		scraper, err := NewDcgmScraper(tt)

		assert.Error(t, err)
		assert.Nil(t, scraper)
	}
}

func TestNewDcgmScraperEndToEnd(t *testing.T) {

	upPtr := false
	gpuTemp := false
	gpuUtil := false
	relabeledPod := false
	podNameParsed := false
	instanceId := false

	consumer := mockConsumer{
		t:             t,
		up:            &upPtr,
		gpuTemp:       &gpuTemp,
		gpuUtil:       &gpuUtil,
		relabeled:     &relabeledPod,
		podNameParsed: &podNameParsed,
		instanceId:    &instanceId,
	}

	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	scraper, err := NewDcgmScraper(DcgmScraperOpts{
		Ctx:               context.TODO(),
		TelemetrySettings: settings,
		Consumer:          mockConsumer{},
		Host:              componenttest.NewNopHost(),
		HostInfoProvider:  mockHostInfoProvider{},
		BearerToken:       "",
	})
	assert.NoError(t, err)
	assert.Equal(t, mockHostInfoProvider{}, scraper.hostInfoProvider)

	// build up a new PR
	promFactory := prometheusreceiver.NewFactory()

	targets := []*mocks.TestData{
		{
			Name: "dcgm",
			Pages: []mocks.MockPrometheusResponse{
				{Code: 200, Data: renameMetric},
			},
		},
	}
	mp, cfg, err := mocks.SetupMockPrometheus(targets...)
	assert.NoError(t, err)

	split := strings.Split(mp.Srv.URL, "http://")

	scrapeConfig := &config.ScrapeConfig{
		HTTPClientConfig: configutil.HTTPClientConfig{
			TLSConfig: configutil.TLSConfig{
				InsecureSkipVerify: true,
			},
		},
		ScrapeInterval:  cfg.ScrapeConfigs[0].ScrapeInterval,
		ScrapeTimeout:   cfg.ScrapeConfigs[0].ScrapeInterval,
		JobName:         fmt.Sprintf("%s/%s", jobName, cfg.ScrapeConfigs[0].MetricsPath),
		HonorTimestamps: true,
		Scheme:          "http",
		MetricsPath:     cfg.ScrapeConfigs[0].MetricsPath,
		ServiceDiscoveryConfigs: discovery.Configs{
			// using dummy static config to avoid service discovery initialization
			&discovery.StaticConfig{
				{
					Targets: []model.LabelSet{
						{
							model.AddressLabel: model.LabelValue(split[1]),
						},
					},
				},
			},
		},
		RelabelConfigs: []*relabel.Config{},
		MetricRelabelConfigs: []*relabel.Config{
			{
				SourceLabels: model.LabelNames{"__name__"},
				Regex:        relabel.MustNewRegexp("DCGM_.*"),
				Action:       relabel.Keep,
			},
			// test hack to inject cluster name as label
			{
				SourceLabels: model.LabelNames{"namespace"},
				TargetLabel:  "InstanceId",
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  scraper.hostInfoProvider.GetInstanceID(),
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"pod"},
				TargetLabel:  "FullPodName",
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  "${1}",
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"pod"},
				TargetLabel:  "PodName",
				Regex:        relabel.MustNewRegexp("(.+)-(.+)"),
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

	// replace the prom receiver
	params := receiver.CreateSettings{
		TelemetrySettings: scraper.settings,
	}
	scraper.prometheusReceiver, err = promFactory.CreateMetricsReceiver(scraper.ctx, params, &promConfig, consumer)
	assert.NoError(t, err)
	assert.NotNil(t, mp)
	defer mp.Close()

	// perform a single scrape, this will kick off the scraper process for additional scrapes
	scraper.GetMetrics()

	t.Cleanup(func() {
		scraper.Shutdown()
	})

	// wait for 2 scrapes, one initiated by us, another by the new scraper process
	mp.Wg.Wait()
	mp.Wg.Wait()

	assert.True(t, *consumer.up)
	assert.True(t, *consumer.gpuTemp)
	assert.True(t, *consumer.gpuUtil)
	assert.True(t, *consumer.relabeled)
	assert.True(t, *consumer.podNameParsed)
	assert.True(t, *consumer.instanceId)
}

func TestDcgmScraperJobName(t *testing.T) {
	// needs to start with containerInsights
	assert.True(t, strings.HasPrefix(jobName, "containerInsightsDCGMExporterScraper"))
}
