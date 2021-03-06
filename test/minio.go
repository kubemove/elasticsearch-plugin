package test

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/appscode/go/types"
	shell "github.com/codeskyblue/go-sh"
	"github.com/kubemove/elasticsearch-plugin/pkg/util"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	DefaultStorageClass = "standard"
	KeyMinioAccessKey   = "MINIO_ACCESS_KEY"
	KeyMinioSecretKey   = "MINIO_SECRET_KEY"
	KeyS3AccessKey      = "s3.client.default.access_key"
	KeyS3SecretKey      = "s3.client.default.secret_key"
	MinioAccessKey      = "not@accesskey"
	MinioSecretKey      = "not@secretkey"
	Minio               = "minio"
	LabelApp            = "app"
	MinioCredentialName = "minio-credentials"
	DefaultNamespace    = "default"
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
			Name:      MinioCredentialName,
			Namespace: DefaultNamespace,
		},
		StringData: map[string]string{
			KeyS3AccessKey: MinioAccessKey,
			KeyS3SecretKey: MinioSecretKey,
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
			Name:      Minio,
			Namespace: DefaultNamespace,
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
				LabelApp: Minio,
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
			Namespace: DefaultNamespace,
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
			StorageClassName: types.StringP(DefaultStorageClass),
		},
	}
	fmt.Println("Creating PVC for Minio server in the destination cluster...")
	return opt.DstKubeClient.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(&pvc)
}

func createMinioDeployment(opt *util.PluginOptions, secret *corev1.Secret, pvc *corev1.PersistentVolumeClaim) error {
	dpl := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Minio,
			Namespace: DefaultNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: types.Int32P(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					LabelApp: Minio,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelApp: Minio,
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
									Name: KeyMinioAccessKey,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secret.Name,
											},
											Key: KeyS3AccessKey,
										},
									},
								},
								{
									Name: KeyMinioSecretKey,
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secret.Name,
											},
											Key: KeyS3SecretKey,
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
	if err != nil {
		return err
	}
	return err
}

func removeMinioServer(opt *util.PluginOptions) error {
	// Delete Minio Deployment
	err := opt.DstKubeClient.AppsV1().Deployments(DefaultNamespace).Delete(Minio, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	// Delete Minio PVC
	err = opt.DstKubeClient.CoreV1().PersistentVolumeClaims(DefaultNamespace).Delete("minio-pvc", &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	// Delete Minio Service
	err = opt.DstKubeClient.CoreV1().Services(DefaultNamespace).Delete(Minio, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	// Delete Minio Secret from the source cluster
	err = opt.SrcKubeClient.CoreV1().Secrets(DefaultNamespace).Delete(MinioCredentialName, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	// Delete Minio Secret from the destination cluster
	err = opt.DstKubeClient.CoreV1().Secrets(DefaultNamespace).Delete(MinioCredentialName, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func (i *Invocation) EventuallyCreateMinioBucket() GomegaAsyncAssertion {
	return Eventually(func() bool {
		err := i.addMinioConfig()
		if err != nil {
			return false
		}
		err = shell.Command("mc", "mb", fmt.Sprintf("es-repo/%s", i.testID)).Run()
		return err == nil
	},
		DefaultTimeout,
		DefaultRetryInterval,
	)
}

func (i *Invocation) addMinioConfig() error {
	minioURL, err := i.getMinioServerAddress()
	if err != nil {
		return err
	}

	sh := shell.NewSession()
	args := []interface{}{"config", "host", "add", "es-repo", fmt.Sprintf("http://%s", minioURL), MinioAccessKey, MinioSecretKey}

	return sh.Command("mc", args...).Run()
}

func (i *Invocation) getMinioServerAddress() (string, error) {
	// Get Minio Service
	svc, err := i.DstKubeClient.CoreV1().Services(DefaultNamespace).Get(Minio, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%d", i.DstClusterIp, svc.Spec.Ports[0].NodePort), nil
}
