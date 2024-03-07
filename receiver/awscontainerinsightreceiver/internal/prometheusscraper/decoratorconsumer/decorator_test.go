package decoratorconsumer

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

var _ Decorator = (*MockK8sDecorator)(nil)

type MockK8sDecorator struct {
}

func (m *MockK8sDecorator) Decorate(metric stores.CIMetric) stores.CIMetric {
	return metric
}

func (m *MockK8sDecorator) Shutdown() error {
	return nil
}

const (
	util      = "UTIL"
	memUtil   = "USED_PERCENT"
	memUsed   = "FB_USED"
	memTotal  = "FB_TOTAL"
	temp      = "TEMP"
	powerDraw = "POWER_USAGE"
)

var metricToUnit = map[string]string{
	util:      "Percent",
	memUtil:   "Percent",
	memUsed:   "Bytes",
	memTotal:  "Bytes",
	temp:      "None",
	powerDraw: "None",
}

func TestConsumeMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	dc := &DecorateConsumer{
		ContainerOrchestrator: "EKS",
		NextConsumer:          consumertest.NewNop(),
		K8sDecorator:          &MockK8sDecorator{},
		MetricToUnitMap:       metricToUnit,
		Logger:                logger,
	}
	ctx := context.Background()

	testcases := map[string]TestCase{
		"empty": {
			metrics:     pmetric.NewMetrics(),
			want:        pmetric.NewMetrics(),
			shouldError: false,
		},
		"unit": {
			metrics: GenerateMetrics(map[MetricIdentifier]map[string]string{
				{util, pmetric.MetricTypeGauge}: {
					"device": "test0",
				},
				{memUtil, pmetric.MetricTypeGauge}: {
					"device": "test0",
				},
				{memTotal, pmetric.MetricTypeGauge}: {
					"device": "test0",
				},
				{memUsed, pmetric.MetricTypeGauge}: {
					"device": "test0",
				},
				{powerDraw, pmetric.MetricTypeGauge}: {
					"device": "test0",
				},
				{temp, pmetric.MetricTypeGauge}: {
					"device": "test0",
				},
			}),
			want: GenerateMetrics(map[MetricIdentifier]map[string]string{
				{util, pmetric.MetricTypeGauge}: {
					"device": "test0",
					"Unit":   "Percent",
				},
				{memUtil, pmetric.MetricTypeGauge}: {
					"device": "test0",
					"Unit":   "Percent",
				},
				{memTotal, pmetric.MetricTypeGauge}: {
					"device": "test0",
					"Unit":   "Bytes",
				},
				{memUsed, pmetric.MetricTypeGauge}: {
					"device": "test0",
					"Unit":   "Bytes",
				},
				{powerDraw, pmetric.MetricTypeGauge}: {
					"device": "test0",
					"Unit":   "None",
				},
				{temp, pmetric.MetricTypeGauge}: {
					"device": "test0",
					"Unit":   "None",
				},
			}),
			shouldError: false,
		},
		"noUnit": {
			metrics: GenerateMetrics(map[MetricIdentifier]map[string]string{
				{"test", pmetric.MetricTypeGauge}: {
					"device": "test0",
				},
			}),
			want: GenerateMetrics(map[MetricIdentifier]map[string]string{
				{"test", pmetric.MetricTypeGauge}: {
					"device": "test0",
				},
			}),
			shouldError: false,
		},
		"typeUnchanged": {
			metrics: GenerateMetrics(map[MetricIdentifier]map[string]string{
				{util, pmetric.MetricTypeGauge}: {
					"device": "test0",
					"Type":   "TestType",
				},
			}),
			want: GenerateMetrics(map[MetricIdentifier]map[string]string{
				{util, pmetric.MetricTypeGauge}: {
					"device": "test0",
					"Type":   "TestType",
					"Unit":   "Percent",
				},
			}),
			shouldError: false,
		},
	}

	RunDecoratorTestScenarios(t, dc, ctx, testcases)
}
