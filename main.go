package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	progressbar "github.com/schollz/progressbar/v3"

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

func bytesTransferredFn(isDownload bool, size int64, progbar *progressbar.ProgressBar) func(bytesTransferred int64) {
	return func(bytesTransferred int64) {
		progbar.Set64(bytesTransferred)
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
	// https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/storage/azblob/highlevel.go
	desc := fmt.Sprintf("Downloading %s", asset)
	progbar := progressbar.DefaultBytes(*size, desc)
	err = blob.DownloadBlobToFile(ctx, 0, 0, f, azblob.HighLevelDownloadFromBlobOptions{
		// DownloadBlob*() Progress is currently broken
		// https://github.com/Azure/azure-sdk-for-go/issues/16726
		Progress: bytesTransferredFn(true, *size, progbar),
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
	desc := fmt.Sprintf("Uploading to %s", blobPath)
	progbar := progressbar.DefaultBytes(size, desc)
	_, err = newBlob.UploadFileToBlockBlob(ctx, file, azblob.HighLevelUploadToBlockBlobOption{
		Progress: bytesTransferredFn(false, size, progbar),
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
	uploadFile := "azureblobtest"
	f, err := os.Create(uploadFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(40 * 1024 * 1024); err != nil {
		log.Fatal(err)
	}

	if err := az.Upload(ctx, f, uploadFile); err != nil {
		log.Fatal(err)
	}
	f.Close()

	testFileName := "azureblobtest.txt"

	if err := az.Download(ctx, testFileName, testFileName); err != nil {
		log.Fatal(err)
	}

}
