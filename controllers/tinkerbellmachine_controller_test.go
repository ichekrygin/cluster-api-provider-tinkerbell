/*
Copyright 2020 The Kubernetes Authors.

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
	"testing"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrastructurev1 "github.com/tinkerbell/cluster-api-provider-tinkerbell/api/v1beta1"
	"github.com/tinkerbell/cluster-api-provider-tinkerbell/controllers"
	tinkv1 "github.com/tinkerbell/cluster-api-provider-tinkerbell/tink/api/v1alpha1"
)

func notImplemented(t *testing.T) {
	t.Helper()

	// t.Fatalf("not implemented")
	t.Skip("not implemented")
}

//nolint:unparam
func validTinkerbellMachine(name, namespace, machineName, hardwareUUID string) *infrastructurev1.TinkerbellMachine { //nolint:lll
	return &infrastructurev1.TinkerbellMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(hardwareUUID),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "cluster.x-k8s.io/v1beta1",
					Kind:       "Machine",
					Name:       machineName,
					UID:        types.UID(hardwareUUID),
				},
			},
		},
	}
}

//nolint:unparam
func validCluster(name, namespace string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				Name: name,
			},
		},
	}
}

//nolint:unparam
func validTinkerbellCluster(name, namespace string) *infrastructurev1.TinkerbellCluster {
	tinkCluster := &infrastructurev1.TinkerbellCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Finalizers: []string{infrastructurev1.ClusterFinalizer},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "cluster.x-k8s.io/v1beta1",
					Kind:       "Cluster",
					Name:       name,
				},
			},
		},
		Spec: infrastructurev1.TinkerbellClusterSpec{
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Host: hardwareIP,
				Port: controllers.KubernetesAPIPort,
			},
		},
		Status: infrastructurev1.TinkerbellClusterStatus{
			Ready: true,
		},
	}

	tinkCluster.Default()

	return tinkCluster
}

//nolint:unparam
func validMachine(name, namespace, clusterName string) *clusterv1.Machine {
	return &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
			},
		},
		Spec: clusterv1.MachineSpec{
			Version: pointer.StringPtr("1.19.4"),
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: pointer.StringPtr(name),
			},
		},
	}
}

//nolint:unparam
func validSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"value": []byte("not nil bootstrap data"),
		},
	}
}

func validHardware(name, uuid, ip string) *tinkv1.Hardware {
	return &tinkv1.Hardware{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: tinkv1.HardwareSpec{
			ID: uuid,
		},
		Status: tinkv1.HardwareStatus{
			Disks: []tinkv1.Disk{
				{
					Device: "/dev/sda",
				},
			},
			Interfaces: []tinkv1.Interface{
				{
					DHCP: &tinkv1.DHCP{
						IP: &tinkv1.IP{
							Address: ip,
						},
					},
				},
			},
		},
	}
}

//nolint:funlen
func Test_Machine_reconciliation_with_available_hardware(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	hardwareUUID := uuid.New().String()

	objects := []runtime.Object{
		validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, hardwareUUID),
		validCluster(clusterName, clusterNamespace),
		validTinkerbellCluster(clusterName, clusterNamespace),
		validHardware(hardwareName, hardwareUUID, hardwareIP),
		validMachine(machineName, clusterNamespace, clusterName),
		validSecret(machineName, clusterNamespace),
	}

	client := kubernetesClientWithObjects(t, objects)

	_, err := reconcileMachineWithClient(client, tinkerbellMachineName, clusterNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Unexpected reconciliation error")

	ctx := context.Background()

	globalResourceName := types.NamespacedName{
		Name: tinkerbellMachineName,
	}

	t.Run("creates_template", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		template := &tinkv1.Template{}

		g.Expect(client.Get(ctx, globalResourceName, template)).To(Succeed(), "Expected template to be created")

		// Owner reference is required to make use of Kubernetes GC for removing dependent objects, so if
		// machine gets force-removed, template will be cleaned up.
		t.Run("with_owner_reference_set", func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(template.ObjectMeta.OwnerReferences).NotTo(BeEmpty(), "Expected at least one owner reference to be set")

			g.Expect(template.ObjectMeta.OwnerReferences[0].UID).To(BeEquivalentTo(types.UID(hardwareUUID)),
				"Expected owner reference UID to match hardwareUUID")
		})
	})

	t.Run("creates_workflow", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		workflow := &tinkv1.Workflow{}

		g.Expect(client.Get(ctx, globalResourceName, workflow)).To(Succeed(), "Expected workflow to be created")

		// Owner reference is required to make use of Kubernetes GC for removing dependent objects, so if
		// machine gets force-removed, workflow will be cleaned up.
		t.Run("with_owner_reference_set", func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(workflow.ObjectMeta.OwnerReferences).NotTo(BeEmpty(), "Expected at least one owner reference to be set")

			g.Expect(workflow.ObjectMeta.OwnerReferences[0].Name).To(BeEquivalentTo(tinkerbellMachineName),
				"Expected owner reference name to match tinkerbellMachine name")
		})
	})

	namespacedName := types.NamespacedName{
		Name:      tinkerbellMachineName,
		Namespace: clusterNamespace,
	}

	updatedMachine := &infrastructurev1.TinkerbellMachine{}
	g.Expect(client.Get(ctx, namespacedName, updatedMachine)).To(Succeed())

	// From https://cluster-api.sigs.k8s.io/developer/providers/machine-infrastructure.html#normal-resource.
	t.Run("sets_provider_id_with_selected_hardware_id", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(updatedMachine.Spec.ProviderID).To(HaveSuffix(hardwareUUID),
			"Expected ProviderID field to include hardwareUUID")
	})

	// From https://cluster-api.sigs.k8s.io/developer/providers/machine-infrastructure.html#normal-resource.
	t.Run("sets_tinkerbell_machine_status_to_ready", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(updatedMachine.Status.Ready).To(BeTrue(), "Machine is not ready")
	})

	// From https://cluster-api.sigs.k8s.io/developer/providers/machine-infrastructure.html#normal-resource.
	t.Run("sets_tinkerbell_finalizer", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(updatedMachine.ObjectMeta.Finalizers).NotTo(BeEmpty(), "Expected at least one finalizer to be set")
	})

	// From https://cluster-api.sigs.k8s.io/developer/providers/machine-infrastructure.html#normal-resource.
	t.Run("sets_tinkerbell_machine_IP_address", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(updatedMachine.Status.Addresses).NotTo(BeEmpty(), "Expected at least one IP address to be populated")

		g.Expect(updatedMachine.Status.Addresses[0].Address).To(BeEquivalentTo(hardwareIP),
			"Expected first IP address to be %q", hardwareIP)
	})

	// So it becomes unavailable for other clusters.
	t.Run("sets_ownership_label_on_selected_hardware", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		hardwareNamespacedName := types.NamespacedName{
			Name: hardwareName,
		}

		updatedHardware := &tinkv1.Hardware{}
		g.Expect(client.Get(ctx, hardwareNamespacedName, updatedHardware)).To(Succeed())

		g.Expect(updatedHardware.ObjectMeta.Labels).To(
			HaveKeyWithValue(controllers.HardwareOwnerNameLabel, tinkerbellMachineName),
			"Expected owner name label to be set on Hardware")

		g.Expect(updatedHardware.ObjectMeta.Labels).To(
			HaveKeyWithValue(controllers.HardwareOwnerNamespaceLabel, clusterNamespace),
			"Expected owner namespace label to be set on Hardware")
	})

	// Ensure idempotency of reconcile operation. E.g. we shouldn't try to create the template with the same name
	// on every iteration.
	t.Run("succeeds_when_executed_twice", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		_, err := reconcileMachineWithClient(client, tinkerbellMachineName, clusterNamespace)
		g.Expect(err).NotTo(HaveOccurred(), "Unexpected reconciliation error")
	})

	// Status should be updated on every run.
	//
	// Don't execute this test in parallel, as we reset status here.
	t.Run("refreshes_status_when_machine_is_already_provisioned", func(t *testing.T) { //nolint:paralleltest
		updatedMachine.Status.Addresses = nil
		g := NewWithT(t)

		g.Expect(client.Update(context.Background(), updatedMachine)).To(Succeed())
		_, err := reconcileMachineWithClient(client, tinkerbellMachineName, clusterNamespace)
		g.Expect(err).NotTo(HaveOccurred())

		updatedMachine = &infrastructurev1.TinkerbellMachine{}
		g.Expect(client.Get(ctx, namespacedName, updatedMachine)).To(Succeed())
		g.Expect(updatedMachine.Status.Addresses).NotTo(BeEmpty(), "Machine status should be updated on every reconciliation")
	})
}

//nolint:funlen
func Test_Machine_reconciliation(t *testing.T) {
	t.Parallel()

	t.Run("is_not_requeued_when", func(t *testing.T) {
		t.Parallel()

		// Requeue will be handled when resource is created.
		t.Run("is_requeued_when_machine_object_is_missing",
			machineReconciliationIsRequeuedWhenTinkerbellMachineObjectIsMissing)

		// From https://cluster-api.sigs.k8s.io/developer/providers/cluster-infrastructure.html#behavior
		// Requeue will be handled when ownerRef is set
		t.Run("machine_has_no_owner_set", machineReconciliationIsRequeuedWhenTinkerbellMachineHasNoOwnerSet)

		// From https://cluster-api.sigs.k8s.io/developer/providers/cluster-infrastructure.html#behavior
		// Requeue will be handled when bootstrap secret is set through the Watch on Machines
		t.Run("bootstrap_secret_is_not_ready", machineReconciliationIsRequeuedWhenBootstrapSecretIsNotReady)

		// From https://cluster-api.sigs.k8s.io/developer/providers/cluster-infrastructure.html#behavior
		// Requeue will be handled when bootstrap secret is set through the Watch on Clusters
		t.Run("cluster_infrastructure_is_not_ready", machineReconciliationIsRequeuedWhenClusterInfrastructureIsNotReady)
	})

	t.Run("fails_when", func(t *testing.T) {
		t.Parallel()

		t.Run("reconciler_is_nil", machineReconciliationFailsWhenReconcilerIsNil)
		t.Run("reconciler_has_no_client_set", machineReconciliationFailsWhenReconcilerHasNoClientSet)

		// CAPI spec says this is optional, but @detiber says it's effectively required, so treat it as so.
		t.Run("machine_has_no_version_set", machineReconciliationFailsWhenMachineHasNoVersionSet)

		t.Run("associated_cluster_object_does_not_exist",
			machineReconciliationFailsWhenAssociatedClusterObjectDoesNotExist)

		t.Run("associated_tinkerbell_cluster_object_does_not_exist",
			machineReconciliationFailsWhenAssociatedTinkerbellClusterObjectDoesNotExist)

		// If for example CAPI changes key used to store bootstrap date, we shouldn't try to create machines
		// with empty bootstrap config, we should fail early instead.
		t.Run("bootstrap_config_is_empty", machineReconciliationFailsWhenBootstrapConfigIsEmpty)
		t.Run("bootstrap_config_has_no_value_key", machineReconciliationFailsWhenBootstrapConfigHasNoValueKey)

		t.Run("there_is_no_hardware_available", machineReconciliationFailsWhenThereIsNoHardwareAvailable)

		t.Run("selected_hardware_has_no_ip_address_set", machineReconciliationFailsWhenSelectedHardwareHasNoIPAddressSet)
	})

	// Single hardware should only ever be used for a single machine.
	t.Run("selects_unique_and_available_hardware_for_each_machine", //nolint:paralleltest
		machineReconciliationSelectsUniqueAndAvailablehardwareForEachMachine)

	// Patching Hardware and TinkerbellMachine are not atomic operations, so we should handle situation, when
	// misspelling process is aborted in the middle.
	//
	// Without that, new Hardware will be selected each time.
	t.Run("uses_already_selected_hardware_if_patching_tinkerbell_machine_failed", //nolint:paralleltest
		machineReconciliationUsesAlreadySelectedHardwareIfPatchingTinkerbellMachineFailed)

	t.Run("when_machine_is_scheduled_for_removal_it", func(t *testing.T) {
		t.Parallel()

		// From https://cluster-api.sigs.k8s.io/developer/providers/machine-infrastructure.html#behavior
		t.Run("removes_tinkerbell_finalizer", notImplemented)

		// Removing machine should release used hardware.
		t.Run("marks_hardware_as_available_for_other_machines", notImplemented)
	})
}

func Test_Machine_reconciliation_when_machine_is_scheduled_for_removal_it(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	objects := []runtime.Object{
		validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, ""),
		validCluster(clusterName, clusterNamespace),
		validTinkerbellCluster(clusterName, clusterNamespace),
		validHardware(hardwareName, uuid.New().String(), hardwareIP),
		validMachine(machineName, clusterNamespace, clusterName),
		validSecret(machineName, clusterNamespace),
	}

	client := kubernetesClientWithObjects(t, objects)

	_, err := reconcileMachineWithClient(client, tinkerbellMachineName, clusterNamespace)
	g.Expect(err).NotTo(HaveOccurred())

	ctx := context.Background()

	tinkerbellMachineNamespacedName := types.NamespacedName{
		Name:      tinkerbellMachineName,
		Namespace: clusterNamespace,
	}

	updatedMachine := &infrastructurev1.TinkerbellMachine{}
	g.Expect(client.Get(ctx, tinkerbellMachineNamespacedName, updatedMachine)).To(Succeed())

	now := metav1.Now()
	updatedMachine.ObjectMeta.DeletionTimestamp = &now

	g.Expect(client.Update(ctx, updatedMachine)).To(Succeed())
	_, err = reconcileMachineWithClient(client, tinkerbellMachineName, clusterNamespace)
	g.Expect(err).NotTo(HaveOccurred())

	hardwareNamespacedName := types.NamespacedName{
		Name: hardwareName,
	}

	updatedHardware := &tinkv1.Hardware{}
	g.Expect(client.Get(ctx, hardwareNamespacedName, updatedHardware)).To(Succeed())

	t.Run("removes_tinkerbell_machine_finalizer_from_hardware", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(updatedHardware.ObjectMeta.GetFinalizers()).To(BeEmpty())
	})

	t.Run("makes_hardware_available_for_other_machines", func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		g.Expect(updatedHardware.ObjectMeta.Labels).NotTo(HaveKey(controllers.HardwareOwnerNameLabel),
			"Found hardware owner name label")
		g.Expect(updatedHardware.ObjectMeta.Labels).NotTo(HaveKey(controllers.HardwareOwnerNamespaceLabel),
			"Found hardware owner namespace label")
	})
}

const (
	machineName           = "myMachineName"
	tinkerbellMachineName = "myTinkerbellMachineName"
)

func machineReconciliationFailsWhenReconcilerIsNil(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	var machineController *controllers.TinkerbellMachineReconciler

	request := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: clusterNamespace,
			Name:      tinkerbellMachineName,
		},
	}

	_, err := machineController.Reconcile(context.TODO(), request)
	g.Expect(err).To(MatchError(controllers.ErrConfigurationNil))
}

func machineReconciliationFailsWhenReconcilerHasNoClientSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	machineController := &controllers.TinkerbellMachineReconciler{}

	request := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: clusterNamespace,
			Name:      tinkerbellMachineName,
		},
	}

	_, err := machineController.Reconcile(context.TODO(), request)
	g.Expect(err).To(MatchError(controllers.ErrMissingClient))
}

//nolint:unparam
func reconcileMachineWithClient(client client.Client, name, namespace string) (ctrl.Result, error) {
	machineController := &controllers.TinkerbellMachineReconciler{
		Client: client,
	}

	request := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}

	return machineController.Reconcile(context.TODO(), request) //nolint:wrapcheck
}

func machineReconciliationIsRequeuedWhenTinkerbellMachineObjectIsMissing(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	result, err := reconcileMachineWithClient(kubernetesClientWithObjects(t, nil), tinkerbellMachineName, clusterNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when machine object does not exist should not return error")
	g.Expect(result.IsZero()).To(BeTrue(), "Expected no requeue to be requested")
}

func machineReconciliationIsRequeuedWhenTinkerbellMachineHasNoOwnerSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)
	hardwareUUID := uuid.New().String()
	tinkerbellMachine := validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, hardwareUUID)
	tinkerbellMachine.ObjectMeta.OwnerReferences = nil

	objects := []runtime.Object{
		tinkerbellMachine,
		validCluster(clusterName, clusterNamespace),
		validTinkerbellCluster(clusterName, clusterNamespace),
		validHardware(hardwareName, hardwareUUID, hardwareIP),
		validMachine(machineName, clusterNamespace, clusterName),
		validSecret(machineName, clusterNamespace),
	}

	client := kubernetesClientWithObjects(t, objects)

	result, err := reconcileMachineWithClient(client, tinkerbellMachineName, clusterNamespace)
	g.Expect(err).NotTo(HaveOccurred(), "Reconciling when machine object does not exist should not return error")
	g.Expect(result.IsZero()).To(BeTrue(), "Expected no requeue to be requested")
}

func machineReconciliationIsRequeuedWhenBootstrapSecretIsNotReady(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	hardwareUUID := uuid.New().String()
	machineWithoutSecretReference := validMachine(machineName, clusterNamespace, clusterName)
	machineWithoutSecretReference.Spec.Bootstrap.DataSecretName = nil

	objects := []runtime.Object{
		validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, hardwareUUID),
		validCluster(clusterName, clusterNamespace),
		validTinkerbellCluster(clusterName, clusterNamespace),
		validHardware(hardwareName, hardwareUUID, hardwareIP),
		machineWithoutSecretReference,
		validSecret(machineName, clusterNamespace),
	}

	client := kubernetesClientWithObjects(t, objects)

	result, err := reconcileMachineWithClient(client, tinkerbellMachineName, clusterNamespace)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.IsZero()).To(BeTrue(), "Expected no requeue to be requested")
}

func machineReconciliationIsRequeuedWhenClusterInfrastructureIsNotReady(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	hardwareUUID := uuid.New().String()
	notReadyTinkerbellCluster := validTinkerbellCluster(clusterName, clusterNamespace)
	notReadyTinkerbellCluster.Status.Ready = false

	objects := []runtime.Object{
		validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, hardwareUUID),
		validCluster(clusterName, clusterNamespace),
		notReadyTinkerbellCluster,
		validHardware(hardwareName, hardwareUUID, hardwareIP),
		validMachine(machineName, clusterNamespace, clusterName),
		validSecret(machineName, clusterNamespace),
	}

	client := kubernetesClientWithObjects(t, objects)

	result, err := reconcileMachineWithClient(client, tinkerbellMachineName, clusterNamespace)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.IsZero()).To(BeTrue(), "Expected no requeue to be requested")
}

func machineReconciliationFailsWhenMachineHasNoVersionSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	hardwareUUID := uuid.New().String()
	machineWithoutVersion := validMachine(machineName, clusterNamespace, clusterName)
	machineWithoutVersion.Spec.Version = nil

	objects := []runtime.Object{
		validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, hardwareUUID),
		validCluster(clusterName, clusterNamespace),
		validTinkerbellCluster(clusterName, clusterNamespace),
		validHardware(hardwareName, hardwareUUID, hardwareIP),
		machineWithoutVersion,
		validSecret(machineName, clusterNamespace),
	}

	client := kubernetesClientWithObjects(t, objects)

	_, err := reconcileMachineWithClient(client, tinkerbellMachineName, clusterNamespace)
	g.Expect(err).To(MatchError(controllers.ErrMachineVersionEmpty))
}

func machineReconciliationFailsWhenBootstrapConfigIsEmpty(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	hardwareUUID := uuid.New().String()
	emptySecret := validSecret(machineName, clusterNamespace)
	emptySecret.Data["value"] = nil

	objects := []runtime.Object{
		validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, hardwareUUID),
		validCluster(clusterName, clusterNamespace),
		validTinkerbellCluster(clusterName, clusterNamespace),
		validHardware(hardwareName, hardwareUUID, hardwareIP),
		validMachine(machineName, clusterNamespace, clusterName),
		emptySecret,
	}

	_, err := reconcileMachineWithClient(kubernetesClientWithObjects(t, objects), tinkerbellMachineName, clusterNamespace)
	g.Expect(err).To(MatchError(controllers.ErrBootstrapUserDataEmpty))
}

func machineReconciliationFailsWhenBootstrapConfigHasNoValueKey(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	hardwareUUID := uuid.New().String()
	emptySecret := validSecret(machineName, clusterNamespace)
	emptySecret.Data = nil

	objects := []runtime.Object{
		validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, hardwareUUID),
		validCluster(clusterName, clusterNamespace),
		validTinkerbellCluster(clusterName, clusterNamespace),
		validHardware(hardwareName, hardwareUUID, hardwareIP),
		validMachine(machineName, clusterNamespace, clusterName),
		emptySecret,
	}

	_, err := reconcileMachineWithClient(kubernetesClientWithObjects(t, objects), tinkerbellMachineName, clusterNamespace)
	g.Expect(err).To(MatchError(controllers.ErrMissingBootstrapDataSecretValueKey))
}

func machineReconciliationFailsWhenAssociatedClusterObjectDoesNotExist(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	hardwareUUID := uuid.New().String()
	objects := []runtime.Object{
		validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, hardwareUUID),
		// validCluster(clusterName, clusterNamespace),
		validTinkerbellCluster(clusterName, clusterNamespace),
		validHardware(hardwareName, hardwareUUID, hardwareIP),
		validMachine(machineName, clusterNamespace, clusterName),
		validSecret(machineName, clusterNamespace),
	}

	_, err := reconcileMachineWithClient(kubernetesClientWithObjects(t, objects), tinkerbellMachineName, clusterNamespace)
	g.Expect(err).To(SatisfyAll(
		MatchError(ContainSubstring("not found")),
		MatchError(ContainSubstring("getting cluster from metadata")),
	))
}

func machineReconciliationFailsWhenAssociatedTinkerbellClusterObjectDoesNotExist(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	hardwareUUID := uuid.New().String()
	objects := []runtime.Object{
		validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, hardwareUUID),
		validCluster(clusterName, clusterNamespace),
		// validTinkerbellCluster(clusterName, clusterNamespace),
		validHardware(hardwareName, hardwareUUID, hardwareIP),
		validMachine(machineName, clusterNamespace, clusterName),
		validSecret(machineName, clusterNamespace),
	}

	_, err := reconcileMachineWithClient(kubernetesClientWithObjects(t, objects), tinkerbellMachineName, clusterNamespace)
	g.Expect(err).To(MatchError(ContainSubstring("getting TinkerbellCluster object")))
}

func machineReconciliationFailsWhenThereIsNoHardwareAvailable(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	hardwareUUID := uuid.New().String()
	objects := []runtime.Object{
		validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, hardwareUUID),
		validCluster(clusterName, clusterNamespace),
		validTinkerbellCluster(clusterName, clusterNamespace),
		// validHardware(hardwareName, hardwareUUID, hardwareIP),
		validMachine(machineName, clusterNamespace, clusterName),
		validSecret(machineName, clusterNamespace),
	}

	_, err := reconcileMachineWithClient(kubernetesClientWithObjects(t, objects), tinkerbellMachineName, clusterNamespace)
	g.Expect(err).To(MatchError(controllers.ErrNoHardwareAvailable))
}

func machineReconciliationFailsWhenSelectedHardwareHasNoIPAddressSet(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	hardwareUUID := uuid.New().String()
	malformedHardware := validHardware(hardwareName, hardwareUUID, "")
	objects := []runtime.Object{
		validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, hardwareUUID),
		validCluster(clusterName, clusterNamespace),
		validTinkerbellCluster(clusterName, clusterNamespace),
		malformedHardware,
		validMachine(machineName, clusterNamespace, clusterName),
		validSecret(machineName, clusterNamespace),
	}

	_, err := reconcileMachineWithClient(kubernetesClientWithObjects(t, objects), tinkerbellMachineName, clusterNamespace)
	g.Expect(err).To(MatchError(controllers.ErrHardwareFirstInterfaceDHCPMissingIP))
}

func machineReconciliationSelectsUniqueAndAvailablehardwareForEachMachine(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	secondMachineName := "secondMachineName"
	secondHardwareName := "secondHardwareName"

	firstHardwareUUID := uuid.New().String()
	secondHardwareUUID := uuid.New().String()

	secondTinkerbellMachineName := "secondTinkerbellMachineName"

	objects := []runtime.Object{
		validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, firstHardwareUUID),
		validTinkerbellMachine(secondTinkerbellMachineName, clusterNamespace, secondMachineName, secondHardwareUUID),

		validCluster(clusterName, clusterNamespace),
		validTinkerbellCluster(clusterName, clusterNamespace),

		validHardware(hardwareName, firstHardwareUUID, hardwareIP),
		validHardware(secondHardwareName, secondHardwareUUID, "2.2.2.2"),

		validMachine(machineName, clusterNamespace, clusterName),
		validMachine(secondMachineName, clusterNamespace, clusterName),

		validSecret(machineName, clusterNamespace),
		validSecret(secondMachineName, clusterNamespace),
	}

	client := kubernetesClientWithObjects(t, objects)

	_, err := reconcileMachineWithClient(client, tinkerbellMachineName, clusterNamespace)
	g.Expect(err).NotTo(HaveOccurred())

	tinkerbellMachineNamespacedName := types.NamespacedName{
		Name:      tinkerbellMachineName,
		Namespace: clusterNamespace,
	}

	ctx := context.Background()

	firstMachine := &infrastructurev1.TinkerbellMachine{}
	g.Expect(client.Get(ctx, tinkerbellMachineNamespacedName, firstMachine)).To(Succeed(), "Getting first updated machine")

	_, err = reconcileMachineWithClient(client, secondTinkerbellMachineName, clusterNamespace)
	g.Expect(err).NotTo(HaveOccurred())

	tinkerbellMachineNamespacedName.Name = secondTinkerbellMachineName

	secondMachine := &infrastructurev1.TinkerbellMachine{}
	g.Expect(client.Get(ctx, tinkerbellMachineNamespacedName, secondMachine)).To(Succeed())

	g.Expect(firstMachine.Spec.HardwareName).NotTo(BeEquivalentTo(secondMachine.Spec.HardwareName),
		"Two machines use the same hardware")
}

func machineReconciliationUsesAlreadySelectedHardwareIfPatchingTinkerbellMachineFailed(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	expectedHardwareName := "alreadyOwnedHardware"
	alreadyOwnedHardware := validHardware(expectedHardwareName, uuid.New().String(), "2.2.2.2")
	alreadyOwnedHardware.ObjectMeta.Labels = map[string]string{
		controllers.HardwareOwnerNameLabel:      tinkerbellMachineName,
		controllers.HardwareOwnerNamespaceLabel: clusterNamespace,
	}

	hardwareUUID := uuid.New().String()

	objects := []runtime.Object{
		validTinkerbellMachine(tinkerbellMachineName, clusterNamespace, machineName, hardwareUUID),
		validCluster(clusterName, clusterNamespace),
		validTinkerbellCluster(clusterName, clusterNamespace),
		validHardware(hardwareName, hardwareUUID, hardwareIP),
		alreadyOwnedHardware,
		validMachine(machineName, clusterNamespace, clusterName),
		validSecret(machineName, clusterNamespace),
	}

	client := kubernetesClientWithObjects(t, objects)

	_, err := reconcileMachineWithClient(client, tinkerbellMachineName, clusterNamespace)
	g.Expect(err).NotTo(HaveOccurred())

	ctx := context.Background()

	tinkerbellMachineNamespacedName := types.NamespacedName{
		Name:      tinkerbellMachineName,
		Namespace: clusterNamespace,
	}

	updatedMachine := &infrastructurev1.TinkerbellMachine{}
	g.Expect(client.Get(ctx, tinkerbellMachineNamespacedName, updatedMachine)).To(Succeed())

	g.Expect(updatedMachine.Spec.HardwareName).To(BeEquivalentTo(expectedHardwareName),
		"Wrong hardware selected. Expected %q", expectedHardwareName)
}
