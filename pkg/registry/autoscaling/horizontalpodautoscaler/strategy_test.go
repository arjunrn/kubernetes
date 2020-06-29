package horizontalpodautoscaler

import (
	"context"
	"testing"

	utilfeature "k8s.io/apiserver/pkg/util/feature"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/utils/pointer"
)

func TestAutoscalerStrategy_PrepareForCreate(t *testing.T) {
	defer featuregatetesting.SetFeatureGateDuringTest(t, utilfeature.DefaultFeatureGate, features.HPAContainerMetrics, false)()
	testHPA := &autoscaling.HorizontalPodAutoscaler{
		Spec: autoscaling.HorizontalPodAutoscalerSpec{
			Metrics: []autoscaling.MetricSpec{
				{
					Type: autoscaling.ContainerResourceMetricSourceType,
					ContainerResource: &autoscaling.ContainerResourceMetricSource{
						Name:      core.ResourceCPU,
						Container: "test-container",
						Target: autoscaling.MetricTarget{
							Type:               autoscaling.UtilizationMetricType,
							AverageUtilization: pointer.Int32Ptr(30),
						},
					},
				},
			},
		},
	}
	Strategy.PrepareForCreate(context.Background(), testHPA)
	if testHPA.Spec.Metrics[0].ContainerResource != nil {
		t.Errorf("container metric source was not dropped")
	}
}
