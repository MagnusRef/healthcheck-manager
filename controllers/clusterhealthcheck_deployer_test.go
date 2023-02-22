/*
Copyright 2023. projectsveltos.io. All rights reserved.

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

package controllers_test

import (
	"context"
	"reflect"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/projectsveltos/healthcheck-manager/controllers"
	"github.com/projectsveltos/healthcheck-manager/pkg/scope"
	libsveltosv1alpha1 "github.com/projectsveltos/libsveltos/api/v1alpha1"
	"github.com/projectsveltos/libsveltos/lib/deployer"
	fakedeployer "github.com/projectsveltos/libsveltos/lib/deployer/fake"
	libsveltosset "github.com/projectsveltos/libsveltos/lib/set"
)

var _ = Describe("ClusterHealthCheck deployer", func() {
	It("removeConditionEntry removes cluster entry", func() {
		clusterNamespace := randomString()
		clusterName := randomString()
		clusterType := libsveltosv1alpha1.ClusterTypeCapi

		chc := &libsveltosv1alpha1.ClusterHealthCheck{
			ObjectMeta: metav1.ObjectMeta{
				Name: randomString(),
			},
			Status: libsveltosv1alpha1.ClusterHealthCheckStatus{
				ClusterConditions: []libsveltosv1alpha1.ClusterCondition{
					*getClusterCondition(clusterNamespace, clusterName, clusterType),
					*getClusterCondition(clusterNamespace, randomString(), clusterType),
					*getClusterCondition(randomString(), clusterName, clusterType),
					*getClusterCondition(clusterNamespace, clusterName, libsveltosv1alpha1.ClusterTypeSveltos),
				},
			},
		}

		initObjects := []client.Object{
			chc,
		}

		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()

		length := len(chc.Status.ClusterConditions)

		Expect(controllers.RemoveConditionEntry(context.TODO(), c, clusterNamespace, clusterName,
			clusterType, chc, klogr.New())).To(Succeed())

		currentChc := &libsveltosv1alpha1.ClusterHealthCheck{}
		Expect(c.Get(context.TODO(), types.NamespacedName{Name: chc.Name}, currentChc)).To(Succeed())

		Expect(len(currentChc.Status.ClusterConditions)).To(Equal(length - 1))
	})

	It("updateNotificationSummariesForCluster updates entry for cluster", func() {
		clusterNamespace := randomString()
		clusterName := randomString()
		clusterType := libsveltosv1alpha1.ClusterTypeCapi

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: clusterNamespace,
				Name:      clusterName,
			},
		}

		chc := &libsveltosv1alpha1.ClusterHealthCheck{
			ObjectMeta: metav1.ObjectMeta{
				Name: randomString(),
			},
			Status: libsveltosv1alpha1.ClusterHealthCheckStatus{
				ClusterConditions: []libsveltosv1alpha1.ClusterCondition{
					*getClusterCondition(clusterNamespace, clusterName, clusterType),
					*getClusterCondition(clusterNamespace, randomString(), clusterType),
					*getClusterCondition(randomString(), clusterName, clusterType),
					*getClusterCondition(clusterNamespace, clusterName, libsveltosv1alpha1.ClusterTypeSveltos),
				},
			},
		}

		initObjects := []client.Object{
			chc, cluster,
		}

		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()

		notificationSummary := libsveltosv1alpha1.NotificationSummary{
			Name:   randomString(),
			Status: libsveltosv1alpha1.NotificationStatusDelivered,
		}

		summaries := []libsveltosv1alpha1.NotificationSummary{
			notificationSummary,
		}

		Expect(controllers.UpdateNotificationSummariesForCluster(context.TODO(), c, clusterNamespace, clusterName, clusterType,
			chc, summaries, klogr.New())).To(Succeed())

		currentChc := &libsveltosv1alpha1.ClusterHealthCheck{}
		Expect(c.Get(context.TODO(), types.NamespacedName{Name: chc.Name}, currentChc)).To(Succeed())
		Expect(len(currentChc.Status.ClusterConditions)).To(Equal(len(chc.Status.ClusterConditions)))

		var currentNotificationSummaries []libsveltosv1alpha1.NotificationSummary
		for i := range currentChc.Status.ClusterConditions {
			cc := &currentChc.Status.ClusterConditions[i]
			if cc.ClusterInfo.Cluster.Namespace == clusterNamespace &&
				cc.ClusterInfo.Cluster.Name == clusterName &&
				cc.ClusterInfo.Cluster.Kind == "Cluster" {
				currentNotificationSummaries = cc.NotificationSummaries
			}
		}

		Expect(currentNotificationSummaries).ToNot((BeNil()))
		Expect(len(currentNotificationSummaries)).To(Equal(1))
		Expect(reflect.DeepEqual(currentNotificationSummaries[0], notificationSummary)).To(BeTrue())
	})

	It("updateConditionsForCluster updates entry for cluster", func() {
		clusterNamespace := randomString()
		clusterName := randomString()
		clusterType := libsveltosv1alpha1.ClusterTypeCapi

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: clusterNamespace,
				Name:      clusterName,
			},
		}

		chc := &libsveltosv1alpha1.ClusterHealthCheck{
			ObjectMeta: metav1.ObjectMeta{
				Name: randomString(),
			},
			Status: libsveltosv1alpha1.ClusterHealthCheckStatus{
				ClusterConditions: []libsveltosv1alpha1.ClusterCondition{
					*getClusterCondition(clusterNamespace, clusterName, clusterType),
					*getClusterCondition(clusterNamespace, randomString(), clusterType),
					*getClusterCondition(randomString(), clusterName, clusterType),
					*getClusterCondition(clusterNamespace, clusterName, libsveltosv1alpha1.ClusterTypeSveltos),
				},
			},
		}

		initObjects := []client.Object{
			chc, cluster,
		}

		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjects...).Build()

		livenessCheck := libsveltosv1alpha1.LivenessCheck{
			Name: randomString(),
			Type: libsveltosv1alpha1.LivenessTypeAddons,
		}

		conditions := []libsveltosv1alpha1.Condition{
			{
				Type:   libsveltosv1alpha1.ConditionType(controllers.GetConditionType(&livenessCheck)),
				Status: corev1.ConditionTrue,
			},
		}

		Expect(controllers.UpdateConditionsForCluster(context.TODO(), c, clusterNamespace, clusterName, clusterType,
			chc, conditions, klogr.New())).To(Succeed())

		currentChc := &libsveltosv1alpha1.ClusterHealthCheck{}
		Expect(c.Get(context.TODO(), types.NamespacedName{Name: chc.Name}, currentChc)).To(Succeed())
		Expect(len(currentChc.Status.ClusterConditions)).To(Equal(len(chc.Status.ClusterConditions)))

		var currentConditions []libsveltosv1alpha1.Condition
		for i := range currentChc.Status.ClusterConditions {
			cc := &currentChc.Status.ClusterConditions[i]
			if cc.ClusterInfo.Cluster.Namespace == clusterNamespace &&
				cc.ClusterInfo.Cluster.Name == clusterName &&
				cc.ClusterInfo.Cluster.Kind == "Cluster" {
				currentConditions = cc.Conditions
			}
		}

		Expect(currentConditions).ToNot((BeNil()))
		Expect(len(currentConditions)).To(Equal(1))
		Expect(reflect.DeepEqual(conditions, currentConditions)).To(BeTrue())
	})

	It("evaluateClusterHealthCheckForCluster ", func() {
		clusterNamespace := randomString()
		clusterName := randomString()
		clusterType := libsveltosv1alpha1.ClusterTypeCapi

		// Following creates a ClusterSummary and an empty ClusterHealthCheck
		c := prepareClientWithClusterSummaryAndCHC(clusterNamespace, clusterName, clusterType)

		// Verify clusterHealthCheck has been created
		chcs := &libsveltosv1alpha1.ClusterHealthCheckList{}
		Expect(c.List(context.TODO(), chcs)).To(Succeed())
		Expect(len(chcs.Items)).To(Equal(1))

		Expect(c.List(context.TODO(), chcs)).To(Succeed())
		Expect(len(chcs.Items)).To(Equal(1))

		// Because ClusterSummary has been created with all add-ons provisioned, expect:
		// - passing to be true
		// - condition status to be true
		Expect(len(chcs.Items[0].Spec.LivenessChecks)).To(Equal(1))
		livenessCheck := chcs.Items[0].Spec.LivenessChecks[0]
		conditions, passing, err := controllers.EvaluateClusterHealthCheckForCluster(context.TODO(), c, clusterNamespace, clusterName,
			clusterType, &chcs.Items[0], klogr.New())
		Expect(err).To(BeNil())
		Expect(passing).To(BeTrue())
		Expect(conditions).ToNot(BeNil())
		Expect(len(conditions)).To(Equal(1))
		Expect(conditions[0].Status).To(Equal(corev1.ConditionTrue))
		Expect(conditions[0].Type).To(Equal(libsveltosv1alpha1.ConditionType(controllers.GetConditionType(&livenessCheck))))
	})

	It("processClusterHealthCheckForCluster updating ClusterHealthCheck Status", func() {
		clusterNamespace := randomString()
		clusterName := randomString()
		clusterType := libsveltosv1alpha1.ClusterTypeCapi

		// Following creates a ClusterSummary and an empty ClusterHealthCheck
		c := prepareClientWithClusterSummaryAndCHC(clusterNamespace, clusterName, clusterType)

		// Verify clusterHealthCheck has been created
		chcs := &libsveltosv1alpha1.ClusterHealthCheckList{}
		Expect(c.List(context.TODO(), chcs)).To(Succeed())
		Expect(len(chcs.Items)).To(Equal(1))

		chc := chcs.Items[0]
		chc.Status.ClusterConditions = make([]libsveltosv1alpha1.ClusterCondition, 1)
		chc.Status.ClusterConditions[0] = libsveltosv1alpha1.ClusterCondition{
			ClusterInfo: libsveltosv1alpha1.ClusterInfo{
				Cluster: corev1.ObjectReference{
					Namespace:  clusterNamespace,
					Name:       clusterName,
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		}
		Expect(c.Status().Update(context.TODO(), &chc)).To(Succeed())

		Expect(controllers.ProcessClusterHealthCheckForCluster(context.TODO(), c, clusterNamespace, clusterName, chc.Name,
			libsveltosv1alpha1.FeatureClusterHealthCheck, clusterType, deployer.Options{}, klogr.New())).To(Succeed())

		currentClusterHealthCheck := &libsveltosv1alpha1.ClusterHealthCheck{}
		Expect(c.Get(context.TODO(), types.NamespacedName{Name: chc.Name}, currentClusterHealthCheck)).To(Succeed())
		Expect(currentClusterHealthCheck.Status.ClusterConditions).ToNot(BeNil())
		Expect(len(currentClusterHealthCheck.Status.ClusterConditions)).To(Equal(1))
		Expect(currentClusterHealthCheck.Status.ClusterConditions[0].Conditions).ToNot(BeNil())
		Expect(len(currentClusterHealthCheck.Status.ClusterConditions[0].Conditions)).To(Equal(1))
		Expect(currentClusterHealthCheck.Status.ClusterConditions[0].Conditions[0].Status).To(Equal(corev1.ConditionTrue))
	})

	It("processClusterHealthCheck ", func() {
		clusterNamespace := randomString()
		clusterName := randomString()
		clusterType := libsveltosv1alpha1.ClusterTypeCapi

		// Following creates a ClusterSummary and an empty ClusterHealthCheck
		c := prepareClientWithClusterSummaryAndCHC(clusterNamespace, clusterName, clusterType)

		// Add machine to mark Cluster ready
		cpMachine := &clusterv1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: clusterNamespace,
				Name:      randomString(),
				Labels: map[string]string{
					clusterv1.ClusterLabelName:             clusterName,
					clusterv1.MachineControlPlaneLabelName: "ok",
				},
			},
		}
		cpMachine.Status.SetTypedPhase(clusterv1.MachinePhaseRunning)

		Expect(c.Create(context.TODO(), cpMachine)).To(Succeed())

		// Verify clusterHealthCheck has been created
		chcs := &libsveltosv1alpha1.ClusterHealthCheckList{}
		Expect(c.List(context.TODO(), chcs)).To(Succeed())
		Expect(len(chcs.Items)).To(Equal(1))

		chc := chcs.Items[0]

		dep := fakedeployer.GetClient(context.TODO(), klogr.New(), testEnv.Client)
		controllers.RegisterFeatures(dep, klogr.New())

		reconciler := controllers.ClusterHealthCheckReconciler{
			Client:                c,
			Deployer:              dep,
			Scheme:                c.Scheme(),
			Mux:                   sync.Mutex{},
			ClusterMap:            make(map[corev1.ObjectReference]*libsveltosset.Set),
			ClusterHealthCheckMap: make(map[corev1.ObjectReference]*libsveltosset.Set),
			ClusterHealthChecks:   make(map[corev1.ObjectReference]libsveltosv1alpha1.Selector),
		}

		chcScope, err := scope.NewClusterHealthCheckScope(scope.ClusterHealthCheckScopeParams{
			Client:             c,
			Logger:             klogr.New(),
			ClusterHealthCheck: &chc,
			ControllerName:     "classifier",
		})
		Expect(err).To(BeNil())

		currentCluster := &clusterv1.Cluster{}
		Expect(c.Get(context.TODO(), types.NamespacedName{Namespace: clusterNamespace, Name: clusterName}, currentCluster)).To(Succeed())
		Expect(addTypeInformationToObject(c.Scheme(), currentCluster)).To(Succeed())

		f := controllers.GetHandlersForFeature(libsveltosv1alpha1.FeatureClusterHealthCheck)
		clusterInfo, err := controllers.ProcessClusterHealthCheck(&reconciler, context.TODO(), chcScope,
			controllers.GetKeyFromObject(c.Scheme(), currentCluster), f, klogr.New())
		Expect(err).To(BeNil())

		Expect(clusterInfo).ToNot(BeNil())
		Expect(clusterInfo.Status).To(Equal(libsveltosv1alpha1.SveltosStatusProvisioning))

		// Expect job to be queued
		Expect(dep.IsInProgress(clusterNamespace, clusterName, chc.Name, libsveltosv1alpha1.FeatureClusterHealthCheck,
			clusterType, false)).To(BeTrue())
	})

	It("isClusterEntryRemoved returns true when there is no entry for a Cluster in ClusterHealthCheck status", func() {
		clusterNamespace := randomString()
		clusterName := randomString()
		clusterType := libsveltosv1alpha1.ClusterTypeCapi

		// Following creates a ClusterSummary and an empty ClusterHealthCheck
		c := prepareClientWithClusterSummaryAndCHC(clusterNamespace, clusterName, clusterType)

		dep := fakedeployer.GetClient(context.TODO(), klogr.New(), testEnv.Client)
		controllers.RegisterFeatures(dep, klogr.New())

		reconciler := controllers.ClusterHealthCheckReconciler{
			Client:                c,
			Deployer:              dep,
			Scheme:                c.Scheme(),
			Mux:                   sync.Mutex{},
			ClusterMap:            make(map[corev1.ObjectReference]*libsveltosset.Set),
			ClusterHealthCheckMap: make(map[corev1.ObjectReference]*libsveltosset.Set),
			ClusterHealthChecks:   make(map[corev1.ObjectReference]libsveltosv1alpha1.Selector),
		}

		// Verify clusterHealthCheck has been created
		chcs := &libsveltosv1alpha1.ClusterHealthCheckList{}
		Expect(c.List(context.TODO(), chcs)).To(Succeed())
		Expect(len(chcs.Items)).To(Equal(1))

		chc := chcs.Items[0]

		currentCluster := &clusterv1.Cluster{}
		Expect(c.Get(context.TODO(), types.NamespacedName{Namespace: clusterNamespace, Name: clusterName}, currentCluster)).To(Succeed())
		Expect(addTypeInformationToObject(c.Scheme(), currentCluster)).To(Succeed())

		Expect(controllers.IsClusterEntryRemoved(&reconciler, &chc, controllers.GetKeyFromObject(c.Scheme(), currentCluster))).To(BeTrue())

		chc.Status.ClusterConditions = []libsveltosv1alpha1.ClusterCondition{
			{
				ClusterInfo: libsveltosv1alpha1.ClusterInfo{
					Cluster: *controllers.GetKeyFromObject(c.Scheme(), currentCluster),
				},
			},
		}
		Expect(c.Status().Update(context.TODO(), &chc)).To(Succeed())

		Expect(controllers.IsClusterEntryRemoved(&reconciler, &chc, controllers.GetKeyFromObject(c.Scheme(), currentCluster))).To(BeFalse())
	})
})

func getClusterCondition(clusterNamespace, clusterName string, clusterType libsveltosv1alpha1.ClusterType) *libsveltosv1alpha1.ClusterCondition {
	var apiVersion, kind string
	if clusterType == libsveltosv1alpha1.ClusterTypeCapi {
		apiVersion = clusterv1.GroupVersion.String()
		kind = "Cluster"
	} else {
		apiVersion = libsveltosv1alpha1.GroupVersion.String()
		kind = libsveltosv1alpha1.SveltosClusterKind
	}

	return &libsveltosv1alpha1.ClusterCondition{
		ClusterInfo: libsveltosv1alpha1.ClusterInfo{
			Cluster: corev1.ObjectReference{
				Namespace:  clusterNamespace,
				Name:       clusterName,
				Kind:       kind,
				APIVersion: apiVersion,
			},
		},
	}
}