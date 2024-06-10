/*
Copyright (c) 2019 Cisco Systems, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"text/template"
	"time"
	"v1alpha1"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	DEFAULT_NAMESPACE       = "default"
	DEFAULT_CONFIG_MAP_NAME = "webhook-instrumentor-config"
)

var (
	client      dynamic.Interface
	clientset   *kubernetes.Clientset
	onOpenShift bool
)

type Config struct {
	MyNamespace                   string
	ConfigMapName                 string
	ControllerConfig              *ControllerConfig
	AppdCloudConfig               *AppdCloudConfig
	TelescopeConfig               *TelescopeConfig
	InstrumentationConfig         *InstrumentationConfig            // This comes from the config map and is configured by Helm chart
	InstrumentationClusterCrds    *InstrumentationConfig            // This comes from GlobalInstrumentation CRDs
	InstrumentationNamespacedCrds map[string]*InstrumentationConfig // This comes from Instrumentation CRDs ans is namespace specific
	FlexMatchTemplate             *template.Template
	CrdsDisabled                  bool // When set to true, namespaced Instrumentation is disabled
	mutex                         sync.Mutex
}

type ControllerConfig struct {
	Host               string `json:"host,omitempty" yaml:"host,omitempty"`
	Port               string `json:"port,omitempty" yaml:"port,omitempty"`
	IsSecure           bool   `json:"isSecure,omitempty" yaml:"isSecure,omitempty"`
	AccountName        string `json:"accountName,omitempty" yaml:"accountName,omitempty"`
	AccessKeySecret    string `json:"accessKeySecret,omitempty" yaml:"accessKeySecret,omitempty"`
	AccessKeySecretKey string `json:"accessKeySecretKey,omitempty" yaml:"accessKeySecretKey,omitempty"`
	AccessKey          string `json:"accessKey,omitempty" yaml:"accessKey,omitempty"`
	UseProxy           bool   `json:"useProxy,omitempty" yaml:"useProxy,omitempty"`
	ProxyHost          string `json:"proxyHost,omitempty" yaml:"proxyHost,omitempty"`
	ProxyPort          string `json:"proxyPort,omitempty" yaml:"proxyPort,omitempty"`
	ProxyUser          string `json:"proxyUser,omitempty" yaml:"proxyUser,omitempty"`
	ProxyPassword      string `json:"proxyPassword,omitempty" yaml:"proxyPassword,omitempty"`
	ProxyDomain        string `json:"proxyDomain,omitempty" yaml:"proxyDomain,omitempty"`
	OtelEndpoint       string `json:"otelEndpoint,omitempty" yaml:"otelEndpoint,omitempty"`
	OtelHeaderKey      string `json:"otelHeaderKey,omitempty" yaml:"otelHeaderKey,omitempty"`
}

type AppdCloudConfig struct {
}

// TODO - Obsolete, should be cleaned out
type TelescopeConfig struct {
	TracesEndpoint string `json:"traces_endpoint,omitempty" yaml:"traces_endpoint,omitempty"`
	Token          string `json:"token,omitempty" yaml:"token,omitempty"`
}

type InstrumentationConfig []v1alpha1.InstrumentationSpec

type InjectionTemplates []v1alpha1.InjectionTemplate

type groupResource struct {
	APIGroup        string
	APIGroupVersion string
	APIResource     metav1.APIResource
}

var config = Config{
	InstrumentationClusterCrds:    &InstrumentationConfig{},
	InstrumentationNamespacedCrds: map[string]*InstrumentationConfig{},
}

func runConfigWatcher() {

	initClient()

	go configurationWatcher(config.MyNamespace)

	ticker := time.NewTicker(5 * 60 * 1000 * time.Millisecond) // every 5 minutes

	for {
		select {
		case <-ticker.C:
			fmt.Printf("Running round because of timer\n")
		}
	}
}

func initClient() {
	var restConfig *rest.Config
	if fileExists("/var/run/secrets/kubernetes.io") {
		fmt.Println("running in Kubernetes")
		var err error
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}

		config.MyNamespace = getMyNamespace()

	} else {
		var kubeconfig *string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()

		var err error
		restConfig, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err)
		}
	}
	var err error
	client, err = dynamic.NewForConfig(restConfig)
	if err != nil {
		panic(err)
	}
	clientset, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		panic(err.Error())
	}

	onOpenShift = runsOnOpenShift()
	if onOpenShift {
		fmt.Println("running in OpenShift")
	}

	if config.MyNamespace == "" {
		config.MyNamespace = DEFAULT_NAMESPACE
	}

	config.ConfigMapName = os.Getenv("WEBHOOK_INSTRUMENTOR_CONFIG_MAP_NAME")
	if config.ConfigMapName == "" {
		config.ConfigMapName = DEFAULT_CONFIG_MAP_NAME
	}

}

func configurationWatcher(namespace string) {
	watchlist := cache.NewListWatchFromClient(clientset.CoreV1().RESTClient(), "configmaps", namespace, fields.Everything())
	_, controller := cache.NewInformer(
		watchlist,
		&v1.ConfigMap{},
		time.Second*0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				// fmt.Printf("config map added: %s \n", obj)
				//fmt.Printf("config map added %s\n", obj)
				updateConfig(obj)
			},
			DeleteFunc: func(obj interface{}) {
				// fmt.Printf("config map deleted: %s \n", obj)
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				//fmt.Printf("config map changed %s\n", newObj)
				updateConfig(newObj)
			},
		},
	)
	stop := make(chan struct{})
	defer close(stop)
	go controller.Run(stop)
	select {}
}

func updateConfig(obj interface{}) {
	fmt.Printf("Type: %T\n", obj)
	cm, isType := obj.(*v1.ConfigMap)
	if !isType {
		log.Printf("Error getting configmap from object\n")
		return
	}
	fmt.Printf("ConfigMap Name: %s\n", cm.GetName())
	if cm.Name != config.ConfigMapName && cm.Name != OTEL_COLL_CONFIG_MAP_NAME {
		return
	}

	if cm.Name == OTEL_COLL_CONFIG_MAP_NAME {
		loadOtelConfig(cm.Data)
		return
	}

	data := cm.Data

	controller := data["controller"]
	if controller == "" {
		log.Printf("Error getting required controller configuration\n")
		return
	}
	fmt.Printf("Controller Config:\n%s\n", controller)

	instrumentation := data["instrumentation"]
	if instrumentation == "" {
		log.Printf("Error getting required instrumentation configuration\n")
		return
	}
	fmt.Printf("Instrumentation Config:\n%s\n", instrumentation)

	templates := data["injectionTemplates"]
	if templates == "" {
		log.Printf("No injection templates specified\n")
	}
	fmt.Printf("Injection templates:\n%s\n", templates)

	appdCloud := data["appdCloud"]
	if appdCloud == "" {
		log.Printf("No appdCloud config specification\n")
	}
	log.Printf("apodCloud config: \n%s\n", appdCloud)

	telescope := data["telescope"]
	if telescope == "" {
		log.Printf("No telescope config specification\n")
	}
	log.Printf("telescope config: \n%s\n", telescope)

	flexMatchConfig := data["flexMatch"]
	if telescope == "" {
		log.Printf("No flexMatch config specification\n")
	}
	log.Printf("FlexMatch config: \n%s\n", flexMatchConfig)

	controllerConfig := &ControllerConfig{}
	instrumentationConfig := &InstrumentationConfig{}
	injectionTemplates := &InjectionTemplates{}
	appdCloudConfig := &AppdCloudConfig{}
	telescopeConfig := &TelescopeConfig{}

	err := yaml.Unmarshal([]byte(controller), controllerConfig)
	if err != nil {
		log.Printf("Error parsing required controller configuration: %v\n", err)
		return
	}
	err = yaml.Unmarshal([]byte(instrumentation), instrumentationConfig)
	if err != nil {
		log.Printf("Error parsing required instrumentation configuration: %v\n", err)
		return
	}
	err = yaml.Unmarshal([]byte(templates), injectionTemplates)
	if err != nil {
		log.Printf("Error parsing injection templates configuration: %v\n", err)
		return
	}
	err = yaml.Unmarshal([]byte(appdCloud), appdCloudConfig)
	if err != nil {
		log.Printf("Error parsing injection appdCloud configuration: %v\n", err)
		return
	}
	err = yaml.Unmarshal([]byte(telescope), telescopeConfig)
	if err != nil {
		log.Printf("Error parsing injection telescope configuration: %v\n", err)
		return
	}

	fmt.Printf("Controller Config:\n%v\nInstrumentation Config:\n%v\nInjection Templates:\n%v\nAppD Cloud:\n%v\nTelescope:\n%v\n",
		*controllerConfig, *instrumentationConfig, *injectionTemplates, *appdCloudConfig, *telescopeConfig)

	// validations
	valid := true

	valid = valid && validateControllerConfig(controllerConfig)
	valid = valid && validateInjectionTemplates(injectionTemplates)

	applyInjectionTemplates(injectionTemplates, instrumentationConfig)
	applyInjectionRulesDefaults(instrumentationConfig)

	valid = valid && validateInstrumentationConfig(instrumentationConfig)

	if !valid {
		return
	}

	config.mutex.Lock()
	defer config.mutex.Unlock()
	config.ControllerConfig = controllerConfig
	config.InstrumentationConfig = instrumentationConfig
	config.TelescopeConfig = telescopeConfig
	config.AppdCloudConfig = appdCloudConfig
	if flexMatchConfig != "" {
		config.FlexMatchTemplate, err = template.New("test").Parse(flexMatchConfig)
		if err != nil {
			log.Printf("Error parsing flex match template %s: %v\n", flexMatchConfig, err)
			config.FlexMatchTemplate = nil
		}
	} else {
		config.FlexMatchTemplate = nil
	}
}

// apply injection rules defaults
func applyInjectionRulesDefaults(instrumentationConfig *InstrumentationConfig) {
	for _, instrRule := range *instrumentationConfig {
		if instrRule.InjectionRules != nil {
			instrRule.InjectionRules = injectionRuleDefaults(instrRule.InjectionRules)
		}
		// if ruleset exists, then apply defaults
		for idx, instrRuleInSet := range instrRule.InjectionRuleSet {
			instrRule.InjectionRuleSet[idx] = *injectionRuleDefaults(&instrRuleInSet)
		}
	}
}

func injectionRuleDefaults(injRules *v1alpha1.InjectionRule) *v1alpha1.InjectionRule {
	injRules.ApplicationName = applyTemplateString(injRules.ApplicationName, "DEFAULT_APP_NAME")
	if injRules.DoNotInstrument == nil {
		falseValue := false
		injRules.DoNotInstrument = &falseValue
	}
	if injRules.UsePodNameForNodeName == nil {
		falseValue := false
		injRules.UsePodNameForNodeName = &falseValue
	}
	if injRules.InjectK8SOtelResourceAttrs == nil {
		trueValue := true
		injRules.InjectK8SOtelResourceAttrs = &trueValue
	}
	injRules.ApplicationNameSource = applyTemplateString(injRules.ApplicationNameSource, "namespace")
	injRules.JavaEnvVar = applyTemplateString(injRules.JavaEnvVar, "JAVA_TOOL_OPTIONS")
	injRules.TierNameSource = applyTemplateString(injRules.TierNameSource, "auto")
	if injRules.ResourceReservation == nil {
		injRules.ResourceReservation = &v1alpha1.ResourceReservation{}
	}
	injRules.ResourceReservation.CPU = applyTemplateString(injRules.ResourceReservation.CPU, "100m")
	injRules.ResourceReservation.Memory = applyTemplateString(injRules.ResourceReservation.Memory, "50M")

	injRules.NetvizPort = applyTemplateString(injRules.NetvizPort, "3892")

	return injRules
}

// apply injection templates to instrumentation rules
func applyInjectionTemplates(injectionTemplates *InjectionTemplates, instrumentationConfig *InstrumentationConfig) bool {
	valid := true

	injectionTemplateMap := map[string]*v1alpha1.InjectionRule{}

	for _, injTemplate := range *injectionTemplates {
		if injTemplate.Name == "" {
			log.Printf("Injection template name is required but is empty\n")
			continue
		}
		injectionTemplateMap[injTemplate.Name] = injTemplate.InjectionRules
	}

	for _, instrRule := range *instrumentationConfig {
		injRules := instrRule.InjectionRules
		if injRules != nil {
			if injRules.Template != "" {
				injTempRules, found := injectionTemplateMap[injRules.Template]
				if !found {
					log.Printf("Injection template '%s' not found for instrumentation rule '%s'\n", injRules.Template, instrRule.Name)
					valid = false
					continue
				}
				instrRule.InjectionRules = injectionRuleTemplate(injRules, injTempRules)
			}
		}
		for idx, instrRuleInSet := range instrRule.InjectionRuleSet {
			if instrRuleInSet.Template != "" {
				injTempRules, found := injectionTemplateMap[instrRuleInSet.Template]
				if !found {
					log.Printf("Injection template '%s' not found for instrumentation rule in set '%s'\n", injRules.Template, instrRule.Name)
					valid = false
					continue
				}
				instrRule.InjectionRuleSet[idx] = *injectionRuleTemplate(&instrRuleInSet, injTempRules)
			}
		}
	}
	return valid
}

func injectionRuleTemplate(injRules *v1alpha1.InjectionRule, injTempRules *v1alpha1.InjectionRule) *v1alpha1.InjectionRule {
	injRules.ApplicationName = applyTemplateString(injRules.ApplicationName, injTempRules.ApplicationName)
	injRules.ApplicationNameAnnotation = applyTemplateString(injRules.ApplicationNameAnnotation, injTempRules.ApplicationNameAnnotation)
	injRules.ApplicationNameExpression = applyTemplateString(injRules.ApplicationNameExpression, injTempRules.ApplicationNameExpression)
	injRules.ApplicationNameLabel = applyTemplateString(injRules.ApplicationNameLabel, injTempRules.ApplicationNameLabel)
	injRules.ApplicationNameSource = applyTemplateString(injRules.ApplicationNameSource, injTempRules.ApplicationNameSource)
	injRules.DoNotInstrument = applyTemplateBool(injRules.DoNotInstrument, injTempRules.DoNotInstrument, false)
	injRules.Image = applyTemplateString(injRules.Image, injTempRules.Image)
	injRules.JavaCustomConfig = applyTemplateString(injRules.JavaCustomConfig, injTempRules.JavaCustomConfig)
	injRules.JavaEnvVar = applyTemplateString(injRules.JavaEnvVar, injTempRules.JavaEnvVar)
	injRules.LogLevel = applyTemplateString(injRules.LogLevel, injTempRules.LogLevel)
	injRules.Technology = applyTemplateString(injRules.Technology, injTempRules.Technology)
	injRules.TierName = applyTemplateString(injRules.TierName, injTempRules.TierName)
	injRules.TierNameAnnotation = applyTemplateString(injRules.TierNameAnnotation, injTempRules.TierNameAnnotation)
	injRules.TierNameExpression = applyTemplateString(injRules.TierNameExpression, injTempRules.TierNameExpression)
	injRules.TierNameLabel = applyTemplateString(injRules.TierNameLabel, injTempRules.TierNameLabel)
	injRules.TierNameSource = applyTemplateString(injRules.TierNameSource, injTempRules.TierNameSource)
	injRules.UsePodNameForNodeName = applyTemplateBool(injRules.UsePodNameForNodeName, injTempRules.UsePodNameForNodeName, false)
	if injRules.ResourceReservation == nil && injTempRules.ResourceReservation != nil {
		injRules.ResourceReservation = &v1alpha1.ResourceReservation{}
		injRules.ResourceReservation.CPU = applyTemplateString(injRules.ResourceReservation.CPU, injTempRules.ResourceReservation.CPU)
		injRules.ResourceReservation.Memory = applyTemplateString(injRules.ResourceReservation.Memory, injTempRules.ResourceReservation.Memory)
	}
	injRules.NetvizPort = applyTemplateString(injRules.NetvizPort, injTempRules.NetvizPort)
	injRules.OpenTelemetryCollector = applyTemplateString(injRules.OpenTelemetryCollector, injTempRules.OpenTelemetryCollector)
	injRules.EnvVars = mergeNameValues(injRules.EnvVars, injTempRules.EnvVars)
	injRules.Options = mergeNameValues(injRules.Options, injTempRules.Options)
	injRules.InjectK8SOtelResourceAttrs = applyTemplateBool(injRules.InjectK8SOtelResourceAttrs, injTempRules.InjectK8SOtelResourceAttrs, false)
	///
	return injRules
}

func mergeNameValues(specific []v1alpha1.NameValue, templated []v1alpha1.NameValue) []v1alpha1.NameValue {
	merged := []v1alpha1.NameValue{}
	temp := map[string]string{}

	for _, item := range templated {
		temp[item.Name] = item.Value
	}
	for _, item := range specific {
		temp[item.Name] = item.Value
	}
	for name, value := range temp {
		merged = append(merged, v1alpha1.NameValue{Name: name, Value: value})
	}

	return merged
}

// validate controller config
func validateControllerConfig(controllerConfig *ControllerConfig) bool {
	valid := true
	if controllerConfig.Host == "" {
		log.Printf("Controller host configuration is empty\n")
		valid = false
	}
	if controllerConfig.Port == "" {
		log.Printf("Controller port configuration is empty\n")
		valid = false
	}
	if controllerConfig.AccountName == "" {
		log.Printf("Controller account name configuration is empty\n")
		valid = false
	}
	if controllerConfig.AccessKey == "" && controllerConfig.AccessKeySecret == "" {
		log.Printf("Controller accessKey or accessKeySecret must be specified\n")
		valid = false
	}
	return valid
}

// validate injection templates
func validateInjectionTemplates(injectionTemplates *InjectionTemplates) bool {
	valid := true
	for _, injTemplate := range *injectionTemplates {
		if injTemplate.Name == "" {
			log.Printf("Injection template name is required but is empty\n")
			valid = false
		}
	}

	return valid
}

// validate instrumentation config
func validateInstrumentationConfig(instrumentationConfig *InstrumentationConfig) bool {
	valid := true
	for _, instrRule := range *instrumentationConfig {
		if instrRule.Name == "" {
			log.Printf("Instrumentation rule name is required but is empty\n")
			valid = false
		}
		matchRules := instrRule.MatchRules
		if matchRules.NamespaceRegex != "" {
			_, err := regexp.Compile(matchRules.NamespaceRegex)
			if err != nil {
				log.Printf("Error in match rule '%s' namespace regex: %v", instrRule.Name, err)
				valid = false
			}
		}
		if matchRules.PodNameRegex != "" {
			_, err := regexp.Compile(matchRules.PodNameRegex)
			if err != nil {
				log.Printf("Error in match rule '%s' pod name regex: %v", instrRule.Name, err)
				valid = false
			}
		}
		if matchRules.Annotations != nil {
			for _, annotRule := range *matchRules.Annotations {
				for annot, regex := range annotRule {
					_, err := regexp.Compile(regex)
					if err != nil {
						log.Printf("Error in match rule '%s' annotation '%s' regex: %v", instrRule.Name, annot, err)
						valid = false
					}
				}
			}
		}
		if matchRules.Labels != nil {
			for _, labelRule := range *matchRules.Labels {
				for label, regex := range labelRule {
					_, err := regexp.Compile(regex)
					if err != nil {
						log.Printf("Error in match rule '%s' label '%s' regex: %v", instrRule.Name, label, err)
						valid = false
					}
				}
			}
		}
	}
	return valid
}

func fileExists(filename string) bool {
	if _, err := os.Stat(filename); err == nil {
		return true
	}
	return false
}

func runsOnOpenShift() bool {
	resourceList := getApiResourceList()
	for _, res := range resourceList {
		if res.APIGroup == "apps.openshift.io" && res.APIResource.Kind == "DeploymentConfig" {
			return true
		}
	}
	return false
}

func getMyNamespace() string {
	namespaceBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	namespace := string(namespaceBytes)
	if err != nil {
		log.Printf("Cannot read namespace from serviceaccount directory: %v\n", err)
		namespace = DEFAULT_NAMESPACE
	}
	return namespace
}

func getApiResourceList() []groupResource {
	resources := []groupResource{}
	discoveryclient := clientset.DiscoveryClient
	lists, _ := discoveryclient.ServerPreferredResources()
	for _, list := range lists {

		if len(list.APIResources) == 0 {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, resource := range list.APIResources {
			if len(resource.Verbs) == 0 {
				continue
			}
			resources = append(resources, groupResource{
				APIGroup:        gv.Group,
				APIGroupVersion: gv.String(),
				APIResource:     resource,
			})
		}
	}

	return resources
}

func applyTemplateString(specific string, template string) string {
	if specific == "" {
		return template
	} else {
		return specific
	}
}

func applyTemplateBool(specific *bool, template *bool, def bool) *bool {
	if specific == nil {
		if template == nil {
			boolValue := def
			return &boolValue
		} else {
			return template
		}
	} else {
		return specific
	}
}

// admitFuncHandler takes an admitFunc and wraps it into a http.Handler by means of calling serveAdmitFunc.
func configHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			log.Printf("Config GET called\n")
		case "POST":
			log.Printf("Config POST called\n")
		default:
			log.Printf("Config Unsupported Method called\n")
		}
	})
}

func upsertCrdInstrumentation(namespace string, name string, instr v1alpha1.InstrumentationSpec) {
	config.mutex.Lock()
	defer config.mutex.Unlock()

	// Instrumentation rule name is always taken from the CRD name
	// NamespaceRegex is set tu current namespace only
	instr.Name = namespace + "/" + name
	instr.MatchRules.NamespaceRegex = "^" + namespace + "$"

	ok := false
	if _, ok = config.InstrumentationNamespacedCrds[namespace]; !ok {
		config.InstrumentationNamespacedCrds[namespace] = &InstrumentationConfig{}
	}
	namespaceInstrs := config.InstrumentationNamespacedCrds[namespace]

	upsertInstrumentationSpecInConfig(namespaceInstrs, name, instr)
}

func deleteCrdInstrumentation(namespace string, name string) {
	config.mutex.Lock()
	defer config.mutex.Unlock()

	ok := false
	if _, ok = config.InstrumentationNamespacedCrds[namespace]; !ok {
		config.InstrumentationNamespacedCrds[namespace] = &InstrumentationConfig{}
	}
	namespaceInstrs := config.InstrumentationNamespacedCrds[namespace]

	deleteInstrumentationSpecInConfig(namespaceInstrs, name)
}

func upsertCrdClusterInstrumentation(name string, instr v1alpha1.InstrumentationSpec) {
	config.mutex.Lock()
	defer config.mutex.Unlock()

	// Instrumentation rule name is always taken from the CRD name
	instr.Name = "*cluster*/" + name

	upsertInstrumentationSpecInConfig(config.InstrumentationClusterCrds, name, instr)
}

func deleteCrdClusterInstrumentation(name string) {
	config.mutex.Lock()
	defer config.mutex.Unlock()

	deleteInstrumentationSpecInConfig(config.InstrumentationClusterCrds, name)
}

func upsertInstrumentationSpecInConfig(specs *InstrumentationConfig, name string, instr v1alpha1.InstrumentationSpec) {

	found := false
	for i, spec := range *specs {
		if name == spec.Name {
			(*specs)[i] = instr
			found = true
			break
		}
	}
	if !found {
		(*specs) = append((*specs), instr)
	}

	slices.SortFunc((*specs), func(a, b v1alpha1.InstrumentationSpec) int {
		if a.Priority > b.Priority {
			return 1
		} else if a.Priority == b.Priority {
			return 0
		} else {
			return -1
		}
	})
}

func deleteInstrumentationSpecInConfig(specs *InstrumentationConfig, name string) {
	found := -1
	for i, spec := range *specs {
		if name == spec.Name {
			found = i
			break
		}
	}
	if found >= 0 {
		(*specs) = append((*specs)[:found], (*specs)[found+1:]...)
	}
}

func instrumentationAsString() string {
	otelCollsConfigStr, _ := json.MarshalIndent(otelCollsConfig, "", "  ")
	otelCollsConfigNamespacedStr, _ := json.MarshalIndent(otelCollsConfigNamespaced, "", "  ")
	instrumentationConfigStr, _ := json.MarshalIndent(config.InstrumentationConfig, "", "  ")
	instrumentationNamespacedStr, _ := json.MarshalIndent(config.InstrumentationNamespacedCrds, "", "  ")
	instrumentationClusterStr, _ := json.MarshalIndent(config.InstrumentationClusterCrds, "", "  ")

	configStr := fmt.Sprintf(`
	OpenTelemetry Collectors from config map
	========================================================================================
	%s

	OpenTelemetry Collectors namespaced
	========================================================================================
	%s

	Instrumentation Rules from config map
	========================================================================================
	%s
	
	Instrumentation Rules namespaced
	========================================================================================
	%s
	
	Instrumentation Rules cluster-wide
	========================================================================================
	%s
	
	`,
		string(otelCollsConfigStr),
		string(otelCollsConfigNamespacedStr),
		string(instrumentationConfigStr),
		string(instrumentationNamespacedStr),
		string(instrumentationClusterStr),
	)

	return configStr
}

func asJson(v any, header string) string {
	str, _ := json.MarshalIndent(v, "", "  ")
	return string(str)
}
