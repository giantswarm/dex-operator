package util

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	AppLabel         = "app.kubernetes.io/name"
	DexAppLabelValue = "dex-app"
)

func DexLabelSelector() metav1.LabelSelector {
	return metav1.LabelSelector{
		MatchLabels: map[string]string{
			AppLabel: DexAppLabelValue,
		},
	}
}
