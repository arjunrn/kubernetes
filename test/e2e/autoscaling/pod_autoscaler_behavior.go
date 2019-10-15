package autoscaling

import (
	"time"

	"github.com/onsi/ginkgo"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/test/e2e/common"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	toleranceSeconds = 20
)

var _ = SIGDescribe("[Feature:HPABehavior] HPA Scaling with Behavior", func() {
	f := framework.NewDefaultFramework("horizontal-pod-autoscaling")
	SIGDescribe("[Serial] [Slow] Scale Up 1 Pod at a time over 1 minute", func() {
		ginkgo.It("should scale up by 1 pod", func() {
			scaleUpWithBehavior("test-deployment", f)
		})

		ginkgo.It("should scale down by 1 pod", func() {
			scaleDownWithBehavior("test-deployment", f)
		})
	})

	SIGDescribe("[Serial] [Slow] Scale Up 50% of current replicas over 1 minute", func() {
		ginkgo.It("Should scale up by 50% of current replicas", func() {
			scaleUpWithBehaviorPercent("test-deployment", f)
		})

		ginkgo.It("Should scale down by 50% of current replicas", func() {
			scaleDownWithBehaviorPercent("test-deployment", f)
		})
	})
})

func scaleUpWithBehavior(name string, f *framework.Framework) {
	scaleTest := &HPABehaviorTest{
		minReplicas:          1,
		maxReplicas:          5,
		podCPULimit:          100,
		scaleUpPods:          1,
		scaleUpPeriod:        90,
		scaleUpStabilization: 0,
		startCPUConsume:      0,
		startUtilization:     0,
		startReplicas:        1,
		endCPUConsume:        500,
		endUtilization:       10,
		endReplicas:          3,
		duration:             90,
	}
	scaleTest.Run(name, common.KindDeployment, f)
}

func scaleUpWithBehaviorPercent(name string, f *framework.Framework) {
	scaleTest := &HPABehaviorTest{
		minReplicas:          3,
		maxReplicas:          10,
		podCPULimit:          100,
		scaleUpPercent:       40,
		scaleUpPeriod:        60,
		scaleUpStabilization: 0,
		startCPUConsume:      0,
		startUtilization:     0,
		startReplicas:        3,
		endCPUConsume:        500,
		endUtilization:       10,
		endReplicas:          7,
		duration:             60,
	}
	scaleTest.Run(name, common.KindDeployment, f)
}

func scaleDownWithBehaviorPercent(name string, f *framework.Framework) {
	scaleTest := &HPABehaviorTest{
		minReplicas:            1,
		maxReplicas:            7,
		podCPULimit:            100,
		scaleDownPercent:       20,
		scaleDownPeriod:        60,
		scaleDownStabilization: 0,
		startCPUConsume:        100,
		startUtilization:       10,
		startReplicas:          7,
		endCPUConsume:          0,
		endUtilization:         10,
		endReplicas:            4,
		duration:               60,
	}
	scaleTest.Run(name, common.KindDeployment, f)
}

func scaleDownWithBehavior(name string, f *framework.Framework) {
	scaleTest := &HPABehaviorTest{
		minReplicas:            1,
		maxReplicas:            5,
		podCPULimit:            100,
		scaleDownStabilization: 0,
		scaleDownPods:          1,
		scaleDownPeriod:        150,
		startCPUConsume:        500,
		startUtilization:       10,
		startReplicas:          5,
		endCPUConsume:          0,
		endUtilization:         10,
		endReplicas:            3,
		duration:               150,
	}
	scaleTest.Run(name, common.KindDeployment, f)
}

type HPABehaviorTest struct {
	minReplicas          int32
	maxReplicas          int32
	podCPULimit          int64
	scaleUpStabilization int32
	scaleUpPods          int32
	scaleUpPeriod        int32
	scaleUpPercent       int32

	scaleDownStabilization int32
	scaleDownPods          int32
	scaleDownPercent       int32
	scaleDownPeriod        int32

	startCPUConsume  int
	startUtilization int32
	startReplicas    int

	endCPUConsume  int
	endUtilization int32
	endReplicas    int

	duration int32
}

// run is a method which runs an HPA lifecycle, from a starting state, to an expected
// The initial state is defined by the initPods parameter.
// The first state change is due to the CPU being consumed initially, which HPA responds to by changing pod counts.
// The second state change (optional) is due to the CPU burst parameter, which HPA again responds to.
func (tc *HPABehaviorTest) Run(name string, kind schema.GroupVersionKind, f *framework.Framework) {
	const timeToWait = 15 * time.Minute
	rc := common.NewDynamicResourceConsumer(name, f.Namespace.Name, kind, tc.startReplicas, tc.startCPUConsume, 0, 0, tc.podCPULimit, 200, f.ClientSet, f.ScalesGetter)
	defer rc.CleanUp()

	creator := common.HPACreator{}
	creator.MinReplicas(tc.minReplicas).MaxReplicas(tc.maxReplicas).CPUUtilization(10)
	if tc.scaleUpPods > 0 {
		creator.ScaleUpPodPolicy(tc.scaleUpPods, tc.scaleUpPeriod, tc.scaleUpStabilization)
	}
	if tc.scaleUpPercent > 0 {
		creator.ScaleUpPercentPolicy(tc.scaleUpPercent, tc.scaleUpPeriod, tc.scaleUpStabilization)
	}
	if tc.scaleDownPods > 0 {
		creator.ScaleDownPodPolicy(tc.scaleDownPods, tc.scaleDownPeriod, tc.scaleDownStabilization)
	}
	if tc.scaleDownPercent > 0 {
		creator.ScaleDownPercentPolicy(tc.scaleDownPercent, tc.scaleDownPeriod, tc.scaleDownStabilization)
	}

	hpa, err := creator.Create(rc)
	framework.ExpectNoError(err)
	defer common.DeleteHorizontalPodAutoscaler(rc, hpa.Name)
	rc.ConsumeCPU(tc.startCPUConsume)

	rc.WaitForHPADetectMetric(hpa.Name, tc.startUtilization, timeToWait, false)
	rc.WaitForReplicas(tc.startReplicas, timeToWait)

	rc.ConsumeCPU(tc.endCPUConsume)
	rc.WaitForHPADetectMetric(hpa.Name, tc.endUtilization, timeToWait, tc.endUtilization <= tc.startUtilization)
	maxAllowed := time.Duration(tc.duration+toleranceSeconds) * time.Second
	minAllowed := time.Duration(tc.duration-toleranceSeconds) * time.Second
	period := rc.WaitForReplicas(tc.endReplicas, maxAllowed)
	if period > maxAllowed || period < minAllowed {
		framework.Failf("change took too long: %d seconds, expected: %d seconds with tolerance of %d seconds", period/time.Second, tc.duration, toleranceSeconds)
	}
	framework.Logf("expected duration: %d actual duration: %d seconds", tc.duration, period/time.Second)
}
