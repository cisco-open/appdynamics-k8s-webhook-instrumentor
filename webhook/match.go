package main

import (
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"
)

func isMatch(pod corev1.Pod, rules MatchRules) bool {

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

func getInstrumentationRule(pod corev1.Pod) *InstrumentationRule {

	config.mutex.Lock()
	defer config.mutex.Unlock()

	fmt.Printf("Config: %v\n", config)

	for _, rule := range *config.InstrumentationConfig {
		fmt.Printf("Checking rule: %s\n", rule.Name)
		if isMatch(pod, *rule.MatchRules) {
			return &rule
		}
	}

	return nil
}
