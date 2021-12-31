package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// AzureBlobDownloader is a logical grouping of the various clients needed for Blob downloads
type AzureBlobDownloader struct {
	ClientID        string
	TenantID        string
	ContainerName   string
	containerClient *azblob.ContainerClient
	ctx             context.Context
}

// InitCredAndContainerClient returns an authenticated Azure Blob Container Client.
// For now, there is no choice of credential for caller, *azidentity.DeviceCodeCredential is always used.
// https://github.com/Azure/azure-sdk-for-go/issues/16723
func (c *AzureBlobDownloader) InitCredAndContainerClient() (*azblob.ContainerClient, error) {
	// https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/device_code_credential.go
	credential, err := azidentity.NewDeviceCodeCredential(&azidentity.DeviceCodeCredentialOptions{
		TenantID: tenantID,
		ClientID: clientID,
		// Customizes the UserPrompt. Replaces VerificationURL with shortlink.
		// Providing a custom UserPrompt can also allow the URL to be rewritten anywhere, instead of just stdout
		UserPrompt: func(ctx context.Context, deviceCodeMessage azidentity.DeviceCodeMessage) error {
			msg := strings.Replace(deviceCodeMessage.Message, "https://microsoft.com/devicelogin", "https://aka.ms/devicelogin", 1)
			fmt.Println(msg)
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	container, err := azblob.NewContainerClient(
		// Construct container url
		fmt.Sprintf("https://%s.blob.core.windows.net/%s", storageAccount, containerName),
		credential,
		&azblob.ClientOptions{},
	)
	if err != nil {
		return nil, err
	}
	return &container, nil
}

// init sets the container client and creates a context if these aren't already initialized
func (c *AzureBlobDownloader) init() error {
	if c.containerClient == nil {
		client, err := c.InitCredAndContainerClient()
		if err != nil {
			return err
		}
		// safe client in c for reuse
		c.containerClient = client
	}
	if c.ctx == nil {
		c.ctx = context.Background()
	}
	return nil
}

// WriteToFile is use to write the output of azblob.BlobClient.Download() to a file.
func WriteToFile(content io.ReadCloser, destination string) error {
	out, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, content)
	if err != nil {
		return err
	}
	return nil
}

// Download downloads a blob to a local file. If AzureBlobDownloader is not yet authenicated, Download will execute authentication flow.
func (c *AzureBlobDownloader) Download(asset, destination string) error {
	if err := c.init(); err != nil {
		return err
	}
	blob := c.containerClient.NewBlobClient(asset)
	resp, err := blob.Download(c.ctx, &azblob.DownloadBlobOptions{})
	if err != nil {
		return err
	}
	body := resp.Body(azblob.RetryReaderOptions{})
	return WriteToFile(body, destination)
}

func main() {
	az := AzureBlobDownloader{
		ClientID:      clientID,
		TenantID:      tenantID,
		ContainerName: containerName,
	}
	if err := az.Download("azureblobtest", "azureblobtest.txt"); err != nil {
		log.Fatal(err)
	}
	b, err := ioutil.ReadFile("azureblobtest.txt")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))

}
