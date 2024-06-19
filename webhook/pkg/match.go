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
	"bytes"
	"fmt"
	"log"
	"regexp"
	"text/template"
	"v1alpha1"

	corev1 "k8s.io/api/core/v1"
)

func getFlexMatch(pod corev1.Pod, flexMatchTmpl *template.Template) string {

	var res bytes.Buffer
	err := flexMatchTmpl.Execute(&res, pod)
	if err != nil {
		log.Printf("Error executing flex match %s: %v\n", flexMatchTmpl.Name(), err)
		return ""
	}

	log.Printf("Pod: %s, %s, Flex Match Result: %s\n", pod.Name, pod.GetName(), res.String())
	return res.String()
}

func isMatch(pod corev1.Pod, rules v1alpha1.MatchRule) bool {

	if rules.NamespaceRegex != "" {
		match, _ := regexp.MatchString(rules.NamespaceRegex, pod.GetNamespace())
		if !match {
			fmt.Printf("Namespace regex '%s' did not match %s\n", rules.NamespaceRegex, pod.GetNamespace())
			return false
		}
	}
	if rules.PodNameRegex != "" {
		match, _ := regexp.MatchString(rules.PodNameRegex, pod.GetName())
		if !match {
			return false
		}
	}
	if rules.Annotations != nil {
		for _, annotRule := range *rules.Annotations {
			for annot, regex := range annotRule {
				// lookup rule annotation name in pod. If not found, return false. If found, check regex
				podAnnots := pod.GetAnnotations()
				podAnnotVal, found := podAnnots[annot]
				if !found {
					return false
				}
				match, _ := regexp.MatchString(regex, podAnnotVal)
				if !match {
					return false
				}
			}
		}
	}
	if rules.Labels != nil {
		for _, labelRule := range *rules.Labels {
			for label, regex := range labelRule {
				// lookup rule annotation name in pod. If not found, return false. If found, check regex
				podLabels := pod.GetLabels()
				podLabelVal, found := podLabels[label]
				if !found {
					return false
				}
				match, _ := regexp.MatchString(regex, podLabelVal)
				if !match {
					return false
				}
			}
		}
	}
	return true
}

func getInstrumentationRule(pod corev1.Pod) *v1alpha1.InstrumentationSpec {

	config.mutex.Lock()
	defer config.mutex.Unlock()

	// fmt.Printf("Config: %v\n", config)

	if config.FlexMatchTemplate != nil {
		getFlexMatch(pod, config.FlexMatchTemplate)
	}

	log.Default().Printf("Matching started\n")

	// First check if pod matches any namespaced Instrumentation - based rule
	if !config.CrdsDisabled {
		log.Default().Printf("Checking namespaced rules\n")
		if instrConfig, ok := config.InstrumentationNamespacedCrds[pod.GetNamespace()]; ok {
			for _, rule := range *instrConfig {
				log.Default().Printf("Checking namespaced rule: %s\n", rule.Name)
				if isMatch(pod, *rule.MatchRules) {
					return &rule
				}
			}
		}
	}

	log.Default().Printf("Checking global rules\n")

	for _, rule := range *config.InstrumentationClusterCrds {
		log.Default().Printf("Checking cluster-wide rule: %s\n", rule.Name)
		if isMatch(pod, *rule.MatchRules) {
			return &rule
		}
	}

	log.Default().Printf("Checking config map rules\n")

	for _, rule := range *config.InstrumentationConfig {
		log.Default().Printf("Checking config map rule: %s\n", rule.Name)
		if isMatch(pod, *rule.MatchRules) {
			return &rule
		}
	}

	return nil
}
