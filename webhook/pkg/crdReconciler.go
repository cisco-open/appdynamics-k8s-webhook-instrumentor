package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	clientk8s "sigs.k8s.io/controller-runtime/pkg/client"
	configK8s "sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sourceK8s "sigs.k8s.io/controller-runtime/pkg/source"
)

func startCrdReconciler() {
	entryLog := log.Log.WithName("instr-controller")

	// Setup a Manager
	entryLog.Info("setting up manager")
	mgr, err := manager.New(configK8s.GetConfigOrDie(), manager.Options{
		Scheme: scheme,
	})

	if err != nil {
		entryLog.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	// Setup a new controller to reconcile Instrumentation and ClusterInstrumentation
	entryLog.Info("Setting up instr-controller")
	instrController, err := controller.New("instr-controller", mgr, controller.Options{
		Reconciler: &reconcileInstrCrd{client: mgr.GetClient()},
	})
	if err != nil {
		entryLog.Error(err, "unable to set up instr-controller")
		os.Exit(1)
	}

	entryLog.Info("Setting up ginstr-controller")
	gInstrController, err := controller.New("ginstr-controller", mgr, controller.Options{
		Reconciler: &reconcileGInstrCrd{client: mgr.GetClient()},
	})
	if err != nil {
		entryLog.Error(err, "unable to set up ginstr-controller")
		os.Exit(1)
	}

	entryLog.Info("Setting up otelcol-controller")

	reconciler := &reconcileOtelColCrd{client: mgr.GetClient()}
	err = reconciler.SetupWithManager(mgr)
	// otelcolController, err := controller.New("otelcol-controller", mgr, controller.Options{
	// 	Reconciler: reconciler,
	// })
	if err != nil {
		entryLog.Error(err, "unable to set up otelcol-controller")
		os.Exit(1)
	}

	entryLog.Info("Registering CRDs to controller")

	// Watch Instrumentation and ClusterInstrumentation
	if err := instrController.Watch(sourceK8s.Kind(mgr.GetCache(), &v1alpha1.Instrumentation{}), &handler.EnqueueRequestForObject{}); err != nil {
		entryLog.Error(err, "unable to watch Instrumentation")
		os.Exit(1)
	}
	if err := gInstrController.Watch(sourceK8s.Kind(mgr.GetCache(), &v1alpha1.ClusterInstrumentation{}), &handler.EnqueueRequestForObject{}); err != nil {
		entryLog.Error(err, "unable to watch ClusterInstrumentation")
		os.Exit(1)
	}
	// Watch OpenTelemetry collector
	// if err := otelcolController.Watch(sourceK8s.Kind(mgr.GetCache(), &v1alpha1.OpenTelemetryCollector{}), &handler.EnqueueRequestForObject{}); err != nil {
	// 	entryLog.Error(err, "unable to watch OpenTelemetryCollector")
	// 	os.Exit(1)
	// }
	entryLog.Info("starting manager")
	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		entryLog.Error(err, "unable to run manager")
		os.Exit(1)
	}
}

type reconcileInstrCrd struct {
	client clientk8s.Client
}

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &reconcileInstrCrd{}

func (r *reconcileInstrCrd) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// set up a convenient log object so we don't have to type request over and over again
	log := log.FromContext(ctx)

	// Fetch the Instrumentation from the cache
	instr := &v1alpha1.Instrumentation{}
	err := r.client.Get(ctx, request.NamespacedName, instr)
	if errors.IsNotFound(err) {
		log.Info("Deleting Instrumentation", "namespace", request.Namespace, "name", request.Name)
		deleteCrdInstrumentation(request.Namespace, request.Name)
		return reconcile.Result{}, nil
	}

	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not fetch Instrumentation: %+v", err)
	}

	if !instr.DeletionTimestamp.IsZero() {
		log.Info("Deleting Instrumentation", "data", *instr)
		deleteCrdInstrumentation(request.Namespace, instr.Name)
	} else {
		log.Info("Upserting Instrumentation", "data", *instr)
		injectionRuleDefaults(instr.Spec.InjectionRules)
		upsertCrdInstrumentation(request.Namespace, instr.Name, instr.Spec)
		// Set the label if it is missing
		if instr.Annotations == nil {
			instr.Annotations = map[string]string{}
		}
		instr.Annotations["processingStatus"] = "processed"

		err = r.client.Update(ctx, instr)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("could not write Instrumentation: %+v", err)
		}
	}

	return reconcile.Result{}, nil
}

type reconcileGInstrCrd struct {
	client clientk8s.Client
}

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &reconcileGInstrCrd{}

func (r *reconcileGInstrCrd) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// set up a convenient log object so we don't have to type request over and over again
	log := log.FromContext(ctx)

	// Fetch the Instrumentation from the cache
	instr := &v1alpha1.ClusterInstrumentation{}
	err := r.client.Get(ctx, request.NamespacedName, instr)
	if errors.IsNotFound(err) {
		log.Info("Deleting ClusterInstrumentation", "name", request.Name)
		deleteCrdClusterInstrumentation(request.Name)
		return reconcile.Result{}, nil
	}

	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not fetch ClusterInstrumentation: %+v", err)
	}

	if !instr.DeletionTimestamp.IsZero() {
		log.Info("Deleting ClusterInstrumentation", "data", *instr)
		deleteCrdClusterInstrumentation(instr.Name)
	} else {
		log.Info("Upserting ClusterInstrumentation", "data", *instr)
		injectionRuleDefaults(instr.Spec.InjectionRules)
		upsertCrdClusterInstrumentation(instr.Name, instr.Spec)
		// Set the label if it is missing
		if instr.Annotations == nil {
			instr.Annotations = map[string]string{}
		}
		instr.Annotations["processingStatus"] = "processed"

		err = r.client.Update(ctx, instr)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("could not write ClusterInstrumentation: %+v", err)
		}
	}

	return reconcile.Result{}, nil
}

type reconcileOtelColCrd struct {
	client clientk8s.Client
	Scheme *runtime.Scheme
}

// Implement reconcile.Reconciler so the controller can reconcile objects
var _ reconcile.Reconciler = &reconcileOtelColCrd{}

func (r *reconcileOtelColCrd) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.OpenTelemetryCollector{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		WithEventFilter(predicate.Or(
			predicate.GenerationChangedPredicate{},
			/*, predicate.LabelChangedPredicate{} */)).
		Complete(r)
}

func (r *reconcileOtelColCrd) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// set up a convenient log object so we don't have to type request over and over again
	log := log.FromContext(ctx)

	// Fetch the Instrumentation from the cache
	otelcol := &v1alpha1.OpenTelemetryCollector{}
	err := r.client.Get(ctx, request.NamespacedName, otelcol)
	if errors.IsNotFound(err) {
		log.Info("Deleting OpenTelemetryCollector", "name", request.Name)
		r.deleteOtelCollector(ctx, request.Namespace, request.Name)
		return reconcile.Result{}, nil
	}

	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not fetch OpenTelemetryCollector: %+v", err)
	}

	if !otelcol.DeletionTimestamp.IsZero() {
		log.Info("Deleting OpenTelemetryCollector", "data", *otelcol)
		r.deleteOtelCollector(ctx, request.Namespace, request.Name)
	} else {
		log.Info("Upserting OpenTelemetryCollector", "data", *otelcol)
		r.upsertOtelCollector(ctx, request.Namespace, request.Name, otelcol)
	}
	return reconcile.Result{}, nil
}

func (r *reconcileOtelColCrd) deleteOtelCollector(ctx context.Context, namespace string, name string) error {

	unregisterNamespacedCollector(namespace, name)
	return nil
}

const (
	OTELCOL_APP_NAME              = "otelcol"
	OTELCOL_RESOURCE_PREFIX       = "otel-collector-"
	OTELCOL_CONFIG_INJECTOR       = "config-injector"
	OTELCOL_CONFIG_INJECTOR_IMAGE = "alpine:latest"
)

func (r *reconcileOtelColCrd) upsertOtelCollector(ctx context.Context, namespace string, name string, otelcol *v1alpha1.OpenTelemetryCollector) error {
	log := log.FromContext(ctx)

	name = OTELCOL_RESOURCE_PREFIX + name

	// if sidecar definition, which is only config as such, register it for later use
	// when instrumented pods are instatiated
	if otelcol.Spec.Mode == v1alpha1.ModeSidecar {
		registerNamespacedSidecarCollector(namespace, otelcol)
		return nil
	}

	// if not sidecar definition, proceed to instantiate collector pods
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "Error getting cluster config")
		return err
	}

	clientset, err := clientappsv1.NewForConfig(config)
	if err != nil {
		log.Error(err, "Error getting clientset")
		return err
	}

	deploymentClient := clientset.Deployments(namespace)

	create := false
	presentDeployment, err := deploymentClient.Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		create = true
		log.Info("OpenTelemetryCollector Deployment not present - creating", "namespace", namespace, "name", name)
	} else if err == nil {
		log.Info("OpenTelemetryCollector Deployment present - updating", "namespace", namespace, "name", name)
	} else {
		log.Error(err, "Error getting OpenTelemetryCollector Deployment", "namespace", namespace, "name", name)
		return err
	}

	// build the Deployment of OpenTelemetry Collector

	otelcolSelector := map[string]string{
		"ext.appd.com/instance": "appd-collector",
		"ext.appd.com/name":     name,
	}

	// ownerReference := metav1.OwnerReference{
	// 	APIVersion: otelcol.APIVersion,
	// 	Kind:       otelcol.Kind,
	// 	Name:       otelcol.Name,
	// 	UID:        otelcol.UID,
	// }

	limCPU, _ := resource.ParseQuantity("200m")
	limMem, _ := resource.ParseQuantity("75M")
	reqCPU, _ := resource.ParseQuantity("10m")
	reqMem, _ := resource.ParseQuantity("50M")

	initScript := "echo \"" + otelcol.Spec.Config + "\" > /conf/otel-collector-config.yaml"

	if otelcol.Spec.Image == "" {
		otelcol.Spec.Image = "otel/opentelemetry-collector-contrib:latest"
	}
	if otelcol.Spec.Replicas == nil {
		otelcol.Spec.Replicas = int32Ptr(1)
	}

	otelcolDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			// OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(*otelcol.Spec.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: otelcolSelector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: otelcolSelector,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: otelcol.Spec.ServiceAccount,
					Containers: []corev1.Container{
						{
							Name:            OTELCOL_APP_NAME,
							Image:           otelcol.Spec.Image,
							ImagePullPolicy: otelcol.Spec.ImagePullPolicy,
							Ports: []corev1.ContainerPort{
								{
									Name:          "grpc",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: 4317,
								},
								{
									Name:          "http",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: 4318,
								},
							},
							Resources: otelcol.Spec.Resources,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "otelcol-config",
									MountPath: "/conf",
								},
							},
							Args: []string{"--config", "/conf/otel-collector-config.yaml"},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:  OTELCOL_CONFIG_INJECTOR,
							Image: OTELCOL_CONFIG_INJECTOR_IMAGE,
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    limCPU,
									corev1.ResourceMemory: limMem,
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    reqCPU,
									corev1.ResourceMemory: reqMem,
								},
							},
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{initScript},
							VolumeMounts: append(otelcol.Spec.VolumeMounts, corev1.VolumeMount{
								Name:      "otelcol-config",
								MountPath: "/conf",
							},
							),
							Env:     otelcol.Spec.Env,
							EnvFrom: otelcol.Spec.EnvFrom,
						},
					},
					Volumes: append(otelcol.Spec.Volumes,
						corev1.Volume{
							Name: "otelcol-config",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					),
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(otelcol, otelcolDeployment, scheme); err != nil {
		log.Error(err, "Error setting up controller reference for deployment", "namespace", namespace, "name", name)
		return err
	}

	if create {
		_, err = deploymentClient.Create(ctx, otelcolDeployment, metav1.CreateOptions{})
		if err != nil {
			log.Error(err, "Error creating OpenTelemetryCollector Deployment", "namespace", namespace, "name", name)
		}
	} else {
		desiredDeployment, err := deploymentClient.Update(ctx, otelcolDeployment, metav1.UpdateOptions{
			DryRun: []string{"All"},
		})
		if err != nil {
			log.Error(err, "Error dry run updating OpenTelemetryCollector Deployment", "namespace", namespace, "name", name)
		}
		if reflect.DeepEqual(presentDeployment.Spec, desiredDeployment.Spec) {
			log.Info("Deployment is not changed, skip update", "namespace", namespace, "name", name)
		} else {
			_, err := deploymentClient.Update(ctx, otelcolDeployment, metav1.UpdateOptions{})
			if err != nil {
				log.Error(err, "Error updating OpenTelemetryCollector Deployment", "namespace", namespace, "name", name)
			}
		}
	}

	// Create Service for the OpenTelemetry Collector created above
	coreclientset, err := clientcorev1.NewForConfig(config)
	if err != nil {
		log.Error(err, "Error getting clientset")
		return err
	}

	serviceClient := coreclientset.Services(namespace)

	create = false
	presentService, err := serviceClient.Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		create = true
		log.Info("OpenTelemetryCollector Service not present - creating", "namespace", namespace, "name", name)
	} else if err == nil {
		log.Info("OpenTelemetryCollector Service present - updating", "namespace", namespace, "name", name)
	} else {
		log.Error(err, "Error getting OpenTelemetryCollector Service", "namespace", namespace, "name", name)
		return err
	}

	otelcolService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			// OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
		Spec: corev1.ServiceSpec{
			Selector: otelcolSelector,
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc",
					Protocol:   corev1.ProtocolTCP,
					Port:       4317,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 4317},
				},
				{
					Name:       "http",
					Protocol:   corev1.ProtocolTCP,
					Port:       4318,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 4318},
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	if err := controllerutil.SetControllerReference(otelcol, otelcolService, scheme); err != nil {
		log.Error(err, "Error setting up controller reference for service", "namespace", namespace, "name", name)
		return err
	}

	if create {
		_, err = serviceClient.Create(ctx, otelcolService, metav1.CreateOptions{})
		if err != nil {
			log.Error(err, "Error creating OpenTelemetryCollector Service", "namespace", namespace, "name", name)
		}
	} else {
		desiredService, err := serviceClient.Update(ctx, otelcolService, metav1.UpdateOptions{
			DryRun: []string{"All"},
		})
		if err != nil {
			log.Error(err, "Error dry run updating OpenTelemetryCollector Service", "namespace", namespace, "name", name)
		}
		if reflect.DeepEqual(presentService.Spec, desiredService.Spec) {
			log.Info("Service is not changed, skip update", "namespace", namespace, "name", name)
		} else {
			_, err := serviceClient.Update(ctx, otelcolService, metav1.UpdateOptions{})
			if err != nil {
				log.Error(err, "Error updating OpenTelemetryCollector Service", "namespace", namespace, "name", name)
			}
		}
	}

	registerNamespacedStandaloneCollector(namespace, otelcol)

	return nil
}

func int32Ptr(i int32) *int32 {
	var v int32
	v = i
	return &v
}
