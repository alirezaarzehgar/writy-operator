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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	defaultStorageClaimName       = "data"
	defaultStorageClassName       = "standard"
	writyImage                    = "alirezaarzehgar/writy"
	writyBinaryPath               = "/bin/writy"
	defaultWrityImageVersion      = "v1.0.0"
	defaultWrityPort         uint = 8000
	defaultWrityLogLevel          = "warn"
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
	if err := createOrPatchStatefulSet(ctx, logger, writyCluster, c); err != nil {
		return err
	}

	if err := createOrPatchLoadbalancer(ctx, logger, writyCluster, c); err != nil {
		return err
	}

	return nil
}

func createOrPatchStatefulSet(ctx context.Context, logger logr.Logger, wc *apiv1.WrityCluster, c client.Client) error {
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
	if ws.Port == 0 {
		ws.Port = defaultWrityPort
	}
	if ws.LogLevel == "" {
		ws.LogLevel = defaultWrityLogLevel
	}

	ss := wc.Spec.StorageSpec
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
					Value: fmt.Sprintf(":%d", ws.Port),
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

	stfs := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      wc.Name,
			Namespace: wc.Namespace,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: wc.Spec.Size,
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
	controllerutil.CreateOrPatch(ctx, c, stfs, func() error {
		return nil
	})

	return nil
}

func createOrPatchLoadbalancer(ctx context.Context, logger logr.Logger, wc *apiv1.WrityCluster, c client.Client) error {

	return nil
}
