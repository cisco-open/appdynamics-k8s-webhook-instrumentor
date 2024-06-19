package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
	"v1alpha1"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	sch "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
)

const SLEEP_SECONDS_ON_FAIL = 300

func (t *TestFrame) runOsCommand(command string, args []string) ([]byte, []byte, error) {
	cmd := exec.Command(command, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdoutBytes, err := cmd.Output()
	stderrBytes := stderr.Bytes()
	return stdoutBytes, stderrBytes, err
}

// ref: https://github.com/kubernetes/client-go/issues/193#issuecomment-363318588
func (t *TestFrame) parseK8sYaml(yamlDefinitions []byte) ([]runtime.Object, error) {

	yamlDefinitionsString := string(yamlDefinitions[:])
	objectYamlDefinitions := strings.Split(yamlDefinitionsString, "---")
	retVal := make([]runtime.Object, 0, len(objectYamlDefinitions))
	for _, f := range objectYamlDefinitions {
		if f == "\n" || f == "" {
			// ignore empty cases
			continue
		}

		decode := sch.Codecs.UniversalDeserializer().Decode
		obj, _, err := decode([]byte(f), nil, nil)

		if err != nil {
			log.Println(fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
			continue
		}

		retVal = append(retVal, obj)

	}
	return retVal, nil
}

func (t *TestFrame) loadObjectsFile(filename string) ([]runtime.Object, error) {

	content, err := t.loadFile(filename)
	if err != nil {
		return []runtime.Object{}, err
	}

	// replace secrets if any
	doctoredContentStr := t.applySecretsToString(string(content))

	log.Default().Printf("read file %s", string(doctoredContentStr))

	resourceObjects, err := t.parseK8sYaml([]byte(doctoredContentStr))

	return resourceObjects, err
}

func (t *TestFrame) loadSecrets(filename string) error {

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		secretName, secretValue, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		t.addSecret(secretName, secretValue)
	}

	return nil
}

func (t *TestFrame) addSecret(name string, value string) {
	if t.secrets == nil {
		t.secrets = map[string]string{}
	}
	t.secrets[name] = value
	replacerConfig := []string{}
	for n, v := range t.secrets {
		replacerConfig = append(replacerConfig, "<<"+n+">>")
		replacerConfig = append(replacerConfig, v)
	}
	t.replacer = strings.NewReplacer(replacerConfig...)
}

func (t *TestFrame) getSecret(name string) string {
	val, ok := t.secrets[name]
	if !ok {
		return "N/A"
	}
	return val
}

func (t *TestFrame) applySecretsToFile(inFilename string, outFilename string) error {
	inFile, err := os.Open(inFilename)
	if err != nil {
		return err
	}
	defer inFile.Close()

	outFile, err := os.Create(outFilename)
	if err != nil {
		return err
	}
	defer outFile.Close()

	scanner := bufio.NewScanner(inFile)
	for scanner.Scan() {
		line := scanner.Text()
		outLine := t.applySecretsToString(line) + "\n"
		outFile.WriteString(outLine)
	}
	return nil
}

func (t *TestFrame) applySecretsToString(in string) string {
	return t.replacer.Replace(in)
}

func (t *TestFrame) loadFile(filename string) ([]byte, error) {
	retVal := []byte{}

	file, err := os.Open(filename)
	if err != nil {
		return retVal, err
	}
	defer file.Close()

	// Get the file size
	stat, err := file.Stat()
	if err != nil {
		return retVal, err
	}

	// Read the file into a byte slice
	retVal = make([]byte, stat.Size())
	_, err = bufio.NewReader(file).Read(retVal)
	if err != nil && err != io.EOF {
		return retVal, err
	}

	return retVal, nil
}

func (t *TestFrame) traverseObject(obj any) {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		log.Default().Printf("error traversing object %v - err: %v", obj, err)
		return
	}
	t.traverseUnstructured(objMap)
}

func (t *TestFrame) traverseUnstructured(obj map[string]any) {
	visitFn := func(key string, value interface{}) {
		// fmt.Printf("Key: %s, Value: %v\n", key, value)
	}

	t.traverseUnstructuredInternal(obj, visitFn)
}

func (t *TestFrame) traverseUnstructuredInternal(data map[string]any, fn func(key string, value interface{})) {

	// Recursive traversal function
	var traverseMap func(m map[string]interface{}, path []string)
	traverseMap = func(m map[string]interface{}, path []string) {
		for key, value := range m {
			currentPath := append(path, key)
			if value == nil {
				fn(strings.Join(currentPath, "."), value)
			} else {
				switch reflect.TypeOf(value).Kind() {
				case reflect.Map:
					traverseMap(value.(map[string]interface{}), currentPath)
				case reflect.Slice:
					for i, item := range value.([]interface{}) {
						traverseMap(map[string]interface{}{strconv.Itoa(i): item}, currentPath)
					}
				default:
					fn(strings.Join(currentPath, "."), value)
				}
			}
		}
	}

	traverseMap(data, []string{})
}

func (t *TestFrame) compareObjects(real any, obj any) (bool, []string) {
	objMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		log.Default().Printf("error traversing object %v - err: %v", obj, err)
		return false, []string{}
	}
	realMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(real)
	if err != nil {
		log.Default().Printf("error traversing object %v - err: %v", obj, err)
		return false, []string{}
	}
	eq, diffs := t.compareUnstructured(realMap, objMap)
	return eq, diffs
}

func (t *TestFrame) compareUnstructured(real, obj map[string]any) (bool, []string) {
	visitFn := func(key string, rValue, value interface{}) []string {
		// fmt.Printf("Key: %s, Value: %v = %v\n", key, rValue, value)
		if reflect.TypeOf(value).Kind() == reflect.String && reflect.TypeOf(rValue).Kind() == reflect.String {
			valueStr := value.(string)
			if len(value.(string)) > 0 {
				if valueStr[0] == '~' {
					regexStr := valueStr[1:]
					match, err := regexp.MatchString(regexStr, rValue.(string))
					if err != nil {
						return []string{
							fmt.Sprintf("values for key %s regex to check invalid - %v", key, value),
						}
					}
					if !match {
						return []string{
							fmt.Sprintf("values for key %s don't regex match - real \n%v\n != expected \n%v", key, rValue, value),
						}
					}
				}
				return []string{}
			}
		}
		if rValue != value {
			return []string{
				fmt.Sprintf("values for key %s don't match - real \n%v\n != expected \n%v", key, rValue, value),
			}
		}
		return []string{}
	}

	diffs := t.compareUnstructuredInternal(real, obj, visitFn)
	eq := len(diffs) == 0
	return eq, diffs
}

func (t *TestFrame) compareUnstructuredInternal(real, data map[string]any, fn func(key string, rValue, value interface{}) []string) []string {

	// Recursive traversal function
	var traverseMap func(r, m map[string]interface{}, path []string) []string
	traverseMap = func(r, m map[string]interface{}, path []string) []string {
		diffs := []string{}
		for key, value := range m {
			currentPath := append(path, key)
			if _, found := ignoredComapredFields[strings.Join(currentPath, ".")]; found {
				continue
			}
			rValue, ok := r[key]
			if !ok {
				diffs = append(diffs, fmt.Sprintf("real is missing key %v, desired value is %v", strings.Join(currentPath, "."), value))
				continue
			}
			if value == nil {
				diffs = append(diffs, fn(strings.Join(currentPath, "."), rValue, value)...)
			} else {
				switch reflect.TypeOf(value).Kind() {
				case reflect.Map:
					if reflect.TypeOf(rValue).Kind() != reflect.Map {
						diffs = append(diffs, fmt.Sprintf("real is wrong type at key %v, desired type is map", strings.Join(currentPath, ".")))
						continue
					}
					diffs = append(diffs, traverseMap(rValue.(map[string]interface{}), value.(map[string]interface{}), currentPath)...)
				case reflect.Slice:
					if reflect.TypeOf(rValue).Kind() != reflect.Slice {
						diffs = append(diffs, fmt.Sprintf("real is wrong type at key %v, desired type is slice", strings.Join(currentPath, ".")))
						continue
					}
					for i, item := range value.([]interface{}) {
						if i >= len(rValue.([]interface{})) {
							diffs = append(diffs, fmt.Sprintf("real slice at key %v shorter than desired %d != %d",
								strings.Join(currentPath, "."), len(rValue.([]interface{})), len(value.([]interface{}))))
							continue
						}
						rItem := rValue.([]interface{})[i]
						diffs = append(diffs,
							traverseMap(
								map[string]interface{}{strconv.Itoa(i): rItem},
								map[string]interface{}{strconv.Itoa(i): item},
								currentPath,
							)...)
					}
				default:
					diffs = append(diffs, fn(strings.Join(currentPath, "."), rValue, value)...)
				}
			}
		}
		return diffs
	}

	diffs := traverseMap(real, data, []string{})
	return diffs
}

func (t *TestFrame) convertObjToUnstructured(obj any) (*unstructured.Unstructured, error) {
	objUnstr := &unstructured.Unstructured{}

	objJson, err := json.Marshal(obj)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot parse object")
	}

	err = json.Unmarshal(objJson, objUnstr)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot unmarshal object to unstructured")
	}

	return objUnstr, nil
}

func (t *TestFrame) getNamespaceKey(test *testing.T) string {
	// When we pass test.Name() from inside an `assess` step, the name is in the form TestName/Features/Assess
	if strings.Contains(test.Name(), "/") {
		return strings.Split(test.Name(), "/")[0]
	}

	// When pass test.Name() from inside a `testenv.BeforeEachFeature` function, the name is just TestName
	return test.Name()
}

func (t *TestFrame) fullFilename(filename string) string {
	return BASE_TEST_DIRECTORY + "/" + filename
}

func (t *TestFrame) deployInstrumentation(ctx context.Context, test *testing.T, cfg *envconf.Config, filename string, delay int) error {
	client, err := cfg.NewClient()
	if err != nil {
		test.Error(err, "cannot get k8s client")
		test.FailNow()
	}

	ns := ctx.Value(testenv.getNamespaceKey(test)).(string)

	resourceFile := t.fullFilename(filename)
	objs, err := testenv.loadObjectsFile(resourceFile)
	if err != nil {
		test.Error(err, "cannot read instrumentation resource definition file", resourceFile)
		test.FailNow()
	}

	obj := objs[0].(*v1alpha1.Instrumentation)
	obj.SetNamespace(ns)

	if err := client.Resources(ns).Create(ctx, obj); err != nil {
		test.Error(err, "failed when creating instrumentation")
		test.FailNow()
	}

	time.Sleep(time.Duration(time.Duration(delay) * time.Second))

	return nil
}

func (t *TestFrame) deployOtelCol(ctx context.Context, test *testing.T, cfg *envconf.Config, filename string, delay int) error {
	client, err := cfg.NewClient()
	if err != nil {
		test.Error(err, "cannot get k8s client")
		test.FailNow()
	}

	ns := ctx.Value(testenv.getNamespaceKey(test)).(string)

	resourceFile := t.fullFilename(filename)
	objs, err := testenv.loadObjectsFile(resourceFile)
	if err != nil {
		test.Error(err, "cannot read otelcol resource definition file", resourceFile)
		test.FailNow()
	}

	obj := objs[0].(*v1alpha1.OpenTelemetryCollector)
	obj.SetNamespace(ns)

	if err := client.Resources(ns).Create(ctx, obj); err != nil {
		test.Error(err, "failed when creating otelcol")
		test.FailNow()
	}

	time.Sleep(time.Duration(time.Duration(delay) * time.Second))

	return nil
}

func (t *TestFrame) deployAndAssertPod(ctx context.Context, test *testing.T, cfg *envconf.Config, filenamePodToDeploy, filenamePodToAssert string) (bool, error, []string) {
	client, err := cfg.NewClient()
	if err != nil {
		test.Error(err, "cannot get k8s client")
		test.FailNow()
	}

	ns := ctx.Value(testenv.getNamespaceKey(test)).(string)

	resourceFile := t.fullFilename(filenamePodToDeploy)
	objs, err := testenv.loadObjectsFile(resourceFile)
	if err != nil {
		test.Error(err, "cannot read pod resource definition file", resourceFile)
		test.FailNow()
	}

	obj := objs[0].(*v1.Pod)
	obj.SetNamespace(ns)
	kind := obj.Kind
	apiVersion := obj.APIVersion

	if err := client.Resources(ns).Create(ctx, obj); err != nil {
		test.Error(err, "failed when creating pod")
		test.FailNow()
	}

	err = wait.For(conditions.New(client.Resources()).PodConditionMatch(obj, v1.PodReady, v1.ConditionTrue), wait.WithTimeout(time.Minute*5))
	if err != nil {
		test.Error(err, "failed when waiting for pod")
		test.FailNow()
	}

	eobj := &v1.Pod{}

	if err := client.Resources(ns).Get(ctx, obj.Name, ns, eobj); err != nil {
		test.Error(err, "failed when reading pod")
		test.FailNow()
	}
	eobj.Kind = kind
	eobj.APIVersion = apiVersion

	assertFile := t.fullFilename(filenamePodToAssert)
	assertObj, err := testenv.loadObjectsFile(assertFile)
	if err != nil {
		test.Error(err, "cannot read pod assertion definition file", assertFile)
		test.FailNow()
	}

	eq, diffs := testenv.compareObjects(eobj, assertObj[0])
	test.Logf("objects are equal: %t, %s", eq, diffs)

	return eq, nil, diffs
}

func (t *TestFrame) formatDiffs(diffs []string) string {
	out := ""
	for _, d := range diffs {
		out = out + d + "\n"
	}
	return out
}

func (t *TestFrame) wait() {
	time.Sleep(SLEEP_SECONDS_ON_FAIL * time.Second)
}

func (t *TestFrame) getEnv(env string, defVal string) string {
	val := defVal
	envVal := os.Getenv(env)
	if envVal != "" {
		val = envVal
	}
	return val
}

func (t *TestFrame) deleteAppDAppAfterTest(name string) {
	return

	// TODO - unfinished job here

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	protocol := "https"
	if t.getSecret("APPD_CONTROLLER_SECURE") == "false" {
		protocol = "http"
	}

	req, err := http.NewRequest("DELETE", protocol+"://"+t.getSecret("APPD_CONTROLLER")+
		":"+t.getSecret("APPD_CONTROLLER_PORT")+"/bucket/sample", nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	req.SetBasicAuth("singularity-agent@"+t.getSecret("APPD_ACCOUNT"), t.getSecret("APPD_ACCESS_KEY"))

	// Fetch Request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer resp.Body.Close()

	// Read Response Body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Display Results
	fmt.Println("response Status : ", resp.Status)
	fmt.Println("response Headers : ", resp.Header)
	fmt.Println("response Body : ", string(respBody))

	// TIER_ID=`wget --user singularity-agent@${APPDYNAMICS_CONTROLLER_ACCOUNT_NAME} --password ${APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY} https://${APPDYNAMICS_CONTROLLER_HOST_NAME}:${APPDYNAMICS_CONTROLLER_PORT}/controller/rest/applications/${APPDYNAMICS_AGENT_APPLICATION_NAME}/tiers/${APPDYNAMICS_AGENT_TIER_NAME}?output=json -O -| grep -o '"id".*' | cut -d ":" -f 2 | sed 's/^[ \t,]*//;s/[ \t,]*$//'`

}
