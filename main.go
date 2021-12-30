package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

var (
	deviceCode *bool
)

func init() {
	deviceCode = flag.Bool("device_code", false, "Determines if credential for Azure Blob operations should be device_code")
	flag.Parse()
}

func main() {
	var credential azcore.TokenCredential
	// https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/device_code_credential.go
	var err error
	if *deviceCode {
		credential, err = azidentity.NewDeviceCodeCredential(&azidentity.DeviceCodeCredentialOptions{
			TenantID: tenantID,
			ClientID: clientID,
		})
	} else {
		credential, err = azidentity.NewInteractiveBrowserCredential(&azidentity.InteractiveBrowserCredentialOptions{
			TenantID:    tenantID,
			ClientID:    clientID,
			RedirectURL: "http://localhost:9090",
		})
	}
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	container, err := azblob.NewContainerClient(
		fmt.Sprintf("https://%s.blob.core.windows.net/%s", storageAccount, containerName),
		credential,
		&azblob.ClientOptions{},
	)
	if err != nil {
		log.Fatal(err)
	}
	blob := container.NewBlobClient("azureblobtest")
	blobProps, err := blob.GetProperties(ctx, &azblob.GetBlobPropertiesOptions{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("got this far")
	fmt.Println(*blobProps.ContentType)
	// resp, err := blob.Download(ctx, &azblob.DownloadBlobOptions{})
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// b, _ := ioutil.ReadAll(resp.Body(azblob.RetryReaderOptions{}))
	// fmt.Println(string(b))
}
