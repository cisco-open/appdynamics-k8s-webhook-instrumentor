package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

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
	MyNamespace           string
	ConfigMapName         string
	ControllerConfig      *ControllerConfig
	InstrumentationConfig *InstrumentationConfig
	mutex                 sync.Mutex
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
	// ProxyUser          string `json:"proxyUser,omitempty" yaml:"proxyUser,omitempty"`
}

type MatchRules struct {
	NamespaceRegex string               `json:"namespaceRegex,omitempty" yaml:"namespaceRegex,omitempty"`
	Labels         *[]map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Annotations    *[]map[string]string `json:"annotations,omitempty" yaml:"annotations,omitempty"`
	PodNameRegex   string               `json:"podNameRegex,omitempty" yaml:"podNameRegex,omitempty"`
}

type InjectionRules struct {
	Template                  string               `json:"template,omitempty" yaml:"template,omitempty"`
	Technology                string               `json:"technology,omitempty" yaml:"technology,omitempty"`
	Image                     string               `json:"image,omitempty" yaml:"image,omitempty"`
	JavaEnvVar                string               `json:"javaEnvVar,omitempty" yaml:"javaEnvVar,omitempty"`
	JavaCustomConfig          string               `json:"javaCustomConfig,omitempty" yaml:"javaCustomConfig,omitempty"`
	ApplicationNameSource     string               `json:"applicationNameSource,omitempty" yaml:"applicationNameSource,omitempty"` // manual,namespace,label,annotation,expression
	ApplicationName           string               `json:"applicationName,omitempty" yaml:"applicationName,omitempty"`
	ApplicationNameLabel      string               `json:"applicationNameLabel,omitempty" yaml:"applicationNameLabel,omitempty"`
	ApplicationNameAnnotation string               `json:"applicationNameAnnotation,omitempty" yaml:"applicationNameAnnotation,omitempty"`
	ApplicationNameExpression string               `json:"applicationNameExpression,omitempty" yaml:"applicationNameExpression,omitempty"`
	TierNameSource            string               `json:"tierNameSource,omitempty" yaml:"tierNameSource,omitempty"` // auto,manual,namespace,label,annotation,expression
	TierName                  string               `json:"tierName,omitempty" yaml:"tierName,omitempty"`
	TierNameLabel             string               `json:"tierNameLabel,omitempty" yaml:"tierNameLabel,omitempty"`
	TierNameAnnotation        string               `json:"tierNameAnnotation,omitempty" yaml:"tierNameAnnotation,omitempty"`
	TierNameExpression        string               `json:"tierNameExpression,omitempty" yaml:"tierNameExpression,omitempty"`
	UsePodNameForNodeName     *bool                `json:"usePodNameForNodeName,omitempty" yaml:"usePodNameForNodeName,omitempty"`
	DoNotInstrument           *bool                `json:"doNotInstrument,omitempty" yaml:"doNotInstrument,omitempty"`
	ResourceReservation       *ResourceReservation `json:"resourceReservation,omitempty" yaml:"resourceReservation,omitempty"`
	LogLevel                  string               `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
	NetvizPort                string               `json:"netvizPort,omitempty" yaml:"netvizPort,omitempty"`
}

type ResourceReservation struct {
	CPU    string `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory string `json:"memory,omitempty" yaml:"memory,omitempty"`
}

type InstrumentationConfig []InstrumentationRule

type InjectionTemplates []InjectionTemplate

type InjectionTemplate struct {
	Name           string          `json:"name,omitempty" yaml:"name,omitempty" `
	InjectionRules *InjectionRules `json:"injectionRules,omitempty" yaml:"injectionRules,omitempty" `
}

type InstrumentationRule struct {
	Name           string          `json:"name,omitempty" yaml:"name,omitempty" `
	MatchRules     *MatchRules     `json:"matchRules,omitempty" yaml:"matchRules,omitempty"`
	InjectionRules *InjectionRules `json:"injectionRules,omitempty" yaml:"injectionRules,omitempty"`
}

type groupResource struct {
	APIGroup        string
	APIGroupVersion string
	APIResource     metav1.APIResource
}

var config = Config{}

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
	if cm.Name != config.ConfigMapName {
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

	controllerConfig := &ControllerConfig{}
	instrumentationConfig := &InstrumentationConfig{}
	injectionTemplates := &InjectionTemplates{}

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
	fmt.Printf("Controller Config:\n%v\nInstrumentation Config:\n%v\nInjection Templates:\n%v\n", *controllerConfig, *instrumentationConfig, *injectionTemplates)

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
}

//apply injection rules defaults
func applyInjectionRulesDefaults(instrumentationConfig *InstrumentationConfig) {
	for _, instrRule := range *instrumentationConfig {
		injRules := instrRule.InjectionRules
		injRules.ApplicationName = applyTemplateString(injRules.ApplicationName, "DEFAULT_APP_NAME")
		if injRules.DoNotInstrument == nil {
			falseValue := false
			injRules.DoNotInstrument = &falseValue
		}
		if injRules.UsePodNameForNodeName == nil {
			falseValue := false
			injRules.UsePodNameForNodeName = &falseValue
		}
		injRules.ApplicationNameSource = applyTemplateString(injRules.ApplicationNameSource, "namespace")
		injRules.JavaEnvVar = applyTemplateString(injRules.JavaEnvVar, "JAVA_TOOL_OPTIONS")
		injRules.TierNameSource = applyTemplateString(injRules.TierNameSource, "auto")
		if injRules.ResourceReservation == nil {
			injRules.ResourceReservation = &ResourceReservation{}
		}
		injRules.ResourceReservation.CPU = applyTemplateString(injRules.ResourceReservation.CPU, "100m")
		injRules.ResourceReservation.Memory = applyTemplateString(injRules.ResourceReservation.Memory, "50M")

		injRules.NetvizPort = applyTemplateString(injRules.NetvizPort, "3892")

	}
}

//apply injection templates to instrumentation rules
func applyInjectionTemplates(injectionTemplates *InjectionTemplates, instrumentationConfig *InstrumentationConfig) bool {
	valid := true

	injectionTemplateMap := map[string]*InjectionRules{}

	for _, injTemplate := range *injectionTemplates {
		if injTemplate.Name == "" {
			log.Printf("Injection template name is required but is empty\n")
			continue
		}
		injectionTemplateMap[injTemplate.Name] = injTemplate.InjectionRules
	}

	for _, instrRule := range *instrumentationConfig {
		injRules := instrRule.InjectionRules
		if injRules.Template != "" {
			injTempRules, found := injectionTemplateMap[injRules.Template]
			if !found {
				log.Printf("Injection template '%s' not found for instrumentation rule '%s'\n", injRules.Template, instrRule.Name)
				valid = false
				continue
			}
			injRules.ApplicationName = applyTemplateString(injRules.ApplicationName, injTempRules.ApplicationName)
			injRules.ApplicationNameAnnotation = applyTemplateString(injRules.ApplicationNameAnnotation, injTempRules.ApplicationNameAnnotation)
			injRules.ApplicationNameExpression = applyTemplateString(injRules.ApplicationNameExpression, injTempRules.ApplicationNameExpression)
			injRules.ApplicationNameLabel = applyTemplateString(injRules.ApplicationNameLabel, injTempRules.ApplicationNameLabel)
			injRules.ApplicationNameSource = applyTemplateString(injRules.ApplicationNameSource, injTempRules.ApplicationNameSource)
			injRules.DoNotInstrument = applyTemplateBool(injRules.DoNotInstrument, injTempRules.DoNotInstrument)
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
			injRules.UsePodNameForNodeName = applyTemplateBool(injRules.UsePodNameForNodeName, injTempRules.UsePodNameForNodeName)
			if injRules.ResourceReservation == nil && injTempRules.ResourceReservation != nil {
				injRules.ResourceReservation = &ResourceReservation{}
				injRules.ResourceReservation.CPU = applyTemplateString(injRules.ResourceReservation.CPU, injTempRules.ResourceReservation.CPU)
				injRules.ResourceReservation.Memory = applyTemplateString(injRules.ResourceReservation.Memory, injTempRules.ResourceReservation.Memory)
			}
			injRules.NetvizPort = applyTemplateString(injRules.NetvizPort, injTempRules.NetvizPort)
			///
		}
	}
	return valid
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

func applyTemplateBool(specific *bool, template *bool) *bool {
	if specific == nil {
		if template == nil {
			falseValue := false
			return &falseValue
		} else {
			return template
		}
	} else {
		return specific
	}
}
