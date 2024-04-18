// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deltatosparseprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatosparseprocessor"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatosparseprocessor/internal/metadata"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
)

func TestUnmarshalDefaultConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id           component.ID
		expected     component.Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				Include: MatchMetrics{
					Metrics: []string{
						"metric1",
						"metric2",
					},
					Config: filterset.Config{
						MatchType:    "strict",
						RegexpConfig: nil,
					},
				},
			},
		},
		{
			id:       component.NewIDWithName(metadata.Type, "empty"),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "regexp"),
			expected: &Config{
				Include: MatchMetrics{
					Metrics: []string{
						"a*",
					},
					Config: filterset.Config{
						MatchType:    "regexp",
						RegexpConfig: nil,
					},
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_match_type"),
			errorMessage: "match_type must be set if metrics are supplied",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "missing_name"),
			errorMessage: "metrics must be supplied if match_type is set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expected == nil {
				assert.EqualError(t, component.ValidateConfig(cfg), tt.errorMessage)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
