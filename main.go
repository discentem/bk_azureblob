package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	progressbar "github.com/schollz/progressbar/v3"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

type AzureBlobCredentialOptions struct {
	InteractiveCredential bool
}

// AzureBlobClient is an abstraction of the various clients needed for Blob downloads
type AzureBlobClient struct {
	ClientID          string
	TenantID          string
	StorageAccount    string
	ContainerName     string
	containerClient   *azblob.ContainerClient
	CredentialOptions *AzureBlobCredentialOptions
}

// InitCredential returns either an interactive credential or device code credential
// Interative is attempted first. If it fails, device Code is then attempted.
func (c *AzureBlobClient) InitCredential(credOpts *AzureBlobCredentialOptions) (*azcore.TokenCredential, error) {
	credList := []azcore.TokenCredential{}
	if credOpts.InteractiveCredential {
		interactive, err := azidentity.NewInteractiveBrowserCredential(&azidentity.InteractiveBrowserCredentialOptions{
			TenantID:    c.TenantID,
			ClientID:    c.ClientID,
			RedirectURL: "http://localhost:9090",
		})
		if err != nil {
			return nil, err
		}
		credList = append(credList, interactive)
	}
	// https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/device_code_credential.go
	deviceCode, err := azidentity.NewDeviceCodeCredential(&azidentity.DeviceCodeCredentialOptions{
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
	credList = append(credList, deviceCode)
	chain, err := azidentity.NewChainedTokenCredential(
		credList,
		&azidentity.ChainedTokenCredentialOptions{},
	)
	if err != nil {
		return nil, err
	}
	tokenCred := azcore.TokenCredential(chain)
	return &tokenCred, nil
}

func (c *AzureBlobClient) InitContainerClient(tokenCred *azcore.TokenCredential) (*azblob.ContainerClient, error) {
	container, err := azblob.NewContainerClient(
		// Construct container url
		fmt.Sprintf("https://%s.blob.core.windows.net/%s", c.StorageAccount, c.ContainerName),
		*tokenCred,
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
		credential, err := c.InitCredential(c.CredentialOptions)
		if err != nil {
			return err
		}
		client, err := c.InitContainerClient(credential)
		if err != nil {
			return err
		}
		// save client in c for reuse
		c.containerClient = client
	}
	return nil
}

func bytesTransferredFn(isDownload bool, size int64, progbar *progressbar.ProgressBar) func(bytesTransferred int64) {
	return func(bytesTransferred int64) {
		progbar.Set64(bytesTransferred)
		f := bufio.NewWriter(os.Stdout)
		defer f.Flush()
		f.Write([]byte(progbar.String()))
	}
}

// Download downloads a blob to a local file. If AzureBlobDownloader is not yet authenticated, Download will execute authentication flow.
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
	progbar := progressbar.DefaultBytesSilent(*size, desc)
	err = blob.DownloadBlobToFile(ctx, 0, 0, f, azblob.HighLevelDownloadFromBlobOptions{
		// DownloadBlob*() Progress is currently broken
		// https://github.com/Azure/azure-sdk-for-go/issues/16726
		Progress: bytesTransferredFn(true, *size, progbar),
	})
	if err != nil {
		return err
	}
	fmt.Println(progbar.String())
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
	progbar := progressbar.DefaultBytesSilent(size, desc)
	_, err = newBlob.UploadFileToBlockBlob(ctx, file, azblob.HighLevelUploadToBlockBlobOption{
		Progress: bytesTransferredFn(false, size, progbar),
	})
	if err != nil {
		return err
	}
	fmt.Println(progbar.String())
	return nil
}

func NewAzureBlobClientDefault(clientID, tenantID, containerName, storageAccount string) *AzureBlobClient {
	return &AzureBlobClient{
		ClientID:       clientID,
		TenantID:       tenantID,
		ContainerName:  containerName,
		StorageAccount: storageAccount,
		CredentialOptions: &AzureBlobCredentialOptions{
			InteractiveCredential: false,
		},
	}
}

func NewAzureBlobClientInteractive(clientID, tenantID, containerName, storageAccount string) *AzureBlobClient {
	client := NewAzureBlobClientDefault(clientID, tenantID, containerName, storageAccount)
	client.CredentialOptions.InteractiveCredential = true
	return client
}

func main() {
	az := NewAzureBlobClientDefault(
		clientID,
		tenantID,
		containerName,
		storageAccount,
	)

	ctx := context.Background()
	testFileName := "azureblobtest.txt"

	if err := az.Download(ctx, testFileName, testFileName); err != nil {
		log.Fatal(err)
	}

}
