package main

import (
	"context"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestInstrumentJavaInstr(t *testing.T) {
	f := features.New("Java AppD Instrumentation via CRD").
		Assess("setup instrumentation", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			instrFilename := "../e2e-tests/java/instr/instrumentation.yaml"
			err := testenv.deployInstrumentation(ctx, t, cfg, instrFilename, 00)
			if err != nil {
				t.Error(err, "cannot deploy instrumentation per file: "+instrFilename)
				t.FailNow()
			}

			podFilename := "../e2e-tests/java/instr/pod.yaml"
			podAssertFilename := "../e2e-tests/java/instr/pod-assert.yaml"

			eq, err, diffs := testenv.deployAndAssertPod(ctx, t, cfg, podFilename, podAssertFilename)
			if err != nil {
				t.Error(err, "cannot deploy and assert pod per file: "+podFilename)
				t.FailNow()
			}

			if !eq {
				t.Error(err, "assert failed for: "+podFilename)
				t.Error(err, testenv.formatDiffs(diffs))
				if testenv.getEnv("TEST_WAIT_ON_FAIL", "") != "" {
					testenv.wait()
				}
				t.FailNow()
			}

			return ctx
		})

	testenv.env.Test(t, f.Feature())
}

func TestInstrumentJavaInstrOtel(t *testing.T) {
	f := features.New("Java AppD Instrumentation via CRD with Opentelemetry Collector").
		Assess("setup instrumentation", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			otelFilename := "../e2e-tests/java/instr-otel/crd-otelcol.yaml"
			err := testenv.deployOtelCol(ctx, t, cfg, otelFilename, 0)
			if err != nil {
				t.Error(err, "cannot deploy opentelemetry collector per file: "+otelFilename)
				t.FailNow()
			}

			instrFilename := "../e2e-tests/java/instr-otel/instrumentation.yaml"
			err = testenv.deployInstrumentation(ctx, t, cfg, instrFilename, 00)
			if err != nil {
				t.Error(err, "cannot deploy instrumentation per file: "+instrFilename)
				t.FailNow()
			}

			podFilename := "../e2e-tests/java/instr-otel/pod.yaml"
			podAssertFilename := "../e2e-tests/java/instr-otel/pod-assert.yaml"

			eq, err, diffs := testenv.deployAndAssertPod(ctx, t, cfg, podFilename, podAssertFilename)
			if err != nil {
				t.Error(err, "cannot deploy and assert pod per file: "+podFilename)
				t.FailNow()
			}

			if !eq {
				t.Error(err, "assert failed for: "+podFilename)
				t.Error(err, testenv.formatDiffs(diffs))
				if testenv.getEnv("TEST_WAIT_ON_FAIL", "") != "" {
					testenv.wait()
				}
				t.FailNow()
			}

			return ctx
		})

	testenv.env.Test(t, f.Feature())
}

func TestInstrumentJavaConfigMap(t *testing.T) {
	f := features.New("Java AppD Instrumentation via ConfigMap").
		Assess("setup instrumentation", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {

			podFilename := "../e2e-tests/java/cm/pod.yaml"
			podAssertFilename := "../e2e-tests/java/cm/pod-assert.yaml"

			eq, err, diffs := testenv.deployAndAssertPod(ctx, t, cfg, podFilename, podAssertFilename)
			if err != nil {
				t.Error(err, "cannot deploy and assert pod per file: "+podFilename)
				t.FailNow()
			}

			if !eq {
				t.Error(err, "assert failed for: "+podFilename)
				t.Error(err, testenv.formatDiffs(diffs))
				if testenv.getEnv("TEST_WAIT_ON_FAIL", "") != "" {
					testenv.wait()
				}
				t.FailNow()
			}

			return ctx
		})

	testenv.env.Test(t, f.Feature())
}
