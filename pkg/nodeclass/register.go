package nodeclass

import (
	"fmt"
	"reflect"
	"time"

	"github.com/kube-node/nodeset/pkg/nodeset/v1alpha1"

	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
)

func EnsureCustomResourceDefinitions(clientset apiextensionsclient.Interface) error {
	type resource struct {
		plural string
		kind   string
	}

	resourceNames := []resource{
		{
			plural: v1alpha1.NodeSetResourcePlural,
			kind:   reflect.TypeOf(v1alpha1.NodeSet{}).Name(),
		},
		{
			plural: v1alpha1.NodeClassResourcePlural,
			kind:   reflect.TypeOf(v1alpha1.NodeClass{}).Name(),
		},
	}

	for _, res := range resourceNames {
		if err := createCustomResourceDefinition(res.plural, res.kind, clientset); err != nil {
			return err
		}
	}

	return nil
}

func createCustomResourceDefinition(plural, kind string, clientset apiextensionsclient.Interface) error {
	name := plural + "." + v1alpha1.GroupName
	crd := &apiextensionsv1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: apiextensionsv1beta1.CustomResourceDefinitionSpec{
			Group:   v1alpha1.GroupName,
			Version: v1alpha1.SchemeGroupVersion.Version,
			Scope:   apiextensionsv1beta1.ClusterScoped,
			Names: apiextensionsv1beta1.CustomResourceDefinitionNames{
				Plural: plural,
				Kind:   kind,
			},
		},
	}
	_, err := clientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err != nil {
		if kerrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}

	// wait for CRD being established
	err = wait.Poll(500*time.Millisecond, 60*time.Second, func() (bool, error) {
		crd, err = clientset.ApiextensionsV1beta1().CustomResourceDefinitions().Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, cond := range crd.Status.Conditions {
			switch cond.Type {
			case apiextensionsv1beta1.Established:
				if cond.Status == apiextensionsv1beta1.ConditionTrue {
					return true, err
				}
			case apiextensionsv1beta1.NamesAccepted:
				if cond.Status == apiextensionsv1beta1.ConditionFalse {
					fmt.Printf("Name conflict: %v\n", cond.Reason)
				}
			}
		}
		return false, err
	})
	if err != nil {
		deleteErr := clientset.ApiextensionsV1beta1().CustomResourceDefinitions().Delete(name, nil)
		if deleteErr != nil {
			return errors.NewAggregate([]error{err, deleteErr})
		}
		return err
	}
	return nil
}
