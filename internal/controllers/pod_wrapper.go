package controllers

import (
	v1 "k8s.io/api/core/v1"
)

// podWrapper contains all operations releated to a Pod.
type podWrapper struct {
	pod *v1.Pod
}

func (w podWrapper) isKetchApplicationPod() bool {
	_, ok := w.pod.Labels[labelAppName]
	return ok
}

func (w podWrapper) AppName() string {
	return w.pod.Labels[labelAppName]
}