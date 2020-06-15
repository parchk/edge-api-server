package devicetemplateapi

import (
	"context"
	"errors"

	"github.com/cnrancher/edge-api-server/pkg/apis/edgeapi.cattle.io/v1alpha1"
	"github.com/cnrancher/edge-api-server/pkg/auth"
	"github.com/rancher/steve/pkg/accesscontrol"
	"github.com/rancher/steve/pkg/schemaserver/types"
	"github.com/rancher/wrangler/pkg/data/convert"
	"github.com/sirupsen/logrus"
)

type Store struct {
	types.Store
	asl  accesscontrol.AccessSetLookup
	ctx  context.Context
	auth auth.Authenticator
}

const (
	templateDeviceTypeName    = "edgeapi.cattle.io/template-device-type"
	templateDeviceVersionName = "edgeapi.cattle.io/template-device-version"
	templateOwnerName         = "edgeapi.cattle.io/template-owner"
)

func (s *Store) Create(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject) (types.APIObject, error) {
	var deviceTemplate v1alpha1.DeviceTemplate
	err := convert.ToObj(data.Data(), &deviceTemplate)
	if err != nil {
		logrus.Errorf("failed to convert device template data, error: %s", err.Error())
		return data, err
	}

	if err := ValidateTemplateRequest(deviceTemplate.Spec); err != nil {
		logrus.Errorf("invalid device template request, error: %s", err.Error())
		return data, err
	}

	authed, user, err := s.auth.Authenticate(apiOp.Request)
	if !authed || err != nil {
		logrus.Error("Invalid user error:", err.Error())
		return data, err
	}

	deviceTemplate.Labels = map[string]string{
		templateDeviceTypeName:    deviceTemplate.Spec.DeviceKind,
		templateDeviceVersionName: deviceTemplate.Spec.DeviceVersion,
		templateOwnerName:         user,
	}

	err = convert.ToObj(deviceTemplate, &data.Object)
	if err != nil {
		logrus.Errorf("failed to convert device template data, error: %s", err.Error())
		return data, err
	}
	return s.Store.Create(apiOp, schema, data)
}

func (s *Store) Update(apiOp *types.APIRequest, schema *types.APISchema, data types.APIObject, id string) (types.APIObject, error) {
	var deviceTemplate v1alpha1.DeviceTemplate
	err := convert.ToObj(data.Data(), &deviceTemplate)
	if err != nil {
		logrus.Errorf("failed to parse device template data, error: %s", err.Error())
		return data, err
	}

	if err := ValidateTemplateRequest(deviceTemplate.Spec); err != nil {
		logrus.Errorf("invalid device template request, error: %s", err.Error())
		return data, err
	}

	return s.Store.Update(apiOp, schema, data, id)
}

func ValidateTemplateRequest(spec v1alpha1.DeviceTemplateSpec) error {
	if spec.DeviceKind == "" {
		return errors.New("deviceKind is required of DeviceTemplate")
	}
	if spec.DeviceVersion == "" {
		return errors.New("deviceVersion is required of DeviceTemplate")
	}
	if spec.DeviceGroup == "" {
		return errors.New("deviceGroup is required of DeviceTemplate")
	}
	if spec.DeviceResource == "" {
		return errors.New("deviceResource is required of DeviceTemplate")
	}
	if spec.DisplayName == "" {
		return errors.New("displayName is required of DeviceTemplate")
	}
	return nil
}
