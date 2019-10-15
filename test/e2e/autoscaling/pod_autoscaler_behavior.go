package autoscaling

import (
	"context"
	"time"

	"github.com/onsi/ginkgo"
	gcm "google.golang.org/api/monitoring/v3"
	appsv1 "k8s.io/api/apps/v1"
	as "k8s.io/api/autoscaling/v2beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/common"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/instrumentation/monitoring"
)

var _ = SIGDescribe("[Feature:HPABehavior] HPA Scaling with Behavior", func() {
	var rc *common.ResourceConsumer
	f := framework.NewDefaultFramework("horizontal-pod-autoscaling")
	SIGDescribe("[Serial] [Slow] Scale Up 1 Pod at a time over 1 minute", func() {
		ginkgo.It("should scale up 1 pod", func() {
			scaleUpWithBehavior("test-deployment", common.KindDeployment, false, rc, f)
		})
	})
})

func scaleUpWithBehavior(name string, kind schema.GroupVersionKind, checkStability bool, rc *common.ResourceConsumer, f *framework.Framework) {
	test := &HPABehaviorTest{
		framework:       ,
		hpa:             nil,
		kubeClient:      nil,
		deployment:      nil,
		pod:             nil,
		initialReplicas: 0,
		scaledReplicas:  0,
	}
	test.Run()
	framework.Logf("Waiting for %d minutes", 10)
}

type HPABehaviorTest struct {
	framework       *framework.Framework
	hpa             *as.HorizontalPodAutoscaler
	kubeClient      clientset.Interface
	deployment      *appsv1.Deployment
	pod             *v1.Pod
	initialReplicas int
	scaledReplicas  int
}

// run is a method which runs an HPA lifecycle, from a starting state, to an expected
// The initial state is defined by the initPods parameter.
// The first state change is due to the CPU being consumed initially, which HPA responds to by changing pod counts.
// The second state change (optional) is due to the CPU burst parameter, which HPA again responds to.
func (tc *HPABehaviorTest) Run() {
	projectID := framework.TestContext.CloudConfig.ProjectID

	ctx := context.Background()
	//client, err := google.DefaultClient(ctx, gcm.CloudPlatformScope)

	// Hack for running tests locally, needed to authenticate in Stackdriver
	// If this is your use case, create application default credentials:
	// $ gcloud auth application-default login
	// and uncomment following lines:
	/*
		ts, err := google.DefaultTokenSource(oauth2.NoContext)
		framework.Logf("Couldn't get application default credentials, %v", err)
		if err != nil {
			framework.Failf("Error accessing application default credentials, %v", err)
		}
		client := oauth2.NewClient(oauth2.NoContext, ts)
	*/

	gcmService, err := gcm.NewService(ctx)
	if err != nil {
		framework.Failf("Failed to create gcm service, %v", err)
	}

	// Set up a cluster: create a custom metric and set up k8s-sd adapter
	err = monitoring.CreateDescriptors(gcmService, projectID)
	if err != nil {
		framework.Failf("Failed to create metric descriptor: %v", err)
	}
	defer monitoring.CleanupDescriptors(gcmService, projectID)

	err = monitoring.CreateAdapter(monitoring.AdapterDefault)
	if err != nil {
		framework.Failf("Failed to set up: %v", err)
	}
	defer monitoring.CleanupAdapter(monitoring.AdapterDefault)

	// Run application that exports the metric
	err = createDeploymentToScale(tc.framework, tc.kubeClient, tc.deployment, tc.pod)
	if err != nil {
		framework.Failf("Failed to create stackdriver-exporter pod: %v", err)
	}
	defer cleanupDeploymentsToScale(tc.framework, tc.kubeClient, tc.deployment, tc.pod)

	// Wait for the deployment to run
	waitForReplicas(tc.deployment.ObjectMeta.Name, tc.framework.Namespace.ObjectMeta.Name, tc.kubeClient, 15*time.Minute, tc.initialReplicas)

	// Autoscale the deployment
	_, err = tc.kubeClient.AutoscalingV2beta1().HorizontalPodAutoscalers(tc.framework.Namespace.ObjectMeta.Name).Create(tc.hpa)
	if err != nil {
		framework.Failf("Failed to create HPA: %v", err)
	}
	defer tc.kubeClient.AutoscalingV2beta1().HorizontalPodAutoscalers(tc.framework.Namespace.ObjectMeta.Name).Delete(tc.hpa.ObjectMeta.Name, &metav1.DeleteOptions{})

	waitForReplicas(tc.deployment.ObjectMeta.Name, tc.framework.Namespace.ObjectMeta.Name, tc.kubeClient, 15*time.Minute, tc.scaledReplicas)
}
