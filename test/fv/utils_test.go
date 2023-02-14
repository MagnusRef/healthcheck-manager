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

package fv_test

import (
	"context"
	"fmt"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	libsveltosv1alpha1 "github.com/projectsveltos/libsveltos/api/v1alpha1"
	configv1alpha1 "github.com/projectsveltos/sveltos-manager/api/v1alpha1"
	"github.com/projectsveltos/sveltos-manager/controllers"
)

const (
	key   = "env"
	value = "fv"
)

// Byf is a simple wrapper around By.
func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...)) // ignore_by_check
}

func randomString() string {
	const length = 10
	return util.RandomString(length)
}

func getClusterSummary(ctx context.Context,
	clusterProfileName, clusterNamespace, clusterName string) (*configv1alpha1.ClusterSummary, error) {

	listOptions := []client.ListOption{
		client.InNamespace(clusterNamespace),
		client.MatchingLabels{
			controllers.ClusterProfileLabelName: clusterProfileName,
			configv1alpha1.ClusterNameLabel:     clusterName,
			configv1alpha1.ClusterTypeLabel:     string(libsveltosv1alpha1.ClusterTypeCapi),
		},
	}

	clusterSummaryList := &configv1alpha1.ClusterSummaryList{}
	if err := k8sClient.List(ctx, clusterSummaryList, listOptions...); err != nil {
		return nil, err
	}

	if len(clusterSummaryList.Items) == 0 {
		return nil, apierrors.NewNotFound(
			schema.GroupResource{Group: configv1alpha1.GroupVersion.Group, Resource: configv1alpha1.ClusterSummaryKind}, "")
	}

	if len(clusterSummaryList.Items) != 1 {
		return nil, fmt.Errorf("more than one clustersummary found for cluster %s/%s created by %s",
			clusterNamespace, clusterName, clusterProfileName)
	}

	return &clusterSummaryList.Items[0], nil
}

func verifyClusterSummary(clusterProfile *configv1alpha1.ClusterProfile,
	clusterNamespace, clusterName string) *configv1alpha1.ClusterSummary {

	Byf("Verifying ClusterSummary is created")
	Eventually(func() bool {
		clusterSummary, err := getClusterSummary(context.TODO(),
			clusterProfile.Name, clusterNamespace, clusterName)
		return err == nil &&
			clusterSummary != nil
	}, timeout, pollingInterval).Should(BeTrue())

	clusterSummary, err := getClusterSummary(context.TODO(),
		clusterProfile.Name, clusterNamespace, clusterName)
	Expect(err).To(BeNil())
	Expect(clusterSummary).ToNot(BeNil())

	Byf("Verifying ClusterSummary ownerReference")
	ref, err := getClusterSummaryOwnerReference(clusterSummary)
	Expect(err).To(BeNil())
	Expect(ref).ToNot(BeNil())
	Expect(ref.Name).To(Equal(clusterProfile.Name))

	Byf("Verifying ClusterSummary configuration")
	Eventually(func() error {
		var currentClusterSummary *configv1alpha1.ClusterSummary
		currentClusterSummary, err = getClusterSummary(context.TODO(),
			clusterProfile.Name, clusterNamespace, clusterName)
		if err != nil {
			return err
		}

		if !reflect.DeepEqual(currentClusterSummary.Spec.ClusterProfileSpec.HelmCharts,
			clusterProfile.Spec.HelmCharts) {

			return fmt.Errorf("helmCharts do not match")
		}
		if !reflect.DeepEqual(currentClusterSummary.Spec.ClusterProfileSpec.PolicyRefs,
			clusterProfile.Spec.PolicyRefs) {

			return fmt.Errorf("policyRefs do not match")
		}
		if !reflect.DeepEqual(currentClusterSummary.Spec.ClusterNamespace, clusterNamespace) {
			return fmt.Errorf("clusterNamespace does not match")
		}
		if !reflect.DeepEqual(currentClusterSummary.Spec.ClusterName, clusterName) {
			return fmt.Errorf("clusterName does not match")
		}
		return nil
	}, timeout, pollingInterval).Should(BeNil())

	clusterSummary, err = getClusterSummary(context.TODO(),
		clusterProfile.Name, clusterNamespace, clusterName)
	Expect(err).To(BeNil())
	Expect(clusterSummary).ToNot(BeNil())

	return clusterSummary
}

func verifyFeatureStatusIsProvisioned(clusterSummaryNamespace, clusterSummaryName string, featureID configv1alpha1.FeatureID) {
	Eventually(func() bool {
		currentClusterSummary := &configv1alpha1.ClusterSummary{}
		err := k8sClient.Get(context.TODO(),
			types.NamespacedName{Namespace: clusterSummaryNamespace, Name: clusterSummaryName},
			currentClusterSummary)
		if err != nil {
			return false
		}
		for i := range currentClusterSummary.Status.FeatureSummaries {
			if currentClusterSummary.Status.FeatureSummaries[i].FeatureID == featureID &&
				currentClusterSummary.Status.FeatureSummaries[i].Status == configv1alpha1.FeatureStatusProvisioned {

				return true
			}
		}
		return false
	}, timeout, pollingInterval).Should(BeTrue())
}

func getClusterSummaryOwnerReference(clusterSummary *configv1alpha1.ClusterSummary) (*configv1alpha1.ClusterProfile, error) {
	Byf("Checking clusterSummary %s owner reference is set", clusterSummary.Name)
	for _, ref := range clusterSummary.OwnerReferences {
		if ref.Kind != configv1alpha1.ClusterProfileKind {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if gv.Group == configv1alpha1.GroupVersion.Group {
			clusterProfile := &configv1alpha1.ClusterProfile{}
			err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: ref.Name}, clusterProfile)
			return clusterProfile, err
		}
	}
	return nil, nil
}

func verifyClusterProfileMatches(clusterProfile *configv1alpha1.ClusterProfile) {
	Byf("Verifying Cluster %s/%s is a match for ClusterProfile %s",
		kindWorkloadCluster.Namespace, kindWorkloadCluster.Name, clusterProfile.Name)
	Eventually(func() bool {
		currentClusterProfile := &configv1alpha1.ClusterProfile{}
		err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: clusterProfile.Name}, currentClusterProfile)
		return err == nil &&
			len(currentClusterProfile.Status.MatchingClusterRefs) == 1 &&
			currentClusterProfile.Status.MatchingClusterRefs[0].Namespace == kindWorkloadCluster.Namespace &&
			currentClusterProfile.Status.MatchingClusterRefs[0].Name == kindWorkloadCluster.Name &&
			currentClusterProfile.Status.MatchingClusterRefs[0].APIVersion == clusterv1.GroupVersion.String()
	}, timeout, pollingInterval).Should(BeTrue())
}

func verifyClusterHealthCheckStatus(clusterHealthCheckName, clusterNamespace, clusterName string) {
	Eventually(func() bool {
		currentClusterHealthCheck := &libsveltosv1alpha1.ClusterHealthCheck{}
		err := k8sClient.Get(context.TODO(),
			types.NamespacedName{Name: clusterHealthCheckName},
			currentClusterHealthCheck)
		if err != nil {
			return false
		}

		clusterMatching := false
		for i := range currentClusterHealthCheck.Status.MatchingClusterRefs {
			mCluster := &currentClusterHealthCheck.Status.MatchingClusterRefs[i]
			if mCluster.Namespace == clusterNamespace && mCluster.Name == clusterName {
				clusterMatching = true
			}
		}
		if !clusterMatching {
			return false
		}

		clusterConditionFound := false
		for i := range currentClusterHealthCheck.Status.ClusterConditions {
			cc := &currentClusterHealthCheck.Status.ClusterConditions[i]
			if isClusterConditionForCluster(cc, clusterNamespace, clusterName) {
				clusterConditionFound = true

				if !verifyLivenessChecks(cc) {
					return false
				}

				if !verifyNotifications(cc) {
					return false
				}
			}
		}

		return clusterConditionFound
	}, timeout, pollingInterval).Should(BeTrue())
}

func verifyLivenessChecks(cc *libsveltosv1alpha1.ClusterCondition) bool {
	if len(cc.Conditions) == 0 {
		return false
	}

	for i := range cc.Conditions {
		c := &cc.Conditions[i]
		if c.Status != corev1.ConditionTrue {
			return false
		}
	}

	return true
}

func verifyNotifications(cc *libsveltosv1alpha1.ClusterCondition) bool {
	if len(cc.NotificationSummaries) == 0 {
		return false
	}

	for i := range cc.NotificationSummaries {
		ns := &cc.NotificationSummaries[i]
		if ns.Status != libsveltosv1alpha1.NotificationStatusDelivered {
			return false
		}
	}

	return true
}

func getClusterHealthCheck(namePrefix string, clusterLabels map[string]string,
	lc []libsveltosv1alpha1.LivenessCheck, notifications []libsveltosv1alpha1.Notification) *libsveltosv1alpha1.ClusterHealthCheck {
	selector := ""
	for k := range clusterLabels {
		if selector != "" {
			selector += ","
		}
		selector += fmt.Sprintf("%s=%s", k, clusterLabels[k])
	}
	clusterHealthCheck := &libsveltosv1alpha1.ClusterHealthCheck{
		ObjectMeta: metav1.ObjectMeta{
			Name: namePrefix + randomString(),
		},
		Spec: libsveltosv1alpha1.ClusterHealthCheckSpec{
			ClusterSelector: libsveltosv1alpha1.Selector(selector),
			LivenessChecks:  lc,
			Notifications:   notifications,
		},
	}

	return clusterHealthCheck
}

// getClusterProfile returns a ClusterProfile with Kyverno helm chart
func getClusterProfile(namePrefix string, clusterLabels map[string]string) *configv1alpha1.ClusterProfile {
	selector := ""
	for k := range clusterLabels {
		if selector != "" {
			selector += ","
		}
		selector += fmt.Sprintf("%s=%s", k, clusterLabels[k])
	}
	clusterProfile := &configv1alpha1.ClusterProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: namePrefix + randomString(),
		},
		Spec: configv1alpha1.ClusterProfileSpec{
			ClusterSelector: libsveltosv1alpha1.Selector(selector),
			HelmCharts: []configv1alpha1.HelmChart{
				{
					RepositoryURL:    "https://kyverno.github.io/kyverno/",
					RepositoryName:   "kyverno",
					ChartName:        "kyverno/kyverno",
					ChartVersion:     "v2.6.5",
					ReleaseName:      "kyverno-latest",
					ReleaseNamespace: "kyverno",
					HelmChartAction:  configv1alpha1.HelmChartActionInstall,
				},
			},
		},
	}

	return clusterProfile
}

// isClusterConditionForCluster returns true if the ClusterCondition is for the cluster clusterType, clusterNamespace,
// clusterName
func isClusterConditionForCluster(cc *libsveltosv1alpha1.ClusterCondition, clusterNamespace, clusterName string) bool {

	return cc.ClusterInfo.Cluster.Namespace == clusterNamespace &&
		cc.ClusterInfo.Cluster.Name == clusterName
}

func deleteClusterHealthCheck(clusterHealthCheckName string) {
	currentClusterHealthCheck := &libsveltosv1alpha1.ClusterHealthCheck{}
	err := k8sClient.Get(context.TODO(),
		types.NamespacedName{Name: clusterHealthCheckName},
		currentClusterHealthCheck)
	Expect(err).To(BeNil())

	Expect(k8sClient.Delete(context.TODO(), currentClusterHealthCheck)).To(Succeed())

	Byf("Verifying ClusterHealthCheck %s is gone", clusterHealthCheckName)

	Eventually(func() bool {
		err = k8sClient.Get(context.TODO(),
			types.NamespacedName{Name: clusterHealthCheckName},
			currentClusterHealthCheck)
		return apierrors.IsNotFound(err)
	}, timeout, pollingInterval).Should(BeTrue())
}

// deleteClusterProfile deletes ClusterProfile and verifies all ClusterSummaries created by this ClusterProfile
// instances are also gone
func deleteClusterProfile(clusterProfile *configv1alpha1.ClusterProfile) {
	listOptions := []client.ListOption{
		client.MatchingLabels{
			controllers.ClusterProfileLabelName: clusterProfile.Name,
		},
	}
	clusterSummaryList := &configv1alpha1.ClusterSummaryList{}
	Expect(k8sClient.List(context.TODO(), clusterSummaryList, listOptions...)).To(Succeed())

	Byf("Deleting the ClusterProfile %s", clusterProfile.Name)
	currentClusterProfile := &configv1alpha1.ClusterProfile{}
	Expect(k8sClient.Get(context.TODO(), types.NamespacedName{Name: clusterProfile.Name}, currentClusterProfile)).To(BeNil())
	Expect(k8sClient.Delete(context.TODO(), currentClusterProfile)).To(Succeed())

	for i := range clusterSummaryList.Items {
		Byf("Verifying ClusterSummary %s are gone", clusterSummaryList.Items[i].Name)
	}
	Eventually(func() bool {
		for i := range clusterSummaryList.Items {
			clusterSummaryNamespace := clusterSummaryList.Items[i].Namespace
			clusterSummaryName := clusterSummaryList.Items[i].Name
			currentClusterSummary := &configv1alpha1.ClusterSummary{}
			err := k8sClient.Get(context.TODO(),
				types.NamespacedName{Namespace: clusterSummaryNamespace, Name: clusterSummaryName}, currentClusterSummary)
			if err == nil || !apierrors.IsNotFound(err) {
				return false
			}
		}
		return true
	}, timeout, pollingInterval).Should(BeTrue())

	Byf("Verifying ClusterProfile %s is gone", clusterProfile.Name)

	Eventually(func() bool {
		err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: clusterProfile.Name}, currentClusterProfile)
		return apierrors.IsNotFound(err)
	}, timeout, pollingInterval).Should(BeTrue())
}
