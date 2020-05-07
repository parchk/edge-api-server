package catalogapi

import (
	"github.com/cnrancher/edge-api-server/pkg/apis/edgeapi.cattle.io/v1alpha1"
	catalogcontroller "github.com/cnrancher/edge-api-server/pkg/generated/controllers/edgeapi.cattle.io/v1alpha1"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/schemaserver/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Store struct {
	types.Store
	asl        accesscontrol.AccessSetLookup
	controller catalogcontroller.CatalogController
}

func (s *Store) Create(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject) (types.APIObject, error) {
	var catalogs []v1alpha1.Catalog
	namespace := apiOp.Namespace
	object := types.APIObject{}
	if namespace != "" {
		catalog, err := s.controller.Get(namespace, apiOp.Name, metav1.GetOptions{})
		if err != nil {
			return object, err
		}
		catalogs = append(catalogs, *catalog)
	} else {
		catalogList, err := s.controller.List(namespace, metav1.ListOptions{})
		if err != nil {
			return object, err
		}
		for _, catalog := range catalogList.Items {
			catalogs = append(catalogs, catalog)
		}
	}
	for _, catalog := range catalogs {
		if err := s.refreshCatalog(&catalog); err != nil {
			return object, err
		}
	}
	return data, nil
}

func (s *Store) refreshCatalog(catalog *v1alpha1.Catalog) (err error) {
	catalog, err = s.controller.Get(catalog.Namespace, catalog.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	v1alpha1.CatalogConditionRefreshed.Unknown(catalog)
	_, err = s.controller.Update(catalog)
	return err
}
