package test

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/appscode/go/types"
	"github.com/kubemove/elasticsearch-plugin/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func deployMinioServer(opt *util.PluginOptions) error {
	// Create Minio Secret in both source and destination cluster
	secret, err := createMinioSecret(opt)
	if err != nil {
		return err
	}

	// Create Minio Service in the destination cluster
	_, err = createMinioService(opt)
	if err != nil {
		return err
	}
	// Create PVC for Minio server
	pvc, err := createMinioPVC(opt)
	if err != nil {
		return err
	}
	// Create Minio Deployment in the destination cluster
	return createMinioDeployment(opt, secret, pvc)
}

func createMinioSecret(opt *util.PluginOptions) (*corev1.Secret, error) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "minio-credentials",
			Namespace: "default",
		},
		StringData: map[string]string{
			"s3.client.default.access_key": "not@accesskey",
			"s3.client.default.secret_key": "not@secretkey",
		},
		Type: corev1.SecretTypeOpaque,
	}
	fmt.Println("Creating Minio Secret in the source cluster...")
	_, err := opt.SrcKubeClient.CoreV1().Secrets(secret.Namespace).Create(&secret)
	if err != nil {
		return nil, err
	}
	fmt.Println("Creating Minio Secret in the destination cluster...")
	return opt.DstKubeClient.CoreV1().Secrets(secret.Namespace).Create(&secret)
}

func createMinioService(opt *util.PluginOptions) (*corev1.Service, error) {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "minio",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Protocol:   "TCP",
					Port:       9000,
					TargetPort: intstr.IntOrString{IntVal: 9000},
				},
			},
			Selector: map[string]string{
				"app": "minio",
			},
		},
	}
	fmt.Println("Creating Minio service in the destination cluster...")
	return opt.DstKubeClient.CoreV1().Services(service.Namespace).Create(&service)
}

func createMinioPVC(opt *util.PluginOptions) (*corev1.PersistentVolumeClaim, error) {
	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "minio-pvc",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("2Gi"),
				},
			},
			StorageClassName: types.StringP(StandardStorageClass),
		},
	}
	fmt.Println("Creating PVC for Minio server in the destination cluster...")
	return opt.DstKubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(&pvc)
}

func createMinioDeployment(opt *util.PluginOptions, secret *corev1.Secret, pvc *corev1.PersistentVolumeClaim) error {
	dpl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "minio",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: types.Int32P(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "minio",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "minio",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "minio-server",
							Image: "minio/minio",
							Args:  []string{"server", "/storage"},
							Env: []corev1.EnvVar{
								{
									Name: "MINIO_ACCESS_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secret.Name,
											},
											Key: "s3.client.default.access_key",
										},
									},
								},
								{
									Name: "MINIO_SECRET_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secret.Name,
											},
											Key: "s3.client.default.secret_key",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      pvc.Name,
									MountPath: "/storage",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: pvc.Name,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvc.Name,
								},
							},
						},
					},
				},
			},
		},
	}
	fmt.Println("Creating Minio Deployment in the destination cluster...")
	_, err := opt.DstKubeClient.AppsV1().Deployments(dpl.Namespace).Create(&dpl)
	return err
}

func removeMinioServer(opt *util.PluginOptions) error {
	// Delete Minio Deployment
	err := opt.DstKubeClient.AppsV1().Deployments("default").Delete("minio", &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	// Delete Minio PVC
	err = opt.DstKubeClient.CoreV1().PersistentVolumeClaims("default").Delete("minio-pvc", &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	// Delete Minio Service
	err = opt.DstKubeClient.CoreV1().Services("default").Delete("minio", &metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	// Delete Minio Secret from the source cluster
	err = opt.SrcKubeClient.CoreV1().Secrets("default").Delete("minio-credentials", &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	// Delete Minio Secret from the destination cluster
	return opt.SrcKubeClient.CoreV1().Secrets("default").Delete("minio-credentials", &metav1.DeleteOptions{})
}

func (i *Invocation) createMinioBucket() error {
	// TODO:
	return nil
}
