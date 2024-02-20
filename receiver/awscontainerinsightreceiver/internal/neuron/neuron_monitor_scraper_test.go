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
# HELP python_gc_objects_collected_total Objects collected during gc
# TYPE python_gc_objects_collected_total counter
python_gc_objects_collected_total{generation="0"} 75.0
# HELP python_gc_objects_uncollectable_total Uncollectable objects found during GC
# TYPE python_gc_objects_uncollectable_total counter
python_gc_objects_uncollectable_total{generation="0"} 0.0
# HELP python_gc_collections_total Number of times this generation was collected
# TYPE python_gc_collections_total counter
python_gc_collections_total{generation="0"} 44.0
# HELP python_info Python platform information
# TYPE python_info gauge
python_info{implementation="CPython",major="3",minor="8",patchlevel="10",version="3.8.10"} 1.0
# HELP process_virtual_memory_bytes Virtual memory size in bytes.
# TYPE process_virtual_memory_bytes gauge
process_virtual_memory_bytes 1.80707328e+08
# HELP process_resident_memory_bytes Resident memory size in bytes.
# TYPE process_resident_memory_bytes gauge
process_resident_memory_bytes 2.11968e+07
# HELP process_start_time_seconds Start time of the process since unix epoch in seconds.
# TYPE process_start_time_seconds gauge
process_start_time_seconds 1.7083389395e+09
# HELP process_cpu_seconds_total Total user and system CPU time spent in seconds.
# TYPE process_cpu_seconds_total counter
process_cpu_seconds_total 0.08
# HELP process_open_fds Number of open file descriptors.
# TYPE process_open_fds gauge
process_open_fds 6.0
# HELP execution_errors_total Execution errors total
# TYPE execution_errors_total counter
execution_errors_total{availability_zone="us-east-1c",error_type="generic",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",region="us-east-1",runtime_tag="367",subnet_id="subnet-06a7754948e8a000f"} 0.0
# HELP execution_errors_created Execution errors total
# TYPE execution_errors_created gauge
execution_errors_created{availability_zone="us-east-1c",error_type="generic",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",region="us-east-1",runtime_tag="367",subnet_id="subnet-06a7754948e8a000f"} 1.7083389404380567e+09
# HELP execution_status_total Execution status total
# TYPE execution_status_total counter
execution_status_total{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",region="us-east-1",runtime_tag="367",status_type="completed",subnet_id="subnet-06a7754948e8a000f"} 0.0
# HELP execution_status_created Execution status total
# TYPE execution_status_created gauge
execution_status_created{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",region="us-east-1",runtime_tag="367",status_type="completed",subnet_id="subnet-06a7754948e8a000f"} 1.7083389404381733e+09
# HELP neuron_runtime_memory_used_bytes Runtime memory used bytes
# TYPE neuron_runtime_memory_used_bytes gauge
neuron_runtime_memory_used_bytes{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",memory_location="host",region="us-east-1",runtime_tag="367",subnet_id="subnet-06a7754948e8a000f"} 9.043968e+06
# HELP neuroncore_memory_usage_constants NeuronCore memory utilization for constants
# TYPE neuroncore_memory_usage_constants gauge
neuroncore_memory_usage_constants{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",memory_location="None",neuroncore="0",region="us-east-1",runtime_tag="367",subnet_id="subnet-06a7754948e8a000f"} 0.0
# HELP neuroncore_memory_usage_model_code NeuronCore memory utilization for model_code
# TYPE neuroncore_memory_usage_model_code gauge
neuroncore_memory_usage_model_code{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",memory_location="None",neuroncore="0",region="us-east-1",runtime_tag="367",subnet_id="subnet-06a7754948e8a000f"} 1.00752896e+08
# HELP neuroncore_memory_usage_model_shared_scratchpad NeuronCore memory utilization for model_shared_scratchpad
# TYPE neuroncore_memory_usage_model_shared_scratchpad gauge
neuroncore_memory_usage_model_shared_scratchpad{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",memory_location="None",neuroncore="0",region="us-east-1",runtime_tag="367",subnet_id="subnet-06a7754948e8a000f"} 0.0
# HELP neuroncore_memory_usage_runtime_memory NeuronCore memory utilization for runtime_memory
# TYPE neuroncore_memory_usage_runtime_memory gauge
neuroncore_memory_usage_runtime_memory{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",memory_location="None",neuroncore="0",region="us-east-1",runtime_tag="367",subnet_id="subnet-06a7754948e8a000f"} 0.0
# HELP neuroncore_memory_usage_tensors NeuronCore memory utilization for tensors
# TYPE neuroncore_memory_usage_tensors gauge
neuroncore_memory_usage_tensors{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",memory_location="None",neuroncore="0",region="us-east-1",runtime_tag="367",subnet_id="subnet-06a7754948e8a000f"} 6.315872e+06
# HELP neuroncore_utilization_ratio NeuronCore utilization ratio
# TYPE neuroncore_utilization_ratio gauge
neuroncore_utilization_ratio{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",neuroncore="0",region="us-east-1",runtime_tag="367",subnet_id="subnet-06a7754948e8a000f"} 0.1
# HELP system_memory_total_bytes System memory total_bytes bytes
# TYPE system_memory_total_bytes gauge
system_memory_total_bytes{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",region="us-east-1",subnet_id="subnet-06a7754948e8a000f"} 5.32523487232e+011
# HELP system_memory_used_bytes System memory used_bytes bytes
# TYPE system_memory_used_bytes gauge
system_memory_used_bytes{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",region="us-east-1",subnet_id="subnet-06a7754948e8a000f"} 7.6337672192e+010
# HELP system_swap_total_bytes System swap total_bytes bytes
# TYPE system_swap_total_bytes gauge
system_swap_total_bytes{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",region="us-east-1",subnet_id="subnet-06a7754948e8a000f"} 0.0
# HELP system_swap_used_bytes System swap used_bytes bytes
# TYPE system_swap_used_bytes gauge
system_swap_used_bytes{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",region="us-east-1",subnet_id="subnet-06a7754948e8a000f"} 0.0
# HELP system_vcpu_count System vCPU count
# TYPE system_vcpu_count gauge
system_vcpu_count{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",region="us-east-1",subnet_id="subnet-06a7754948e8a000f"} 128.0
# HELP system_vcpu_usage_ratio System CPU utilization ratio
# TYPE system_vcpu_usage_ratio gauge
system_vcpu_usage_ratio{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",region="us-east-1",subnet_id="subnet-06a7754948e8a000f",usage_type="user"} 0.008199999999999999
# HELP instance_info EC2 instance information
# TYPE instance_info gauge
instance_info{availability_zone="us-east-1c",instance_id="i-09db9b55e0095612f",instance_name="",instance_type="trn1n.32xlarge",region="us-east-1",subnet_id="subnet-06a7754948e8a000f"} 1.0
`

const dummyInstanceId = "i-0000000000"
const nueronCoreID = "0"

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
	coreUtil      *bool
	memUsed       *bool
	instanceId    *bool
	podName       *bool
	containerName *bool
	namespace     *bool
	relabeled     *bool
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
		if metric.Name() == "neuroncore_utilization_ratio" {
			assert.Equal(m.t, float64(0.1), metric.Gauge().DataPoints().At(0).DoubleValue())
			*m.coreUtil = true
		}

		if strings.Contains(metric.Name(), "neuroncore") {
			instanceId, _ := metric.Gauge().DataPoints().At(0).Attributes().Get("InstanceId")
			podName, _ := metric.Gauge().DataPoints().At(0).Attributes().Get("PodName")
			namespace, _ := metric.Gauge().DataPoints().At(0).Attributes().Get("Namespace")
			containerName, _ := metric.Gauge().DataPoints().At(0).Attributes().Get("Namespace")
			_, relabeled := metric.Gauge().DataPoints().At(0).Attributes().Get("DeviceId")
			*m.instanceId = instanceId.Str() == "i-09db9b55e0095612f"
			*m.podName = podName.Str() == nueronCoreID
			*m.namespace = namespace.Str() == nueronCoreID
			*m.containerName = containerName.Str() == nueronCoreID
			*m.relabeled = relabeled
		}

		if strings.Contains(metric.Name(), "system") {
			instanceId, _ := metric.Gauge().DataPoints().At(0).Attributes().Get("InstanceId")
			_, podNameFound := metric.Gauge().DataPoints().At(0).Attributes().Get("PodName")
			_, namespaceFound := metric.Gauge().DataPoints().At(0).Attributes().Get("Namespace")
			_, relabelFound := metric.Gauge().DataPoints().At(0).Attributes().Get("DeviceId")
			assert.Equal(m.t, instanceId.Str(), "i-09db9b55e0095612f")
			assert.False(m.t, podNameFound)
			assert.False(m.t, namespaceFound)
			assert.False(m.t, relabelFound)
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
	containerName := false
	namespace := false
	relabeled := false

	consumer := mockConsumer{
		t:             t,
		up:            &upPtr,
		coreUtil:      &coreUtil,
		memUsed:       &memUsed,
		instanceId:    &instanceId,
		podName:       &podName,
		containerName: &containerName,
		namespace:     &namespace,
		relabeled:     &relabeled,
	}

	settings := componenttest.NewNopTelemetrySettings()
	settings.Logger, _ = zap.NewDevelopment()

	scraper, err := NewNeuronMonitorScraper(NeuronMonitorScraperOpts{
		Ctx:               context.TODO(),
		TelemetrySettings: settings,
		Consumer:          mockConsumer{},
		Host:              componenttest.NewNopHost(),
		HostInfoProvider:  mockHostInfoProvider{},
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
		RelabelConfigs: []*relabel.Config{},
		MetricRelabelConfigs: []*relabel.Config{
			{
				SourceLabels: model.LabelNames{"__name__"},
				Regex:        relabel.MustNewRegexp("neuron.*|system_.*|execution_.*"),
				Action:       relabel.Keep,
			},
			{
				SourceLabels: model.LabelNames{"instance_id"},
				TargetLabel:  "InstanceId",
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  "${1}",
				Action:       relabel.Replace,
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
				SourceLabels: model.LabelNames{"instance_id"},
				TargetLabel:  "ClusterName",
				Regex:        relabel.MustNewRegexp("neuron.*"),
				Replacement:  scraper.hostInfoProvider.GetClusterName(),
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"neuroncore"},
				TargetLabel:  "Namespace",
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  "${1}",
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"neuroncore"},
				TargetLabel:  "PodName",
				Regex:        relabel.MustNewRegexp("(.*)"),
				Replacement:  "${1}",
				Action:       relabel.Replace,
			},
			{
				SourceLabels: model.LabelNames{"neuroncore"},
				TargetLabel:  "ContainerName",
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
	assert.True(t, *consumer.containerName)
	assert.True(t, *consumer.namespace)
	assert.True(t, *consumer.relabeled)
}

func TestDcgmScraperJobName(t *testing.T) {
	// needs to start with containerInsights
	assert.True(t, strings.HasPrefix(jobName, "containerInsightsNeuronMonitorScraper"))
}
