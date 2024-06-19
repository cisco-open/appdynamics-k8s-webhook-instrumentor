package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"
	"v1alpha1"

	// "k8s.io/apimachinery/pkg/runtime"

	v1 "k8s.io/api/core/v1"
	sch "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

const BASE_TEST_DIRECTORY = "../e2e-tests"
const WAIT_SEC_AFTER_HELM = 10

type TestFrame struct {
	env       env.Environment
	namespace string
	secrets   map[string]string
	replacer  *strings.Replacer
}

var testenv TestFrame

var ignoredComapredFields = map[string]bool{
	"metadata.creationTimestamp": true,
}

func TestMain(m *testing.M) {
	testenv.env = env.New()
	testenv.env.Setup(
		// Setup func: install the instrumentation Helm chart
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			log.Default().Print("starting")

			secretsFile := testenv.getEnv("TEST_SECRETS", "secrets.txt")

			err := testenv.loadSecrets(secretsFile)
			if err != nil {
				log.Default().Printf("Secrets file %s not loaded: %v", secretsFile, err)
			}

			helmValuesTemplate := testenv.getEnv("TEST_HELM_VALUES", "helmDefaultValues.yaml")
			helmValuesTemplateTemp := testenv.getEnv("TEST_HELM_VALUES_TEMP", "helmDefaultValuesTemp.yaml")

			err = testenv.applySecretsToFile(helmValuesTemplate, helmValuesTemplateTemp)
			if err != nil {
				log.Default().Printf("Helm values file %s not created: %v", secretsFile, err)
				os.Exit(2)
			}

			helmCmd := []string{"install", "--namespace", "mwh", "mwh", "../helm", "--values=" + helmValuesTemplateTemp}
			stdout, stderr, err := testenv.runOsCommand("helm", helmCmd)
			if err != nil {
				log.Default().Printf("%s", string(stderr))
				log.Default().Fatalf("Error while installing Helm chart: %v", err)
				os.Exit(2)
			}
			log.Default().Printf("Helm returns: \n%s", string(stdout))

			testenv.registerResources()

			time.Sleep(WAIT_SEC_AFTER_HELM * time.Second)

			return ctx, nil
		},
	).Finish(
		// Teardown func:
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			log.Default().Print("finishing")
			helmCmd := []string{"delete", "--namespace", "mwh", "mwh"}
			stdout, stderr, err := testenv.runOsCommand("helm", helmCmd)
			if err != nil {
				log.Default().Printf("%s", string(stderr))
				log.Default().Fatalf("Error while removing Helm chart: %v", err)
				os.Exit(2)
			}
			log.Default().Printf("Helm returns: \n%s", string(stdout))

			helmValuesTemplateTemp := testenv.getEnv("TEST_HELM_VALUES_TEMP", "helmDefaultValuesTemp.yaml")

			if false {
				os.Remove(helmValuesTemplateTemp)
			}

			return ctx, nil
		},
	).BeforeEachFeature(
		func(ctx context.Context, cfg *envconf.Config, t *testing.T, feature features.Feature) (context.Context, error) {
			ns := envconf.RandomName("test", 10)
			ctx = context.WithValue(ctx, testenv.getNamespaceKey(t), ns)

			t.Logf("Creating NS %v for test %v feature %s", ns, t.Name(), feature.Name())
			nsObj := v1.Namespace{}
			nsObj.Name = ns
			return ctx, cfg.Client().Resources().Create(ctx, &nsObj)
		},
	).AfterEachFeature(
		func(ctx context.Context, cfg *envconf.Config, t *testing.T, feature features.Feature) (context.Context, error) {
			ns := fmt.Sprint(ctx.Value(testenv.getNamespaceKey(t)))
			t.Logf("Deleting NS %v for test %v feature %s", ns, t.Name(), feature.Name())

			nsObj := v1.Namespace{}
			nsObj.Name = ns
			err := cfg.Client().Resources().Delete(ctx, &nsObj)
			testenv.deleteAppDAppAfterTest(ns)
			return ctx, err
		},
	)

	os.Exit(testenv.env.Run(m))
}

func (t *TestFrame) registerResources() {
	v1alpha1.AddToScheme(sch.Scheme)
}
