package nueron

import (
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/prometheusscraper"
	"github.com/stretchr/testify/assert"
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

const dummyClusterName = "cluster-name"
const dummyHostName = "i-000000000"

type mockHostInfoProvider struct {
}

func (m mockHostInfoProvider) GetClusterName() string {
	return dummyClusterName
}

func (m mockHostInfoProvider) GetInstanceID() string {
	return dummyHostName
}

func TestNewNeuronScraperEndToEnd(t *testing.T) {
	expectedMetrics := make(map[string]prometheusscraper.ExpectedMetricStruct)
	expectedMetrics["neuroncore_utilization_ratio"] = prometheusscraper.ExpectedMetricStruct{
		MetricValue: 0.1,
		MetricLabels: []prometheusscraper.MetricLabel{
			{LabelName: "InstanceId", LabelValue: "i-09db9b55e0095612f"},
			{LabelName: "ClusterName", LabelValue: dummyClusterName},
			{LabelName: "DeviceId", LabelValue: "0"},
		},
	}
	expectedMetrics["neuron_runtime_memory_used_bytes"] = prometheusscraper.ExpectedMetricStruct{
		MetricValue: 9.043968e+06,
		MetricLabels: []prometheusscraper.MetricLabel{
			{LabelName: "InstanceId", LabelValue: "i-09db9b55e0095612f"},
			{LabelName: "ClusterName", LabelValue: dummyClusterName},
		},
	}

	expectedMetrics["execution_errors_created"] = prometheusscraper.ExpectedMetricStruct{
		MetricValue: 1.7083389404380567e+09,
		MetricLabels: []prometheusscraper.MetricLabel{
			{LabelName: "InstanceId", LabelValue: "i-09db9b55e0095612f"},
			{LabelName: "ClusterName", LabelValue: dummyClusterName},
		},
	}

	expectedMetrics["up"] = prometheusscraper.ExpectedMetricStruct{
		MetricValue:  1.0,
		MetricLabels: []prometheusscraper.MetricLabel{},
	}

	consumer := prometheusscraper.MockConsumer{
		T:               t,
		ExpectedMetrics: expectedMetrics,
	}

	mockedScraperOpts := prometheusscraper.GetMockedScraperOpts(consumer, mockHostInfoProvider{})

	prometheusscraper.TestSimplePrometheusEndToEnd(prometheusscraper.TestSimplePrometheusEndToEndOpts{
		T:                   t,
		Consumer:            consumer,
		DataReturned:        renameMetric,
		ScraperOpts:         mockedScraperOpts,
		ScrapeConfig:        GetNueronScrapeConfig(mockedScraperOpts),
		MetricRelabelConfig: GetNueronMetricRelabelConfigs(mockedScraperOpts),
	})
}

func TestNeuronMonitorScraperJobName(t *testing.T) {
	// needs to start with containerInsights
	assert.True(t, strings.HasPrefix(jobName, "containerInsightsNeuronMonitorScraper"))
}
