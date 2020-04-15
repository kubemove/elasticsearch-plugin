package test

import (
	"k8s.io/client-go/dynamic"

	"github.com/kubemove/kubemove/pkg/apis/kubemove/v1alpha1"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var dataSyncGVR = schema.GroupVersionResource{
	Group:    "kubemove.io",
	Version:  "v1alpha1",
	Resource: "datasyncs",
}

func (i *Invocation) EventuallySourceDataSyncCreated(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(func() bool {
		dataSyncs, err := listDataSyncs(i.SrcDmClient, meta)
		if err != nil {
			return false
		}
		for i := range dataSyncs {
			if dataSyncs[i].Spec.MoveEngine == meta.Name {
				return true
			}
		}
		return false
	},
		DefaultTimeout,
		DefaultRetryInterval,
	)
}

func (i *Invocation) EventuallyDestinationDataSyncCreated(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(func() bool {
		dataSyncs, err := listDataSyncs(i.DstDmClient, meta)
		if err != nil {
			return false
		}
		for i := range dataSyncs {
			if dataSyncs[i].Spec.MoveEngine == meta.Name {
				return true
			}
		}
		return false
	},
		DefaultTimeout,
		DefaultRetryInterval,
	)
}
func listDataSyncs(dmClient dynamic.Interface, meta metav1.ObjectMeta) ([]v1alpha1.DataSync, error) {
	dataSyncs := v1alpha1.DataSyncList{}
	obj, err := dmClient.Resource(dataSyncGVR).Namespace(meta.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &dataSyncs)
	if err != nil {
		return nil, err
	}
	return dataSyncs.Items, nil
}

func (i *Invocation) deleteDataSync() error {
	// Delete DataSyncs from source cluster
	dataSyncs, err := listDataSyncs(i.SrcDmClient, metav1.ObjectMeta{Namespace: KubemoveNamespace})
	if err != nil {
		return err
	}
	for idx := range dataSyncs {
		if dataSyncs[idx].Spec.MoveEngine == i.testID {
			err := i.SrcDmClient.Resource(dataSyncGVR).Namespace(KubemoveNamespace).Delete(dataSyncs[idx].Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}
	// Delete DataSync from destination cluster
	dataSyncs, err = listDataSyncs(i.DstDmClient, metav1.ObjectMeta{Namespace: KubemoveNamespace})
	if err != nil {
		return err
	}
	for idx := range dataSyncs {
		if dataSyncs[idx].Spec.MoveEngine == i.testID {
			err := i.DstDmClient.Resource(dataSyncGVR).Namespace(KubemoveNamespace).Delete(dataSyncs[idx].Name, &metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}
	return nil
}
