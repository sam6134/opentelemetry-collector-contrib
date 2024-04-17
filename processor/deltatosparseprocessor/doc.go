// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

// package deltatosparseprocessor implements a processor which
// converts delta sum metrics to sparse metrics if value of delta datapoin is 0.
package deltatosparseprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatosparseprocessor"
