package yandex

import (
	"context"
	"log"

	"github.com/hashicorp/packer/helper/useragent"
	"github.com/hashicorp/packer/packer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/yandex-cloud/go-genproto/yandex/cloud/compute/v1"
	"github.com/yandex-cloud/go-genproto/yandex/cloud/endpoint"
	ycsdk "github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/iamkey"
	"github.com/yandex-cloud/go-sdk/pkg/requestid"
)

type Driver interface {
	DeleteImage(id string) error
	SDK() *ycsdk.SDK
	GetImage(imageID string) (*Image, error)
	GetImageFromFolder(ctx context.Context, folderID string, family string) (*Image, error)
}

type driverYC struct {
	sdk *ycsdk.SDK
	ui  packer.Ui
}

func (d *driverYC) GetImage(imageID string) (*Image, error) {
	image, err := d.sdk.Compute().Image().Get(context.Background(), &compute.GetImageRequest{
		ImageId: imageID,
	})
	if err != nil {
		return nil, err
	}

	return &Image{
		ID:            image.Id,
		Labels:        image.Labels,
		Licenses:      image.ProductIds,
		Name:          image.Name,
		FolderID:      image.FolderId,
		MinDiskSizeGb: toGigabytes(image.MinDiskSize),
		SizeGb:        toGigabytes(image.StorageSize),
	}, nil
}

func (d *driverYC) GetImageFromFolder(ctx context.Context, folderID string, family string) (*Image, error) {
	image, err := d.sdk.Compute().Image().GetLatestByFamily(ctx, &compute.GetImageLatestByFamilyRequest{
		FolderId: folderID,
		Family:   family,
	})
	if err != nil {
		return nil, err
	}

	return &Image{
		ID:            image.Id,
		Labels:        image.Labels,
		Licenses:      image.ProductIds,
		Name:          image.Name,
		FolderID:      image.FolderId,
		MinDiskSizeGb: toGigabytes(image.MinDiskSize),
		SizeGb:        toGigabytes(image.StorageSize),
	}, nil
}

func NewDriverYandexCloud(ui packer.Ui, config *Config) (Driver, error) {
	log.Printf("[INFO] Initialize Yandex Cloud client...")

	sdkConfig := ycsdk.Config{}

	if config.Endpoint != "" {
		sdkConfig.Endpoint = config.Endpoint
	}

	switch {
	case config.Token != "":
		sdkConfig.Credentials = ycsdk.OAuthToken(config.Token)

	case config.ServiceAccountKeyFile != "":
		key, err := iamkey.ReadFromJSONFile(config.ServiceAccountKeyFile)
		if err != nil {
			return nil, err
		}

		credentials, err := ycsdk.ServiceAccountKey(key)
		if err != nil {
			return nil, err
		}

		sdkConfig.Credentials = credentials
	}

	userAgentMD := metadata.Pairs("user-agent", useragent.String())

	sdk, err := ycsdk.Build(context.Background(), sdkConfig,
		grpc.WithDefaultCallOptions(grpc.Header(&userAgentMD)),
		grpc.WithUnaryInterceptor(requestid.Interceptor()))

	if err != nil {
		return nil, err
	}

	if _, err = sdk.ApiEndpoint().ApiEndpoint().List(context.Background(), &endpoint.ListApiEndpointsRequest{}); err != nil {
		return nil, err
	}

	return &driverYC{
		sdk: sdk,
		ui:  ui,
	}, nil

}

func (d *driverYC) DeleteImage(ID string) error {
	return nil
}

func (d *driverYC) SDK() *ycsdk.SDK {
	return d.sdk
}
