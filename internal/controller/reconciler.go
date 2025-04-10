package controller

import (
	"context"
	"fmt"

	apiv1 "github.com/alirezaarzehgar/writy-operator/api/v1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	defaultStorageClaimName        = "data"
	defaultStorageClassName        = "standard"
	writyImage                     = "alirezaarzehgar/writy"
	writyBinaryPath                = "/bin/writy"
	defaultWrityImageVersion       = "v1.0.0"
	defaultWrityPort         int32 = 8000
	defaultLoadbalancerPort  int32 = 3000
	defaultLogLevel                = "warn"
)

func getOwnerReferences(wc *apiv1.WrityCluster, c client.Client) ([]metav1.OwnerReference, error) {

	gvk, err := apiutil.GVKForObject(wc, c.Scheme())
	if err != nil {
		return nil, err
	}

	ref := metav1.OwnerReference{
		APIVersion:         gvk.Version,
		Kind:               gvk.Kind,
		Name:               wc.GetName(),
		UID:                wc.GetUID(),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}
	owners := []metav1.OwnerReference{ref}

	return owners, nil
}

func reconcielWrityCluster(ctx context.Context, logger logr.Logger, writyCluster *apiv1.WrityCluster, c client.Client) error {
	if err := createOrPatchDbSertvice(ctx, logger, writyCluster, c); err != nil {
		return err
	}

	if err := createOrPatchDbStatefulSet(ctx, logger, writyCluster, c); err != nil {
		return err
	}

	if err := createOrPatchLoadbalancerService(ctx, logger, writyCluster, c); err != nil {
		return err
	}

	if err := createOrPatchLoadbalancer(ctx, logger, writyCluster, c); err != nil {
		return err
	}

	return nil
}

func createOrPatchDbStatefulSet(ctx context.Context, logger logr.Logger, wc *apiv1.WrityCluster, c client.Client) error {
	owners, err := getOwnerReferences(wc, c)
	if err != nil {
		return err
	}

	lables := map[string]string{
		"apps":       wc.Name,
		"controller": wc.Name,
	}

	ws := wc.Spec.WritySpec
	if ws == nil {
		ws = &apiv1.WritySpec{}
	}
	if ws.Version == "" {
		ws.Version = defaultWrityImageVersion
	}
	if ws.Port == nil {
		ws.Port = &defaultWrityPort
	}
	if ws.LogLevel == "" {
		ws.LogLevel = defaultLogLevel
	}

	ss := wc.Spec.StorageSpec
	if ss == nil {
		ss = &apiv1.StorageSpec{}
	}
	if ss.ClaimName == "" {
		ss.ClaimName = "data"
	}
	if ss.Class == "" {
		ss.Class = defaultStorageClassName
	}
	if ss.VolumeSizeRequest.Cmp(resource.MustParse("1Mi")) < 0 {
		return fmt.Errorf("system needs more than 1MB storages")
	}
	if ss.VolumeSizeLimit.IsZero() {
		logger.Info("limit is zero: set volume limit to volume request")
		ss.VolumeSizeLimit = ss.VolumeSizeRequest
	}

	specs := v1.PodSpec{
		Containers: []v1.Container{{
			Name:            wc.Name,
			Image:           fmt.Sprintf("%s:%s", writyImage, ws.Version),
			ImagePullPolicy: v1.PullIfNotPresent,
			Command:         []string{writyBinaryPath},
			Env: []v1.EnvVar{
				{
					Name:  "RUNNING_ADDR",
					Value: fmt.Sprintf(":%d", *ws.Port),
				},
				{
					Name:  "LOG_LEVEL",
					Value: ws.LogLevel,
				},
			},
			Args: []string{"--addr=$(RUNNING_ADDR)", "--db=/data", "--leveler=$(LOG_LEVEL)"},
			VolumeMounts: []v1.VolumeMount{{
				Name:      ss.ClaimName,
				MountPath: "/data",
			}},
		}},
	}

	logger.Info("volume claim template", "claim name", ss.ClaimName, "storage class name", ss.Class, "VolumeSizeRequest", ss.VolumeSizeRequest)
	var vct []v1.PersistentVolumeClaim
	if wc.Spec.StorageSpec != nil {
		vct = []v1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{
				Name:            ss.ClaimName,
				OwnerReferences: owners,
			},
			Spec: v1.PersistentVolumeClaimSpec{
				AccessModes: []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce},
				Resources: v1.VolumeResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceStorage: ss.VolumeSizeRequest,
					},
					Limits: v1.ResourceList{
						v1.ResourceStorage: ss.VolumeSizeLimit,
					},
				},
				StorageClassName: &ss.Class,
			},
		}}
	}

	serviceName := fmt.Sprintf("%s-service", wc.Name)

	stfs := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wc.Name,
			Namespace: wc.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    wc.Spec.Size,
			ServiceName: serviceName,
			Selector: &metav1.LabelSelector{
				MatchLabels: lables,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: lables,
				},
				Spec: specs,
			},
			VolumeClaimTemplates: vct,
		},
	}

	logger.Info("create/patch statefulset", "StatefulSet", stfs)
	_, err = controllerutil.CreateOrPatch(ctx, c, stfs, func() error { return nil })
	return err
}

func createOrPatchDbSertvice(ctx context.Context, logger logr.Logger, wc *apiv1.WrityCluster, c client.Client) error {
	labels := map[string]string{
		"apps":       wc.Name,
		"controller": wc.Name,
	}

	port := defaultWrityPort
	if wc.Spec.WritySpec.Port != nil {
		port = *wc.Spec.WritySpec.Port
	}

	serviceName := fmt.Sprintf("%s-service", wc.Name)
	service := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: wc.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: labels,
			Ports: []v1.ServicePort{{
				Port:       port,
				TargetPort: intstr.FromInt32(port),
			}},
		},
	}

	logger.Info("create/update writy service", "port", port, "labels", labels)
	_, err := controllerutil.CreateOrPatch(ctx, c, &service, func() error { return nil })
	return err
}

func createOrPatchLoadbalancerService(ctx context.Context, logger logr.Logger, wc *apiv1.WrityCluster, c client.Client) error {
	balancerName := fmt.Sprintf("%sloadbalancer", wc.Name)
	labels := map[string]string{
		"apps":       balancerName,
		"controller": wc.Name,
	}

	lbs := wc.Spec.LoadBalancerSpec
	if lbs == nil {
		lbs = &apiv1.LoadBalancerSpec{}
	}
	port := defaultLoadbalancerPort
	if lbs.Port != nil {
		port = *lbs.Port
	}

	balancerService := fmt.Sprintf("%s-service", balancerName)
	service := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      balancerService,
			Namespace: wc.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: labels,
			Ports: []v1.ServicePort{{
				Port:       port,
				TargetPort: intstr.FromInt32(port),
			}},
		},
	}

	logger.Info("create/update loadbalancer service", "port", port, "labels", labels)
	_, err := controllerutil.CreateOrPatch(ctx, c, &service, func() error { return nil })
	return err
}

func createOrPatchLoadbalancer(ctx context.Context, logger logr.Logger, wc *apiv1.WrityCluster, c client.Client) error {
	balancerName := fmt.Sprintf("%sloadbalancer", wc.Name)
	labels := map[string]string{
		"apps":       balancerName,
		"controller": wc.Name,
	}

	lbs := wc.Spec.LoadBalancerSpec
	if lbs == nil {
		lbs = &apiv1.LoadBalancerSpec{}
	}
	balancerPort := defaultLoadbalancerPort
	if lbs.Port != nil {
		balancerPort = *lbs.Port
	}
	if lbs.LogLevel == "" {
		lbs.LogLevel = defaultLogLevel
	}

	ws := wc.Spec.WritySpec
	if ws == nil {
		ws = &apiv1.WritySpec{}
	}
	if ws.Version == "" {
		ws.Version = defaultWrityImageVersion
	}

	writyPort := defaultWrityPort
	if wc.Spec.WritySpec.Port != nil {
		writyPort = *wc.Spec.WritySpec.Port
	}

	args := []string{"--addr=$(RUNNING_ADDR)", "--db=/data", "--leveler=$(LOG_LEVEL)", "--reflection", "--balancer"}
	dbServiceName := fmt.Sprintf("%s-service", wc.Name)

	for i := 0; i < int(*wc.Spec.Size); i++ {
		replica := fmt.Sprintf("--replica=%s-%d.%s:%d", wc.Name, i, dbServiceName, writyPort)
		args = append(args, replica)
	}

	balancerSpec := v1.PodSpec{
		Containers: []v1.Container{{
			Name:    balancerName,
			Ports:   []v1.ContainerPort{{ContainerPort: balancerPort}},
			Image:   fmt.Sprintf("%s:%s", writyImage, ws.Version),
			Command: []string{writyBinaryPath},
			Args:    args,
			Env: []v1.EnvVar{
				{
					Name:  "RUNNING_ADDR",
					Value: fmt.Sprintf(":%d", balancerPort),
				},
				{
					Name:  "LOG_LEVEL",
					Value: ws.LogLevel,
				},
			},
		}},
	}

	owners, err := getOwnerReferences(wc, c)
	if err != nil {
		return err
	}

	depl := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:            balancerName,
			Namespace:       wc.Namespace,
			OwnerReferences: owners,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      balancerName,
					Namespace: wc.Namespace,
					Labels:    labels,
				},
				Spec: balancerSpec,
			},
		},
	}

	logger.Info("create/patch deployment", "deployment", depl)
	_, err = controllerutil.CreateOrPatch(ctx, c, depl, func() error { return nil })
	return err
}
