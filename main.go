package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// AzureBlobClient is an abstraction of the various clients needed for Blob downloads
type AzureBlobClient struct {
	ClientID        string
	ClientSecret    string
	TenantID        string
	StorageAccount  string
	ContainerName   string
	containerClient *azblob.ContainerClient
}

// InitCredAndContainerClient returns an authenticated Azure Blob Container Client.
// For now, there is no choice of credential for caller, *azidentity.DeviceCodeCredential is always used.
// https://github.com/Azure/azure-sdk-for-go/issues/16723
func (c *AzureBlobClient) InitCredAndContainerClient() (*azblob.ContainerClient, error) {
	// https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/device_code_credential.go
	credential, err := azidentity.NewDeviceCodeCredential(&azidentity.DeviceCodeCredentialOptions{
		TenantID: c.TenantID,
		ClientID: c.ClientID,
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
		fmt.Sprintf("https://%s.blob.core.windows.net/%s", c.StorageAccount, c.ContainerName),
		credential,
		&azblob.ClientOptions{},
	)
	if err != nil {
		return nil, err
	}
	return &container, nil
}

// init sets the container client and creates a context if these aren't already initialized
func (c *AzureBlobClient) init() error {
	if c.containerClient == nil {
		client, err := c.InitCredAndContainerClient()
		if err != nil {
			return err
		}
		// safe client in c for reuse
		c.containerClient = client
	}
	return nil
}

func bytesTransferredFn(isDownload bool, size int64) func(bytesTransferred int64) {
	return func(bytesTransferred int64) {
		percent := float64(bytesTransferred) / float64(size) * 100
		var msg string
		if isDownload {
			msg = fmt.Sprintf("Download: %.2f...", percent)
		} else {
			msg = fmt.Sprintf("Upload: %.2f...", percent)
		}

		fmt.Println(msg)
	}
}

// Download downloads a blob to a local file. If AzureBlobDownloader is not yet authenicated, Download will execute authentication flow.
func (c *AzureBlobClient) Download(ctx context.Context, asset, destination string) error {
	if err := c.init(); err != nil {
		return err
	}
	blob := c.containerClient.NewBlobClient(asset)
	f, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer f.Close()
	blobProps, err := blob.GetProperties(ctx, &azblob.GetBlobPropertiesOptions{})
	size := blobProps.ContentLength
	if err != nil {
		return err
	}
	if err := f.Truncate(*size); err != nil {
		return err
	}
	fmt.Println(*size * 1024)
	// https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/storage/azblob/highlevel.go
	err = blob.DownloadBlobToFile(ctx, 0, 0, f, azblob.HighLevelDownloadFromBlobOptions{
		// DownloadBlob*() Progress is currently broken
		// https://github.com/Azure/azure-sdk-for-go/issues/16726
		Progress: bytesTransferredFn(true, *size),
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *AzureBlobClient) Upload(ctx context.Context, file *os.File, blobPath string) error {
	if err := c.init(); err != nil {
		return err
	}
	newBlob := c.containerClient.NewBlockBlobClient(blobPath)
	if file == nil {
		return errors.New("file cannot be nil")
	}
	fileStats, err := file.Stat()
	if err != nil {
		return err
	}
	size := fileStats.Size()

	_, err = newBlob.UploadFileToBlockBlob(ctx, file, azblob.HighLevelUploadToBlockBlobOption{
		Progress: bytesTransferredFn(false, size),
	})
	if err != nil {
		return err
	}
	return nil
}

func main() {
	az := AzureBlobClient{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		TenantID:       tenantID,
		ContainerName:  containerName,
		StorageAccount: storageAccount,
	}

	ctx := context.Background()

	testFileName := "azureblobtest.txt"

	if err := az.Download(ctx, testFileName, testFileName); err != nil {
		log.Fatal(err)
	}

}
