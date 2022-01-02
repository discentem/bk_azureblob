module github.com/discentem/bk_azureblob

go 1.17

require (
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v0.12.0
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v0.2.0
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v0.20.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v0.8.3 // indirect
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8 // indirect
	golang.org/x/crypto v0.0.0-20211215153901-e495a2d5b3d3 // indirect
	golang.org/x/net v0.0.0-20211216030914-fe4d6282115f // indirect
	golang.org/x/sys v0.0.0-20210616045830-e2b7044e8c71 // indirect
	golang.org/x/text v0.3.7 // indirect
)

// https://medium.com/swlh/semantic-version-tags-in-go-mod-file-f6ad903a972d
replace github.com/Azure/azure-sdk-for-go/sdk/storage/azblob => github.com/discentem/azure-sdk-for-go/sdk/storage/azblob v0.0.0-20220101061852-f4515d6fffcc
