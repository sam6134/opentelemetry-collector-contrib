package neuron

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
# HELP neuron_runtime_memory_used_bytes Runtime memory used bytes
# TYPE neuron_runtime_memory_used_bytes gauge
neuron_runtime_memory_used_bytes{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",memory_location="host",region="us-east-1",runtime_tag="367",subnet_id="subnet-06a7754948e8a000f"} 9.043968e+06
# HELP neuroncore_utilization_ratio NeuronCore utilization ratio
# TYPE neuroncore_utilization_ratio gauge
neuroncore_utilization_ratio{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",neuroncore="0",region="us-east-1",runtime_tag="367",subnet_id="subnet-06a7754948e8a000f"} 0.1
`

const dummyInstanceId = "i-0000000000"
const dummyPodName = "podname"
const dummyNamespace = "kube-system"

type mockHostInfoProvider struct {
}

type mockPodInfoProvider struct {
}

func (m mockHostInfoProvider) GetClusterName() string {
	return "cluster-name"
}

func (m mockHostInfoProvider) GetInstanceID() string {
	return dummyInstanceId
}

func (m mockPodInfoProvider) GetPodName() string {
	return dummyPodName
}

func (m mockPodInfoProvider) GetNamespace() string {
	return dummyNamespace
}

type mockConsumer struct {
	t          *testing.T
	up         *bool
	coreUtil   *bool
	memUsed    *bool
	instanceId *bool
	podName    *bool
	namespace  *bool
	relabeled  *bool
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
		fmt.Println("!!!Metric Name is ", metric.Name())
		if metric.Name() == "neuroncore_utilization_ratio" {
			assert.Equal(m.t, float64(0.1), metric.Gauge().DataPoints().At(0).DoubleValue())
			instanceId, _ := metric.Gauge().DataPoints().At(0).Attributes().Get("InstanceId")
			podName, _ := metric.Gauge().DataPoints().At(0).Attributes().Get("PodName")
			namespace, _ := metric.Gauge().DataPoints().At(0).Attributes().Get("Namespace")
			_, relabeled := metric.Gauge().DataPoints().At(0).Attributes().Get("DeviceId")
			fmt.Println("!!!InstanceId is ", instanceId.Str())
			*m.instanceId = instanceId.Str() == dummyInstanceId
			*m.podName = podName.Str() == dummyPodName
			*m.namespace = namespace.Str() == dummyNamespace
			*m.coreUtil = true
			*m.relabeled = relabeled
		}

		if metric.Name() == "neuron_runtime_memory_used_bytes" {
			assert.Equal(m.t, float64(9.043968e+06), metric.Gauge().DataPoints().At(0).DoubleValue())
			*m.memUsed = true
		}

		if metric.Name() == "up" {
			assert.Equal(m.t, float64(1.0), metric.Gauge().DataPoints().At(0).DoubleValue())
			*m.up = true
		}
	}

	return nil
}

func TestNewNeuronScraperBadInputs(t *testing.T) {
	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	tests := []NeuronMonitorScraperOpts{
		{
			Ctx:                 context.TODO(),
			TelemetrySettings:   settings,
			Consumer:            nil,
			Host:                componenttest.NewNopHost(),
			HostInfoProvider:    mockHostInfoProvider{},
			PodNameInfoProvider: mockPodInfoProvider{},
		},
		{
			Ctx:                 context.TODO(),
			TelemetrySettings:   settings,
			Consumer:            mockConsumer{},
			Host:                nil,
			HostInfoProvider:    mockHostInfoProvider{},
			PodNameInfoProvider: mockPodInfoProvider{},
		},
		{
			Ctx:                 context.TODO(),
			TelemetrySettings:   settings,
			Consumer:            mockConsumer{},
			Host:                componenttest.NewNopHost(),
			HostInfoProvider:    nil,
			PodNameInfoProvider: mockPodInfoProvider{},
		},
	}

	for _, tt := range tests {
		scraper, err := NewNeuronMonitorScraper(tt)

		assert.Error(t, err)
		assert.Nil(t, scraper)
	}
}

func TestNewNeuronScraperEndToEnd(t *testing.T) {

	upPtr := false
	coreUtil := false
	instanceId := false
	memUsed := false
	podName := false
	namespace := false
	relabeled := false

	consumer := mockConsumer{
		t:          t,
		up:         &upPtr,
		coreUtil:   &coreUtil,
		memUsed:    &memUsed,
		instanceId: &instanceId,
		podName:    &podName,
		namespace:  &namespace,
		relabeled:  &relabeled,
	}

	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	scraper, err := NewNeuronMonitorScraper(NeuronMonitorScraperOpts{
		Ctx:                 context.TODO(),
		TelemetrySettings:   settings,
		Consumer:            mockConsumer{},
		Host:                componenttest.NewNopHost(),
		HostInfoProvider:    mockHostInfoProvider{},
		PodNameInfoProvider: mockPodInfoProvider{},
		BearerToken:         "",
	})
	assert.NoError(t, err)
	assert.Equal(t, mockHostInfoProvider{}, scraper.hostInfoProvider)

	// build up a new PR
	promFactory := prometheusreceiver.NewFactory()

	targets := []*mocks.TestData{
		{
			Name: "neuron",
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
		RelabelConfigs: []*relabel.Config{
			// doesn't seem like there is a good way to unit test relabeling rules https://github.com/prometheus/prometheus/issues/8606
			//{
			//	SourceLabels: model.LabelNames{"__address__"},
			//	Regex:        relabel.MustNewRegexp("([^:]+)(?::\\d+)?"),
			//	Replacement:  "${1}:9400",
			//	TargetLabel:  "__address__",
			//	Action:       relabel.Replace,
			//},
			//{
			//	SourceLabels: model.LabelNames{"__meta_kubernetes_namespace"},
			//	TargetLabel:  "Namespace",
			//	Regex:        relabel.MustNewRegexp("(.*)"),
			//	Replacement:  "$1",
			//	Action:       relabel.Replace,
			//},
			//{
			//	SourceLabels: model.LabelNames{"__meta_kubernetes_pod_name"},
			//	TargetLabel:  "pod",
			//	Regex:        relabel.MustNewRegexp("(.*)"),
			//	Replacement:  "$1",
			//	Action:       relabel.Replace,
			//},
			//{
			//	SourceLabels: model.LabelNames{"__meta_kubernetes_pod_node_name"},
			//	TargetLabel:  "NodeName",
			//	Regex:        relabel.MustNewRegexp("(.*)"),
			//	Replacement:  "$1",
			//	Action:       relabel.Replace,
			//},
		},
		MetricRelabelConfigs: []*relabel.Config{
			{
				SourceLabels: model.LabelNames{"__name__"},
				Regex:        relabel.MustNewRegexp("neuron.*"),
				Action:       relabel.Keep,
			},
			{
				SourceLabels: model.LabelNames{"neuroncore"},
				TargetLabel:  "DeviceId",
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  "${1}",
				Action:       relabel.Replace,
			},
			// test hack to inject cluster name as label
			{
				SourceLabels: model.LabelNames{"neuroncore"},
				TargetLabel:  "Namespace",
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  scraper.podNameInfoProvider.GetNamespace(),
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"neuroncore"},
				TargetLabel:  "ClusterName",
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  scraper.hostInfoProvider.GetClusterName(),
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"neuroncore"},
				TargetLabel:  "InstanceId",
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  scraper.hostInfoProvider.GetInstanceID(),
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"neuroncore"},
				TargetLabel:  "PodName",
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  scraper.podNameInfoProvider.GetPodName(),
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
	assert.True(t, *consumer.coreUtil)
	assert.True(t, *consumer.memUsed)
	assert.True(t, *consumer.instanceId)
	assert.True(t, *consumer.podName)
	assert.True(t, *consumer.namespace)
	assert.True(t, *consumer.relabeled)
}

func TestDcgmScraperJobName(t *testing.T) {
	// needs to start with containerInsights
	assert.True(t, strings.HasPrefix(jobName, "containerInsightsNeuronMonitorScraper"))
}
