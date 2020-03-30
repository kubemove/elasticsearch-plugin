package test

import (
	"k8s.io/client-go/dynamic"

	"github.com/kubemove/kubemove/pkg/apis/kubemove/v1alpha1"
	. "github.com/onsi/gomega"
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
		for i := range dataSyncs.Items {
			if dataSyncs.Items[i].Spec.MoveEngine == meta.Name {
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
		for i := range dataSyncs.Items {
			if dataSyncs.Items[i].Spec.MoveEngine == meta.Name {
				return true
			}
		}
		return false
	},
		DefaultTimeout,
		DefaultRetryInterval,
	)
}
func listDataSyncs(dmClient dynamic.Interface, meta metav1.ObjectMeta) (*v1alpha1.DataSyncList, error) {
	dataSyncs := &v1alpha1.DataSyncList{}
	obj, err := dmClient.Resource(dataSyncGVR).Namespace(meta.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, dataSyncs)
	if err != nil {
		return nil, err
	}
	return dataSyncs, nil
}
